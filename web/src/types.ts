export interface User {
  id: number;
  oidc_sub: string;
  email: string;
  display_name: string;
  role: 'admin' | 'user';
  max_peers_override: number | null;
  effective_max_peers: number;
  peer_count: number;
  created_at: string;
}

export interface AuthState {
  authenticated: boolean;
  user: User | null;
  oidc_enabled: boolean;
  dev_auth_enabled: boolean;
  routeros_mock: boolean;
  routeros_url: string;
}

export interface WGNetwork {
  id: number;
  name: string;
  description: string;
  ros_interface: string;
  server_public_key: string;
  subnet_cidr: string;
  server_ip: string;
  endpoint_host: string;
  endpoint_port: number;
  dns_servers: string;
  allowed_ips: string;
  persistent_keepalive: number;
  enabled: boolean;
  allocated_count: number;
  subnet_capacity: number;
  created_at: string;
}

export interface WGPeer {
  id: number;
  user_id: number;
  user_email?: string;
  user_name?: string;
  network_id: number;
  network_name: string;
  ros_interface: string;
  ros_peer_id: string;
  name: string;
  allocated_ip: string;
  public_key: string;
  key_mode: 'server' | 'custom';
  created_at: string;
  last_handshake?: string;
  rx_bytes: number;
  tx_bytes: number;
  online: boolean;
}

export interface PeerProvisionResponse {
  peer: WGPeer;
  config_text: string;
  qr_supported: boolean;
  private_key_included: boolean;
}

export interface ROSInterface {
  '.id': string;
  name: string;
  'public-key': string;
  'listen-port': number;
  mtu: number;
  running: boolean;
  disabled: boolean;
  comment?: string;
}
