/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"strconv"
	"time"

	appsv1 "k8s.io/api/apps/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	networkingv1 "k8s.io/api/networking/v1"

	"github.com/golang-jwt/jwt/v5"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/utils/ptr"

	devboxv1alpha1 "github.com/labring/sealos/controllers/devbox/api/v1alpha1"
	"github.com/labring/sealos/controllers/devbox/internal/controller/helper"
	"github.com/labring/sealos/controllers/devbox/label"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// DevboxReconciler reconciles a Devbox object
type DevboxReconciler struct {
	CommitImageRegistry     string
	RequestCPURate          float64
	RequestMemoryRate       float64
	RequestEphemeralStorage string
	LimitEphemeralStorage   string
	WebSocketImage          string
	DebugMode               bool
	WebsocketProxyDomain    string
	IngressClass            string
	EnableAutoShutdown      bool
	ShutdownServerKey       string
	ShutdownServerAddr      string
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=devbox.sealos.io,resources=devboxes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=devbox.sealos.io,resources=devboxes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=devbox.sealos.io,resources=devboxes/finalizers,verbs=update
// +kubebuilder:rbac:groups=devbox.sealos.io,resources=runtimes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=devbox.sealos.io,resources=runtimeclasses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=pods,verbs=*
// +kubebuilder:rbac:groups="",resources=pods/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=services,verbs=*
// +kubebuilder:rbac:groups="",resources=secrets,verbs=*
// +kubebuilder:rbac:groups="",resources=events,verbs=*

func (r *DevboxReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	devbox := &devboxv1alpha1.Devbox{}
	if err := r.Get(ctx, req.NamespacedName, devbox); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	recLabels := label.RecommendedLabels(&label.Recommended{
		Name:      devbox.Name,
		ManagedBy: label.DefaultManagedBy,
		PartOf:    devboxv1alpha1.DevBoxPartOf,
	})

	if devbox.ObjectMeta.DeletionTimestamp.IsZero() {
		// retry add finalizer
		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			latestDevbox := &devboxv1alpha1.Devbox{}
			if err := r.Get(ctx, req.NamespacedName, latestDevbox); err != nil {
				return client.IgnoreNotFound(err)
			}
			if controllerutil.AddFinalizer(latestDevbox, devboxv1alpha1.FinalizerName) {
				return r.Update(ctx, latestDevbox)
			}
			return nil
		})
		if err != nil {
			return ctrl.Result{}, err
		}
	} else {
		logger.Info("devbox deleted, remove all resources")
		if err := r.removeAll(ctx, devbox, recLabels); err != nil {
			return ctrl.Result{}, err
		}

		logger.Info("devbox deleted, remove finalizer")
		if controllerutil.RemoveFinalizer(devbox, devboxv1alpha1.FinalizerName) {
			if err := r.Update(ctx, devbox); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	devbox.Status.Network.Type = devbox.Spec.NetworkSpec.Type
	_ = r.Status().Update(ctx, devbox)

	// create or update secret
	logger.Info("syncing secret")
	if err := r.syncSecret(ctx, devbox, recLabels); err != nil {
		logger.Error(err, "sync secret failed")
		r.Recorder.Eventf(devbox, corev1.EventTypeWarning, "Sync secret failed", "%v", err)
		return ctrl.Result{}, err
	}
	logger.Info("sync secret success")
	r.Recorder.Eventf(devbox, corev1.EventTypeNormal, "Sync secret success", "Sync secret success")

	logger.Info("syncing network")
	if err := r.syncNetwork(ctx, devbox, recLabels); err != nil {
		logger.Error(err, "sync network failed")
		r.Recorder.Eventf(devbox, corev1.EventTypeWarning, "Sync network failed", "%v", err)
		return ctrl.Result{}, err
	}
	logger.Info("sync network success")

	// create or update pod
	logger.Info("syncing pod")
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := r.Get(ctx, req.NamespacedName, devbox); err != nil {
			return err
		}
		return r.syncPod(ctx, devbox, recLabels)
	}); err != nil {
		logger.Error(err, "sync pod failed")
		r.Recorder.Eventf(devbox, corev1.EventTypeWarning, "Sync pod failed", "%v", err)
		return ctrl.Result{}, err
	}
	logger.Info("sync pod success")
	r.Recorder.Eventf(devbox, corev1.EventTypeNormal, "Sync pod success", "Sync pod success")

	logger.Info("devbox reconcile success")
	return ctrl.Result{}, nil
}

