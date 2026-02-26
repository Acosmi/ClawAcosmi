'use client';

import { createContext, useContext, useState, useCallback, useEffect, type ReactNode } from 'react';
import zh, { type LangKeys } from './zh';
import en from './en';

/** 支持的语言类型 */
export type Lang = 'zh' | 'en';

/** 语言字典映射 */
const dictionaries: Record<Lang, Record<LangKeys, string>> = { zh, en };

/** localStorage 持久化键名 */
const STORAGE_KEY = 'argus_lang';

/** i18n 上下文类型 */
interface I18nContextType {
    lang: Lang;
    setLang: (lang: Lang) => void;
    t: (key: LangKeys) => string;
}

const I18nContext = createContext<I18nContextType>({
    lang: 'zh',
    setLang: () => { },
    t: (key) => zh[key],
});

/** 获取 i18n hook */
export function useI18n() {
    return useContext(I18nContext);
}

/** 快捷翻译 hook */
export function useT() {
    const { t } = useContext(I18nContext);
    return t;
}

/** 获取当前语言 hook */
export function useLang() {
    const { lang, setLang } = useContext(I18nContext);
    return { lang, setLang };
}

/** i18n Provider 组件 */
export function I18nProvider({ children }: { children: ReactNode }) {
    const [lang, setLangState] = useState<Lang>('zh');

    // 从 localStorage 恢复语言设置
    useEffect(() => {
        const saved = localStorage.getItem(STORAGE_KEY);
        if (saved === 'en' || saved === 'zh') {
            setLangState(saved);
        }
    }, []);

    const setLang = useCallback((newLang: Lang) => {
        setLangState(newLang);
        localStorage.setItem(STORAGE_KEY, newLang);
        document.documentElement.lang = newLang === 'zh' ? 'zh-CN' : 'en';
    }, []);

    const t = useCallback((key: LangKeys): string => {
        return dictionaries[lang][key] ?? zh[key] ?? key;
    }, [lang]);

    return (
        <I18nContext.Provider value= {{ lang, t, setLang }
}>
    { children }
    </I18nContext.Provider>
  );
}

export type { LangKeys };
