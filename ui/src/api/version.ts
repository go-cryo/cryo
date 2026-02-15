import { api } from 'src/boot/axios';

export type VersionResponse = {
  version: string;
};

async function getVersion(): Promise<VersionResponse> {
  const response = await api.get<VersionResponse>('/version');
  return response.data;
}

export default {
  getVersion,
};