func (r *DevboxReconciler) syncSecret(ctx context.Context, devbox *devboxv1alpha1.Devbox, recLabels map[string]string) error {
	objectMeta := metav1.ObjectMeta{
		Name:      devbox.Name,
		Namespace: devbox.Namespace,
		Labels:    recLabels,
	}
	devboxSecret := &corev1.Secret{
		ObjectMeta: objectMeta,
	}

	err := r.Get(ctx, client.ObjectKey{Namespace: devbox.Namespace, Name: devbox.Name}, devboxSecret)
	if err == nil {
		// Secret already exists, no need to create

		// TODO: delete this code after we have a way to sync secret to devbox
		// check if SEALOS_DEVBOX_JWT_SECRET is exist, if not exist, create it
		if _, ok := devboxSecret.Data["SEALOS_DEVBOX_JWT_SECRET"]; !ok {
			devboxSecret.Data["SEALOS_DEVBOX_JWT_SECRET"] = []byte(rand.String(32))
			if err := r.Update(ctx, devboxSecret); err != nil {
				return fmt.Errorf("failed to update secret: %w", err)
			}
		}

		if _, ok := devboxSecret.Data["SEALOS_DEVBOX_AUTHORIZED_KEYS"]; !ok {
			devboxSecret.Data["SEALOS_DEVBOX_AUTHORIZED_KEYS"] = devboxSecret.Data["SEALOS_DEVBOX_PUBLIC_KEY"]
			if err := r.Update(ctx, devboxSecret); err != nil {
				return fmt.Errorf("failed to update secret: %w", err)
			}
		}

		return nil
	}
	if client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("failed to get secret: %w", err)
	}

	// Secret not found, create a new one
	publicKey, privateKey, err := helper.GenerateSSHKeyPair()
	if err != nil {
		return fmt.Errorf("failed to generate SSH key pair: %w", err)
	}

	secret := &corev1.Secret{
		ObjectMeta: objectMeta,
		Data: map[string][]byte{
			"SEALOS_DEVBOX_JWT_SECRET":      []byte(rand.String(32)),
			"SEALOS_DEVBOX_PUBLIC_KEY":      publicKey,
			"SEALOS_DEVBOX_PRIVATE_KEY":     privateKey,
			"SEALOS_DEVBOX_AUTHORIZED_KEYS": publicKey,
		},
	}

	if err := controllerutil.SetControllerReference(devbox, secret, r.Scheme); err != nil {
		return fmt.Errorf("failed to set controller reference: %w", err)
	}

	if err := r.Create(ctx, secret); err != nil {
		return fmt.Errorf("failed to create secret: %w", err)
	}
	return nil
}

