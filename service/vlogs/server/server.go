package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/labring/sealos/service/pkg/api"
	"github.com/labring/sealos/service/vlogs/request"
)

type VLogsServer struct {
	path     string
	username string
	password string
}

func NewVLogsServer(config *Config) (*VLogsServer, error) {
	vl := &VLogsServer{
		path:     config.Server.Path,
		username: config.Server.Username,
		password: config.Server.Password,
	}
	return vl, nil
}

func (vl *VLogsServer) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	pathPrefix := ""
	switch {
	case req.URL.Path == pathPrefix+"/queryLogsByParams":
		vl.queryLogsByParams(rw, req)
	default:
		http.Error(rw, "Not found", http.StatusNotFound)
		return
	}
}

func (vl *VLogsServer) queryLogsByParams(rw http.ResponseWriter, req *http.Request) {
	_, _, query, err := vl.generateParamsRequest(req)
	if err != nil {
		http.Error(rw, fmt.Sprintf("Bad request (%s)", err), http.StatusBadRequest)
		log.Printf("Bad request (%s)\n", err)
		return
	}

	//err = auth.Authenticate(namespace, kubeConfig)
	//if err != nil {
	//	http.Error(rw, fmt.Sprintf("Authentication failed (%s)", err), http.StatusInternalServerError)
	//	log.Printf("Authentication failed (%s)\n", err)
	//	return
	//}

	fmt.Println("query: " + query)
	err = request.QueryLogsByParams(vl.path, vl.username, vl.password, query, rw)
	if err != nil {
		http.Error(rw, fmt.Sprintf("Query failed (%s)", err), http.StatusInternalServerError)
		log.Printf("Query failed (%s)\n", err)
		return
	}
	return
}

func (vl *VLogsServer) generateParamsRequest(req *http.Request) (string, string, string, error) {
	kubeConfig := req.Header.Get("Authorization")
	if config, err := url.PathUnescape(kubeConfig); err == nil {
		kubeConfig = config
	} else {
		return "", "", "", err
	}

	var query string
	vlogsReq := &api.VlogsRequest{}
	err := json.NewDecoder(req.Body).Decode(&vlogsReq)
	if err != nil {
		return "", "", "", errors.New("invalid JSON data,decode error")
	}
	if vlogsReq.Namespace == "" {
		return "", "", "", errors.New("invalid JSON data,namespace not found")
	}
	var vlogs VLogsQuery
	query, err = vlogs.getQuery(vlogsReq)
	if err != nil {
		return "", "", "", err
	}
	return kubeConfig, vlogsReq.Namespace, query, nil
}

type VLogsQuery struct {
	query string
}

func (v *VLogsQuery) getQuery(req *api.VlogsRequest) (string, error) {
	v.generateKeywordQuery(req)
	v.generateStreamQuery(req)
	v.generateCommonQuery(req)
	err := v.generateJsonQuery(req)
	if err != nil {
		return "", err
	}
	v.generateDropQuery()
	v.generateNumberQuery(req)
	return v.query, nil
}

func (v *VLogsQuery) generateKeywordQuery(req *api.VlogsRequest) {
	if req.JsonMode != "true" {
		var builder strings.Builder
		builder.WriteString(req.Keyword)
		builder.WriteString(" ")
		v.query += builder.String()
	}
}

func (v *VLogsQuery) generateJsonQuery(req *api.VlogsRequest) error {
	if req.JsonMode == "true" {
		var builder strings.Builder
		builder.WriteString(" | unpack_json")
		if len(req.JsonQuery) > 0 {
			for _, jsonQuery := range req.JsonQuery {
				var item string
				switch jsonQuery.Mode {
				case "=":
					item = fmt.Sprintf("| %s:=%s ", jsonQuery.Key, jsonQuery.Value)
				case "!=":
					item = fmt.Sprintf("| %s:(!=%s) ", jsonQuery.Key, jsonQuery.Value)
				case "~":
					item = fmt.Sprintf("| %s:%s ", jsonQuery.Key, jsonQuery.Value)
				default:
					return errors.New("invalid JSON data,jsonMode value err")
				}
				builder.WriteString(item)
			}
		}
		v.query += builder.String()
	}
	return nil
}

func (v *VLogsQuery) generateStreamQuery(req *api.VlogsRequest) {
	var builder strings.Builder
	addItems := func(namespace string, key string, values []string) {
		for i, value := range values {
			builder.WriteString(fmt.Sprintf(`{%s="%s",namespace="%s"}`, key, value, namespace))
			if i != len(values)-1 {
				builder.WriteString(" OR ")
			}
		}
	}
	switch {
	case len(req.Pod) == 0 && len(req.Container) == 0:
		builder.WriteString(fmt.Sprintf(`{namespace="%s"}`, req.Namespace))
	case len(req.Pod) == 0:
		addItems(req.Namespace, "container", req.Container)
	case len(req.Container) == 0:
		addItems(req.Namespace, "pod", req.Pod)
	default:
		for i, container := range req.Container {
			for j, pod := range req.Pod {
				builder.WriteString(fmt.Sprintf(`{container="%s",namespace="%s",pod="%s"}`, container, req.Namespace, pod))
				if i != len(req.Container)-1 || j != len(req.Pod)-1 {
					builder.WriteString(" OR ")
				}
			}
		}
	}
	v.query += builder.String()
}

func (v *VLogsQuery) generateCommonQuery(req *api.VlogsRequest) {
	var builder strings.Builder
	item := fmt.Sprintf(`_time:%s app:="%s" `, req.Time, req.App)
	builder.WriteString(item)
	if req.StderrMode == "true" {
		item := fmt.Sprintf(` stream:="stderr" `)
		builder.WriteString(item)
	}
	// if query number,dont use limit param
	if req.NumberMode == "false" {
		item := fmt.Sprintf(`  | limit %s  `, req.Limit)
		builder.WriteString(item)
	}
	v.query += builder.String()
}

func (v *VLogsQuery) generateDropQuery() {
	var builder strings.Builder
	builder.WriteString("| Drop _stream_id,_stream,app,container,job,namespace,node,pod ")
	v.query += builder.String()
}

func (v *VLogsQuery) generateNumberQuery(req *api.VlogsRequest) {
	var builder strings.Builder
	if req.NumberMode == "true" {
		item := fmt.Sprintf(" | stats by (_time:1%s) count() logs_total ", req.NumberLevel)
		builder.WriteString(item)
		v.query += builder.String()
	}
}
