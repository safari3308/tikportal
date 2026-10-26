import { useLingui } from '@lingui/react';
import { LogOut, Settings, Wifi } from 'lucide-react';
import type { SupportedLocale } from '../../i18n';
import type { User } from '../../types';
import { LanguageSwitcher } from './LanguageSwitcher';

interface HeaderProps {
  user: User;
  routerosMock: boolean;
  activeTab: 'user' | 'admin';
  onSelectTab: (tab: 'user' | 'admin') => void;
  onLogout: () => void;
  locale: SupportedLocale;
  onLocaleChange: (next: SupportedLocale) => void;
}

export function Header({
  user,
  routerosMock,
  activeTab,
  onSelectTab,
  onLogout,
  locale,
  onLocaleChange,
}: HeaderProps) {
  const { i18n } = useLingui();

  return (
    <header className="border-b border-slate-800/80 bg-slate-900/60 backdrop-blur">
      <div className="mx-auto flex max-w-7xl flex-wrap items-center justify-between gap-4 px-6 py-4">
        <div className="flex items-center gap-3">
          <img
            src="/logo.svg"
            alt="TikPortal Logo"
            className="h-10 w-10 rounded-xl shadow-md"
          />
          <div>
            <div className="flex items-center gap-2.5">
              <span className="text-lg font-bold tracking-tight text-white">TikPortal</span>
              <span
                className={`inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-medium ${
                  routerosMock
                    ? 'bg-amber-500/10 text-amber-300 ring-1 ring-amber-500/30'
                    : 'bg-emerald-500/10 text-emerald-300 ring-1 ring-emerald-500/30'
                }`}
              >
                <span className="h-1.5 w-1.5 rounded-full bg-current" />
                {routerosMock
                  ? i18n._('RouterOS Mock Mode')
                  : i18n._('RouterOS REST Connected')}
              </span>
            </div>
            <p className="text-xs text-slate-400">
              {i18n._('MikroTik RouterOS WireGuard Self-Service Portal')}
            </p>
          </div>
        </div>

        <div className="flex flex-wrap items-center gap-3">
          <LanguageSwitcher locale={locale} onChange={onLocaleChange} />

          {user.role === 'admin' && (
            <div className="flex rounded-xl border border-slate-800 bg-slate-950 p-1">
              <button
                type="button"
                onClick={() => onSelectTab('user')}
                className={`flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs font-medium transition ${
                  activeTab === 'user'
                    ? 'bg-cyan-500 text-slate-950'
                    : 'text-slate-400 hover:text-white'
                }`}
              >
                <Wifi className="h-3.5 w-3.5" />
                {i18n._('My VPNs')}
              </button>
              <button
                type="button"
                onClick={() => onSelectTab('admin')}
                className={`flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs font-medium transition ${
                  activeTab === 'admin'
                    ? 'bg-cyan-500 text-slate-950'
                    : 'text-slate-400 hover:text-white'
                }`}
              >
                <Settings className="h-3.5 w-3.5" />
                {i18n._('Admin Console')}
              </button>
            </div>
          )}

          <div className="hidden items-center gap-2 rounded-xl border border-slate-800 bg-slate-950 px-3 py-1.5 text-xs sm:flex">
            <span className="font-medium text-slate-200">{user.display_name}</span>
            <span className="text-slate-500">({user.email})</span>
            <span className="rounded bg-slate-800 px-1.5 py-0.5 text-[10px] font-semibold uppercase text-cyan-300">
              {user.role}
            </span>
          </div>

          <button
            type="button"
            onClick={onLogout}
            className="inline-flex items-center gap-1.5 rounded-xl border border-slate-800 bg-slate-900 px-3 py-2 text-xs font-medium text-slate-300 transition hover:border-slate-700 hover:text-white"
            title={i18n._('Sign Out')}
          >
            <LogOut className="h-4 w-4" />
            <span className="hidden sm:inline">{i18n._('Sign Out')}</span>
          </button>
        </div>
      </div>
    </header>
  );
}