func (r *DevboxReconciler) syncPod(ctx context.Context, devbox *devboxv1alpha1.Devbox, recLabels map[string]string) error {
	logger := log.FromContext(ctx)

	var podList corev1.PodList
	if err := r.List(ctx, &podList, client.InNamespace(devbox.Namespace), client.MatchingLabels(recLabels)); err != nil {
		return err
	}
	// only one pod is allowed, if more than one pod found, return error
	if len(podList.Items) > 1 {
		return fmt.Errorf("more than one pod found")
	}
	logger.Info("pod list", "length", len(podList.Items))

	// update devbox status after pod is created or updated
	defer func() {
		if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			logger.Info("update devbox status after pod synced")
			latestDevbox := &devboxv1alpha1.Devbox{}
			if err := r.Client.Get(ctx, client.ObjectKey{Namespace: devbox.Namespace, Name: devbox.Name}, latestDevbox); err != nil {
				logger.Error(err, "get latest devbox failed")
				return err
			}
			// update devbox status with latestDevbox status
			logger.Info("updating devbox status")
			logger.Info("merge commit history", "devbox", devbox.Status.CommitHistory, "latestDevbox", latestDevbox.Status.CommitHistory)
			devbox.Status.Phase = helper.GenerateDevboxPhase(devbox, podList)
			helper.UpdateDevboxStatus(devbox, latestDevbox)
			return r.Status().Update(ctx, latestDevbox)
		}); err != nil {
			logger.Error(err, "sync pod failed")
			r.Recorder.Eventf(devbox, corev1.EventTypeWarning, "Sync pod failed", "%v", err)
			return
		}
		logger.Info("update devbox status success")
		r.Recorder.Eventf(devbox, corev1.EventTypeNormal, "Sync pod success", "Sync pod success")
	}()

	switch devbox.Spec.State {
	case devboxv1alpha1.DevboxStateRunning:
		runtimecr, err := r.getRuntime(ctx, devbox)
		if err != nil {
			return err
		}
		nextCommitHistory := r.generateNextCommitHistory(devbox)
		expectPod := r.generateDevboxPod(devbox, runtimecr, nextCommitHistory)

		switch len(podList.Items) {
		case 0:
			logger.Info("create pod")
			logger.Info("next commit history", "commit", nextCommitHistory)
			err := r.createPod(ctx, devbox, expectPod, nextCommitHistory)
			if err != nil && helper.IsExceededQuotaError(err) {
				logger.Info("devbox is exceeded quota, change devbox state to Stopped")
				r.Recorder.Eventf(devbox, corev1.EventTypeWarning, "Devbox is exceeded quota", "Devbox is exceeded quota")
				devbox.Spec.State = devboxv1alpha1.DevboxStateStopped
				_ = r.Update(ctx, devbox)
				return nil
			}
			if err != nil {
				logger.Error(err, "create pod failed")
				return err
			}
			return nil
		case 1:
			pod := &podList.Items[0]
			// check pod container size, if it is 0, it means the pod is not running, return an error
			if len(pod.Status.ContainerStatuses) == 0 {
				return fmt.Errorf("pod container size is 0")
			}
			devbox.Status.State = pod.Status.ContainerStatuses[0].State
			// update commit predicated status by pod status, this should be done once find a pod
			helper.UpdatePredicatedCommitStatus(devbox, pod)
			// pod has been deleted, handle it, next reconcile will create a new pod, and we will update commit history status by predicated status
			if !pod.DeletionTimestamp.IsZero() {
				logger.Info("pod has been deleted")
				return r.handlePodDeleted(ctx, devbox, pod)
			}
			switch helper.PodMatchExpectations(expectPod, pod) {
			case true:
				// pod match expectations
				logger.Info("pod match expectations")
				switch pod.Status.Phase {
				case corev1.PodPending, corev1.PodRunning:
					// pod is running or pending, do nothing here
					logger.Info("pod is running or pending")
					// update commit history status by pod status
					helper.UpdateCommitHistory(devbox, pod, false)
					return nil
				case corev1.PodFailed, corev1.PodSucceeded:
					// pod failed or succeeded, we need delete pod and remove finalizer
					logger.Info("pod failed or succeeded, recreate pod")
					return r.deletePod(ctx, devbox, pod)
				}
			case false:
				// pod not match expectations, delete pod anyway
				logger.Info("pod not match expectations, recreate pod")
				return r.deletePod(ctx, devbox, pod)
			}
		}
	case devboxv1alpha1.DevboxStateStopped:
		switch len(podList.Items) {
		case 0:
			return nil
		case 1:
			pod := &podList.Items[0]
			// update state to empty since devbox is stopped
			devbox.Status.State = corev1.ContainerState{}
			// update commit predicated status by pod status, this should be done once find a pod
			helper.UpdatePredicatedCommitStatus(devbox, pod)
			// pod has been deleted, handle it, next reconcile will create a new pod, and we will update commit history status by predicated status
			if !pod.DeletionTimestamp.IsZero() {
				return r.handlePodDeleted(ctx, devbox, pod)
			}
			// we need delete pod because devbox state is stopped
			// we don't care about the pod status, just delete it
			return r.deletePod(ctx, devbox, pod)
		}
	}
	return nil
}

