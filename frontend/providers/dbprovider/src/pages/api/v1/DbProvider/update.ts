import { authSession } from '@/services/backend/auth';
import { getK8s } from '@/services/backend/kubernetes';
import { handleK8sError, jsonRes } from '@/services/backend/response';
import { ApiResp } from '@/services/kubernet';
import { KbPgClusterType } from '@/types/cluster';
import { BackupItemType, DBEditType } from '@/types/db';
import { json2ResourceOps } from '@/utils/json2Yaml';
import type { NextApiRequest, NextApiResponse } from 'next';
import { updateBackupPolicyApi } from '../../backup/updatePolicy';
import { BackupSupportedDBTypeList } from '@/constants/db';
import { adaptDBDetail, convertBackupFormToSpec } from '@/utils/adapt';
import { CustomObjectsApi, PatchUtils } from '@kubernetes/client-node';

export default async function handler(req: NextApiRequest, res: NextApiResponse<ApiResp>) {
  try {
    const { dbForm } = req.body as {
      dbForm: any;
      isEdit: boolean;
      backupInfo?: BackupItemType;
    };

    const { k8sCustomObjects, namespace, applyYamlList } = await getK8s({
      kubeconfig: await authSession(req)
    });

    const { body } = (await k8sCustomObjects.getNamespacedCustomObject(
      'apps.kubeblocks.io',
      'v1alpha1',
      namespace,
      'clusters',
      dbForm.dbName
    )) as {
      body: KbPgClusterType;
    };
    const { cpu, memory, replicas, storage, terminationPolicy } = adaptDBDetail(body);

    const opsRequests = [];

    if ((dbForm.cpu || dbForm.memory) && (cpu !== dbForm.cpu || memory !== dbForm.memory)) {
      const verticalScalingYaml = json2ResourceOps(dbForm, 'VerticalScaling');
      opsRequests.push(verticalScalingYaml);
    }

    if (dbForm.replicas && replicas !== dbForm.replicas) {
      const horizontalScalingYaml = json2ResourceOps(dbForm, 'HorizontalScaling');
      opsRequests.push(horizontalScalingYaml);
    }

    if (dbForm.storage && dbForm.storage > storage) {
      const volumeExpansionYaml = json2ResourceOps(dbForm, 'VolumeExpansion');
      opsRequests.push(volumeExpansionYaml);
    }

    if (opsRequests.length > 0) {
      await applyYamlList(opsRequests, 'create');
    }

    if (dbForm.dbType && BackupSupportedDBTypeList.includes(dbForm.dbType) && dbForm?.autoBackup) {
      const autoBackup = convertBackupFormToSpec({
        autoBackup: dbForm?.autoBackup,
        dbType: dbForm.dbType
      });

      await updateBackupPolicyApi({
        dbName: dbForm.dbName,
        dbType: dbForm.dbType,
        autoBackup,
        k8sCustomObjects,
        namespace
      });

      if (dbForm.terminationPolicy && terminationPolicy !== dbForm.terminationPolicy) {
        await updateTerminationPolicyApi({
          dbName: dbForm.dbName,
          terminationPolicy: dbForm.terminationPolicy,
          k8sCustomObjects,
          namespace
        });
      }
    }

    return jsonRes(res, {
      data: `Successfully submitted ${opsRequests.length} change requests`
    });
  } catch (err: any) {
    console.log('error update db', err);
    jsonRes(res, handleK8sError(err));
  }
}

export async function updateTerminationPolicyApi({
  dbName,
  terminationPolicy,
  k8sCustomObjects,
  namespace
}: {
  dbName: string;
  terminationPolicy: string;
  k8sCustomObjects: CustomObjectsApi;
  namespace: string;
}) {
  const group = 'apps.kubeblocks.io';
  const version = 'v1alpha1';
  const plural = 'clusters';

  const patch = [
    {
      op: 'replace',
      path: '/spec/terminationPolicy',
      value: terminationPolicy
    }
  ];

  const result = await k8sCustomObjects.patchNamespacedCustomObject(
    group,
    version,
    namespace,
    plural,
    dbName,
    patch,
    undefined,
    undefined,
    undefined,
    { headers: { 'Content-type': PatchUtils.PATCH_FORMAT_JSON_PATCH } }
  );

  return result;
}
