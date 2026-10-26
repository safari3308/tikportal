import React from 'react';
import { useLingui } from '@lingui/react';
import { KeyRound, QrCode } from 'lucide-react';
import type { WGPeer } from '../../types';
import { ModalDialog } from '../ModalDialog';

interface RegenerateKeyModalProps {
  target: WGPeer | null;
  onClose: () => void;
  regenModeChoice: 'server' | 'custom';
  onChangeRegenModeChoice: (mode: 'server' | 'custom') => void;
  regenCustomPubKey: string;
  onChangeRegenCustomPubKey: (key: string) => void;
  submitting: boolean;
  onSubmit: (e: React.FormEvent) => void;
}

export function RegenerateKeyModal({
  target,
  onClose,
  regenModeChoice,
  onChangeRegenModeChoice,
  regenCustomPubKey,
  onChangeRegenCustomPubKey,
  submitting,
  onSubmit,
}: RegenerateKeyModalProps) {
  const { i18n } = useLingui();

  return (
    <ModalDialog
      open={target !== null}
      onClose={onClose}
      title={i18n._('Regenerate Keys — {name}', { name: target?.name || '' })}
      subtitle={i18n._('Updates the peer on RouterOS in-place while keeping IP {ip}.', {
        ip: target?.allocated_ip || '',
      })}
    >
      <form onSubmit={onSubmit} className="space-y-4">
        <div className="grid grid-cols-2 gap-2.5">
          <button
            type="button"
            onClick={() => onChangeRegenModeChoice('server')}
            className={`rounded-xl border p-3 text-left text-xs transition ${
              regenModeChoice === 'server'
                ? 'border-cyan-500 bg-cyan-500/10 text-white'
                : 'border-slate-800 bg-slate-950 text-slate-400'
            }`}
          >
            <div className="flex items-center gap-1.5 font-semibold text-cyan-300">
              <QrCode className="h-4 w-4" />
              {i18n._('Generate New Keypair + QR')}
            </div>
            <p className="mt-1 text-[11px] text-slate-400">
              {i18n._(
                'Rotates keys on RouterOS and displays a fresh QR code for mobile scanning.'
              )}
            </p>
          </button>

          <button
            type="button"
            onClick={() => onChangeRegenModeChoice('custom')}
            className={`rounded-xl border p-3 text-left text-xs transition ${
              regenModeChoice === 'custom'
                ? 'border-cyan-500 bg-cyan-500/10 text-white'
                : 'border-slate-800 bg-slate-950 text-slate-400'
            }`}
          >
            <div className="flex items-center gap-1.5 font-semibold text-cyan-300">
              <KeyRound className="h-4 w-4" />
              {i18n._('Set Custom Public Key')}
            </div>
            <p className="mt-1 text-[11px] text-slate-400">
              {i18n._(
                'Replaces public key on RouterOS and returns a .conf file without QR code.'
              )}
            </p>
          </button>
        </div>

        {regenModeChoice === 'custom' && (
          <div>
            <label className="block text-xs font-medium text-slate-300">
              {i18n._('New WireGuard Public Key')}
            </label>
            <input
              type="text"
              required
              value={regenCustomPubKey}
              onChange={(e) => onChangeRegenCustomPubKey(e.target.value)}
              placeholder="44-character base64 public key..."
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
            {submitting ? i18n._('Updating RouterOS...') : i18n._('Rotate Keys Now')}
          </button>
        </div>
      </form>
    </ModalDialog>
  );
}