func (r *DevboxReconciler) syncNodePortNetwork(ctx context.Context, devbox *devboxv1alpha1.Devbox, recLabels map[string]string, servicePorts []corev1.ServicePort) error {
	var err error
	expectServiceSpec := corev1.ServiceSpec{
		Selector: recLabels,
		Type:     corev1.ServiceTypeNodePort,
		Ports:    servicePorts,
	}
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      devbox.Name + "-svc",
			Namespace: devbox.Namespace,
			Labels:    recLabels,
		},
	}

	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, service, func() error {
		// only update some specific fields
		service.Spec.Selector = expectServiceSpec.Selector
		service.Spec.Type = expectServiceSpec.Type
		if len(service.Spec.Ports) == 0 {
			service.Spec.Ports = expectServiceSpec.Ports
		} else {
			service.Spec.Ports[0].Name = expectServiceSpec.Ports[0].Name
			service.Spec.Ports[0].Port = expectServiceSpec.Ports[0].Port
			service.Spec.Ports[0].TargetPort = expectServiceSpec.Ports[0].TargetPort
			service.Spec.Ports[0].Protocol = expectServiceSpec.Ports[0].Protocol
		}
		return controllerutil.SetControllerReference(devbox, service, r.Scheme)
	}); err != nil {
		return err
	}

	// Retrieve the updated Service to get the NodePort
	var updatedService corev1.Service
	err = retry.OnError(
		retry.DefaultRetry,
		func(err error) bool { return client.IgnoreNotFound(err) == nil },
		func() error {
			return r.Client.Get(ctx, client.ObjectKey{Namespace: service.Namespace, Name: service.Name}, &updatedService)
		})
	if err != nil {
		return fmt.Errorf("failed to get updated service: %w", err)
	}

	// Extract the NodePort
	nodePort := int32(0)
	for _, port := range updatedService.Spec.Ports {
		if port.NodePort != 0 {
			nodePort = port.NodePort
			break
		}
	}
	if nodePort == 0 {
		return fmt.Errorf("NodePort not found for service %s", service.Name)
	}
	devbox.Status.Network.Type = devboxv1alpha1.NetworkTypeNodePort
	devbox.Status.Network.NodePort = nodePort

	return r.Status().Update(ctx, devbox)
}

// get the runtime
func (r *DevboxReconciler) getRuntime(ctx context.Context, devbox *devboxv1alpha1.Devbox) (*devboxv1alpha1.Runtime, error) {
	runtimeNamespace := devbox.Spec.RuntimeRef.Namespace
	if runtimeNamespace == "" {
		runtimeNamespace = devbox.Namespace
	}
	runtimecr := &devboxv1alpha1.Runtime{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: runtimeNamespace, Name: devbox.Spec.RuntimeRef.Name}, runtimecr); err != nil {
		return nil, err
	}
	return runtimecr, nil
}

func (r *DevboxReconciler) getServicePort(ctx context.Context, devbox *devboxv1alpha1.Devbox, recLabels map[string]string) ([]corev1.ServicePort, error) {
	runtimecr, err := r.getRuntime(ctx, devbox)
	if err != nil {
		return nil, err
	}
	var servicePorts []corev1.ServicePort
	for _, port := range runtimecr.Spec.Config.Ports {
		servicePorts = append(servicePorts, corev1.ServicePort{
			Name:       port.Name,
			Port:       port.ContainerPort,
			TargetPort: intstr.FromInt32(port.ContainerPort),
			Protocol:   port.Protocol,
		})
	}
	if len(servicePorts) == 0 {
		servicePorts = []corev1.ServicePort{
			{
				Name:       "devbox-ssh-port",
				Port:       22,
				TargetPort: intstr.FromInt32(22),
				Protocol:   corev1.ProtocolTCP,
			},
		}
	}
	return servicePorts, nil
}

func (r *DevboxReconciler) syncNetwork(ctx context.Context, devbox *devboxv1alpha1.Devbox, recLabels map[string]string) error {
	servicePorts, err := r.getServicePort(ctx, devbox, recLabels)
	if err != nil {
		return err
	}
	switch devbox.Spec.NetworkSpec.Type {
	case devboxv1alpha1.NetworkTypeNodePort:
		return r.syncNodePortNetwork(ctx, devbox, recLabels, servicePorts)
	case devboxv1alpha1.NetworkTypeWebSocket:
		return r.syncWebSocketNetwork(ctx, devbox, recLabels, servicePorts)
	}
	return nil
}

func (r *DevboxReconciler) syncWebSocketNetwork(ctx context.Context, devbox *devboxv1alpha1.Devbox, recLabels map[string]string, servicePorts []corev1.ServicePort) error {
	devbox.Status.Network.Type = devboxv1alpha1.NetworkTypeWebSocket
	if err := r.Status().Update(ctx, devbox); err != nil {
		return err
	}
	if err := r.syncPodSvc(ctx, devbox, recLabels, servicePorts); err != nil {
		return err
	}
	if err := r.syncProxyPod(ctx, devbox, recLabels, servicePorts); err != nil {
		return err
	}
	if err := r.syncProxySvc(ctx, devbox, recLabels, servicePorts); err != nil {
		return err
	}
	if hostName, err := r.syncProxyIngress(ctx, devbox); err != nil {
		return err
	} else {
		devbox.Status.Network.WebSocket = hostName
	}
	return r.Status().Update(ctx, devbox)
}

