// 安全级别信息类型（从 security.get API 返回）
export interface SecurityLevelInfo {
    id: string;
    label: string;
    labelZh: string;
    description: string;
    descriptionZh: string;
    risk: string;
    active: boolean;
}
