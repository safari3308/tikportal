import React from 'react';
import { useLingui } from '@lingui/react';
import { Shield } from 'lucide-react';
import type { SupportedLocale } from '../../i18n';
import type { AuthState } from '../../types';
import { GITHUB_REPO_URL } from '../../utils/format';
import { GitHubIcon, LanguageSwitcher } from '../layout/LanguageSwitcher';

interface LoginViewProps {
  auth: AuthState | null;
  errorBanner: string | null;
  devEmail: string;
  onChangeDevEmail: (v: string) => void;
  devName: string;
  onChangeDevName: (v: string) => void;
  onSubmitDevLogin: (e: React.FormEvent) => void;
  locale: SupportedLocale;
  onLocaleChange: (next: SupportedLocale) => void;
}

export function LoginView({
  auth,
  errorBanner,
  devEmail,
  onChangeDevEmail,
  devName,
  onChangeDevName,
  onSubmitDevLogin,
  locale,
  onLocaleChange,
}: LoginViewProps) {
  const { i18n } = useLingui();

  return (
    <div className="flex min-h-screen flex-col justify-between bg-slate-950 px-4 py-8">
      <div className="flex justify-end px-2">
        <LanguageSwitcher locale={locale} onChange={onLocaleChange} />
      </div>

      <div className="mx-auto my-8 w-full max-w-md rounded-2xl border border-slate-800 bg-slate-900/90 p-8 shadow-2xl">
        <div className="flex items-center gap-3">
          <img
            src="/logo.svg"
            alt="TikPortal Logo"
            className="h-11 w-11 rounded-xl shadow-md"
          />
          <div>
            <h1 className="text-xl font-bold tracking-tight text-white">TikPortal</h1>
            <p className="text-xs text-slate-400">
              {i18n._('MikroTik RouterOS WireGuard Self-Service Portal')}
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
              {i18n._('Sign in with SSO (OIDC)')}
            </a>
          </div>
        )}

        {auth?.dev_auth_enabled && (
          <form onSubmit={onSubmitDevLogin} className="mt-6 space-y-4">
            {auth?.oidc_enabled && (
              <div className="relative flex items-center py-1">
                <div className="grow border-t border-slate-800" />
                <span className="mx-3 shrink text-xs text-slate-500">
                  {i18n._('or Dev Login')}
                </span>
                <div className="grow border-t border-slate-800" />
              </div>
            )}

            <div>
              <label className="block text-xs font-medium text-slate-300">
                {i18n._('Email Address')}
              </label>
              <input
                type="email"
                required
                value={devEmail}
                onChange={(e) => onChangeDevEmail(e.target.value)}
                className="mt-1 w-full rounded-xl border border-slate-700 bg-slate-950 px-3.5 py-2 text-sm text-white focus:border-cyan-500 focus:outline-none"
                placeholder="admin@localhost"
              />
            </div>

            <div>
              <label className="block text-xs font-medium text-slate-300">
                {i18n._('Display Name')}
              </label>
              <input
                type="text"
                required
                value={devName}
                onChange={(e) => onChangeDevName(e.target.value)}
                className="mt-1 w-full rounded-xl border border-slate-700 bg-slate-950 px-3.5 py-2 text-sm text-white focus:border-cyan-500 focus:outline-none"
                placeholder="Network Admin"
              />
            </div>

            <div className="flex gap-2 pt-1">
              <button
                type="button"
                onClick={() => {
                  onChangeDevEmail('admin@localhost');
                  onChangeDevName('Network Admin');
                }}
                className="flex-1 rounded-lg border border-slate-800 bg-slate-950 px-2.5 py-1.5 text-xs text-slate-400 hover:border-slate-700 hover:text-slate-200"
              >
                {i18n._('Preset: Admin')}
              </button>
              <button
                type="button"
                onClick={() => {
                  onChangeDevEmail('alice@example.com');
                  onChangeDevName('Alice Nguyen');
                }}
                className="flex-1 rounded-lg border border-slate-800 bg-slate-950 px-2.5 py-1.5 text-xs text-slate-400 hover:border-slate-700 hover:text-slate-200"
              >
                {i18n._('Preset: Standard User')}
              </button>
            </div>

            <button
              type="submit"
              className="w-full rounded-xl bg-cyan-500 px-4 py-2.5 text-sm font-semibold text-slate-950 transition hover:bg-cyan-400"
            >
              {i18n._('Continue to TikPortal')}
            </button>
          </form>
        )}
      </div>

      <footer className="mx-auto flex w-full max-w-md flex-wrap items-center justify-between gap-2 text-xs text-slate-500">
        <span>TikPortal</span>
        <a
          href={GITHUB_REPO_URL}
          target="_blank"
          rel="noopener noreferrer"
          className="inline-flex items-center gap-1.5 text-slate-400 transition hover:text-cyan-400"
        >
          <GitHubIcon className="h-4 w-4" />
          <span>{i18n._('GitHub Repository')}</span>
        </a>
      </footer>
    </div>
  );
}