func (r *DevboxReconciler) generateProxyIngressHost() string {
	return rand.String(12) + "." + r.WebsocketProxyDomain
}

func (r *DevboxReconciler) syncProxyIngress(ctx context.Context, devbox *devboxv1alpha1.Devbox) (string, error) {
	host := r.generateProxyIngressHost()

	pathType := networkingv1.PathTypePrefix
	ingressPath := []networkingv1.HTTPIngressPath{
		{
			Path:     "/",
			PathType: &pathType,
			Backend: networkingv1.IngressBackend{
				Service: &networkingv1.IngressServiceBackend{
					Name: devbox.Name + "-proxy-svc",
					Port: networkingv1.ServiceBackendPort{
						Number: 80,
					},
				},
			},
		},
	}

	ingressSpec := networkingv1.IngressSpec{
		IngressClassName: &r.IngressClass,
		Rules: []networkingv1.IngressRule{
			{
				Host: host,
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: ingressPath,
					},
				},
			},
		},
	}

	wsIngress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      devbox.Name + "-proxy-ingress",
			Namespace: devbox.Namespace,
		},
		Spec: ingressSpec,
	}

	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, wsIngress, func() error {
		return controllerutil.SetControllerReference(devbox, wsIngress, r.Scheme)
	}); err != nil {
		return "", err
	}

	return host, nil
}

func (r *DevboxReconciler) syncProxySvc(ctx context.Context, devbox *devboxv1alpha1.Devbox, recLabels map[string]string, servicePorts []corev1.ServicePort) error {
	runtimecr, err := r.getRuntime(ctx, devbox)
	if err != nil {
		return err
	}
	servicePort := []corev1.ServicePort{
		{
			Name:       "devbox-ssh-port",
			Port:       80,
			TargetPort: intstr.FromInt32(80),
			Protocol:   corev1.ProtocolTCP,
		},
	}
	expectServiceSpec := corev1.ServiceSpec{
		Selector: helper.GenerateProxyPodLabels(devbox, runtimecr),
		Type:     corev1.ServiceTypeClusterIP,
		Ports:    servicePort,
	}
	proxySvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      devbox.Name + "-proxy-svc",
			Namespace: devbox.Namespace,
			Labels:    helper.GenerateProxyPodLabels(devbox, runtimecr),
		},
		Spec: expectServiceSpec,
	}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, proxySvc, func() error {
		proxySvc.Spec.Selector = expectServiceSpec.Selector
		proxySvc.Spec.Type = expectServiceSpec.Type
		proxySvc.Spec.Ports[0].Name = expectServiceSpec.Ports[0].Name
		proxySvc.Spec.Ports[0].Port = expectServiceSpec.Ports[0].Port
		proxySvc.Spec.Ports[0].TargetPort = expectServiceSpec.Ports[0].TargetPort
		proxySvc.Spec.Ports[0].Protocol = expectServiceSpec.Ports[0].Protocol
		return controllerutil.SetControllerReference(devbox, proxySvc, r.Scheme)
	}); err != nil {
		return err
	}
	return nil
}

func (r *DevboxReconciler) generateProxyPodName(devbox *devboxv1alpha1.Devbox) string {
	return devbox.Name + "-proxy-pod" + "-" + rand.String(5)
}

func (r *DevboxReconciler) generateProxyPodDeploymentName(devbox *devboxv1alpha1.Devbox) string {
	return devbox.Name + "-proxy-deployment"
}

type DevboxClaims struct {
	DevboxName string `json:"devbox_name"`
	NameSpace  string `json:"namespace"`
	jwt.RegisteredClaims
}

func (r *DevboxReconciler) generateProxyPodJWT(ctx context.Context, devbox *devboxv1alpha1.Devbox) (string, error) {
	claims := DevboxClaims{
		DevboxName: devbox.Name,
		NameSpace:  devbox.Namespace,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 7 * 24)),
			Issuer:    "devbox-controller",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(r.ShutdownServerKey))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}
	return signedToken, nil
}

