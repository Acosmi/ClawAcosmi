import type { GatewayBrowserClient } from "../gateway.ts";
import type { CommandRule, RuleTestResult } from "./rules-types.ts";

// ---------- State 类型 ----------

export type { CommandRule, RuleTestResult };

export interface RulesState {
    client: GatewayBrowserClient | null;
    connected: boolean;
    rulesLoading: boolean;
    rulesError: string | null;
    rules: CommandRule[];
    presetCount: number;
    userCount: number;
    // 添加规则表单
    addFormOpen: boolean;
    addPattern: string;
    addAction: "allow" | "ask" | "deny";
    addDescription: string;
    // 命令测试
    testCommand: string;
    testResult: RuleTestResult | null;
    testLoading: boolean;
}

// ---------- 加载规则列表 ----------

export async function loadRules(state: RulesState): Promise<void> {
    if (!state.client || !state.connected) return;
    state.rulesLoading = true;
    state.rulesError = null;
    try {
        const result = await state.client.request<{
            rules: CommandRule[];
            total: number;
            presetCount: number;
            userCount: number;
        }>("security.rules.list", {});
        state.rules = result.rules ?? [];
        state.presetCount = result.presetCount ?? 0;
        state.userCount = result.userCount ?? 0;
    } catch (err) {
        state.rulesError = err instanceof Error ? err.message : String(err);
    } finally {
        state.rulesLoading = false;
    }
}

// ---------- 添加规则 ----------

export async function addRule(state: RulesState): Promise<void> {
    if (!state.client || !state.connected) return;
    if (!state.addPattern.trim()) return;
    state.rulesLoading = true;
    state.rulesError = null;
    try {
        await state.client.request("security.rules.add", {
            pattern: state.addPattern.trim(),
            action: state.addAction,
            description: state.addDescription.trim(),
        });
        // 重置表单
        state.addPattern = "";
        state.addAction = "deny";
        state.addDescription = "";
        state.addFormOpen = false;
        // 刷新列表
        await loadRules(state);
    } catch (err) {
        state.rulesError = err instanceof Error ? err.message : String(err);
    } finally {
        state.rulesLoading = false;
    }
}

// ---------- 删除规则 ----------

export async function removeRule(
    state: RulesState,
    ruleId: string,
): Promise<void> {
    if (!state.client || !state.connected) return;
    state.rulesLoading = true;
    state.rulesError = null;
    try {
        await state.client.request("security.rules.remove", { id: ruleId });
        await loadRules(state);
    } catch (err) {
        state.rulesError = err instanceof Error ? err.message : String(err);
    } finally {
        state.rulesLoading = false;
    }
}

// ---------- 测试命令 ----------

export async function testRule(state: RulesState): Promise<void> {
    if (!state.client || !state.connected) return;
    if (!state.testCommand.trim()) return;
    state.testLoading = true;
    state.testResult = null;
    try {
        const result = await state.client.request<RuleTestResult>(
            "security.rules.test",
            { command: state.testCommand.trim() },
        );
        state.testResult = result;
    } catch (err) {
        state.rulesError = err instanceof Error ? err.message : String(err);
    } finally {
        state.testLoading = false;
    }
}
