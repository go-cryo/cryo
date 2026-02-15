import { api } from 'src/boot/axios';

export type RetentionPolicy = {
  keepLast?: number;
  keepDaily?: number;
  keepWeekly?: number;
  keepMonthly?: number;
};

export type Settings = {
  defaultStorageClassName: string;
  defaultRetention?: RetentionPolicy;
  jobTTLSeconds: number;
};

export type UpdateSettingsRequest = {
  defaultStorageClassName?: string;
  defaultRetention?: RetentionPolicy;
  jobTTLSeconds?: number;
};

async function getSettings(): Promise<Settings> {
  const response = await api.get<Settings>('/settings');
  return response.data;
}

async function updateSettings(req: UpdateSettingsRequest): Promise<Settings> {
  const response = await api.put<Settings>('/settings', req);
  return response.data;
}

export default {
  getSettings,
  updateSettings,
};