func (r *DevboxReconciler) generateProxyPodEnv(ctx context.Context, devbox *devboxv1alpha1.Devbox, servicePorts []corev1.ServicePort) ([]corev1.EnvVar, error) {
	var envVars []corev1.EnvVar
	autoShutdownEnabled := devbox.Spec.AutoShutdownSpec.Enable && r.EnableAutoShutdown
	envVars = append(envVars, corev1.EnvVar{
		Name:  "ENABLE_AUTO_SHUTDOWN",
		Value: strconv.FormatBool(autoShutdownEnabled),
	})

	if autoShutdownEnabled {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "AUTO_SHUTDOWN_INTERVAL",
			Value: devbox.Spec.AutoShutdownSpec.Time,
		})
	}

	token, err := r.generateProxyPodJWT(ctx, devbox)
	if err != nil {
		return nil, err
	}

	envVars = append(envVars, corev1.EnvVar{
		Name:  "JWT_TOKEN",
		Value: token,
	})

	sshPort := "22"
	for _, port := range servicePorts {
		if port.Name == "devbox-ssh-port" {
			sshPort = port.TargetPort.String()
			break
		}
	}
	envVars = append(envVars, corev1.EnvVar{
		Name:  "TARGET",
		Value: fmt.Sprintf("%s-pod-svc:%s", devbox.Name, sshPort),
	})

	envVars = append(envVars, corev1.EnvVar{
		Name:  "LISTEN",
		Value: "0.0.0.0:80",
	})

	envVars = append(envVars, corev1.EnvVar{
		Name:  "AUTO_SHUTDOWN_SERVICE_URL",
		Value: r.ShutdownServerAddr,
	})

	return envVars, nil
}

func (r *DevboxReconciler) generateProxyPodDeployment(ctx context.Context, devbox *devboxv1alpha1.Devbox, recLabels map[string]string, servicePorts []corev1.ServicePort) (*appsv1.Deployment, error) {
	runtimecr, err := r.getRuntime(ctx, devbox)
	if err != nil {
		return nil, err
	}

	podEnv, err := r.generateProxyPodEnv(ctx, devbox, servicePorts)
	if err != nil {
		return nil, err
	}

	podSpec := corev1.PodSpec{
		Containers: []corev1.Container{
			{
				Name:      "ws-proxy",
				Image:     r.WebSocketImage,
				Env:       podEnv,
				Resources: helper.GenerateProxyPodResourceRequirements(),
			},
		},
	}

	replicas := int32(1)
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        r.generateProxyPodDeploymentName(devbox),
			Namespace:   devbox.Namespace,
			Labels:      helper.GenerateProxyPodLabels(devbox, runtimecr),
			Annotations: helper.GeneratePodAnnotations(devbox, runtimecr),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: helper.GenerateProxyPodLabels(devbox, runtimecr),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:        r.generateProxyPodName(devbox),
					Namespace:   devbox.Namespace,
					Labels:      helper.GenerateProxyPodLabels(devbox, runtimecr),
					Annotations: helper.GeneratePodAnnotations(devbox, runtimecr),
				},
				Spec: podSpec,
			},
		},
	}, nil
}

func (r *DevboxReconciler) syncProxyPod(ctx context.Context, devbox *devboxv1alpha1.Devbox, recLabels map[string]string, servicePorts []corev1.ServicePort) error {
	wsDeployment := &appsv1.Deployment{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: r.generateProxyPodDeploymentName(devbox), Namespace: devbox.Namespace}, wsDeployment)

	if devbox.Spec.State == devboxv1alpha1.DevboxStateRunning {
		if errors.IsNotFound(err) {
			wsDeployment, err = r.generateProxyPodDeployment(ctx, devbox, recLabels, servicePorts)
			if err != nil {
				return err
			}
			if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, wsDeployment, func() error {
				return controllerutil.SetControllerReference(devbox, wsDeployment, r.Scheme)
			}); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
	} else {
		if err == nil {
			if err := r.Client.Delete(ctx, wsDeployment); err != nil {
				return err
			}
		} else if !errors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func (r *DevboxReconciler) syncPodSvc(ctx context.Context, devbox *devboxv1alpha1.Devbox, recLabels map[string]string, servicePorts []corev1.ServicePort) error {
	runtimecr, err := r.getRuntime(ctx, devbox)
	if err != nil {
		return err
	}

	expectServiceSpec := corev1.ServiceSpec{
		Selector: recLabels,
		Type:     corev1.ServiceTypeClusterIP,
		Ports:    servicePorts,
	}
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      devbox.Name + "-pod-svc",
			Namespace: devbox.Namespace,
			Labels:    helper.GenerateProxyPodLabels(devbox, runtimecr),
		},
	}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, service, func() error {
		// only update some specific fields
		service.Spec.Selector = expectServiceSpec.Selector
		service.Spec.Type = expectServiceSpec.Type
		if len(service.Spec.Ports) == 0 {
			service.Spec.Ports = expectServiceSpec.Ports
		} else {
			service.Spec.Ports[0].Name = expectServiceSpec.Ports[0].Name
			service.Spec.Ports[0].Port = expectServiceSpec.Ports[0].Port
			service.Spec.Ports[0].TargetPort = expectServiceSpec.Ports[0].TargetPort
			service.Spec.Ports[0].Protocol = expectServiceSpec.Ports[0].Protocol
		}
		return controllerutil.SetControllerReference(devbox, service, r.Scheme)
	}); err != nil {
		return err
	}
	return nil

}

