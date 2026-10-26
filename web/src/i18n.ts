import { i18n } from '@lingui/core';
import type { Messages } from '@lingui/core';
import { messages as enMessages } from './locales/en/messages.po';
import { messages as viMessages } from './locales/vi/messages.po';
import { messages as zhMessages } from './locales/zh/messages.po';
import { messages as jaMessages } from './locales/ja/messages.po';
import { messages as esMessages } from './locales/es/messages.po';
import { messages as frMessages } from './locales/fr/messages.po';
import { messages as deMessages } from './locales/de/messages.po';

export type SupportedLocale = 'en' | 'vi' | 'zh' | 'ja' | 'es' | 'fr' | 'de';

export const SUPPORTED_LOCALES: { code: SupportedLocale; label: string; flag: string }[] = [
  { code: 'en', label: 'English', flag: '🇺🇸' },
  { code: 'vi', label: 'Tiếng Việt', flag: '🇻🇳' },
  { code: 'zh', label: '简体中文', flag: '🇨🇳' },
  { code: 'ja', label: '日本語', flag: '🇯🇵' },
  { code: 'es', label: 'Español', flag: '🇪🇸' },
  { code: 'fr', label: 'Français', flag: '🇫🇷' },
  { code: 'de', label: 'Deutsch', flag: '🇩🇪' },
];

const CATALOGS: Record<SupportedLocale, Messages> = {
  en: enMessages,
  vi: viMessages,
  zh: zhMessages,
  ja: jaMessages,
  es: esMessages,
  fr: frMessages,
  de: deMessages,
};

const STORAGE_KEY = 'tikportal_locale';

export function detectInitialLocale(): SupportedLocale {
  if (typeof window !== 'undefined') {
    const saved = window.localStorage.getItem(STORAGE_KEY) as SupportedLocale | null;
    if (saved && CATALOGS[saved]) {
      return saved;
    }
    const navLang = (window.navigator.language || 'en').toLowerCase();
    for (const loc of SUPPORTED_LOCALES) {
      if (navLang.startsWith(loc.code)) {
        return loc.code;
      }
    }
  }
  return 'en';
}

export function activateLocale(locale: SupportedLocale) {
  const catalog = CATALOGS[locale] || CATALOGS.en;
  i18n.load(locale, catalog);
  i18n.activate(locale);
  if (typeof window !== 'undefined') {
    window.localStorage.setItem(STORAGE_KEY, locale);
    document.documentElement.lang = locale;
  }
}

// Initialize default locale immediately
activateLocale(detectInitialLocale());

export { i18n };
