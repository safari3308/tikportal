import React from 'react';
import { useLingui } from '@lingui/react';
import { KeyRound, QrCode } from 'lucide-react';
import type { WGNetwork } from '../../types';
import { ModalDialog } from '../ModalDialog';

interface CreatePeerModalProps {
  open: boolean;
  onClose: () => void;
  networks: WGNetwork[];
  selectedNetId: number;
  onChangeSelectedNetId: (id: number) => void;
  peerName: string;
  onChangePeerName: (name: string) => void;
  keyModeChoice: 'server' | 'custom';
  onChangeKeyModeChoice: (mode: 'server' | 'custom') => void;
  customPublicKey: string;
  onChangeCustomPublicKey: (key: string) => void;
  submitting: boolean;
  onSubmit: (e: React.FormEvent) => void;
}

export function CreatePeerModal({
  open,
  onClose,
  networks,
  selectedNetId,
  onChangeSelectedNetId,
  peerName,
  onChangePeerName,
  keyModeChoice,
  onChangeKeyModeChoice,
  customPublicKey,
  onChangeCustomPublicKey,
  submitting,
  onSubmit,
}: CreatePeerModalProps) {
  const { i18n } = useLingui();

  return (
    <ModalDialog
      open={open}
      onClose={onClose}
      title={i18n._('Create WireGuard VPN Connection')}
      subtitle={i18n._(
        'Automatically allocates the lowest available IP in the selected network.'
      )}
    >
      <form onSubmit={onSubmit} className="space-y-4">
        <div>
          <label className="block text-xs font-medium text-slate-300">
            {i18n._('Select WireGuard Network')}
          </label>
          <select
            value={selectedNetId}
            onChange={(e) => onChangeSelectedNetId(Number(e.target.value))}
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
            {i18n._('Device / Connection Name')}
          </label>
          <input
            type="text"
            required
            value={peerName}
            onChange={(e) => onChangePeerName(e.target.value)}
            placeholder={i18n._('e.g. MacBook Pro, iPhone 16, Home Router')}
            className="mt-1 w-full rounded-xl border border-slate-700 bg-slate-950 px-3.5 py-2 text-sm text-white"
          />
        </div>

        <div>
          <label className="block text-xs font-medium text-slate-300">
            {i18n._('Key Provisioning Mode')}
          </label>
          <div className="mt-1.5 grid grid-cols-2 gap-2.5">
            <button
              type="button"
              onClick={() => onChangeKeyModeChoice('server')}
              className={`rounded-xl border p-3 text-left text-xs transition ${
                keyModeChoice === 'server'
                  ? 'border-cyan-500 bg-cyan-500/10 text-white'
                  : 'border-slate-800 bg-slate-950 text-slate-400'
              }`}
            >
              <div className="flex items-center gap-1.5 font-semibold text-cyan-300">
                <QrCode className="h-4 w-4" />
                {i18n._('Server Generates Keys')}
              </div>
              <p className="mt-1 text-[11px] text-slate-400">
                {i18n._(
                  'Generates keypair for you. Shows scannable mobile QR code + .conf file once.'
                )}
              </p>
            </button>

            <button
              type="button"
              onClick={() => onChangeKeyModeChoice('custom')}
              className={`rounded-xl border p-3 text-left text-xs transition ${
                keyModeChoice === 'custom'
                  ? 'border-cyan-500 bg-cyan-500/10 text-white'
                  : 'border-slate-800 bg-slate-950 text-slate-400'
              }`}
            >
              <div className="flex items-center gap-1.5 font-semibold text-cyan-300">
                <KeyRound className="h-4 w-4" />
                {i18n._('Bring Your Own Public Key')}
              </div>
              <p className="mt-1 text-[11px] text-slate-400">
                {i18n._(
                  'Paste your public key. No QR code; provides a .conf file where you fill in PrivateKey.'
                )}
              </p>
            </button>
          </div>
        </div>

        {keyModeChoice === 'custom' && (
          <div>
            <label className="block text-xs font-medium text-slate-300">
              {i18n._('Your WireGuard Public Key (Base64, 44 chars)')}
            </label>
            <input
              type="text"
              required
              value={customPublicKey}
              onChange={(e) => onChangeCustomPublicKey(e.target.value)}
              placeholder="e.g. HIgo9xNzJMWLKAShlKl6/bUT1VI9Q0S..."
              className="mt-1 w-full rounded-xl border border-slate-700 bg-slate-950 px-3.5 py-2 font-mono text-xs text-white"
            />
          </div>
        )}

        <div className="flex justify-end gap-2 pt-2">
          <button
            type="button"
            onClick={onClose}
            className="rounded-xl border border-slate-700 px-4 py-2 text-xs font-medium text-slate-300"
          >
            {i18n._('Cancel')}
          </button>
          <button
            type="submit"
            disabled={submitting}
            className="rounded-xl bg-cyan-500 px-4 py-2 text-xs font-semibold text-slate-950 hover:bg-cyan-400 disabled:opacity-50"
          >
            {submitting ? i18n._('Provisioning on RouterOS...') : i18n._('Create Connection')}
          </button>
        </div>
      </form>
    </ModalDialog>
  );
}
