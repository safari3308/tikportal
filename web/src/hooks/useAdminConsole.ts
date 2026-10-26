import { useCallback, useState } from 'react';
import { api } from '../api';
import type { ROSInterface, User, WGNetwork, WGPeer } from '../types';

export interface NetworkFormValues {
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
}

interface UseAdminConsoleParams {
  onError: (msg: string | null) => void;
  onNotice: (msg: string | null) => void;
  onUserDashboardRefresh: () => Promise<void>;
}

export function useAdminConsole({
  onError,
  onNotice,
  onUserDashboardRefresh,
}: UseAdminConsoleParams) {
  const [adminSection, setAdminSection] = useState<'networks' | 'users' | 'peers'>('networks');
  const [adminNetworks, setAdminNetworks] = useState<WGNetwork[]>([]);
  const [rosInterfaces, setRosInterfaces] = useState<ROSInterface[]>([]);
  const [rosInterfaceTag, setRosInterfaceTag] = useState<string>('tikportal');
  const [adminUsers, setAdminUsers] = useState<User[]>([]);
  const [defaultMaxPeers, setDefaultMaxPeers] = useState<number>(5);
  const [adminAllPeers, setAdminAllPeers] = useState<WGPeer[]>([]);

  // Admin Network Modal
  const [netModalOpen, setNetModalOpen] = useState(false);
  const [editingNetwork, setEditingNetwork] = useState<WGNetwork | null>(null);
  const [netForm, setNetForm] = useState<NetworkFormValues>({
    name: '',
    description: '',
    ros_interface: '',
    server_public_key: '',
    subnet_cidr: '10.10.0.0/24',
    server_ip: '10.10.0.1',
    endpoint_host: 'vpn.example.com',
    endpoint_port: 51820,
    dns_servers: '1.1.1.1, 8.8.8.8',
    allowed_ips: '0.0.0.0/0',
    persistent_keepalive: 25,
    enabled: true,
  });

  const refreshAdminData = useCallback(async () => {
    try {
      const [nets, usersRes, peersRes, rosRes] = await Promise.all([
        api.adminListNetworks(),
        api.adminListUsers(),
        api.adminListAllPeers(),
        api
          .adminGetROSInterfaces()
          .catch(() => ({ interfaces: [], mock: false, interface_tag: undefined })),
      ]);
      setAdminNetworks(nets.networks || []);
      setAdminUsers(usersRes.users || []);
      setDefaultMaxPeers(usersRes.default_max_peers_per_user ?? 5);
      setAdminAllPeers(peersRes.peers || []);
      setRosInterfaces(rosRes.interfaces || []);
      if (rosRes.interface_tag !== undefined) {
        setRosInterfaceTag(rosRes.interface_tag);
      }
    } catch (err: any) {
      onError(err.message);
    }
  }, [onError]);

  const openNewNetworkModal = useCallback((rosIface?: ROSInterface) => {
    setEditingNetwork(null);
    const defaultSubnet = '10.10.0.0/24';
    const defaultGw = '10.10.0.1';
    setNetForm({
      name: rosIface ? `${rosIface.name}` : '',
      description: rosIface?.comment || '',
      ros_interface: rosIface ? rosIface.name : '',
      server_public_key: rosIface ? rosIface['public-key'] : '',
      subnet_cidr: defaultSubnet,
      server_ip: defaultGw,
      endpoint_host: 'vpn.example.com',
      endpoint_port: rosIface?.['listen-port'] || 51820,
      dns_servers: defaultGw,
      allowed_ips: '0.0.0.0/0',
      persistent_keepalive: 25,
      enabled: true,
    });
    setNetModalOpen(true);
  }, []);

  const openEditNetworkModal = useCallback((net: WGNetwork) => {
    setEditingNetwork(net);
    setNetForm({
      name: net.name,
      description: net.description,
      ros_interface: net.ros_interface,
      server_public_key: net.server_public_key,
      subnet_cidr: net.subnet_cidr,
      server_ip: net.server_ip,
      endpoint_host: net.endpoint_host,
      endpoint_port: net.endpoint_port,
      dns_servers: net.dns_servers,
      allowed_ips: net.allowed_ips,
      persistent_keepalive: net.persistent_keepalive,
      enabled: net.enabled,
    });
    setNetModalOpen(true);
  }, []);

  const saveNetwork = useCallback(async () => {
    onError(null);
    try {
      if (editingNetwork) {
        await api.adminUpdateNetwork(editingNetwork.id, netForm);
        onNotice(`Updated WireGuard network "${netForm.name}"`);
      } else {
        await api.adminCreateNetwork(netForm);
        onNotice(`Created WireGuard network "${netForm.name}"`);
      }
      setNetModalOpen(false);
      await Promise.all([onUserDashboardRefresh(), refreshAdminData()]);
    } catch (err: any) {
      onError(err.message);
    }
  }, [editingNetwork, netForm, onError, onNotice, onUserDashboardRefresh, refreshAdminData]);

  const deleteNetwork = useCallback(
    async (net: WGNetwork) => {
      if (
        !window.confirm(
          `Delete network "${net.name}" (${net.subnet_cidr}) and revoke its ${net.allocated_count} active peers?`
        )
      ) {
        return;
      }
      try {
        await api.adminDeleteNetwork(net.id);
        onNotice(`Deleted WireGuard network "${net.name}"`);
        await Promise.all([onUserDashboardRefresh(), refreshAdminData()]);
      } catch (err: any) {
        onError(err.message);
      }
    },
    [onError, onNotice, onUserDashboardRefresh, refreshAdminData]
  );

  const saveGlobalMaxPeers = useCallback(async () => {
    try {
      await api.adminUpdateSettings(defaultMaxPeers);
      onNotice(`Updated global default limit to ${defaultMaxPeers} peers per user.`);
      await Promise.all([onUserDashboardRefresh(), refreshAdminData()]);
    } catch (err: any) {
      onError(err.message);
    }
  }, [defaultMaxPeers, onError, onNotice, onUserDashboardRefresh, refreshAdminData]);

  const updateUserQuota = useCallback(
    async (u: User, newRole: 'admin' | 'user', overrideStr: string) => {
      const trimmed = overrideStr.trim();
      const parsed = trimmed === '' ? null : parseInt(trimmed, 10);
      try {
        await api.adminUpdateUser(u.id, newRole, Number.isNaN(parsed as number) ? null : parsed);
        onNotice(`Updated quota/role for ${u.email}`);
        await Promise.all([onUserDashboardRefresh(), refreshAdminData()]);
      } catch (err: any) {
        onError(err.message);
      }
    },
    [onError, onNotice, onUserDashboardRefresh, refreshAdminData]
  );

  return {
    adminSection,
    setAdminSection,
    adminNetworks,
    rosInterfaces,
    rosInterfaceTag,
    adminUsers,
    defaultMaxPeers,
    setDefaultMaxPeers,
    adminAllPeers,
    refreshAdminData,
    // Network modal
    netModalOpen,
    setNetModalOpen,
    editingNetwork,
    netForm,
    setNetForm,
    openNewNetworkModal,
    openEditNetworkModal,
    saveNetwork,
    deleteNetwork,
    // Quotas
    saveGlobalMaxPeers,
    updateUserQuota,
  };
}
