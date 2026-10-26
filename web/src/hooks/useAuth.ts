import { useCallback, useState } from 'react';
import { api } from '../api';
import type { AuthState } from '../types';

export function useAuth(onError: (msg: string | null) => void) {
  const [auth, setAuth] = useState<AuthState | null>(null);
  const [loading, setLoading] = useState(true);
  const [devEmail, setDevEmail] = useState('admin@localhost');
  const [devName, setDevName] = useState('Network Admin');

  const fetchAuthMe = useCallback(async (): Promise<AuthState | null> => {
    setLoading(true);
    try {
      const me = await api.getAuthMe();
      setAuth(me);
      return me;
    } catch (err: any) {
      onError(err.message || 'Failed to load application state');
      return null;
    } finally {
      setLoading(false);
    }
  }, [onError]);

  const loginDev = useCallback(
    async (onSuccess: (me: AuthState) => Promise<void>) => {
      onError(null);
      try {
        await api.devLogin(devEmail, devName);
        const me = await fetchAuthMe();
        if (me?.authenticated) {
          await onSuccess(me);
        }
      } catch (err: any) {
        onError(err.message);
      }
    },
    [devEmail, devName, fetchAuthMe, onError]
  );

  const logout = useCallback(async () => {
    await api.logout();
    setAuth((prev) => (prev ? { ...prev, authenticated: false, user: null } : null));
  }, []);

  return {
    auth,
    loading,
    devEmail,
    setDevEmail,
    devName,
    setDevName,
    fetchAuthMe,
    loginDev,
    logout,
  };
}
