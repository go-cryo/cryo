import { api } from 'src/boot/axios';

export type HostType = 's3' | 'local' | 'sftp' | 'rest';

export type RepositoryHost = {
  name: string;
  namespace: string;
  type: HostType;
  baseUrl: string;
};

export type CreateHostRequest = {
  name: string;
  namespace: string;
  baseUrl: string;
  awsAccessKeyId?: string;
  awsSecretAccessKey?: string;
  awsDefaultRegion?: string;
};

export type UpdateHostRequest = {
  baseUrl: string;
  awsAccessKeyId?: string;
  awsSecretAccessKey?: string;
  awsDefaultRegion?: string;
};

async function listHosts(): Promise<RepositoryHost[]> {
  const response = await api.get<RepositoryHost[]>('/hosts');
  return response.data;
}

async function getHost(namespace: string, name: string): Promise<RepositoryHost> {
  const response = await api.get<RepositoryHost>(`/hosts/${namespace}/${name}`);
  return response.data;
}

async function createHost(req: CreateHostRequest): Promise<RepositoryHost> {
  const response = await api.post<RepositoryHost>('/hosts', req);
  return response.data;
}

async function updateHost(namespace: string, name: string, req: UpdateHostRequest): Promise<RepositoryHost> {
  const response = await api.put<RepositoryHost>(`/hosts/${namespace}/${name}`, req);
  return response.data;
}

async function deleteHost(namespace: string, name: string): Promise<void> {
  await api.delete(`/hosts/${namespace}/${name}`);
}

export default {
  listHosts,
  getHost,
  createHost,
  updateHost,
  deleteHost,
};
