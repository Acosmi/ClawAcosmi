import './globals.css';
import { I18nProvider } from '@/i18n';

export const metadata = {
    title: 'Argus — 24小时之眼',
    description: '智能体视觉皮层控制中心',
};

export default function RootLayout({
    children,
}: {
    children: React.ReactNode;
}) {
    return (
        <html lang="zh-CN">
            <body>
                <I18nProvider>{children}</I18nProvider>
            </body>
        </html>
    );
}
