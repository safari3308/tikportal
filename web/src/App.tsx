import React, { useEffect, useState } from 'react';
import { QRCodeSVG } from 'qrcode.react';
import {
  Activity,
  Check,
  Copy,
  Download,
  KeyRound,
  LogOut,
  Network,
  Plus,
  QrCode,
  RefreshCw,
  Router,
  Settings,
  Shield,
  Trash2,
  Users,
  Wifi,
} from 'lucide-react';
import { api } from './api';
import { ModalDialog } from './components/ModalDialog';
import type {
  AuthState,
  PeerProvisionResponse,
  ROSInterface,
  User,
  WGNetwork,
  WGPeer,
} from './types';

function formatBytes(bytes: number): string {
  if (!bytes || bytes <= 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  return `${(bytes / Math.pow(1024, i)).toFixed(i === 0 ? 0 : 1)} ${units[i]}`;
}

function downloadConfFile(filename: string, content: string) {
  const blob = new Blob([content], { type: 'text/plain;charset=utf-8' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}

export function App() {
  const [auth, setAuth] = useState<AuthState | null>(null);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState<'user' | 'admin'>('user');
  const [adminSection, setAdminSection] = useState<'networks' | 'users' | 'peers'>('networks');
  const [errorBanner, setErrorBanner] = useState<string | null>(null);
  const [noticeBanner, setNoticeBanner] = useState<string | null>(null);

  // Login form state
  const [devEmail, setDevEmail] = useState('admin@localhost');
  const [devName, setDevName] = useState('Network Admin');

  // User dashboard state
  const [networks, setNetworks] = useState<WGNetwork[]>([]);
  const [myPeers, setMyPeers] = useState<WGPeer[]>([]);
  const [quota, setQuota] = useState<{ used: number; limit: number }>({ used: 0, limit: 5 });

  // Create Peer Modal state
  const [createModalOpen, setCreateModalOpen] = useState(false);
  const [selectedNetId, setSelectedNetId] = useState<number>(0);
  const [peerName, setPeerName] = useState('');
  const [keyModeChoice, setKeyModeChoice] = useState<'server' | 'custom'>('server');
  const [customPublicKey, setCustomPublicKey] = useState('');
  const [submittingPeer, setSubmittingPeer] = useState(false);

  // Regenerate Keys Modal state
  const [regenTarget, setRegenTarget] = useState<WGPeer | null>(null);
  const [regenModeChoice, setRegenModeChoice] = useState<'server' | 'custom'>('server');
  const [regenCustomPubKey, setRegenCustomPubKey] = useState('');
  const [submittingRegen, setSubmittingRegen] = useState(false);

  // Provisioned Result Modal (QR Code + .conf)
  const [provisionResult, setProvisionResult] = useState<PeerProvisionResponse | null>(null);
  const [copiedConfig, setCopiedConfig] = useState(false);

  // Admin state
  const [adminNetworks, setAdminNetworks] = useState<WGNetwork[]>([]);
  const [rosInterfaces, setRosInterfaces] = useState<ROSInterface[]>([]);
  const [rosInterfaceTag, setRosInterfaceTag] = useState<string>('tikportal');
  const [adminUsers, setAdminUsers] = useState<User[]>([]);
  const [defaultMaxPeers, setDefaultMaxPeers] = useState<number>(5);
  const [adminAllPeers, setAdminAllPeers] = useState<WGPeer[]>([]);

  // Admin Network Modal
  const [netModalOpen, setNetModalOpen] = useState(false);
  const [editingNetwork, setEditingNetwork] = useState<WGNetwork | null>(null);
  const [netForm, setNetForm] = useState({
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

  const loadInitial = async () => {
    setLoading(true);
    try {
      const me = await api.getAuthMe();
      setAuth(me);
      if (me.authenticated) {
        await refreshUserDashboard();
        if (me.user?.role === 'admin') {
          await refreshAdminData();
        }
      }
    } catch (err: any) {
      setErrorBanner(err.message || 'Failed to load application state');
    } finally {
      setLoading(false);
    }
  };

  const refreshUserDashboard = async () => {
    const [netRes, peerRes] = await Promise.all([api.listNetworks(), api.listMyPeers()]);
    const netList = netRes.networks || [];
    setNetworks(netList);
    setMyPeers(peerRes.peers || []);
    setQuota(peerRes.quota || { used: 0, limit: 5 });
    if (netList.length > 0 && !selectedNetId) {
      setSelectedNetId(netList[0].id);
    }
  };

  const refreshAdminData = async () => {
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
      setErrorBanner(err.message);
    }
  };

  useEffect(() => {
    void loadInitial();
  }, []);

  const handleDevLoginSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setErrorBanner(null);
    try {
      await api.devLogin(devEmail, devName);
      await loadInitial();
    } catch (err: any) {
      setErrorBanner(err.message);
    }
  };

  const handleLogout = async () => {
    await api.logout();
    setAuth((prev) => (prev ? { ...prev, authenticated: false, user: null } : null));
    setActiveTab('user');
  };

  const openCreatePeerModal = (networkId?: number) => {
    setErrorBanner(null);
    if (networkId) {
      setSelectedNetId(networkId);
    } else if (networks.length > 0 && !selectedNetId) {
      setSelectedNetId(networks[0].id);
    }
    setPeerName('');
    setKeyModeChoice('server');
    setCustomPublicKey('');
    setCreateModalOpen(true);
  };

  const handleCreatePeer = async (e: React.FormEvent) => {
    e.preventDefault();
    setErrorBanner(null);
    setSubmittingPeer(true);
    try {
      const res = await api.createPeer({
        network_id: selectedNetId,
        name: peerName,
        public_key: keyModeChoice === 'custom' ? customPublicKey : '',
      });
      setCreateModalOpen(false);
      setProvisionResult(res);
      await refreshUserDashboard();
      if (auth?.user?.role === 'admin') {
        await refreshAdminData();
      }
    } catch (err: any) {
      setErrorBanner(err.message);
    } finally {
      setSubmittingPeer(false);
    }
  };

  const openRegenerateModal = (peer: WGPeer) => {
    setErrorBanner(null);
    setRegenTarget(peer);
    setRegenModeChoice('server');
    setRegenCustomPubKey('');
  };

  const handleRegenerateKeys = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!regenTarget) return;
    setErrorBanner(null);
    setSubmittingRegen(true);
    try {
      const res = await api.regeneratePeerKeys(
        regenTarget.id,
        regenModeChoice === 'custom' ? regenCustomPubKey : ''
      );
      setRegenTarget(null);
      setProvisionResult(res);
      setNoticeBanner(
        `Keys regenerated in-place for "${res.peer.name}" — IP ${res.peer.allocated_ip} preserved.`
      );
      await refreshUserDashboard();
      if (auth?.user?.role === 'admin') {
        await refreshAdminData();
      }
    } catch (err: any) {
      setErrorBanner(err.message);
    } finally {
      setSubmittingRegen(false);
    }
  };

  const handleDownloadTemplate = async (peer: WGPeer) => {
    try {
      const res = await api.getPeerConfigTemplate(peer.id);
      const safeName = peer.name.toLowerCase().replace(/[^a-z0-9-_]/g, '-') || 'wg-peer';
      downloadConfFile(`${safeName}-${peer.allocated_ip}.conf`, res.config_text);
    } catch (err: any) {
      setErrorBanner(err.message);
    }
  };

  const handleDeletePeer = async (peer: WGPeer) => {
    if (
      !window.confirm(
        `Delete VPN connection "${peer.name}" (${peer.allocated_ip})?\nIts IP address (${peer.allocated_ip}) will be released for reuse.`
      )
    ) {
      return;
    }
    setErrorBanner(null);
    try {
      const res = await api.deletePeer(peer.id);
      setNoticeBanner(
        `Removed "${peer.name}" from RouterOS. IP ${res.released_ip} is now free for reuse.`
      );
      await refreshUserDashboard();
      if (auth?.user?.role === 'admin') {
        await refreshAdminData();
      }
    } catch (err: any) {
      setErrorBanner(err.message);
    }
  };

  const openNewNetworkModal = (rosIface?: ROSInterface) => {
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
  };

  const openEditNetworkModal = (net: WGNetwork) => {
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
  };

  const handleSaveNetwork = async (e: React.FormEvent) => {
    e.preventDefault();
    setErrorBanner(null);
    try {
      if (editingNetwork) {
        await api.adminUpdateNetwork(editingNetwork.id, netForm);
        setNoticeBanner(`Updated WireGuard network "${netForm.name}"`);
      } else {
        await api.adminCreateNetwork(netForm);
        setNoticeBanner(`Created WireGuard network "${netForm.name}"`);
      }
      setNetModalOpen(false);
      await Promise.all([refreshUserDashboard(), refreshAdminData()]);
    } catch (err: any) {
      setErrorBanner(err.message);
    }
  };

  const handleDeleteNetwork = async (net: WGNetwork) => {
    if (
      !window.confirm(
        `Delete network "${net.name}" (${net.subnet_cidr}) and revoke its ${net.allocated_count} active peers?`
      )
    ) {
      return;
    }
    try {
      await api.adminDeleteNetwork(net.id);
      setNoticeBanner(`Deleted WireGuard network "${net.name}"`);
      await Promise.all([refreshUserDashboard(), refreshAdminData()]);
    } catch (err: any) {
      setErrorBanner(err.message);
    }
  };

  const handleSaveGlobalMaxPeers = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      await api.adminUpdateSettings(defaultMaxPeers);
      setNoticeBanner(`Updated global default limit to ${defaultMaxPeers} peers per user.`);
      await Promise.all([refreshUserDashboard(), refreshAdminData()]);
    } catch (err: any) {
      setErrorBanner(err.message);
    }
  };

  const handleUpdateUserQuota = async (
    u: User,
    newRole: 'admin' | 'user',
    overrideStr: string
  ) => {
    const trimmed = overrideStr.trim();
    const parsed = trimmed === '' ? null : parseInt(trimmed, 10);
    try {
      await api.adminUpdateUser(u.id, newRole, Number.isNaN(parsed as number) ? null : parsed);
      setNoticeBanner(`Updated quota/role for ${u.email}`);
      await Promise.all([refreshUserDashboard(), refreshAdminData()]);
    } catch (err: any) {
      setErrorBanner(err.message);
    }
  };

  if (loading) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-slate-950 text-slate-300">
        <div className="flex items-center gap-3">
          <RefreshCw className="h-5 w-5 animate-spin text-cyan-400" />
          <span>Loading TikPortal...</span>
        </div>
      </div>
    );
  }

  // Login View
  if (!auth?.authenticated || !auth.user) {
    return (
      <div className="flex min-h-screen flex-col justify-center bg-slate-950 px-4 py-12">
        <div className="mx-auto w-full max-w-md rounded-2xl border border-slate-800 bg-slate-900/90 p-8 shadow-2xl">
          <div className="flex items-center gap-3">
            <div className="flex h-11 w-11 items-center justify-center rounded-xl bg-cyan-500/10 text-cyan-400 ring-1 ring-cyan-500/30">
              <Router className="h-6 w-6" />
            </div>
            <div>
              <h1 className="text-xl font-bold tracking-tight text-white">TikPortal</h1>
              <p className="text-xs text-slate-400">
                MikroTik RB5009 WireGuard Self-Service Portal
              </p>
            </div>
          </div>

          {errorBanner && (
            <div className="mt-4 rounded-xl border border-rose-500/30 bg-rose-500/10 px-4 py-3 text-xs text-rose-300">
              {errorBanner}
            </div>
          )}

          {auth?.oidc_enabled && (
            <div className="mt-6">
              <a
                href="/api/auth/login"
                className="flex w-full items-center justify-center gap-2 rounded-xl bg-cyan-500 px-4 py-2.5 text-sm font-semibold text-slate-950 transition hover:bg-cyan-400"
              >
                <Shield className="h-4 w-4" />
                Sign in with SSO (OIDC)
              </a>
            </div>
          )}

          {auth?.dev_auth_enabled && (
            <form onSubmit={handleDevLoginSubmit} className="mt-6 space-y-4">
              {auth?.oidc_enabled && (
                <div className="relative flex items-center py-1">
                  <div className="grow border-t border-slate-800" />
                  <span className="mx-3 shrink text-xs text-slate-500">or Dev Login</span>
                  <div className="grow border-t border-slate-800" />
                </div>
              )}

              <div>
                <label className="block text-xs font-medium text-slate-300">Email Address</label>
                <input
                  type="email"
                  required
                  value={devEmail}
                  onChange={(e) => setDevEmail(e.target.value)}
                  className="mt-1 w-full rounded-xl border border-slate-700 bg-slate-950 px-3.5 py-2 text-sm text-white focus:border-cyan-500 focus:outline-none"
                  placeholder="admin@localhost"
                />
              </div>

              <div>
                <label className="block text-xs font-medium text-slate-300">Display Name</label>
                <input
                  type="text"
                  required
                  value={devName}
                  onChange={(e) => setDevName(e.target.value)}
                  className="mt-1 w-full rounded-xl border border-slate-700 bg-slate-950 px-3.5 py-2 text-sm text-white focus:border-cyan-500 focus:outline-none"
                  placeholder="Network Admin"
                />
              </div>

              <div className="flex gap-2 pt-1">
                <button
                  type="button"
                  onClick={() => {
                    setDevEmail('admin@localhost');
                    setDevName('Network Admin');
                  }}
                  className="flex-1 rounded-lg border border-slate-800 bg-slate-950 px-2.5 py-1.5 text-xs text-slate-400 hover:border-slate-700 hover:text-slate-200"
                >
                  Preset: Admin
                </button>
                <button
                  type="button"
                  onClick={() => {
                    setDevEmail('alice@example.com');
                    setDevName('Alice Nguyen');
                  }}
                  className="flex-1 rounded-lg border border-slate-800 bg-slate-950 px-2.5 py-1.5 text-xs text-slate-400 hover:border-slate-700 hover:text-slate-200"
                >
                  Preset: Standard User
                </button>
              </div>

              <button
                type="submit"
                className="w-full rounded-xl bg-cyan-500 px-4 py-2.5 text-sm font-semibold text-slate-950 transition hover:bg-cyan-400"
              >
                Continue to TikPortal
              </button>
            </form>
          )}
        </div>
      </div>
    );
  }

  const quotaReached = quota.used >= quota.limit;

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100">
      {/* Header */}
      <header className="border-b border-slate-800/80 bg-slate-900/60 backdrop-blur">
        <div className="mx-auto flex max-w-7xl flex-wrap items-center justify-between gap-4 px-6 py-4">
          <div className="flex items-center gap-3">
            <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-cyan-500/10 text-cyan-400 ring-1 ring-cyan-500/30">
              <Router className="h-5 w-5" />
            </div>
            <div>
              <div className="flex items-center gap-2.5">
                <span className="text-lg font-bold tracking-tight text-white">TikPortal</span>
                <span
                  className={`inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-medium ${
                    auth.routeros_mock
                      ? 'bg-amber-500/10 text-amber-300 ring-1 ring-amber-500/30'
                      : 'bg-emerald-500/10 text-emerald-300 ring-1 ring-emerald-500/30'
                  }`}
                >
                  <span className="h-1.5 w-1.5 rounded-full bg-current" />
                  {auth.routeros_mock ? 'RB5009 Mock Mode' : 'RB5009 REST Connected'}
                </span>
              </div>
              <p className="text-xs text-slate-400">
                MikroTik RouterOS WireGuard Self-Service Portal
              </p>
            </div>
          </div>

          <div className="flex items-center gap-3">
            {auth.user.role === 'admin' && (
              <div className="flex rounded-xl border border-slate-800 bg-slate-950 p-1">
                <button
                  type="button"
                  onClick={() => setActiveTab('user')}
                  className={`flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs font-medium transition ${
                    activeTab === 'user'
                      ? 'bg-cyan-500 text-slate-950'
                      : 'text-slate-400 hover:text-white'
                  }`}
                >
                  <Wifi className="h-3.5 w-3.5" />
                  My VPNs
                </button>
                <button
                  type="button"
                  onClick={() => {
                    setActiveTab('admin');
                    void refreshAdminData();
                  }}
                  className={`flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs font-medium transition ${
                    activeTab === 'admin'
                      ? 'bg-cyan-500 text-slate-950'
                      : 'text-slate-400 hover:text-white'
                  }`}
                >
                  <Settings className="h-3.5 w-3.5" />
                  Admin Console
                </button>
              </div>
            )}

            <div className="hidden items-center gap-2 rounded-xl border border-slate-800 bg-slate-950 px-3 py-1.5 text-xs sm:flex">
              <span className="font-medium text-slate-200">{auth.user.display_name}</span>
              <span className="text-slate-500">({auth.user.email})</span>
              <span className="rounded bg-slate-800 px-1.5 py-0.5 text-[10px] font-semibold uppercase text-cyan-300">
                {auth.user.role}
              </span>
            </div>

            <button
              type="button"
              onClick={handleLogout}
              className="inline-flex items-center gap-1.5 rounded-xl border border-slate-800 bg-slate-900 px-3 py-2 text-xs font-medium text-slate-300 transition hover:border-slate-700 hover:text-white"
              title="Sign out"
            >
              <LogOut className="h-4 w-4" />
              <span className="hidden sm:inline">Sign Out</span>
            </button>
          </div>
        </div>
      </header>

      {/* Main Content */}
      <main className="mx-auto max-w-7xl px-6 py-8">
        {errorBanner && (
          <div className="mb-6 flex items-center justify-between rounded-xl border border-rose-500/30 bg-rose-500/10 px-4 py-3 text-sm text-rose-200">
            <span>{errorBanner}</span>
            <button
              type="button"
              onClick={() => setErrorBanner(null)}
              className="text-xs underline"
            >
              Dismiss
            </button>
          </div>
        )}

        {noticeBanner && (
          <div className="mb-6 flex items-center justify-between rounded-xl border border-emerald-500/30 bg-emerald-500/10 px-4 py-3 text-sm text-emerald-200">
            <span>{noticeBanner}</span>
            <button
              type="button"
              onClick={() => setNoticeBanner(null)}
              className="text-xs underline"
            >
              Dismiss
            </button>
          </div>
        )}

        {activeTab === 'user' ? (
          <div className="space-y-8">
            {/* Quota & Quick Action Bar */}
            <div className="flex flex-wrap items-center justify-between gap-4 rounded-2xl border border-slate-800 bg-slate-900/70 p-6">
              <div className="space-y-1">
                <div className="flex items-center gap-2">
                  <h2 className="text-base font-semibold text-white">
                    Global Connection Quota
                  </h2>
                  <span className="rounded-full bg-cyan-500/10 px-2.5 py-0.5 text-xs font-semibold text-cyan-300 ring-1 ring-cyan-500/30">
                    {quota.used} / {quota.limit} Peers Used
                  </span>
                </div>
                <p className="text-xs text-slate-400">
                  Your limit applies across all WireGuard networks. Removing any connection immediately frees its IP and quota slot.
                </p>
              </div>

              <div className="flex items-center gap-3">
                <button
                  type="button"
                  onClick={() => void refreshUserDashboard()}
                  className="inline-flex items-center gap-1.5 rounded-xl border border-slate-800 bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-300 transition hover:border-slate-700 hover:text-white"
                >
                  <RefreshCw className="h-3.5 w-3.5" />
                  Refresh Stats
                </button>
                <button
                  type="button"
                  disabled={quotaReached || networks.length === 0}
                  onClick={() => openCreatePeerModal()}
                  className="inline-flex items-center gap-2 rounded-xl bg-cyan-500 px-4 py-2 text-xs font-semibold text-slate-950 transition hover:bg-cyan-400 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  <Plus className="h-4 w-4" />
                  New VPN Connection
                </button>
              </div>
            </div>

            {/* Available WireGuard Networks */}
            <section>
              <h3 className="mb-3 text-sm font-semibold uppercase tracking-wider text-slate-400">
                Available RB5009 WireGuard Networks
              </h3>
              <div className="grid gap-4 md:grid-cols-2">
                {networks.map((net) => (
                  <div
                    key={net.id}
                    className="flex flex-col justify-between rounded-2xl border border-slate-800 bg-slate-900/60 p-5 transition hover:border-slate-700"
                  >
                    <div>
                      <div className="flex items-start justify-between gap-2">
                        <div>
                          <h4 className="font-semibold text-white">{net.name}</h4>
                          <p className="mt-1 text-xs text-slate-400">{net.description}</p>
                        </div>
                        <span className="rounded-lg border border-slate-700 bg-slate-950 px-2.5 py-1 font-mono text-xs text-cyan-300">
                          {net.ros_interface}
                        </span>
                      </div>
                      <div className="mt-4 flex flex-wrap gap-3 text-xs text-slate-400">
                        <span className="rounded bg-slate-950 px-2 py-1 font-mono">
                          Subnet: {net.subnet_cidr}
                        </span>
                        <span className="rounded bg-slate-950 px-2 py-1 font-mono">
                          Endpoint: {net.endpoint_host}:{net.endpoint_port}
                        </span>
                        <span className="rounded bg-slate-950 px-2 py-1">
                          IPs Used: {net.allocated_count} / {net.subnet_capacity}
                        </span>
                      </div>
                    </div>

                    <div className="mt-4 flex justify-end border-t border-slate-800/80 pt-3">
                      <button
                        type="button"
                        disabled={quotaReached}
                        onClick={() => openCreatePeerModal(net.id)}
                        className="inline-flex items-center gap-1.5 rounded-lg bg-cyan-500/15 px-3 py-1.5 text-xs font-medium text-cyan-300 ring-1 ring-cyan-500/30 transition hover:bg-cyan-500 hover:text-slate-950 disabled:opacity-40"
                      >
                        <Plus className="h-3.5 w-3.5" />
                        Connect to {net.name}
                      </button>
                    </div>
                  </div>
                ))}
              </div>
            </section>

            {/* User's Active Peers */}
            <section>
              <h3 className="mb-3 text-sm font-semibold uppercase tracking-wider text-slate-400">
                My Provisioned Connections ({myPeers.length})
              </h3>

              {myPeers.length === 0 ? (
                <div className="rounded-2xl border border-dashed border-slate-800 bg-slate-900/30 p-10 text-center">
                  <Network className="mx-auto h-8 w-8 text-slate-500" />
                  <p className="mt-3 text-sm font-medium text-slate-300">
                    No WireGuard connections created yet
                  </p>
                  <p className="mt-1 text-xs text-slate-500">
                    Click &ldquo;New VPN Connection&rdquo; above to get a scannable QR code or configuration file.
                  </p>
                </div>
              ) : (
                <div className="grid gap-4 md:grid-cols-2">
                  {myPeers.map((peer) => (
                    <div
                      key={peer.id}
                      className="flex flex-col justify-between rounded-2xl border border-slate-800 bg-slate-900 p-5"
                    >
                      <div>
                        <div className="flex items-start justify-between gap-2">
                          <div>
                            <div className="flex items-center gap-2">
                              <h4 className="text-base font-semibold text-white">{peer.name}</h4>
                              <span
                                className={`inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-medium ${
                                  peer.online
                                    ? 'bg-emerald-500/10 text-emerald-300 ring-1 ring-emerald-500/30'
                                    : 'bg-slate-800 text-slate-400'
                                }`}
                              >
                                <Activity className="h-3 w-3" />
                                {peer.last_handshake ? `Handshake ${peer.last_handshake}` : 'Idle'}
                              </span>
                            </div>
                            <p className="mt-0.5 text-xs text-cyan-400">
                              {peer.network_name} ({peer.ros_interface})
                            </p>
                          </div>

                          <span className="rounded-lg bg-slate-950 px-2.5 py-1 font-mono text-xs font-semibold text-white ring-1 ring-slate-800">
                            {peer.allocated_ip}/32
                          </span>
                        </div>

                        <div className="mt-4 space-y-1.5 rounded-xl bg-slate-950 p-3 text-xs">
                          <div className="flex items-center justify-between gap-2">
                            <span className="text-slate-500">Public Key:</span>
                            <code className="truncate font-mono text-slate-300" title={peer.public_key}>
                              {peer.public_key.slice(0, 20)}...{peer.public_key.slice(-6)}
                            </code>
                          </div>
                          <div className="flex items-center justify-between gap-2">
                            <span className="text-slate-500">Key Mode:</span>
                            <span className="text-slate-300">
                              {peer.key_mode === 'custom'
                                ? 'Mode 2 — User Public Key (.conf only)'
                                : 'Mode 1 — Server Generated (QR + .conf)'}
                            </span>
                          </div>
                          <div className="flex items-center justify-between gap-2">
                            <span className="text-slate-500">Traffic (Rx / Tx):</span>
                            <span className="font-mono text-slate-300">
                              ↓ {formatBytes(peer.rx_bytes)} / ↑ {formatBytes(peer.tx_bytes)}
                            </span>
                          </div>
                        </div>
                      </div>

                      <div className="mt-4 flex flex-wrap items-center justify-between gap-2 border-t border-slate-800 pt-3">
                        <div className="flex flex-wrap gap-2">
                          <button
                            type="button"
                            onClick={() => openRegenerateModal(peer)}
                            className="inline-flex items-center gap-1.5 rounded-lg border border-cyan-500/30 bg-cyan-500/10 px-3 py-1.5 text-xs font-medium text-cyan-300 transition hover:bg-cyan-500 hover:text-slate-950"
                            title="Regenerate or replace keys in-place without losing this IP address"
                          >
                            <KeyRound className="h-3.5 w-3.5" />
                            Regenerate Keys / QR
                          </button>

                          <button
                            type="button"
                            onClick={() => void handleDownloadTemplate(peer)}
                            className="inline-flex items-center gap-1.5 rounded-lg border border-slate-700 bg-slate-950 px-3 py-1.5 text-xs font-medium text-slate-300 transition hover:border-slate-600 hover:text-white"
                            title="Download .conf template (fill in your private key)"
                          >
                            <Download className="h-3.5 w-3.5" />
                            Template .conf
                          </button>
                        </div>

                        <button
                          type="button"
                          onClick={() => void handleDeletePeer(peer)}
                          className="inline-flex items-center gap-1 rounded-lg border border-rose-500/30 bg-rose-500/10 px-2.5 py-1.5 text-xs font-medium text-rose-300 transition hover:bg-rose-500 hover:text-white"
                        >
                          <Trash2 className="h-3.5 w-3.5" />
                          Delete
                        </button>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </section>
          </div>
        ) : (
          /* Admin Console */
          <div className="space-y-6">
            <div className="flex flex-wrap items-center justify-between gap-4 border-b border-slate-800 pb-4">
              <div className="flex gap-2">
                <button
                  type="button"
                  onClick={() => setAdminSection('networks')}
                  className={`inline-flex items-center gap-2 rounded-xl px-4 py-2 text-xs font-semibold transition ${
                    adminSection === 'networks'
                      ? 'bg-cyan-500 text-slate-950'
                      : 'border border-slate-800 bg-slate-900 text-slate-300 hover:text-white'
                  }`}
                >
                  <Network className="h-4 w-4" />
                  WireGuard Networks ({adminNetworks.length})
                </button>
                <button
                  type="button"
                  onClick={() => setAdminSection('users')}
                  className={`inline-flex items-center gap-2 rounded-xl px-4 py-2 text-xs font-semibold transition ${
                    adminSection === 'users'
                      ? 'bg-cyan-500 text-slate-950'
                      : 'border border-slate-800 bg-slate-900 text-slate-300 hover:text-white'
                  }`}
                >
                  <Users className="h-4 w-4" />
                  Users & Quotas ({adminUsers.length})
                </button>
                <button
                  type="button"
                  onClick={() => setAdminSection('peers')}
                  className={`inline-flex items-center gap-2 rounded-xl px-4 py-2 text-xs font-semibold transition ${
                    adminSection === 'peers'
                      ? 'bg-cyan-500 text-slate-950'
                      : 'border border-slate-800 bg-slate-900 text-slate-300 hover:text-white'
                  }`}
                >
                  <Activity className="h-4 w-4" />
                  All Provisioned Peers ({adminAllPeers.length})
                </button>
              </div>
            </div>

            {adminSection === 'networks' && (
              <div className="space-y-6">
                {/* Discovered RB5009 WireGuard Interfaces */}
                <div className="rounded-2xl border border-slate-800 bg-slate-900/60 p-5">
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <div>
                      <div className="flex items-center gap-2">
                        <h3 className="text-sm font-semibold text-white">
                          Discovered RB5009 WireGuard Interfaces (via RouterOS REST API)
                        </h3>
                        {rosInterfaceTag && rosInterfaceTag !== '*' && (
                          <span className="rounded-md bg-cyan-500/10 px-2 py-0.5 font-mono text-[11px] text-cyan-300 ring-1 ring-cyan-500/30">
                            comment ~ &quot;{rosInterfaceTag}&quot;
                          </span>
                        )}
                      </div>
                      <p className="text-xs text-slate-400">
                        Only RouterOS WireGuard interfaces matching your comment tag filter appear here.
                      </p>
                    </div>
                    <button
                      type="button"
                      onClick={() => openNewNetworkModal()}
                      className="inline-flex items-center gap-1.5 rounded-xl bg-cyan-500 px-3.5 py-2 text-xs font-semibold text-slate-950 hover:bg-cyan-400"
                    >
                      <Plus className="h-4 w-4" />
                      Add Network
                    </button>
                  </div>

                  <div className="mt-4 grid gap-3 sm:grid-cols-2">
                    {rosInterfaces.map((iface) => {
                      const alreadyBound = adminNetworks.some(
                        (n) => n.ros_interface === iface.name
                      );
                      return (
                        <div
                          key={iface['.id']}
                          className="flex items-center justify-between rounded-xl border border-slate-800 bg-slate-950 p-3.5 text-xs"
                        >
                          <div>
                            <div className="flex items-center gap-2">
                              <span className="font-mono font-semibold text-cyan-300">
                                {iface.name}
                              </span>
                              <span className="text-slate-500">UDP :{iface['listen-port']}</span>
                            </div>
                            <p className="mt-0.5 truncate font-mono text-[11px] text-slate-400">
                              PubKey: {iface['public-key']}
                            </p>
                          </div>
                          {alreadyBound ? (
                            <span className="rounded-lg bg-emerald-500/10 px-2.5 py-1 text-[11px] font-medium text-emerald-300">
                              Configured
                            </span>
                          ) : (
                            <button
                              type="button"
                              onClick={() => openNewNetworkModal(iface)}
                              className="rounded-lg bg-cyan-500/15 px-2.5 py-1.5 font-medium text-cyan-300 hover:bg-cyan-500 hover:text-slate-950"
                            >
                              Import Interface
                            </button>
                          )}
                        </div>
                      );
                    })}
                  </div>
                </div>

                {/* Configured Networks Table */}
                <div className="grid gap-4 md:grid-cols-2">
                  {adminNetworks.map((net) => (
                    <div
                      key={net.id}
                      className="rounded-2xl border border-slate-800 bg-slate-900 p-5"
                    >
                      <div className="flex items-start justify-between">
                        <div>
                          <div className="flex items-center gap-2">
                            <h4 className="font-semibold text-white">{net.name}</h4>
                            <span
                              className={`rounded-full px-2 py-0.5 text-[10px] font-semibold ${
                                net.enabled
                                  ? 'bg-emerald-500/10 text-emerald-300'
                                  : 'bg-slate-800 text-slate-400'
                              }`}
                            >
                              {net.enabled ? 'ENABLED' : 'DISABLED'}
                            </span>
                          </div>
                          <p className="mt-1 text-xs text-slate-400">{net.description}</p>
                        </div>
                        <span className="rounded bg-slate-950 px-2.5 py-1 font-mono text-xs text-cyan-300">
                          {net.ros_interface}
                        </span>
                      </div>

                      <div className="mt-4 grid grid-cols-2 gap-2 rounded-xl bg-slate-950 p-3 text-xs">
                        <div>
                          <span className="text-slate-500">Subnet CIDR:</span>{' '}
                          <span className="font-mono text-slate-200">{net.subnet_cidr}</span>
                        </div>
                        <div>
                          <span className="text-slate-500">Gateway IP:</span>{' '}
                          <span className="font-mono text-slate-200">{net.server_ip}</span>
                        </div>
                        <div>
                          <span className="text-slate-500">Endpoint:</span>{' '}
                          <span className="font-mono text-slate-200">
                            {net.endpoint_host}:{net.endpoint_port}
                          </span>
                        </div>
                        <div>
                          <span className="text-slate-500">Allocated IPs:</span>{' '}
                          <span className="font-mono text-slate-200">
                            {net.allocated_count} / {net.subnet_capacity}
                          </span>
                        </div>
                      </div>

                      <div className="mt-4 flex justify-end gap-2">
                        <button
                          type="button"
                          onClick={() => openEditNetworkModal(net)}
                          className="rounded-lg border border-slate-700 bg-slate-950 px-3 py-1.5 text-xs font-medium text-slate-200 hover:border-slate-600"
                        >
                          Edit Network
                        </button>
                        <button
                          type="button"
                          onClick={() => void handleDeleteNetwork(net)}
                          className="rounded-lg border border-rose-500/30 bg-rose-500/10 px-3 py-1.5 text-xs font-medium text-rose-300 hover:bg-rose-500 hover:text-white"
                        >
                          Delete
                        </button>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            )}

            {adminSection === 'users' && (
              <div className="space-y-6">
                <form
                  onSubmit={handleSaveGlobalMaxPeers}
                  className="flex flex-wrap items-end justify-between gap-4 rounded-2xl border border-slate-800 bg-slate-900 p-5"
                >
                  <div>
                    <h3 className="text-sm font-semibold text-white">
                      Global Default Max Peers per User
                    </h3>
                    <p className="text-xs text-slate-400">
                      Applies to all users unless overridden individually below (total across all networks).
                    </p>
                  </div>
                  <div className="flex items-center gap-3">
                    <input
                      type="number"
                      min={0}
                      max={1000}
                      value={defaultMaxPeers}
                      onChange={(e) => setDefaultMaxPeers(parseInt(e.target.value || '0', 10))}
                      className="w-24 rounded-xl border border-slate-700 bg-slate-950 px-3 py-2 text-sm text-white"
                    />
                    <button
                      type="submit"
                      className="rounded-xl bg-cyan-500 px-4 py-2 text-xs font-semibold text-slate-950 hover:bg-cyan-400"
                    >
                      Save Default Limit
                    </button>
                  </div>
                </form>

                <div className="overflow-x-auto rounded-2xl border border-slate-800 bg-slate-900">
                  <table className="w-full text-left text-xs">
                    <thead className="border-b border-slate-800 bg-slate-950/60 text-slate-400">
                      <tr>
                        <th className="px-4 py-3">User</th>
                        <th className="px-4 py-3">Role</th>
                        <th className="px-4 py-3">Active Peers</th>
                        <th className="px-4 py-3">Custom Limit Override</th>
                        <th className="px-4 py-3">Effective Limit</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-800">
                      {adminUsers.map((u) => (
                        <tr key={u.id}>
                          <td className="px-4 py-3">
                            <div className="font-medium text-white">{u.display_name}</div>
                            <div className="text-slate-400">{u.email}</div>
                          </td>
                          <td className="px-4 py-3">
                            <select
                              value={u.role}
                              onChange={(e) =>
                                void handleUpdateUserQuota(
                                  u,
                                  e.target.value as 'admin' | 'user',
                                  u.max_peers_override != null ? String(u.max_peers_override) : ''
                                )
                              }
                              className="rounded-lg border border-slate-700 bg-slate-950 px-2.5 py-1 text-xs text-white"
                            >
                              <option value="user">user</option>
                              <option value="admin">admin</option>
                            </select>
                          </td>
                          <td className="px-4 py-3 font-mono text-slate-200">{u.peer_count}</td>
                          <td className="px-4 py-3">
                            <input
                              type="number"
                              min={0}
                              placeholder={`Default (${defaultMaxPeers})`}
                              defaultValue={u.max_peers_override ?? ''}
                              onBlur={(e) =>
                                void handleUpdateUserQuota(u, u.role, e.target.value)
                              }
                              className="w-32 rounded-lg border border-slate-700 bg-slate-950 px-2.5 py-1 text-xs text-white"
                            />
                          </td>
                          <td className="px-4 py-3 font-mono font-semibold text-cyan-300">
                            {u.effective_max_peers}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </div>
            )}

            {adminSection === 'peers' && (
              <div className="overflow-x-auto rounded-2xl border border-slate-800 bg-slate-900">
                <table className="w-full text-left text-xs">
                  <thead className="border-b border-slate-800 bg-slate-950/60 text-slate-400">
                    <tr>
                      <th className="px-4 py-3">Device Name</th>
                      <th className="px-4 py-3">User</th>
                      <th className="px-4 py-3">Network (ROS Interface)</th>
                      <th className="px-4 py-3">Allocated IP</th>
                      <th className="px-4 py-3">Handshake / Traffic</th>
                      <th className="px-4 py-3 text-right">Actions</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-800">
                    {adminAllPeers.map((p) => (
                      <tr key={p.id}>
                        <td className="px-4 py-3 font-medium text-white">{p.name}</td>
                        <td className="px-4 py-3 text-slate-300">{p.user_email}</td>
                        <td className="px-4 py-3">
                          <span className="text-cyan-300">{p.network_name}</span>{' '}
                          <span className="font-mono text-slate-500">({p.ros_interface})</span>
                        </td>
                        <td className="px-4 py-3 font-mono text-white">{p.allocated_ip}</td>
                        <td className="px-4 py-3 font-mono text-slate-300">
                          {p.last_handshake || 'never'} • ↓{formatBytes(p.rx_bytes)} ↑
                          {formatBytes(p.tx_bytes)}
                        </td>
                        <td className="px-4 py-3 text-right">
                          <button
                            type="button"
                            onClick={() => void handleDeletePeer(p)}
                            className="rounded-lg border border-rose-500/30 bg-rose-500/10 px-2.5 py-1 text-rose-300 hover:bg-rose-500 hover:text-white"
                          >
                            Revoke
                          </button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        )}
      </main>

      {/* Modal 1: Create New VPN Peer */}
      <ModalDialog
        open={createModalOpen}
        onClose={() => setCreateModalOpen(false)}
        title="Create WireGuard VPN Connection"
        subtitle="Automatically allocates the lowest available IP in the selected network."
      >
        <form onSubmit={handleCreatePeer} className="space-y-4">
          <div>
            <label className="block text-xs font-medium text-slate-300">
              Select WireGuard Network
            </label>
            <select
              value={selectedNetId}
              onChange={(e) => setSelectedNetId(Number(e.target.value))}
              className="mt-1 w-full rounded-xl border border-slate-700 bg-slate-950 px-3.5 py-2.5 text-sm text-white"
            >
              {networks.map((n) => (
                <option key={n.id} value={n.id}>
                  {n.name} — {n.subnet_cidr} ({n.ros_interface})
                </option>
              ))}
            </select>
          </div>

          <div>
            <label className="block text-xs font-medium text-slate-300">
              Device / Connection Name
            </label>
            <input
              type="text"
              required
              value={peerName}
              onChange={(e) => setPeerName(e.target.value)}
              placeholder="e.g. MacBook Pro, iPhone 16, Home Router"
              className="mt-1 w-full rounded-xl border border-slate-700 bg-slate-950 px-3.5 py-2 text-sm text-white"
            />
          </div>

          <div>
            <label className="block text-xs font-medium text-slate-300">
              Key Provisioning Mode
            </label>
            <div className="mt-1.5 grid grid-cols-2 gap-2.5">
              <button
                type="button"
                onClick={() => setKeyModeChoice('server')}
                className={`rounded-xl border p-3 text-left text-xs transition ${
                  keyModeChoice === 'server'
                    ? 'border-cyan-500 bg-cyan-500/10 text-white'
                    : 'border-slate-800 bg-slate-950 text-slate-400'
                }`}
              >
                <div className="flex items-center gap-1.5 font-semibold text-cyan-300">
                  <QrCode className="h-4 w-4" />
                  Server Generates Keys
                </div>
                <p className="mt-1 text-[11px] text-slate-400">
                  Generates keypair for you. Shows scannable mobile QR code + .conf file once.
                </p>
              </button>

              <button
                type="button"
                onClick={() => setKeyModeChoice('custom')}
                className={`rounded-xl border p-3 text-left text-xs transition ${
                  keyModeChoice === 'custom'
                    ? 'border-cyan-500 bg-cyan-500/10 text-white'
                    : 'border-slate-800 bg-slate-950 text-slate-400'
                }`}
              >
                <div className="flex items-center gap-1.5 font-semibold text-cyan-300">
                  <KeyRound className="h-4 w-4" />
                  Bring Your Own Public Key
                </div>
                <p className="mt-1 text-[11px] text-slate-400">
                  Paste your public key. No QR code; provides a .conf file where you fill in PrivateKey.
                </p>
              </button>
            </div>
          </div>

          {keyModeChoice === 'custom' && (
            <div>
              <label className="block text-xs font-medium text-slate-300">
                Your WireGuard Public Key (Base64, 44 chars)
              </label>
              <input
                type="text"
                required
                value={customPublicKey}
                onChange={(e) => setCustomPublicKey(e.target.value)}
                placeholder="e.g. HIgo9xNzJMWLKAShlKl6/bUT1VI9Q0S..."
                className="mt-1 w-full rounded-xl border border-slate-700 bg-slate-950 px-3.5 py-2 font-mono text-xs text-white"
              />
            </div>
          )}

          <div className="flex justify-end gap-2 pt-2">
            <button
              type="button"
              onClick={() => setCreateModalOpen(false)}
              className="rounded-xl border border-slate-700 px-4 py-2 text-xs font-medium text-slate-300"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={submittingPeer}
              className="rounded-xl bg-cyan-500 px-4 py-2 text-xs font-semibold text-slate-950 hover:bg-cyan-400 disabled:opacity-50"
            >
              {submittingPeer ? 'Provisioning on RB5009...' : 'Create Connection'}
            </button>
          </div>
        </form>
      </ModalDialog>

      {/* Modal 2: In-Place Key Regeneration */}
      <ModalDialog
        open={regenTarget !== null}
        onClose={() => setRegenTarget(null)}
        title={`Regenerate Keys — ${regenTarget?.name || ''}`}
        subtitle={`Updates the peer on RouterOS in-place while keeping IP ${regenTarget?.allocated_ip}.`}
      >
        <form onSubmit={handleRegenerateKeys} className="space-y-4">
          <div className="grid grid-cols-2 gap-2.5">
            <button
              type="button"
              onClick={() => setRegenModeChoice('server')}
              className={`rounded-xl border p-3 text-left text-xs transition ${
                regenModeChoice === 'server'
                  ? 'border-cyan-500 bg-cyan-500/10 text-white'
                  : 'border-slate-800 bg-slate-950 text-slate-400'
              }`}
            >
              <div className="flex items-center gap-1.5 font-semibold text-cyan-300">
                <QrCode className="h-4 w-4" />
                Generate New Keypair + QR
              </div>
              <p className="mt-1 text-[11px] text-slate-400">
                Rotates keys on RB5009 and displays a fresh QR code for mobile scanning.
              </p>
            </button>

            <button
              type="button"
              onClick={() => setRegenModeChoice('custom')}
              className={`rounded-xl border p-3 text-left text-xs transition ${
                regenModeChoice === 'custom'
                  ? 'border-cyan-500 bg-cyan-500/10 text-white'
                  : 'border-slate-800 bg-slate-950 text-slate-400'
              }`}
            >
              <div className="flex items-center gap-1.5 font-semibold text-cyan-300">
                <KeyRound className="h-4 w-4" />
                Set Custom Public Key
              </div>
              <p className="mt-1 text-[11px] text-slate-400">
                Replaces public key on RB5009 and returns a .conf file without QR code.
              </p>
            </button>
          </div>

          {regenModeChoice === 'custom' && (
            <div>
              <label className="block text-xs font-medium text-slate-300">
                New WireGuard Public Key
              </label>
              <input
                type="text"
                required
                value={regenCustomPubKey}
                onChange={(e) => setRegenCustomPubKey(e.target.value)}
                placeholder="44-character base64 public key..."
                className="mt-1 w-full rounded-xl border border-slate-700 bg-slate-950 px-3.5 py-2 font-mono text-xs text-white"
              />
            </div>
          )}

          <div className="flex justify-end gap-2 pt-2">
            <button
              type="button"
              onClick={() => setRegenTarget(null)}
              className="rounded-xl border border-slate-700 px-4 py-2 text-xs font-medium text-slate-300"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={submittingRegen}
              className="rounded-xl bg-cyan-500 px-4 py-2 text-xs font-semibold text-slate-950 hover:bg-cyan-400 disabled:opacity-50"
            >
              {submittingRegen ? 'Updating RB5009...' : 'Rotate Keys Now'}
            </button>
          </div>
        </form>
      </ModalDialog>

      {/* Modal 3: Provisioned Config & QR Code Display */}
      <ModalDialog
        open={provisionResult !== null}
        onClose={() => setProvisionResult(null)}
        title={`WireGuard Configuration — ${provisionResult?.peer.name || ''}`}
        subtitle={`Assigned IP: ${provisionResult?.peer.allocated_ip}/32 on ${provisionResult?.peer.network_name}`}
        maxWidthClass="max-w-xl"
      >
        {provisionResult && (
          <div className="space-y-5">
            {provisionResult.qr_supported ? (
              <div className="flex flex-col items-center rounded-2xl border border-slate-800 bg-slate-950 p-5">
                <div className="rounded-2xl bg-white p-4 shadow-lg">
                  <QRCodeSVG value={provisionResult.config_text} size={210} level="M" />
                </div>
                <p className="mt-3 text-center text-xs text-amber-300">
                  Scan this QR code with the WireGuard mobile app or download the .conf file now.
                  For security, the private key is never stored on the server.
                </p>
              </div>
            ) : (
              <div className="rounded-xl border border-cyan-500/30 bg-cyan-500/10 p-3.5 text-xs text-cyan-200">
                <strong>Custom Public Key Mode:</strong> Download the <code>.conf</code> file below
                and replace <code>&lt;INSERT_YOUR_PRIVATE_KEY&gt;</code> with your private key before importing into WireGuard.
              </div>
            )}

            <div>
              <div className="mb-1.5 flex items-center justify-between">
                <span className="text-xs font-medium text-slate-400">
                  Configuration File (.conf)
                </span>
                <div className="flex gap-2">
                  <button
                    type="button"
                    onClick={() => {
                      void navigator.clipboard.writeText(provisionResult.config_text);
                      setCopiedConfig(true);
                      setTimeout(() => setCopiedConfig(false), 2000);
                    }}
                    className="inline-flex items-center gap-1 rounded-lg border border-slate-700 bg-slate-950 px-2.5 py-1 text-xs text-slate-200 hover:border-slate-600"
                  >
                    {copiedConfig ? (
                      <>
                        <Check className="h-3.5 w-3.5 text-emerald-400" />
                        Copied
                      </>
                    ) : (
                      <>
                        <Copy className="h-3.5 w-3.5" />
                        Copy
                      </>
                    )}
                  </button>
                  <button
                    type="button"
                    onClick={() => {
                      const safeName =
                        provisionResult.peer.name
                          .toLowerCase()
                          .replace(/[^a-z0-9-_]/g, '-') || 'wg-peer';
                      downloadConfFile(
                        `${safeName}-${provisionResult.peer.allocated_ip}.conf`,
                        provisionResult.config_text
                      );
                    }}
                    className="inline-flex items-center gap-1 rounded-lg bg-cyan-500 px-3 py-1 text-xs font-semibold text-slate-950 hover:bg-cyan-400"
                  >
                    <Download className="h-3.5 w-3.5" />
                    Download .conf
                  </button>
                </div>
              </div>
              <pre className="max-h-56 overflow-auto rounded-xl border border-slate-800 bg-slate-950 p-3.5 font-mono text-xs text-slate-200">
                {provisionResult.config_text}
              </pre>
            </div>
          </div>
        )}
      </ModalDialog>

      {/* Modal 4: Admin Create / Edit WireGuard Network */}
      <ModalDialog
        open={netModalOpen}
        onClose={() => setNetModalOpen(false)}
        title={editingNetwork ? 'Edit WireGuard Network' : 'Add WireGuard Network'}
        subtitle="Map a TikPortal network profile to an existing WireGuard interface on your RB5009."
        maxWidthClass="max-w-xl"
      >
        <form onSubmit={handleSaveNetwork} className="space-y-3.5 text-xs">
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block font-medium text-slate-300">Network Display Name</label>
              <input
                type="text"
                required
                value={netForm.name}
                onChange={(e) => setNetForm({ ...netForm, name: e.target.value })}
                placeholder="Direct PPPoE (10.10.x.x)"
                className="mt-1 w-full rounded-xl border border-slate-700 bg-slate-950 px-3 py-2 text-white"
              />
            </div>
            <div>
              <label className="block font-medium text-slate-300">
                RouterOS WireGuard Interface
              </label>
              <input
                type="text"
                required
                value={netForm.ros_interface}
                onChange={(e) => setNetForm({ ...netForm, ros_interface: e.target.value })}
                placeholder="wg-direct"
                className="mt-1 w-full rounded-xl border border-slate-700 bg-slate-950 px-3 py-2 font-mono text-white"
              />
            </div>
          </div>

          <div>
            <label className="block font-medium text-slate-300">Description</label>
            <input
              type="text"
              value={netForm.description}
              onChange={(e) => setNetForm({ ...netForm, description: e.target.value })}
              placeholder="e.g. Primary VPN tunnel / Guest internet-only VPN"
              className="mt-1 w-full rounded-xl border border-slate-700 bg-slate-950 px-3 py-2 text-white"
            />
          </div>

          <div>
            <label className="block font-medium text-slate-300">
              RB5009 Interface Public Key
            </label>
            <input
              type="text"
              required
              value={netForm.server_public_key}
              onChange={(e) => setNetForm({ ...netForm, server_public_key: e.target.value })}
              className="mt-1 w-full rounded-xl border border-slate-700 bg-slate-950 px-3 py-2 font-mono text-white"
            />
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block font-medium text-slate-300">Subnet CIDR</label>
              <input
                type="text"
                required
                value={netForm.subnet_cidr}
                onChange={(e) => setNetForm({ ...netForm, subnet_cidr: e.target.value })}
                placeholder="10.10.0.0/24"
                className="mt-1 w-full rounded-xl border border-slate-700 bg-slate-950 px-3 py-2 font-mono text-white"
              />
            </div>
            <div>
              <label className="block font-medium text-slate-300">Router Gateway IP</label>
              <input
                type="text"
                required
                value={netForm.server_ip}
                onChange={(e) => setNetForm({ ...netForm, server_ip: e.target.value })}
                placeholder="10.10.0.1"
                className="mt-1 w-full rounded-xl border border-slate-700 bg-slate-950 px-3 py-2 font-mono text-white"
              />
            </div>
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block font-medium text-slate-300">Public Endpoint Host</label>
              <input
                type="text"
                required
                value={netForm.endpoint_host}
                onChange={(e) => setNetForm({ ...netForm, endpoint_host: e.target.value })}
                placeholder="vpn.example.com"
                className="mt-1 w-full rounded-xl border border-slate-700 bg-slate-950 px-3 py-2 font-mono text-white"
              />
            </div>
            <div>
              <label className="block font-medium text-slate-300">Public UDP Port</label>
              <input
                type="number"
                required
                value={netForm.endpoint_port}
                onChange={(e) =>
                  setNetForm({ ...netForm, endpoint_port: parseInt(e.target.value || '51820', 10) })
                }
                className="mt-1 w-full rounded-xl border border-slate-700 bg-slate-950 px-3 py-2 font-mono text-white"
              />
            </div>
          </div>

          <div className="grid grid-cols-3 gap-3">
            <div>
              <label className="block font-medium text-slate-300">Client DNS Servers</label>
              <input
                type="text"
                value={netForm.dns_servers}
                onChange={(e) => setNetForm({ ...netForm, dns_servers: e.target.value })}
                className="mt-1 w-full rounded-xl border border-slate-700 bg-slate-950 px-3 py-2 font-mono text-white"
              />
            </div>
            <div>
              <label className="block font-medium text-slate-300">Client Allowed IPs</label>
              <input
                type="text"
                value={netForm.allowed_ips}
                onChange={(e) => setNetForm({ ...netForm, allowed_ips: e.target.value })}
                className="mt-1 w-full rounded-xl border border-slate-700 bg-slate-950 px-3 py-2 font-mono text-white"
              />
            </div>
            <div>
              <label className="block font-medium text-slate-300">Keepalive (sec)</label>
              <input
                type="number"
                min={0}
                max={3600}
                value={netForm.persistent_keepalive}
                onChange={(e) =>
                  setNetForm({
                    ...netForm,
                    persistent_keepalive: parseInt(e.target.value || '25', 10),
                  })
                }
                className="mt-1 w-full rounded-xl border border-slate-700 bg-slate-950 px-3 py-2 font-mono text-white"
              />
            </div>
          </div>

          <label className="flex items-center gap-2 pt-1 text-slate-300">
            <input
              type="checkbox"
              checked={netForm.enabled}
              onChange={(e) => setNetForm({ ...netForm, enabled: e.target.checked })}
            />
            Enable this network for user self-service
          </label>

          <div className="flex justify-end gap-2 pt-3">
            <button
              type="button"
              onClick={() => setNetModalOpen(false)}
              className="rounded-xl border border-slate-700 px-4 py-2 font-medium text-slate-300"
            >
              Cancel
            </button>
            <button
              type="submit"
              className="rounded-xl bg-cyan-500 px-4 py-2 font-semibold text-slate-950 hover:bg-cyan-400"
            >
              Save Network
            </button>
          </div>
        </form>
      </ModalDialog>
    </div>
  );
}
