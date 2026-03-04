---
name: send-media
description: "发送文件/媒体到远程频道（飞书/Discord/Telegram/WhatsApp）"
tools: send_media
---

# send_media 工具 — 文件发送到远程频道

`send_media` 工具将本地文件或内存数据发送到当前对话频道或指定目标频道。

## send_media vs bash vs message

- **send_media**: 发送文件/图片/文档到远程频道（飞书/Discord/Telegram/WhatsApp）
- **bash**: 本地生成文件（无法发送到远程频道）
- **message**: 发送文本消息到频道（无法附带文件）

典型流程: bash（生成文件）→ send_media（投递到频道）

## 何时使用

- 用户要求"把这个文件发到群里"、"发送报告"、"转发文件"等
- 用 bash 生成了文件（PPT/PDF/图片等）后需要发送给用户
- 需要将截图、代码文件、文档等发到频道

## 参数

| 参数 | 必填 | 说明 |
|------|------|------|
| `file_path` | 二选一 | 本地文件路径。大文件优先使用此参数 |
| `media_base64` | 二选一 | Base64 编码数据。适合小型内存数据（如截图） |
| `target` | 可选 | `"channel:id"` 格式（如 `"feishu:oc_xxx"`）。省略时自动发到当前对话频道 |
| `mime_type` | 可选 | MIME 类型。使用 file_path 时自动从扩展名检测 |
| `message` | 可选 | 随文件一起发送的文字说明 |

## 支持的文件类型

| 类型 | 扩展名 | 说明 |
|------|--------|------|
| 办公文档 | .pdf .doc .docx .ppt .pptx .xls .xlsx | 飞书显示为文件卡片 |
| 图片 | .png .jpg .jpeg .gif | 飞书显示为图片消息 |
| 视频 | .mp4 | 飞书显示为视频 |
| 代码/文本 | .go .py .rs .ts .md .txt 等 | 以文件形式发送 |

大小限制: 30MB。

## 工作流示例

### 转发已有文件

```
用户: "帮我把 /workspace/report.pdf 发到这个群"

工具调用:
send_media(file_path="/workspace/report.pdf", message="这是报告文件")
```

### 生成文件后发送

```
用户: "帮我用 Python 生成一个简单的 PPT 然后发过来"

步骤 1 — bash:
pip install python-pptx && python3 -c "
from pptx import Presentation
prs = Presentation()
slide = prs.slides.add_slide(prs.slide_layouts[0])
slide.shapes.title.text = 'Hello World'
prs.save('/tmp/output.pptx')
print('Done')
"

步骤 2 — send_media:
send_media(file_path="/tmp/output.pptx", message="PPT 已生成")
```

### 发送代码文件

```
用户: "把 main.go 发给我看看"

工具调用:
send_media(file_path="/workspace/backend/cmd/acosmi/main.go")
```

## 注意事项

- `file_path` 优先于 `media_base64`（避免 base64 膨胀上下文）
- 不指定 `target` 时自动发到当前会话频道，无需 LLM 猜测目标
- MIME 类型从扩展名自动检测，通常不需要手动指定
- 仅在配置了频道（飞书/Discord 等）时可用，纯 Web 模式下此工具不存在
