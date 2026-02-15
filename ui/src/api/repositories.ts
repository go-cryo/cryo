import { api } from 'src/boot/axios';

export type RepositoryType = 's3' | 'local' | 'sftp' | 'rest';

export type Repository = {
  name: string;
  namespace: string;
  type: RepositoryType;
  url: string;
  hostRef?: string;
  path?: string;
};

export type RepositoryStatus = {
  ok: boolean;
  message?: string;
};

export type SnapshotSummary = {
  total_bytes_processed: number;
};

export type Snapshot = {
  id: string;
  time: string;
  hostname: string;
  tags: string[] | null;
  paths: string[] | null;
  summary?: SnapshotSummary | null;
};

export type CreateRepositoryRequest = {
  name: string;
  namespace: string;
  hostRef: string;
  path: string;
  resticPassword: string;
};

export type UpdateRepositoryRequest = {
  hostRef: string;
  path: string;
  resticPassword: string;
};

async function listRepositories(): Promise<Repository[]> {
  const response = await api.get<Repository[]>('/repositories');
  return response.data;
}

async function getRepository(namespace: string, name: string): Promise<Repository> {
  const response = await api.get<Repository>(`/repositories/${namespace}/${name}`);
  return response.data;
}

async function checkRepository(namespace: string, name: string): Promise<RepositoryStatus> {
  const response = await api.get<RepositoryStatus>(`/repositories/${namespace}/${name}/check`);
  return response.data;
}

async function listSnapshots(namespace: string, name: string): Promise<Snapshot[]> {
  const response = await api.get<Snapshot[]>(`/repositories/${namespace}/${name}/snapshots`);
  return response.data;
}

export type SnapshotEntry = {
  name: string;
  type: 'dir' | 'file';
  path: string;
  size: number;
  mtime: string;
};

export type SnapshotBrowseResponse = {
  snapshotId: string;
  path: string;
  entries: SnapshotEntry[];
};

async function browseSnapshot(namespace: string, name: string, snapshotId: string, path: string): Promise<SnapshotBrowseResponse> {
  const response = await api.get<SnapshotBrowseResponse>(`/repositories/${namespace}/${name}/snapshots/${snapshotId}/browse`, {
    params: { path },
  });
  return response.data;
}

async function createRepository(req: CreateRepositoryRequest): Promise<Repository> {
  const response = await api.post<Repository>('/repositories', req);
  return response.data;
}

async function updateRepository(namespace: string, name: string, req: UpdateRepositoryRequest): Promise<Repository> {
  const response = await api.put<Repository>(`/repositories/${namespace}/${name}`, req);
  return response.data;
}

async function deleteRepository(namespace: string, name: string): Promise<void> {
  await api.delete(`/repositories/${namespace}/${name}`);
}

export default {
  listRepositories,
  getRepository,
  checkRepository,
  listSnapshots,
  browseSnapshot,
  createRepository,
  updateRepository,
  deleteRepository,
};
