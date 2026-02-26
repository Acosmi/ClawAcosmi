// 命令规则类型定义（从 security.rules.* API 返回）
export interface CommandRule {
    id: string;
    pattern: string;
    action: "allow" | "ask" | "deny";
    description: string;
    isPreset: boolean;
    priority: number;
    createdAt?: number;
}

export interface RuleTestResult {
    command: string;
    matched: boolean;
    action?: string;
    reason?: string;
    matchedRule?: {
        id: string;
        pattern: string;
        isPreset: boolean;
    };
}
