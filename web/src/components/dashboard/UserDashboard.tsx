import { useLingui } from '@lingui/react';
import {
  Activity,
  Download,
  KeyRound,
  Network,
  Plus,
  RefreshCw,
  Trash2,
} from 'lucide-react';
import type { WGNetwork, WGPeer } from '../../types';
import { formatBytes } from '../../utils/format';

interface UserDashboardProps {
  networks: WGNetwork[];
  myPeers: WGPeer[];
  quota: { used: number; limit: number };
  quotaReached: boolean;
  onRefresh: () => void;
  onOpenCreatePeer: (networkId?: number) => void;
  onOpenRegenerate: (peer: WGPeer) => void;
  onDownloadTemplate: (peer: WGPeer) => void;
  onDeletePeer: (peer: WGPeer) => void;
}

export function UserDashboard({
  networks,
  myPeers,
  quota,
  quotaReached,
  onRefresh,
  onOpenCreatePeer,
  onOpenRegenerate,
  onDownloadTemplate,
  onDeletePeer,
}: UserDashboardProps) {
  const { i18n } = useLingui();

  return (
    <div className="space-y-8">
      {/* Quota & Quick Action Bar */}
      <div className="flex flex-wrap items-center justify-between gap-4 rounded-2xl border border-slate-800 bg-slate-900/70 p-6">
        <div className="space-y-1">
          <div className="flex items-center gap-2">
            <h2 className="text-base font-semibold text-white">
              {i18n._('Global Connection Quota')}
            </h2>
            <span className="rounded-full bg-cyan-500/10 px-2.5 py-0.5 text-xs font-semibold text-cyan-300 ring-1 ring-cyan-500/30">
              {i18n._('{used} / {limit} Peers Used', {
                used: quota.used,
                limit: quota.limit,
              })}
            </span>
          </div>
          <p className="text-xs text-slate-400">
            {i18n._(
              'Your limit applies across all WireGuard networks. Removing any connection immediately frees its IP and quota slot.'
            )}
          </p>
        </div>

        <div className="flex items-center gap-3">
          <button
            type="button"
            onClick={onRefresh}
            className="inline-flex items-center gap-1.5 rounded-xl border border-slate-800 bg-slate-950 px-3.5 py-2 text-xs font-medium text-slate-300 transition hover:border-slate-700 hover:text-white"
          >
            <RefreshCw className="h-3.5 w-3.5" />
            {i18n._('Refresh Stats')}
          </button>
          <button
            type="button"
            disabled={quotaReached || networks.length === 0}
            onClick={() => onOpenCreatePeer()}
            className="inline-flex items-center gap-2 rounded-xl bg-cyan-500 px-4 py-2 text-xs font-semibold text-slate-950 transition hover:bg-cyan-400 disabled:cursor-not-allowed disabled:opacity-50"
          >
            <Plus className="h-4 w-4" />
            {i18n._('New VPN Connection')}
          </button>
        </div>
      </div>

      {/* Available WireGuard Networks */}
      <section>
        <h3 className="mb-3 text-sm font-semibold uppercase tracking-wider text-slate-400">
          {i18n._('Available WireGuard Networks')}
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
                    {i18n._('Subnet')}: {net.subnet_cidr}
                  </span>
                  <span className="rounded bg-slate-950 px-2 py-1 font-mono">
                    {i18n._('Endpoint')}: {net.endpoint_host}:{net.endpoint_port}
                  </span>
                  <span className="rounded bg-slate-950 px-2 py-1">
                    {i18n._('IPs Used')}: {net.allocated_count} / {net.subnet_capacity}
                  </span>
                </div>
              </div>

              <div className="mt-4 flex justify-end border-t border-slate-800/80 pt-3">
                <button
                  type="button"
                  disabled={quotaReached}
                  onClick={() => onOpenCreatePeer(net.id)}
                  className="inline-flex items-center gap-1.5 rounded-lg bg-cyan-500/15 px-3 py-1.5 text-xs font-medium text-cyan-300 ring-1 ring-cyan-500/30 transition hover:bg-cyan-500 hover:text-slate-950 disabled:opacity-40"
                >
                  <Plus className="h-3.5 w-3.5" />
                  {i18n._('Connect to {name}', { name: net.name })}
                </button>
              </div>
            </div>
          ))}
        </div>
      </section>

      {/* User's Active Peers */}
      <section>
        <h3 className="mb-3 text-sm font-semibold uppercase tracking-wider text-slate-400">
          {i18n._('My Provisioned Connections ({count})', { count: myPeers.length })}
        </h3>

        {myPeers.length === 0 ? (
          <div className="rounded-2xl border border-dashed border-slate-800 bg-slate-900/30 p-10 text-center">
            <Network className="mx-auto h-8 w-8 text-slate-500" />
            <p className="mt-3 text-sm font-medium text-slate-300">
              {i18n._('No WireGuard connections created yet')}
            </p>
            <p className="mt-1 text-xs text-slate-500">
              {i18n._(
                'Click "New VPN Connection" above to get a scannable QR code or configuration file.'
              )}
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
                          {peer.last_handshake
                            ? i18n._('Handshake {time}', { time: peer.last_handshake })
                            : i18n._('Idle')}
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
                      <span className="text-slate-500">{i18n._('Public Key:')}</span>
                      <code className="truncate font-mono text-slate-300" title={peer.public_key}>
                        {peer.public_key.slice(0, 20)}...{peer.public_key.slice(-6)}
                      </code>
                    </div>
                    <div className="flex items-center justify-between gap-2">
                      <span className="text-slate-500">{i18n._('Key Mode:')}</span>
                      <span className="text-slate-300">
                        {peer.key_mode === 'custom'
                          ? i18n._('Mode 2 — User Public Key (.conf only)')
                          : i18n._('Mode 1 — Server Generated (QR + .conf)')}
                      </span>
                    </div>
                    <div className="flex items-center justify-between gap-2">
                      <span className="text-slate-500">{i18n._('Traffic (Rx / Tx):')}</span>
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
                      onClick={() => onOpenRegenerate(peer)}
                      className="inline-flex items-center gap-1.5 rounded-lg border border-cyan-500/30 bg-cyan-500/10 px-3 py-1.5 text-xs font-medium text-cyan-300 transition hover:bg-cyan-500 hover:text-slate-950"
                    >
                      <KeyRound className="h-3.5 w-3.5" />
                      {i18n._('Regenerate Keys / QR')}
                    </button>

                    <button
                      type="button"
                      onClick={() => onDownloadTemplate(peer)}
                      className="inline-flex items-center gap-1.5 rounded-lg border border-slate-700 bg-slate-950 px-3 py-1.5 text-xs font-medium text-slate-300 transition hover:border-slate-600 hover:text-white"
                    >
                      <Download className="h-3.5 w-3.5" />
                      {i18n._('Template .conf')}
                    </button>
                  </div>

                  <button
                    type="button"
                    onClick={() => onDeletePeer(peer)}
                    className="inline-flex items-center gap-1 rounded-lg border border-rose-500/30 bg-rose-500/10 px-2.5 py-1.5 text-xs font-medium text-rose-300 transition hover:bg-rose-500 hover:text-white"
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                    {i18n._('Delete')}
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </section>
    </div>
  );
}
