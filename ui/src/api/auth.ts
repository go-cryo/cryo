import { api } from 'src/boot/axios';

export type AuthInfo = {
  basicEnabled: boolean;
  oidcEnabled: boolean;
  oidcLoginUrl?: string;
};

export type LoginRequest = {
  identifier: string;
  password: string;
};

const BASE = '/auth';

async function getAuthInfo(): Promise<AuthInfo> {
  const response = await api.get<AuthInfo>(`${BASE}/info`);
  return response.data;
}

async function login(identifier: string, password: string): Promise<void> {
  await api.post(`${BASE}/login`, { identifier, password });
}

async function logout(): Promise<void> {
  await api.post(`${BASE}/logout`);
}

async function me(): Promise<{ id: string; username: string }> {
  const response = await api.get(`${BASE}/me`);
  return response.data;
}

// session is a method-agnostic auth probe (BasicAuth or OIDC). It is protected
// by the server's combined auth middleware, so a 2xx means authenticated.
async function session(): Promise<void> {
  await api.get(`${BASE}/session`);
}

export default {
  getAuthInfo,
  login,
  logout,
  me,
  session,
};