// create a new pod, add predicated status to nextCommitHistory
func (r *DevboxReconciler) createPod(ctx context.Context, devbox *devboxv1alpha1.Devbox, expectPod *corev1.Pod, nextCommitHistory *devboxv1alpha1.CommitHistory) error {
	nextCommitHistory.Status = devboxv1alpha1.CommitStatusPending
	nextCommitHistory.PredicatedStatus = devboxv1alpha1.CommitStatusPending
	if err := r.Create(ctx, expectPod); err != nil {
		return err
	}
	devbox.Status.CommitHistory = append(devbox.Status.CommitHistory, nextCommitHistory)
	return nil
}

func (r *DevboxReconciler) deletePod(ctx context.Context, devbox *devboxv1alpha1.Devbox, pod *corev1.Pod) error {
	logger := log.FromContext(ctx)
	// remove finalizer and delete pod
	controllerutil.RemoveFinalizer(pod, devboxv1alpha1.FinalizerName)
	if err := r.Update(ctx, pod); err != nil {
		logger.Error(err, "remove finalizer failed")
		return err
	}
	if err := r.Delete(ctx, pod); err != nil {
		logger.Error(err, "delete pod failed")
		return err
	}
	// update commit history status because pod has been deleted
	devbox.Status.LastTerminationState = pod.Status.ContainerStatuses[0].State
	helper.UpdateCommitHistory(devbox, pod, true)
	return nil
}

func (r *DevboxReconciler) handlePodDeleted(ctx context.Context, devbox *devboxv1alpha1.Devbox, pod *corev1.Pod) error {
	logger := log.FromContext(ctx)
	controllerutil.RemoveFinalizer(pod, devboxv1alpha1.FinalizerName)
	if err := r.Update(ctx, pod); err != nil {
		logger.Error(err, "remove finalizer failed")
		return err
	}
	// update commit history status because pod has been deleted
	helper.UpdateCommitHistory(devbox, pod, true)
	devbox.Status.LastTerminationState = pod.Status.ContainerStatuses[0].State
	return nil
}

func (r *DevboxReconciler) removeAll(ctx context.Context, devbox *devboxv1alpha1.Devbox, recLabels map[string]string) error {
	// Delete Pod
	podList := &corev1.PodList{}
	if err := r.List(ctx, podList, client.InNamespace(devbox.Namespace), client.MatchingLabels(recLabels)); err != nil {
		return err
	}
	for _, pod := range podList.Items {
		if controllerutil.RemoveFinalizer(&pod, devboxv1alpha1.FinalizerName) {
			if err := r.Update(ctx, &pod); err != nil {
				return err
			}
		}
	}
	if err := r.deleteResourcesByLabels(ctx, &corev1.Pod{}, devbox.Namespace, recLabels); err != nil {
		return err
	}
	// Delete Service
	if err := r.deleteResourcesByLabels(ctx, &corev1.Service{}, devbox.Namespace, recLabels); err != nil {
		return err
	}
	// Delete Secret
	return r.deleteResourcesByLabels(ctx, &corev1.Secret{}, devbox.Namespace, recLabels)
}

func (r *DevboxReconciler) deleteResourcesByLabels(ctx context.Context, obj client.Object, namespace string, labels map[string]string) error {
	err := r.DeleteAllOf(ctx, obj,
		client.InNamespace(namespace),
		client.MatchingLabels(labels),
	)
	return client.IgnoreNotFound(err)
}

