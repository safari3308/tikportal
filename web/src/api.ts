import type {
  AuthState,
  PeerProvisionResponse,
  ROSInterface,
  User,
  WGNetwork,
  WGPeer,
} from './types';

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    headers: {
      'Content-Type': 'application/json',
      ...(init?.headers || {}),
    },
    ...init,
  });

  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error(data.error || `Request failed with status ${res.status}`);
  }
  return data as T;
}

export const api = {
  getAuthMe: () => request<AuthState>('/api/auth/me'),

  devLogin: (email: string, displayName: string) =>
    request<{ user: User }>('/api/auth/dev-login', {
      method: 'POST',
      body: JSON.stringify({ email, display_name: displayName }),
    }),

  logout: () =>
    request<{ ok: boolean }>('/api/auth/logout', {
      method: 'POST',
    }),

  listNetworks: () => request<{ networks: WGNetwork[] }>('/api/networks'),

  listMyPeers: () =>
    request<{ peers: WGPeer[]; quota: { used: number; limit: number } }>('/api/peers'),

  createPeer: (payload: { network_id: number; name: string; public_key?: string }) =>
    request<PeerProvisionResponse>('/api/peers', {
      method: 'POST',
      body: JSON.stringify(payload),
    }),

  regeneratePeerKeys: (peerId: number, publicKey?: string) =>
    request<PeerProvisionResponse>(`/api/peers/${peerId}/regenerate`, {
      method: 'POST',
      body: JSON.stringify({ public_key: publicKey || '' }),
    }),

  getPeerConfigTemplate: (peerId: number) =>
    request<{ peer: WGPeer; config_text: string }>(`/api/peers/${peerId}/config`),

  deletePeer: (peerId: number) =>
    request<{ deleted: boolean; released_ip: string }>(`/api/peers/${peerId}`, {
      method: 'DELETE',
    }),

  // Admin API
  adminGetROSInterfaces: () =>
    request<{
      interfaces: ROSInterface[];
      mock: boolean;
      interface_tag?: string;
      allowed_interfaces?: string[];
    }>('/api/admin/routeros/interfaces'),

  adminListNetworks: () => request<{ networks: WGNetwork[] }>('/api/admin/networks'),

  adminCreateNetwork: (net: Partial<WGNetwork>) =>
    request<{ network: WGNetwork }>('/api/admin/networks', {
      method: 'POST',
      body: JSON.stringify(net),
    }),

  adminUpdateNetwork: (id: number, net: Partial<WGNetwork>) =>
    request<{ network: WGNetwork }>(`/api/admin/networks/${id}`, {
      method: 'PUT',
      body: JSON.stringify(net),
    }),

  adminDeleteNetwork: (id: number) =>
    request<{ deleted: boolean }>(`/api/admin/networks/${id}`, {
      method: 'DELETE',
    }),

  adminListUsers: () =>
    request<{ users: User[]; default_max_peers_per_user: number }>('/api/admin/users'),

  adminUpdateUser: (id: number, role: 'admin' | 'user', maxPeersOverride: number | null) =>
    request<{ user: User }>(`/api/admin/users/${id}`, {
      method: 'PUT',
      body: JSON.stringify({ role, max_peers_override: maxPeersOverride }),
    }),

  adminListAllPeers: () => request<{ peers: WGPeer[] }>('/api/admin/peers'),

  adminUpdateSettings: (defaultMaxPeersPerUser: number) =>
    request<{ default_max_peers_per_user: number }>('/api/admin/settings', {
      method: 'PUT',
      body: JSON.stringify({ default_max_peers_per_user: defaultMaxPeersPerUser }),
    }),
};
