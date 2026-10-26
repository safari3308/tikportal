import { useState } from 'react';
import { QRCodeSVG } from 'qrcode.react';
import { useLingui } from '@lingui/react';
import { Check, Copy, Download } from 'lucide-react';
import type { PeerProvisionResponse } from '../../types';
import { downloadConfFile } from '../../utils/format';
import { ModalDialog } from '../ModalDialog';

interface ProvisionResultModalProps {
  result: PeerProvisionResponse | null;
  onClose: () => void;
}

export function ProvisionResultModal({ result, onClose }: ProvisionResultModalProps) {
  const { i18n } = useLingui();
  const [copiedConfig, setCopiedConfig] = useState(false);

  return (
    <ModalDialog
      open={result !== null}
      onClose={onClose}
      title={i18n._('WireGuard Configuration — {name}', {
        name: result?.peer.name || '',
      })}
      subtitle={i18n._('Assigned IP: {ip}/32 on {network}', {
        ip: result?.peer.allocated_ip || '',
        network: result?.peer.network_name || '',
      })}
      maxWidthClass="max-w-xl"
    >
      {result && (
        <div className="space-y-5">
          {result.qr_supported ? (
            <div className="flex flex-col items-center rounded-2xl border border-slate-800 bg-slate-950 p-5">
              <div className="rounded-2xl bg-white p-4 shadow-lg">
                <QRCodeSVG value={result.config_text} size={210} level="M" />
              </div>
              <p className="mt-3 text-center text-xs text-amber-300">
                {i18n._(
                  'Scan this QR code with the WireGuard mobile app or download the .conf file now. For security, the private key is never stored on the server.'
                )}
              </p>
            </div>
          ) : (
            <div className="rounded-xl border border-cyan-500/30 bg-cyan-500/10 p-3.5 text-xs text-cyan-200">
              {i18n._(
                'Custom Public Key Mode: Download the .conf file below and replace <INSERT_YOUR_PRIVATE_KEY> with your private key before importing into WireGuard.'
              )}
            </div>
          )}

          <div>
            <div className="mb-1.5 flex items-center justify-between">
              <span className="text-xs font-medium text-slate-400">
                {i18n._('Configuration File (.conf)')}
              </span>
              <div className="flex gap-2">
                <button
                  type="button"
                  onClick={() => {
                    void navigator.clipboard.writeText(result.config_text);
                    setCopiedConfig(true);
                    setTimeout(() => setCopiedConfig(false), 2000);
                  }}
                  className="inline-flex items-center gap-1 rounded-lg border border-slate-700 bg-slate-950 px-2.5 py-1 text-xs text-slate-200 hover:border-slate-600"
                >
                  {copiedConfig ? (
                    <>
                      <Check className="h-3.5 w-3.5 text-emerald-400" />
                      {i18n._('Copied')}
                    </>
                  ) : (
                    <>
                      <Copy className="h-3.5 w-3.5" />
                      {i18n._('Copy')}
                    </>
                  )}
                </button>
                <button
                  type="button"
                  onClick={() => {
                    const safeName =
                      result.peer.name.toLowerCase().replace(/[^a-z0-9-_]/g, '-') ||
                      'wg-peer';
                    downloadConfFile(
                      `${safeName}-${result.peer.allocated_ip}.conf`,
                      result.config_text
                    );
                  }}
                  className="inline-flex items-center gap-1 rounded-lg bg-cyan-500 px-3 py-1 text-xs font-semibold text-slate-950 hover:bg-cyan-400"
                >
                  <Download className="h-3.5 w-3.5" />
                  {i18n._('Download .conf')}
                </button>
              </div>
            </div>
            <pre className="max-h-56 overflow-auto rounded-xl border border-slate-800 bg-slate-950 p-3.5 font-mono text-xs text-slate-200">
              {result.config_text}
            </pre>
          </div>
        </div>
      )}
    </ModalDialog>
  );
}
