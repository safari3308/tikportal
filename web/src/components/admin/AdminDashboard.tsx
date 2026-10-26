import React from 'react';
import { useLingui } from '@lingui/react';
import { Activity, Network, Plus, Users } from 'lucide-react';
import type { ROSInterface, User, WGNetwork, WGPeer } from '../../types';
import { formatBytes } from '../../utils/format';

interface AdminDashboardProps {
  adminSection: 'networks' | 'users' | 'peers';
  onSelectSection: (sec: 'networks' | 'users' | 'peers') => void;
  adminNetworks: WGNetwork[];
  rosInterfaces: ROSInterface[];
  rosInterfaceTag: string;
  adminUsers: User[];
  defaultMaxPeers: number;
  onChangeDefaultMaxPeers: (v: number) => void;
  onSaveGlobalMaxPeers: (e: React.FormEvent) => void;
  onUpdateUserQuota: (u: User, newRole: 'admin' | 'user', overrideStr: string) => void;
  adminAllPeers: WGPeer[];
  onOpenNewNetwork: (rosIface?: ROSInterface) => void;
  onOpenEditNetwork: (net: WGNetwork) => void;
  onDeleteNetwork: (net: WGNetwork) => void;
  onRevokePeer: (peer: WGPeer) => void;
}

export function AdminDashboard({
  adminSection,
  onSelectSection,
  adminNetworks,
  rosInterfaces,
  rosInterfaceTag,
  adminUsers,
  defaultMaxPeers,
  onChangeDefaultMaxPeers,
  onSaveGlobalMaxPeers,
  onUpdateUserQuota,
  adminAllPeers,
  onOpenNewNetwork,
  onOpenEditNetwork,
  onDeleteNetwork,
  onRevokePeer,
}: AdminDashboardProps) {
  const { i18n } = useLingui();

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-4 border-b border-slate-800 pb-4">
        <div className="flex flex-wrap gap-2">
          <button
            type="button"
            onClick={() => onSelectSection('networks')}
            className={`inline-flex items-center gap-2 rounded-xl px-4 py-2 text-xs font-semibold transition ${
              adminSection === 'networks'
                ? 'bg-cyan-500 text-slate-950'
                : 'border border-slate-800 bg-slate-900 text-slate-300 hover:text-white'
            }`}
          >
            <Network className="h-4 w-4" />
            {i18n._('WireGuard Networks ({count})', { count: adminNetworks.length })}
          </button>
          <button
            type="button"
            onClick={() => onSelectSection('users')}
            className={`inline-flex items-center gap-2 rounded-xl px-4 py-2 text-xs font-semibold transition ${
              adminSection === 'users'
                ? 'bg-cyan-500 text-slate-950'
                : 'border border-slate-800 bg-slate-900 text-slate-300 hover:text-white'
            }`}
          >
            <Users className="h-4 w-4" />
            {i18n._('Users & Quotas ({count})', { count: adminUsers.length })}
          </button>
          <button
            type="button"
            onClick={() => onSelectSection('peers')}
            className={`inline-flex items-center gap-2 rounded-xl px-4 py-2 text-xs font-semibold transition ${
              adminSection === 'peers'
                ? 'bg-cyan-500 text-slate-950'
                : 'border border-slate-800 bg-slate-900 text-slate-300 hover:text-white'
            }`}
          >
            <Activity className="h-4 w-4" />
            {i18n._('All Provisioned Peers ({count})', { count: adminAllPeers.length })}
          </button>
        </div>
      </div>

      {adminSection === 'networks' && (
        <div className="space-y-6">
          {/* Discovered RouterOS WireGuard Interfaces */}
          <div className="rounded-2xl border border-slate-800 bg-slate-900/60 p-5">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <div>
                <div className="flex items-center gap-2">
                  <h3 className="text-sm font-semibold text-white">
                    {i18n._('Discovered RouterOS WireGuard Interfaces (via REST API)')}
                  </h3>
                  {rosInterfaceTag && rosInterfaceTag !== '*' && (
                    <span className="rounded-md bg-cyan-500/10 px-2 py-0.5 font-mono text-[11px] text-cyan-300 ring-1 ring-cyan-500/30">
                      comment ~ &quot;{rosInterfaceTag}&quot;
                    </span>
                  )}
                </div>
                <p className="text-xs text-slate-400">
                  {i18n._(
                    'Only RouterOS WireGuard interfaces matching your comment tag filter appear here.'
                  )}
                </p>
              </div>
              <button
                type="button"
                onClick={() => onOpenNewNetwork()}
                className="inline-flex items-center gap-1.5 rounded-xl bg-cyan-500 px-3.5 py-2 text-xs font-semibold text-slate-950 hover:bg-cyan-400"
              >
                <Plus className="h-4 w-4" />
                {i18n._('Add Network')}
              </button>
            </div>

            <div className="mt-4 grid gap-3 sm:grid-cols-2">
              {rosInterfaces.map((iface) => {
                const alreadyBound = adminNetworks.some((n) => n.ros_interface === iface.name);
                return (
                  <div
                    key={iface['.id']}
                    className="flex items-center justify-between rounded-xl border border-slate-800 bg-slate-950 p-3.5 text-xs"
                  >
                    <div>
                      <div className="flex items-center gap-2">
                        <span className="font-mono font-semibold text-cyan-300">{iface.name}</span>
                        <span className="text-slate-500">UDP :{iface['listen-port']}</span>
                      </div>
                      <p className="mt-0.5 truncate font-mono text-[11px] text-slate-400">
                        PubKey: {iface['public-key']}
                      </p>
                    </div>
                    {alreadyBound ? (
                      <span className="rounded-lg bg-emerald-500/10 px-2.5 py-1 text-[11px] font-medium text-emerald-300">
                        {i18n._('Configured')}
                      </span>
                    ) : (
                      <button
                        type="button"
                        onClick={() => onOpenNewNetwork(iface)}
                        className="rounded-lg bg-cyan-500/15 px-2.5 py-1.5 font-medium text-cyan-300 hover:bg-cyan-500 hover:text-slate-950"
                      >
                        {i18n._('Import Interface')}
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
              <div key={net.id} className="rounded-2xl border border-slate-800 bg-slate-900 p-5">
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
                        {net.enabled ? i18n._('ENABLED') : i18n._('DISABLED')}
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
                    <span className="text-slate-500">{i18n._('Subnet')}:</span>{' '}
                    <span className="font-mono text-slate-200">{net.subnet_cidr}</span>
                  </div>
                  <div>
                    <span className="text-slate-500">{i18n._('Gateway IP:')}</span>{' '}
                    <span className="font-mono text-slate-200">{net.server_ip}</span>
                  </div>
                  <div>
                    <span className="text-slate-500">{i18n._('Endpoint')}:</span>{' '}
                    <span className="font-mono text-slate-200">
                      {net.endpoint_host}:{net.endpoint_port}
                    </span>
                  </div>
                  <div>
                    <span className="text-slate-500">{i18n._('Allocated IPs:')}</span>{' '}
                    <span className="font-mono text-slate-200">
                      {net.allocated_count} / {net.subnet_capacity}
                    </span>
                  </div>
                </div>

                <div className="mt-4 flex justify-end gap-2">
                  <button
                    type="button"
                    onClick={() => onOpenEditNetwork(net)}
                    className="rounded-lg border border-slate-700 bg-slate-950 px-3 py-1.5 text-xs font-medium text-slate-200 hover:border-slate-600"
                  >
                    {i18n._('Edit Network')}
                  </button>
                  <button
                    type="button"
                    onClick={() => onDeleteNetwork(net)}
                    className="rounded-lg border border-rose-500/30 bg-rose-500/10 px-3 py-1.5 text-xs font-medium text-rose-300 hover:bg-rose-500 hover:text-white"
                  >
                    {i18n._('Delete')}
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
            onSubmit={onSaveGlobalMaxPeers}
            className="flex flex-wrap items-end justify-between gap-4 rounded-2xl border border-slate-800 bg-slate-900 p-5"
          >
            <div>
              <h3 className="text-sm font-semibold text-white">
                {i18n._('Global Default Max Peers per User')}
              </h3>
              <p className="text-xs text-slate-400">
                {i18n._(
                  'Applies to all users unless overridden individually below (total across all networks).'
                )}
              </p>
            </div>
            <div className="flex items-center gap-3">
              <input
                type="number"
                min={0}
                max={1000}
                value={defaultMaxPeers}
                onChange={(e) => onChangeDefaultMaxPeers(parseInt(e.target.value || '0', 10))}
                className="w-24 rounded-xl border border-slate-700 bg-slate-950 px-3 py-2 text-sm text-white"
              />
              <button
                type="submit"
                className="rounded-xl bg-cyan-500 px-4 py-2 text-xs font-semibold text-slate-950 hover:bg-cyan-400"
              >
                {i18n._('Save Default Limit')}
              </button>
            </div>
          </form>

          <div className="overflow-x-auto rounded-2xl border border-slate-800 bg-slate-900">
            <table className="w-full text-left text-xs">
              <thead className="border-b border-slate-800 bg-slate-950/60 text-slate-400">
                <tr>
                  <th className="px-4 py-3">{i18n._('User')}</th>
                  <th className="px-4 py-3">{i18n._('Role')}</th>
                  <th className="px-4 py-3">{i18n._('Active Peers')}</th>
                  <th className="px-4 py-3">{i18n._('Custom Limit Override')}</th>
                  <th className="px-4 py-3">{i18n._('Effective Limit')}</th>
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
                          onUpdateUserQuota(
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
                        onBlur={(e) => onUpdateUserQuota(u, u.role, e.target.value)}
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
                <th className="px-4 py-3">{i18n._('Device Name')}</th>
                <th className="px-4 py-3">{i18n._('User')}</th>
                <th className="px-4 py-3">{i18n._('Network (ROS Interface)')}</th>
                <th className="px-4 py-3">{i18n._('Allocated IP')}</th>
                <th className="px-4 py-3">{i18n._('Handshake / Traffic')}</th>
                <th className="px-4 py-3 text-right">{i18n._('Actions')}</th>
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
                      onClick={() => onRevokePeer(p)}
                      className="rounded-lg border border-rose-500/30 bg-rose-500/10 px-2.5 py-1 text-rose-300 hover:bg-rose-500 hover:text-white"
                    >
                      {i18n._('Revoke')}
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
