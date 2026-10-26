import { useLingui } from '@lingui/react';
import { GITHUB_REPO_URL } from '../../utils/format';
import { GitHubIcon } from './LanguageSwitcher';

export function Footer() {
  const { i18n } = useLingui();

  return (
    <footer className="border-t border-slate-800/70 bg-slate-900/40 py-5">
      <div className="mx-auto flex max-w-7xl flex-wrap items-center justify-between gap-4 px-6 text-xs text-slate-400">
        <div className="flex items-center gap-2.5">
          <img src="/logo.svg" alt="TikPortal" className="h-5 w-5 rounded" />
          <span className="font-semibold text-slate-200">TikPortal</span>
          <span className="text-slate-600">•</span>
          <span>{i18n._('Open-source WireGuard VPN Manager for MikroTik RouterOS')}</span>
        </div>

        <a
          href={GITHUB_REPO_URL}
          target="_blank"
          rel="noopener noreferrer"
          className="inline-flex items-center gap-2 rounded-lg border border-slate-800 bg-slate-900 px-3 py-1.5 text-xs font-medium text-slate-300 transition hover:border-cyan-500/40 hover:text-cyan-300"
        >
          <GitHubIcon className="h-4 w-4" />
          <span>{i18n._('GitHub Repository')}</span>
        </a>
      </div>
    </footer>
  );
}
