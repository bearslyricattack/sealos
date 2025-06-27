import { authSession } from '@/services/backend/auth';
import { getK8s } from '@/services/backend/kubernetes';
import { handleK8sError, jsonRes } from '@/services/backend/response';
import { ApiResp } from '@/services/kubernet';
import { KbPgClusterType } from '@/types/cluster';
import { BackupItemType, DBEditType } from '@/types/db';
import { json2Account, json2CreateCluster } from '@/utils/json2Yaml';
import type { NextApiRequest, NextApiResponse } from 'next';
import { updateBackupPolicyApi } from '../../backup/updatePolicy';
import { BackupSupportedDBTypeList } from '@/constants/db';
import { convertBackupFormToSpec } from '@/utils/adapt';

export default async function handler(req: NextApiRequest, res: NextApiResponse<ApiResp>) {
  try {
    const defaultDbForm: DBEditType = {
      dbType: 'postgresql',
      dbVersion: '15.4',
      dbName: 'test-db',
      replicas: 1,
      cpu: 1000,
      memory: 1024,
      storage: 1,
      labels: {},
      terminationPolicy: 'Delete',
      autoBackup: {
        start: true,
        type: 'day',
        week: [],
        hour: '23',
        minute: '00',
        saveTime: 7,
        saveType: 'd'
      }
    };

    const { dbForm: rawDbForm, backupInfo } = req.body as {
      dbForm: DBEditType;
      backupInfo?: BackupItemType;
    };

    const dbForm: DBEditType = {
      ...defaultDbForm,
      ...rawDbForm,
      autoBackup: {
        ...defaultDbForm.autoBackup,
        ...(rawDbForm?.autoBackup || {})
      }
    } as DBEditType;

    const { k8sCustomObjects, namespace, applyYamlList } = await getK8s({
      kubeconfig: await authSession(req)
    });

    const account = json2Account(dbForm);
    const cluster = json2CreateCluster(dbForm, backupInfo, {
      storageClassName: process.env.STORAGE_CLASSNAME
    });

    await applyYamlList([account, cluster], 'create');
    const { body } = (await k8sCustomObjects.getNamespacedCustomObject(
      'apps.kubeblocks.io',
      'v1alpha1',
      namespace,
      'clusters',
      dbForm.dbName
    )) as {
      body: KbPgClusterType;
    };
    const dbUid = body.metadata.uid;

    const updateAccountYaml = json2Account(dbForm, dbUid);

    await applyYamlList([updateAccountYaml], 'replace');

    try {
      if (BackupSupportedDBTypeList.includes(dbForm.dbType) && dbForm?.autoBackup) {
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
      }
    } catch (err: any) {
      // local env will fail to update backup policy
      if (process.env.NODE_ENV === 'production') {
        throw err;
      } else {
        console.log(err);
      }
    }

    jsonRes(res, {
      data: 'success create db'
    });
  } catch (err: any) {
    console.log('error create db', err);
    jsonRes(res, handleK8sError(err));
  }
}
