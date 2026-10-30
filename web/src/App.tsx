import React, { useEffect, useState } from 'react';
import { useLingui } from '@lingui/react';
import { RefreshCw } from 'lucide-react';
import { AdminDashboard } from './components/admin/AdminDashboard';
import { LoginView } from './components/auth/LoginView';
import { UserDashboard } from './components/dashboard/UserDashboard';
import { Footer } from './components/layout/Footer';
import { Header } from './components/layout/Header';
import { CreatePeerModal } from './components/modals/CreatePeerModal';
import { NetworkFormModal } from './components/modals/NetworkFormModal';
import { ProvisionResultModal } from './components/modals/ProvisionResultModal';
import { RegenerateKeyModal } from './components/modals/RegenerateKeyModal';
import { useAdminConsole } from './hooks/useAdminConsole';
import { useAuth } from './hooks/useAuth';
import { useUserVpn } from './hooks/useUserVpn';
import { activateLocale, detectInitialLocale, type SupportedLocale } from './i18n';

export function App() {
  const { i18n } = useLingui();
  const [locale, setLocale] = useState<SupportedLocale>(() => detectInitialLocale());
  const [activeTab, setActiveTab] = useState<'user' | 'admin'>('user');
  const [errorBanner, setErrorBanner] = useState<string | null>(null);
  const [noticeBanner, setNoticeBanner] = useState<string | null>(null);

  const handleLocaleChange = (next: SupportedLocale) => {
    setLocale(next);
    activateLocale(next);
  };

  const authHook = useAuth(setErrorBanner);

  const userVpn = useUserVpn({
    onError: setErrorBanner,
    onNotice: setNoticeBanner,
    onAdminRefresh: async () => {
      if (authHook.auth?.user?.role === 'admin') {
        await adminConsole.refreshAdminData();
      }
    },
  });

  const adminConsole = useAdminConsole({
    onError: setErrorBanner,
    onNotice: setNoticeBanner,
    onUserDashboardRefresh: userVpn.refreshUserDashboard,
  });

  useEffect(() => {
    void (async () => {
      const me = await authHook.initialize();
      if (me?.authenticated) {
        await userVpn.refreshUserDashboard();
        if (me.user?.role === 'admin') {
          await adminConsole.refreshAdminData();
        }
      }
    })();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  if (authHook.loading) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-slate-950 text-slate-300">
        <div className="flex items-center gap-3">
          <RefreshCw className="h-5 w-5 animate-spin text-cyan-400" />
          <span>{i18n._('Loading TikPortal...')}</span>
        </div>
      </div>
    );
  }

  if (!authHook.auth?.authenticated || !authHook.auth.user) {
    return (
      <LoginView
        auth={authHook.auth}
        errorBanner={errorBanner}
        onLoginOIDC={authHook.loginOIDC}
        devEmail={authHook.devEmail}
        onChangeDevEmail={authHook.setDevEmail}
        devName={authHook.devName}
        onChangeDevName={authHook.setDevName}
        onSubmitDevLogin={(e: React.FormEvent) => {
          e.preventDefault();
          void authHook.loginDev(async (me) => {
            await userVpn.refreshUserDashboard();
            if (me.user?.role === 'admin') {
              await adminConsole.refreshAdminData();
            }
          });
        }}
        locale={locale}
        onLocaleChange={handleLocaleChange}
      />
    );
  }

  return (
    <div className="flex min-h-screen flex-col bg-slate-950 text-slate-100">
      <Header
        user={authHook.auth.user}
        routerosMock={Boolean(authHook.auth.routeros_mock)}
        activeTab={activeTab}
        onSelectTab={(tab) => {
          setActiveTab(tab);
          if (tab === 'admin') {
            void adminConsole.refreshAdminData();
          }
        }}
        onLogout={() => {
          void authHook.logout();
          setActiveTab('user');
        }}
        locale={locale}
        onLocaleChange={handleLocaleChange}
      />

      <main className="mx-auto w-full max-w-7xl flex-1 px-6 py-8">
        {errorBanner && (
          <div className="mb-6 flex items-center justify-between rounded-xl border border-rose-500/30 bg-rose-500/10 px-4 py-3 text-sm text-rose-200">
            <span>{errorBanner}</span>
            <button
              type="button"
              onClick={() => setErrorBanner(null)}
              className="text-xs underline"
            >
              {i18n._('Dismiss')}
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
              {i18n._('Dismiss')}
            </button>
          </div>
        )}

        {activeTab === 'user' ? (
          <UserDashboard
            networks={userVpn.networks}
            myPeers={userVpn.myPeers}
            quota={userVpn.quota}
            quotaReached={userVpn.quotaReached}
            onRefresh={() => void userVpn.refreshUserDashboard()}
            onOpenCreatePeer={userVpn.openCreatePeerModal}
            onOpenRegenerate={userVpn.openRegenerateModal}
            onDownloadTemplate={(p) => void userVpn.downloadTemplate(p)}
            onDeletePeer={(p) => void userVpn.deletePeer(p)}
          />
        ) : (
          <AdminDashboard
            adminSection={adminConsole.adminSection}
            onSelectSection={adminConsole.setAdminSection}
            adminNetworks={adminConsole.adminNetworks}
            rosInterfaces={adminConsole.rosInterfaces}
            rosInterfaceTag={adminConsole.rosInterfaceTag}
            adminUsers={adminConsole.adminUsers}
            defaultMaxPeers={adminConsole.defaultMaxPeers}
            onChangeDefaultMaxPeers={adminConsole.setDefaultMaxPeers}
            onSaveGlobalMaxPeers={(e) => {
              e.preventDefault();
              void adminConsole.saveGlobalMaxPeers();
            }}
            onUpdateUserQuota={(u, role, overrideStr) =>
              void adminConsole.updateUserQuota(u, role, overrideStr)
            }
            adminAllPeers={adminConsole.adminAllPeers}
            onOpenNewNetwork={adminConsole.openNewNetworkModal}
            onOpenEditNetwork={adminConsole.openEditNetworkModal}
            onDeleteNetwork={(net) => void adminConsole.deleteNetwork(net)}
            onRevokePeer={(p) => void userVpn.deletePeer(p)}
          />
        )}
      </main>

      <Footer />

      <CreatePeerModal
        open={userVpn.createModalOpen}
        onClose={() => userVpn.setCreateModalOpen(false)}
        networks={userVpn.networks}
        selectedNetId={userVpn.selectedNetId}
        onChangeSelectedNetId={userVpn.setSelectedNetId}
        peerName={userVpn.peerName}
        onChangePeerName={userVpn.setPeerName}
        keyModeChoice={userVpn.keyModeChoice}
        onChangeKeyModeChoice={userVpn.setKeyModeChoice}
        customPublicKey={userVpn.customPublicKey}
        onChangeCustomPublicKey={userVpn.setCustomPublicKey}
        submitting={userVpn.submittingPeer}
        onSubmit={(e) => {
          e.preventDefault();
          void userVpn.submitCreatePeer();
        }}
      />

      <RegenerateKeyModal
        target={userVpn.regenTarget}
        onClose={() => userVpn.setRegenTarget(null)}
        regenModeChoice={userVpn.regenModeChoice}
        onChangeRegenModeChoice={userVpn.setRegenModeChoice}
        regenCustomPubKey={userVpn.regenCustomPubKey}
        onChangeRegenCustomPubKey={userVpn.setRegenCustomPubKey}
        submitting={userVpn.submittingRegen}
        onSubmit={(e) => {
          e.preventDefault();
          void userVpn.submitRegenerateKeys();
        }}
      />

      <ProvisionResultModal
        result={userVpn.provisionResult}
        onClose={() => userVpn.setProvisionResult(null)}
      />

      <NetworkFormModal
        open={adminConsole.netModalOpen}
        onClose={() => adminConsole.setNetModalOpen(false)}
        editingNetwork={adminConsole.editingNetwork}
        netForm={adminConsole.netForm}
        onChangeNetForm={adminConsole.setNetForm}
        onSubmit={(e) => {
          e.preventDefault();
          void adminConsole.saveNetwork();
        }}
      />
    </div>
  );
}
