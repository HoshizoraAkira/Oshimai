import React, { createContext, useContext, useState, useEffect, ReactNode } from 'react';
import enDict from '../locales/en.json';
import idDict from '../locales/id.json';
import jpDict from '../locales/jp.json';

const dictionaries: Record<string, any> = {
  en: enDict,
  id: idDict,
  jp: jpDict,
  ja: jpDict,
};

export interface LanguageOption {
  code: string;
  label: string;
  name: string;
}

export const LANGUAGES: LanguageOption[] = [
  { code: 'id', label: 'ID', name: 'Bahasa Indonesia' },
  { code: 'en', label: 'EN', name: 'English' },
  { code: 'jp', label: 'JP', name: '日本語' },
];

export interface I18nContextValue {
  lang: string;
  setLang: (newLang: string) => void;
  t: (key: string, fallback?: string, params?: any[] | Record<string, any>) => string;
  languages: LanguageOption[];
  localeCode: string;
  formatDate: (date: string | number | Date | null | undefined, options?: Intl.DateTimeFormatOptions) => string;
  formatTime: (date: string | number | Date | null | undefined, options?: Intl.DateTimeFormatOptions) => string;
  formatDateTime: (date: string | number | Date | null | undefined, options?: Intl.DateTimeFormatOptions) => string;
}

const I18nContext = createContext<I18nContextValue | null>(null);

function getNestedValue(obj: any, keyPath: string): string | null {
  if (!obj || !keyPath) return null;
  const parts = keyPath.split('.');
  let curr = obj;
  for (const part of parts) {
    if (curr && typeof curr === 'object' && part in curr) {
      curr = curr[part];
    } else {
      return null;
    }
  }
  if (Array.isArray(curr)) return curr.join(', ');
  return typeof curr === 'string' ? curr : null;
}

export function I18nProvider({ children }: { children: ReactNode }) {
  const [lang, setLangState] = useState<string>(() => {
    return localStorage.getItem('oshimai_lang') || 'id';
  });

  const setLang = (newLang: string) => {
    const code = newLang === 'ja' ? 'jp' : newLang;
    setLangState(code);
    localStorage.setItem('oshimai_lang', code);
    document.documentElement.lang = code;
  };

  useEffect(() => {
    document.documentElement.lang = lang;
  }, [lang]);

  const t = (key: string, fallback?: string, params?: any[] | Record<string, any>): string => {
    const activeDict = dictionaries[lang] || dictionaries.en;
    let text = getNestedValue(activeDict, key);

    if (!text && lang !== 'en') {
      text = getNestedValue(dictionaries.en, key);
    }

    if (!text) {
      text = fallback !== undefined ? fallback : key;
    }

    if (params && typeof params === 'object') {
      if (Array.isArray(params)) {
        let i = 0;
        text = text.replace(/%s/g, () => String(params[i++] ?? ''));
      } else {
        Object.keys(params).forEach(p => {
          text = text.replace(new RegExp(`{${p}}`, 'g'), String(params[p]));
        });
      }
    }

    return text;
  };

  const localeCode = lang === 'id' ? 'id-ID' : (lang === 'jp' || lang === 'ja' ? 'ja-JP' : 'en-US');

  const formatDate = (date: string | number | Date | null | undefined, options?: Intl.DateTimeFormatOptions): string => {
    if (!date) return '-';
    try {
      return new Date(date).toLocaleDateString(localeCode, options);
    } catch {
      return String(date);
    }
  };

  const formatTime = (date: string | number | Date | null | undefined, options?: Intl.DateTimeFormatOptions): string => {
    if (!date) return '-';
    try {
      return new Date(date).toLocaleTimeString(localeCode, options);
    } catch {
      return String(date);
    }
  };

  const formatDateTime = (date: string | number | Date | null | undefined, options?: Intl.DateTimeFormatOptions): string => {
    if (!date) return '-';
    try {
      return new Date(date).toLocaleString(localeCode, options);
    } catch {
      return String(date);
    }
  };

  return (
    <I18nContext.Provider value={{ lang, setLang, t, languages: LANGUAGES, localeCode, formatDate, formatTime, formatDateTime }}>
      {children}
    </I18nContext.Provider>
  );
}

export function useTranslation(): I18nContextValue {
  const ctx = useContext(I18nContext);
  if (!ctx) {
    return {
      lang: 'en',
      localeCode: 'en-US',
      setLang: () => {},
      t: (k, fallback) => fallback || k,
      languages: LANGUAGES,
      formatDate: (d) => d ? new Date(d).toLocaleDateString('en-US') : '-',
      formatTime: (d) => d ? new Date(d).toLocaleTimeString('en-US') : '-',
      formatDateTime: (d) => d ? new Date(d).toLocaleString('en-US') : '-',
    };
  }
  return ctx;
}

export const useI18n = useTranslation;
