import { html, nothing, type TemplateResult } from "lit";
import type { ToolCard } from "../types/chat-types.ts";
import { icons } from "../icons.ts";
import { resolveToolDisplay, formatToolDetail } from "../tool-display.ts";
import { formatToolOutputForSidebar, getTruncatedPreview } from "./tool-helpers.ts";

// ---------- Coder 工具卡片渲染器 ----------

/**
 * 判断是否为 coder_ 前缀的工具。
 */
export function isCoderTool(name: string): boolean {
  return name.startsWith("coder_");
}

/**
 * 为 coder_* 工具渲染增强卡片。
 * - coder_edit: diff 预览 (红色-/绿色+)
 * - coder_write: 文件路径 + 行数
 * - coder_bash: 命令 monospace 预览
 * - 其他 coder 工具: 返回 null，使用默认渲染
 */
export function renderCoderCard(
  card: ToolCard,
  onOpenSidebar?: (content: string) => void,
): TemplateResult | null {
  const tool = card.name.replace("coder_", "");
  switch (tool) {
    case "edit":
      return renderEditCard(card, onOpenSidebar);
    case "write":
      return renderWriteCard(card, onOpenSidebar);
    case "bash":
      return renderBashCard(card, onOpenSidebar);
    default:
      return null; // 其他 coder 工具用默认渲染
  }
}

// ---------- 卡片渲染 ----------

function renderEditCard(
  card: ToolCard,
  onOpenSidebar?: (content: string) => void,
): TemplateResult {
  const display = resolveToolDisplay({ name: card.name, args: card.args });
  const args = card.args as Record<string, unknown> | undefined;
  const filePath = typeof args?.filePath === "string" ? args.filePath : "";
  const oldStr = typeof args?.oldString === "string" ? args.oldString : "";
  const newStr = typeof args?.newString === "string" ? args.newString : "";

  const canClick = Boolean(onOpenSidebar);
  const handleClick = canClick
    ? () => {
        if (card.text?.trim()) {
          onOpenSidebar!(formatToolOutputForSidebar(card.text!));
        } else {
          onOpenSidebar!(buildEditSidebarContent(filePath, oldStr, newStr));
        }
      }
    : undefined;

  // 生成简短 diff 预览
  const diffPreview = buildCompactDiff(oldStr, newStr, 3);

  return html`
    <div
      class="chat-tool-card coder-card ${canClick ? "chat-tool-card--clickable" : ""}"
      @click=${handleClick}
      role=${canClick ? "button" : nothing}
      tabindex=${canClick ? "0" : nothing}
    >
      <div class="chat-tool-card__header">
        <div class="chat-tool-card__title">
          <span class="chat-tool-card__icon">${icons[display.icon]}</span>
          <span>${display.label}</span>
        </div>
        ${canClick
          ? html`<span class="chat-tool-card__action">View ${icons.check}</span>`
          : nothing}
      </div>
      ${filePath
        ? html`<div class="chat-tool-card__detail coder-file-header">${filePath}</div>`
        : nothing}
      ${diffPreview
        ? html`<div class="coder-diff-preview mono">${diffPreview}</div>`
        : nothing}
    </div>
  `;
}

function renderWriteCard(
  card: ToolCard,
  onOpenSidebar?: (content: string) => void,
): TemplateResult {
  const display = resolveToolDisplay({ name: card.name, args: card.args });
  const args = card.args as Record<string, unknown> | undefined;
  const filePath = typeof args?.filePath === "string" ? args.filePath : "";
  const content = typeof args?.content === "string" ? args.content : "";
  const lineCount = content ? content.split("\n").length : 0;

  const canClick = Boolean(onOpenSidebar);
  const handleClick = canClick
    ? () => {
        if (card.text?.trim()) {
          onOpenSidebar!(formatToolOutputForSidebar(card.text!));
        } else {
          const preview = content.length > 2000 ? content.slice(0, 2000) + "\n..." : content;
          onOpenSidebar!(`## Coder Write\n\n**File:** \`${filePath}\`\n**Lines:** ${lineCount}\n\n\`\`\`\n${preview}\n\`\`\``);
        }
      }
    : undefined;

  return html`
    <div
      class="chat-tool-card coder-card ${canClick ? "chat-tool-card--clickable" : ""}"
      @click=${handleClick}
      role=${canClick ? "button" : nothing}
      tabindex=${canClick ? "0" : nothing}
    >
      <div class="chat-tool-card__header">
        <div class="chat-tool-card__title">
          <span class="chat-tool-card__icon">${icons[display.icon]}</span>
          <span>${display.label}</span>
        </div>
        ${canClick
          ? html`<span class="chat-tool-card__action">View ${icons.check}</span>`
          : nothing}
      </div>
      ${filePath
        ? html`<div class="chat-tool-card__detail coder-file-header">${filePath}</div>`
        : nothing}
      ${lineCount > 0
        ? html`<div class="chat-tool-card__detail muted">${lineCount} lines</div>`
        : nothing}
    </div>
  `;
}

