import React from 'react';
import { useLingui } from '@lingui/react';
import type { NetworkFormValues } from '../../hooks/useAdminConsole';
import type { WGNetwork } from '../../types';
import { ModalDialog } from '../ModalDialog';

interface NetworkFormModalProps {
  open: boolean;
  onClose: () => void;
  editingNetwork: WGNetwork | null;
  netForm: NetworkFormValues;
  onChangeNetForm: (next: NetworkFormValues) => void;
  onSubmit: (e: React.FormEvent) => void;
}

export function NetworkFormModal({
  open,
  onClose,
  editingNetwork,
  netForm,
  onChangeNetForm,
  onSubmit,
}: NetworkFormModalProps) {
  const { i18n } = useLingui();

  return (
    <ModalDialog
      open={open}
      onClose={onClose}
      title={
        editingNetwork ? i18n._('Edit WireGuard Network') : i18n._('Add WireGuard Network')
      }
      subtitle={i18n._(
        'Map a TikPortal network profile to an existing WireGuard interface on RouterOS.'
      )}
      maxWidthClass="max-w-xl"
    >
      <form onSubmit={onSubmit} className="space-y-3.5 text-xs">
        <div className="grid grid-cols-2 gap-3">
          <div>
            <label className="block font-medium text-slate-300">
              {i18n._('Network Display Name')}
            </label>
            <input
              type="text"
              required
              value={netForm.name}
              onChange={(e) => onChangeNetForm({ ...netForm, name: e.target.value })}
              placeholder="Primary VPN (10.10.0.0/24)"
              className="mt-1 w-full rounded-xl border border-slate-700 bg-slate-950 px-3 py-2 text-white"
            />
          </div>
          <div>
            <label className="block font-medium text-slate-300">
              {i18n._('RouterOS WireGuard Interface')}
            </label>
            <input
              type="text"
              required
              value={netForm.ros_interface}
              onChange={(e) => onChangeNetForm({ ...netForm, ros_interface: e.target.value })}
              placeholder="wg-vpn"
              className="mt-1 w-full rounded-xl border border-slate-700 bg-slate-950 px-3 py-2 font-mono text-white"
            />
          </div>
        </div>

        <div>
          <label className="block font-medium text-slate-300">{i18n._('Description')}</label>
          <input
            type="text"
            value={netForm.description}
            onChange={(e) => onChangeNetForm({ ...netForm, description: e.target.value })}
            placeholder="e.g. Primary VPN tunnel / Guest internet-only VPN"
            className="mt-1 w-full rounded-xl border border-slate-700 bg-slate-950 px-3 py-2 text-white"
          />
        </div>

        <div>
          <label className="block font-medium text-slate-300">
            {i18n._('RouterOS Interface Public Key')}
          </label>
          <input
            type="text"
            required
            value={netForm.server_public_key}
            onChange={(e) =>
              onChangeNetForm({ ...netForm, server_public_key: e.target.value })
            }
            className="mt-1 w-full rounded-xl border border-slate-700 bg-slate-950 px-3 py-2 font-mono text-white"
          />
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div>
            <label className="block font-medium text-slate-300">{i18n._('Subnet CIDR')}</label>
            <input
              type="text"
              required
              value={netForm.subnet_cidr}
              onChange={(e) => onChangeNetForm({ ...netForm, subnet_cidr: e.target.value })}
              placeholder="10.10.0.0/24"
              className="mt-1 w-full rounded-xl border border-slate-700 bg-slate-950 px-3 py-2 font-mono text-white"
            />
          </div>
          <div>
            <label className="block font-medium text-slate-300">
              {i18n._('Router Gateway IP')}
            </label>
            <input
              type="text"
              required
              value={netForm.server_ip}
              onChange={(e) => onChangeNetForm({ ...netForm, server_ip: e.target.value })}
              placeholder="10.10.0.1"
              className="mt-1 w-full rounded-xl border border-slate-700 bg-slate-950 px-3 py-2 font-mono text-white"
            />
          </div>
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div>
            <label className="block font-medium text-slate-300">
              {i18n._('Public Endpoint Host')}
            </label>
            <input
              type="text"
              required
              value={netForm.endpoint_host}
              onChange={(e) => onChangeNetForm({ ...netForm, endpoint_host: e.target.value })}
              placeholder="vpn.example.com"
              className="mt-1 w-full rounded-xl border border-slate-700 bg-slate-950 px-3 py-2 font-mono text-white"
            />
          </div>
          <div>
            <label className="block font-medium text-slate-300">
              {i18n._('Public UDP Port')}
            </label>
            <input
              type="number"
              required
              value={netForm.endpoint_port}
              onChange={(e) =>
                onChangeNetForm({
                  ...netForm,
                  endpoint_port: parseInt(e.target.value || '51820', 10),
                })
              }
              className="mt-1 w-full rounded-xl border border-slate-700 bg-slate-950 px-3 py-2 font-mono text-white"
            />
          </div>
        </div>

        <div className="grid grid-cols-3 gap-3">
          <div>
            <label className="block font-medium text-slate-300">
              {i18n._('Client DNS Servers')}
            </label>
            <input
              type="text"
              value={netForm.dns_servers}
              onChange={(e) => onChangeNetForm({ ...netForm, dns_servers: e.target.value })}
              className="mt-1 w-full rounded-xl border border-slate-700 bg-slate-950 px-3 py-2 font-mono text-white"
            />
          </div>
          <div>
            <label className="block font-medium text-slate-300">
              {i18n._('Client Allowed IPs')}
            </label>
            <input
              type="text"
              value={netForm.allowed_ips}
              onChange={(e) => onChangeNetForm({ ...netForm, allowed_ips: e.target.value })}
              className="mt-1 w-full rounded-xl border border-slate-700 bg-slate-950 px-3 py-2 font-mono text-white"
            />
          </div>
          <div>
            <label className="block font-medium text-slate-300">
              {i18n._('Keepalive (sec)')}
            </label>
            <input
              type="number"
              min={0}
              max={3600}
              value={netForm.persistent_keepalive}
              onChange={(e) =>
                onChangeNetForm({
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
            onChange={(e) => onChangeNetForm({ ...netForm, enabled: e.target.checked })}
          />
          {i18n._('Enable this network for user self-service')}
        </label>

        <div className="flex justify-end gap-2 pt-3">
          <button
            type="button"
            onClick={onClose}
            className="rounded-xl border border-slate-700 px-4 py-2 font-medium text-slate-300"
          >
            {i18n._('Cancel')}
          </button>
          <button
            type="submit"
            className="rounded-xl bg-cyan-500 px-4 py-2 font-semibold text-slate-950 hover:bg-cyan-400"
          >
            {i18n._('Save Network')}
          </button>
        </div>
      </form>
    </ModalDialog>
  );
}
