import { UserManager, WebStorageStateStore, type User as OidcUser } from 'oidc-client-ts';
import type { AppConfig } from '../types';

let userManager: UserManager | null = null;
let devToken: string | null = sessionStorage.getItem('tikportal_dev_token');

export async function initAuth(config: AppConfig): Promise<UserManager | null> {
  if (config.oidc_issuer && config.oidc_client_id) {
    const origin = window.location.origin;
    userManager = new UserManager({
      authority: config.oidc_issuer,
      client_id: config.oidc_client_id,
      client_secret: config.oidc_client_secret || undefined,
      redirect_uri: `${origin}/`,
      post_logout_redirect_uri: `${origin}/`,
      response_type: 'code',
      scope: config.oidc_scopes || 'openid profile email',
      userStore: new WebStorageStateStore({ store: window.sessionStorage }),
      automaticSilentRenew: true,
    });
  }
  return userManager;
}

export function getUserManager(): UserManager | null {
  return userManager;
}

export async function getAccessToken(): Promise<string | null> {
  if (devToken) {
    return devToken;
  }
  if (!userManager) {
    return null;
  }
  const user = await userManager.getUser();
  if (!user || user.expired) {
    return null;
  }
  return user.access_token || user.id_token || null;
}

export function setDevToken(token: string | null) {
  devToken = token;
  if (token) {
    sessionStorage.setItem('tikportal_dev_token', token);
  } else {
    sessionStorage.removeItem('tikportal_dev_token');
  }
}

export async function loginWithOIDC() {
  if (userManager) {
    await userManager.signinRedirect();
  }
}

export async function handleAuthCallback(): Promise<OidcUser | null> {
  if (userManager && window.location.search.includes('code=')) {
    try {
      const user = await userManager.signinCallback();
      window.history.replaceState({}, document.title, window.location.pathname);
      return user || null;
    } catch (err) {
      console.error('OIDC callback failed', err);
    }
  }
  return null;
}

export async function logout(): Promise<void> {
  const wasDev = !!devToken;
  setDevToken(null);

  if (userManager) {
    try {
      await userManager.removeUser();
    } catch {
      // Ignore cleanup error
    }

    if (!wasDev) {
      try {
        await userManager.signoutRedirect();
        return;
      } catch (err) {
        console.warn('OIDC signout redirect failed, falling back to local reset:', err);
      }
    }
  }
}