function renderBashCard(
  card: ToolCard,
  onOpenSidebar?: (content: string) => void,
): TemplateResult {
  const display = resolveToolDisplay({ name: card.name, args: card.args });
  const detail = formatToolDetail(display);
  const args = card.args as Record<string, unknown> | undefined;
  const command = typeof args?.command === "string" ? args.command : "";
  const hasText = Boolean(card.text?.trim());

  const canClick = Boolean(onOpenSidebar);
  const handleClick = canClick
    ? () => {
        if (hasText) {
          onOpenSidebar!(formatToolOutputForSidebar(card.text!));
        } else {
          onOpenSidebar!(`## Coder Bash\n\n\`\`\`bash\n${command}\n\`\`\`\n\n*No output — command completed successfully.*`);
        }
      }
    : undefined;

  return html`
    <div
      class="chat-tool-card coder-card ${canClick ? "chat-tool-card--clickable" : ""}"
      @click=${handleClick}
      role=${canClick ? "button" : nothing}
      tabindex=${canClick ? "0" : nothing}
    >
      <div class="chat-tool-card__header">
        <div class="chat-tool-card__title">
          <span class="chat-tool-card__icon">${icons[display.icon]}</span>
          <span>${display.label}</span>
        </div>
        ${canClick
          ? html`<span class="chat-tool-card__action">${hasText ? "View" : ""} ${icons.check}</span>`
          : nothing}
      </div>
      ${command
        ? html`<div class="coder-command-mono mono">${truncate(command, 120)}</div>`
        : nothing}
      ${hasText
        ? html`<div class="chat-tool-card__preview mono">${getTruncatedPreview(card.text!)}</div>`
        : nothing}
    </div>
  `;
}

// ---------- 辅助函数 ----------

/**
 * 构建压缩 diff 预览：显示最多 maxLines 行变更。
 */
function buildCompactDiff(
  oldStr: string,
  newStr: string,
  maxLines: number,
): TemplateResult | null {
  if (!oldStr && !newStr) return null;

  const oldLines = oldStr.split("\n").slice(0, maxLines);
  const newLines = newStr.split("\n").slice(0, maxLines);

  const parts: TemplateResult[] = [];

  for (const line of oldLines) {
    parts.push(html`<div class="coder-diff-del">- ${truncate(line, 80)}</div>`);
  }
  if (oldStr.split("\n").length > maxLines) {
    parts.push(html`<div class="coder-diff-context">  ...</div>`);
  }
  for (const line of newLines) {
    parts.push(html`<div class="coder-diff-add">+ ${truncate(line, 80)}</div>`);
  }
  if (newStr.split("\n").length > maxLines) {
    parts.push(html`<div class="coder-diff-context">  ...</div>`);
  }

  return html`${parts}`;
}

/**
 * 构建编辑侧边栏 markdown 内容。
 */
function buildEditSidebarContent(
  filePath: string,
  oldStr: string,
  newStr: string,
): string {
  let md = `## Coder Edit\n\n**File:** \`${filePath}\`\n\n`;
  if (oldStr) {
    md += `**Remove:**\n\`\`\`\n${oldStr}\n\`\`\`\n\n`;
  }
  if (newStr) {
    md += `**Add:**\n\`\`\`\n${newStr}\n\`\`\`\n`;
  }
  return md;
}

function truncate(s: string, maxLen: number): string {
  if (s.length <= maxLen) return s;
  return s.slice(0, maxLen) + "...";
}
