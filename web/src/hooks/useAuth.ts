import { useCallback, useState } from 'react';
import { api } from '../api';
import {
  handleAuthCallback,
  initAuth,
  loginWithOIDC,
  logout as authLogout,
  setDevToken,
} from '../auth/auth';
import type { AppConfig, AuthState } from '../types';

export function useAuth(onError: (msg: string | null) => void) {
  const [config, setConfig] = useState<AppConfig | null>(null);
  const [auth, setAuth] = useState<AuthState | null>(null);
  const [loading, setLoading] = useState(true);
  const [devEmail, setDevEmail] = useState('admin@localhost');
  const [devName, setDevName] = useState('Network Admin');

  const initialize = useCallback(async (): Promise<AuthState | null> => {
    setLoading(true);
    try {
      const cfg = await api.getConfig();
      setConfig(cfg);
      await initAuth(cfg);
      await handleAuthCallback();

      const me = await api.getAuthMe();
      setAuth(me);
      return me;
    } catch (err: any) {
      onError(err.message || 'Failed to initialize authentication');
      return null;
    } finally {
      setLoading(false);
    }
  }, [onError]);

  const loginOIDC = useCallback(async () => {
    onError(null);
    try {
      await loginWithOIDC();
    } catch (err: any) {
      onError(err.message || 'OIDC redirect failed');
    }
  }, [onError]);

  const loginDev = useCallback(
    async (onSuccess: (me: AuthState) => Promise<void>) => {
      onError(null);
      try {
        const res = await api.devLogin(devEmail, devName);
        setDevToken(res.token);
        const me = await api.getAuthMe();
        setAuth(me);
        if (me?.authenticated) {
          await onSuccess(me);
        }
      } catch (err: any) {
        onError(err.message);
      }
    },
    [devEmail, devName, onError]
  );

  const logout = useCallback(async () => {
    try {
      await api.logout();
    } catch {
      // Ignore API logout error
    }
    await authLogout();
    setAuth((prev) => (prev ? { ...prev, authenticated: false, user: null } : null));
  }, []);

  return {
    config,
    auth,
    loading,
    devEmail,
    setDevEmail,
    devName,
    setDevName,
    initialize,
    loginOIDC,
    loginDev,
    logout,
  };
}
