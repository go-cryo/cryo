import { api } from 'src/boot/axios';

export type BackupJobType = 'psql' | 's3' | 'pvc';
export type BackupRunStatus = 'Running' | 'Succeeded' | 'Failed';

export type RetentionPolicy = {
  keepLast?: number;
  keepDaily?: number;
  keepWeekly?: number;
  keepMonthly?: number;
};

export type SecretKeyRef = {
  name: string;
  key: string;
};

export type S3CredentialRef = {
  name: string;
  accessKeyKey: string;
  secretKeyKey: string;
};

export type PSQLConfig = {
  hostname: string;
  port?: number;
  username: string;
  database: string;
  password?: string;
  credentialSecretRef?: string;
  stagingSize?: string;
  stagingStorageClassName?: string;
};

export type S3Config = {
  endpoint: string;
  bucket: string;
  credentialsSecretRef?: S3CredentialRef;
  accessKey?: string;
  secretKey?: string;
  stagingSize?: string;
  stagingStorageClassName?: string;
};

export type PVCConfig = {
  claimName: string;
  volumeSnapshotClassName?: string;
  snapshotRetention?: number;
};

export type BackupRun = {
  jobName: string;
  namespace: string;
  name: string;
  status: BackupRunStatus;
  startTime?: string;
  endTime?: string;
  message?: string;
};

export type BackupJob = {
  name: string;
  namespace: string;
  type: BackupJobType;
  schedule: string;
  suspend: boolean;
  repositoryRef: string;
  image?: string;
  retention?: RetentionPolicy;
  psql?: PSQLConfig;
  s3?: S3Config;
  pvc?: PVCConfig;
  lastRun?: BackupRun;
  nextRun?: string;
};

export type CreateBackupJobRequest = {
  name: string;
  namespace: string;
  type: BackupJobType;
  schedule: string;
  suspend?: boolean;
  repositoryRef: string;
  image?: string;
  retention?: RetentionPolicy;
  psql?: PSQLConfig;
  s3?: S3Config;
  pvc?: PVCConfig;
};

export type UpdateBackupJobRequest = {
  schedule?: string;
  suspend?: boolean;
  repositoryRef?: string;
  image?: string;
  retention?: RetentionPolicy;
  psql?: PSQLConfig;
  s3?: S3Config;
  pvc?: PVCConfig;
};

async function listBackupJobs(): Promise<BackupJob[]> {
  const response = await api.get<BackupJob[]>('/backupjobs');
  return response.data;
}

async function getBackupJob(namespace: string, name: string): Promise<BackupJob> {
  const response = await api.get<BackupJob>(`/backupjobs/${namespace}/${name}`);
  return response.data;
}

async function createBackupJob(req: CreateBackupJobRequest): Promise<BackupJob> {
  const response = await api.post<BackupJob>('/backupjobs', req);
  return response.data;
}

async function updateBackupJob(namespace: string, name: string, req: UpdateBackupJobRequest): Promise<BackupJob> {
  const response = await api.put<BackupJob>(`/backupjobs/${namespace}/${name}`, req);
  return response.data;
}

async function deleteBackupJob(namespace: string, name: string): Promise<void> {
  await api.delete(`/backupjobs/${namespace}/${name}`);
}

async function triggerBackupJob(namespace: string, name: string): Promise<BackupRun> {
  const response = await api.post<BackupRun>(`/backupjobs/${namespace}/${name}/trigger`);
  return response.data;
}

async function listBackupJobRuns(namespace: string, name: string): Promise<BackupRun[]> {
  const response = await api.get<BackupRun[]>(`/backupjobs/${namespace}/${name}/runs`);
  return response.data;
}

export default {
  listBackupJobs,
  getBackupJob,
  createBackupJob,
  updateBackupJob,
  deleteBackupJob,
  triggerBackupJob,
  listBackupJobRuns,
};
