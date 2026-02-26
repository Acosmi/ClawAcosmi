'use client';

import { useLang } from '@/i18n';

/**
 * 语言切换按钮组件
 * 在顶部导航栏显示为小型切换按钮
 */
export default function LangSwitch() {
    const { lang, setLang } = useLang();

    return (
        <div className="lang-switch">
            <button
                className={`lang-btn ${lang === 'zh' ? 'active' : ''}`}
                onClick={() => setLang('zh')}
            >
                中文
            </button>
            <button
                className={`lang-btn ${lang === 'en' ? 'active' : ''}`}
                onClick={() => setLang('en')}
            >
                EN
            </button>
        </div>
    );
}