func (r *DevboxReconciler) generateDevboxPod(devbox *devboxv1alpha1.Devbox, runtime *devboxv1alpha1.Runtime, nextCommitHistory *devboxv1alpha1.CommitHistory) *corev1.Pod {
	objectMeta := metav1.ObjectMeta{
		Name:        nextCommitHistory.Pod,
		Namespace:   devbox.Namespace,
		Labels:      helper.GeneratePodLabels(devbox, runtime),
		Annotations: helper.GeneratePodAnnotations(devbox, runtime),
	}

	// set up ports and env by using runtime ports and devbox extra ports
	ports := runtime.Spec.Config.Ports
	// TODO: add extra ports to pod, currently not support
	// ports = append(ports, devbox.Spec.NetworkSpec.ExtraPorts...)

	envs := runtime.Spec.Config.Env
	envs = append(envs, devbox.Spec.ExtraEnvs...)
	envs = append(envs, helper.GenerateDevboxEnvVars(devbox, nextCommitHistory)...)

	//get image name
	var imageName string
	if r.DebugMode {
		imageName = runtime.Spec.Config.Image
	} else {
		imageName = helper.GetLastSuccessCommitImageName(devbox, runtime)
	}

	volumes := runtime.Spec.Config.Volumes
	volumes = append(volumes, helper.GenerateSSHVolume(devbox))
	volumes = append(volumes, devbox.Spec.ExtraVolumes...)

	volumeMounts := runtime.Spec.Config.VolumeMounts
	volumeMounts = append(volumeMounts, helper.GenerateSSHVolumeMounts()...)
	volumeMounts = append(volumeMounts, devbox.Spec.ExtraVolumeMounts...)

	containers := []corev1.Container{
		{
			Name:         devbox.ObjectMeta.Name,
			Image:        imageName,
			Env:          envs,
			Ports:        ports,
			VolumeMounts: volumeMounts,

			WorkingDir: helper.GenerateWorkingDir(devbox, runtime),
			Command:    helper.GenerateCommand(devbox, runtime),
			Args:       helper.GenerateDevboxArgs(devbox, runtime),
			Resources:  helper.GenerateResourceRequirements(devbox, r.RequestCPURate, r.RequestMemoryRate, r.RequestEphemeralStorage, r.LimitEphemeralStorage),
		},
	}

	terminationGracePeriodSeconds := 300
	automountServiceAccountToken := false

	expectPod := &corev1.Pod{
		ObjectMeta: objectMeta,
		Spec: corev1.PodSpec{
			TerminationGracePeriodSeconds: ptr.To(int64(terminationGracePeriodSeconds)),
			AutomountServiceAccountToken:  ptr.To(automountServiceAccountToken),
			RestartPolicy:                 corev1.RestartPolicyNever,

			Hostname:   devbox.Name,
			Containers: containers,
			Volumes:    volumes,

			Tolerations: devbox.Spec.Tolerations,
			Affinity:    devbox.Spec.Affinity,
		},
	}
	// set controller reference and finalizer
	_ = controllerutil.SetControllerReference(devbox, expectPod, r.Scheme)
	controllerutil.AddFinalizer(expectPod, devboxv1alpha1.FinalizerName)
	return expectPod
}

func (r *DevboxReconciler) generateNextCommitHistory(devbox *devboxv1alpha1.Devbox) *devboxv1alpha1.CommitHistory {
	now := time.Now()
	return &devboxv1alpha1.CommitHistory{
		Image:            r.generateImageName(devbox),
		Time:             metav1.Time{Time: now},
		Pod:              devbox.Name + "-" + rand.String(5),
		Status:           devboxv1alpha1.CommitStatusPending,
		PredicatedStatus: devboxv1alpha1.CommitStatusPending,
	}
}

func (r *DevboxReconciler) generateImageName(devbox *devboxv1alpha1.Devbox) string {
	now := time.Now()
	return fmt.Sprintf("%s/%s/%s:%s-%s", r.CommitImageRegistry, devbox.Namespace, devbox.Name, rand.String(5), now.Format("2006-01-02-150405"))
}

// SetupWithManager sets up the controller with the Manager.
func (r *DevboxReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&devboxv1alpha1.Devbox{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Owns(&corev1.Pod{}, builder.WithPredicates(predicate.ResourceVersionChangedPredicate{})). // enqueue request if pod spec/status is updated
		Owns(&corev1.Service{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Owns(&corev1.Secret{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Owns(&networkingv1.Ingress{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Owns(&appsv1.Deployment{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Complete(r)
}
