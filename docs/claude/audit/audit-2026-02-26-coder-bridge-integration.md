---
document_type: Audit
status: In Progress
created: 2026-02-26
last_updated: 2026-02-26
audit_report: self
skill5_verified: false
---

# Audit Report: CoderBridge Go-Side Integration

## Metadata

| Field | Value |
|---|---|
| Component | CoderBridge (Go backend gateway + runner integration) |
| Audit Date | 2026-02-26 |
| Auditor | Claude Opus 4.6 |
| Trigger | Skill 4 manual audit request |
| Verdict | **CONDITIONAL PASS** (1 HIGH, 3 MEDIUM, 3 LOW, 5 INFO) |

## Scope

| File | Lines Reviewed | Focus |
|---|---|---|
| `backend/internal/agents/runner/attempt_runner.go` | 1-720 | CoderBridgeForAgent interface, tool registration, permission-denied retry path |
| `backend/internal/agents/runner/tool_executor.go` | 1-713 | CoderBridge field in ToolExecParams, executeCoderTool, coder_ dispatch |
| `backend/internal/gateway/boot.go` | 1-460 | CoderBridge in GatewayState, Start lifecycle, resolveCoderBinaryPath |
| `backend/internal/gateway/server.go` | 60-660 | coderBridgeAdapter, injection into AttemptRunner, StopCoder in Close |

---

## Findings

### Finding F-01
- **Location**: `attempt_runner.go:326` (permission-denied retry block)
- **Severity**: HIGH
- **Category**: Correctness
- **Description**: In the permission-denied retry path (lines 310-343), the `ToolExecParams` struct is missing `CoderBridge: r.CoderBridge`. The normal execution path at line 265 correctly includes `CoderBridge: r.CoderBridge`, but the retry path at line 316 omits it. This means if a coder tool call triggers in the same batch as a permission-denied tool, and the user approves the escalation, the retry will have `CoderBridge == nil`. The dispatch at `tool_executor.go:139` checks `params.CoderBridge != nil`, so the coder tool call will fall through to the `default` case and return `[Tool "coder_XXX" is not yet implemented]` instead of being executed.
- **Evidence**:
  ```go
  // Line 316-328 (retry path) — MISSING CoderBridge
  output, toolErr := ExecuteToolCall(ctx, tc.Name, tc.Input, ToolExecParams{
      WorkspaceDir:       params.WorkspaceDir,
      TimeoutMs:          params.TimeoutMs,
      AllowWrite:         secLvl == "full" || secLvl == "allowlist",
      AllowExec:          secLvl == "full" || secLvl == "allowlist",
      SandboxMode:        secLvl == "allowlist",
      Rules:              resolveCommandRules(),
      SecurityLevel:      secLvl,
      OnPermissionDenied: params.OnPermissionDenied,
      ArgusBridge:        r.ArgusBridge,
      RemoteMCPBridge:    r.RemoteMCPBridge,   // present
      NativeSandbox:      r.NativeSandbox,
      SkillsCache:        r.skillsCache,
      // CoderBridge: r.CoderBridge,  <-- MISSING
  })
  ```
  Compare with the normal path at line 255-268 which correctly includes `CoderBridge: r.CoderBridge`.
- **Recommendation**: Add `CoderBridge: r.CoderBridge,` to the retry ToolExecParams at line 326, between `ArgusBridge` and `RemoteMCPBridge`.

---

### Finding F-02
- **Location**: `tool_executor.go:680` (executeCoderTool)
- **Severity**: MEDIUM
- **Category**: Security
- **Description**: Empty tool name after prefix stripping. If the LLM emits a tool call with name exactly `"coder_"` (no suffix), `strings.TrimPrefix(name, "coder_")` yields `""`, which is passed to `AgentCallTool`. The MCP client will send a `tools/call` request with `name: ""`, which is technically valid JSON but semantically invalid. The MCP server behavior is undefined for empty tool names. The same issue exists for `argus_` and `remote_` but is documented here for coder specifically.
- **Evidence**:
  ```go
  func executeCoderTool(ctx context.Context, name string, inputJSON json.RawMessage, params ToolExecParams) (string, error) {
      mcpToolName := strings.TrimPrefix(name, "coder_")
      // mcpToolName could be "" if name == "coder_"
      slog.Debug("coder tool call", "tool", mcpToolName)
      output, err := params.CoderBridge.AgentCallTool(ctx, mcpToolName, inputJSON, 30*time.Second)
  ```
- **Recommendation**: Add a guard after TrimPrefix: `if mcpToolName == "" { return "[Coder tool error: empty tool name]", nil }`. Apply the same pattern to executeArgusTool and executeRemoteTool for consistency.

---

### Finding F-03
- **Location**: `tool_executor.go:684` (executeCoderTool hardcoded 30s timeout)
- **Severity**: MEDIUM
- **Category**: Correctness
- **Description**: The timeout for coder tool calls is hardcoded to `30*time.Second`. Coder tools are likely to perform code generation, compilation, or test execution, which can legitimately take longer than 30 seconds. The same 30s hardcoding applies to argus and remote tools. However, for a coding sub-agent, this is especially likely to be insufficient. The parent context (`ctx`) may have a higher timeout (up to 5 minutes from `attempt_runner.go:135`), but the inner 30s timeout created by the bridge's CallTool will trigger first.
- **Evidence**:
  ```go
  output, err := params.CoderBridge.AgentCallTool(ctx, mcpToolName, inputJSON, 30*time.Second)
  ```
  In `mcpclient/client.go:202`:
  ```go
  ctx, cancel := context.WithTimeout(ctx, timeout)  // Creates child ctx with 30s
  ```
- **Recommendation**: Either (a) derive the timeout from `params.TimeoutMs` (e.g., `time.Duration(params.TimeoutMs)*time.Millisecond` capped at a maximum), or (b) make the timeout configurable per bridge type, or (c) at minimum increase the coder timeout to 120s given the nature of coding tasks.

---

### Finding F-04
- **Location**: `tool_executor.go:686` (error message format)
- **Severity**: LOW
- **Category**: Security (Information Leakage)
- **Description**: The error format `[Coder tool error: %s]` exposes the full error message from the MCP client, which may include internal details such as file paths, process IDs, or stack traces from the coder subprocess. This text is returned to the LLM as tool output and may be echoed to the user.
- **Evidence**:
  ```go
  if err != nil {
      return fmt.Sprintf("[Coder tool error: %s]", err), nil
  }
  ```
  The underlying bridge error from `bridge.go:387` can include messages like:
  ```
  argus: bridge not available (state: degraded)
  mcpclient: tools/call coder_write error -32602: invalid params
  ```
- **Recommendation**: This is consistent with the argus (`[Argus tool error: %s]`) and remote (`[Remote tool error: %s]`) patterns, so the risk is accepted across all bridges. For defense-in-depth, consider sanitizing the error to remove filesystem paths before exposing to the LLM. Severity is LOW because the LLM sees it, not an untrusted external party.

---

### Finding F-05
- **Location**: `boot.go:171` (coderBinaryPath empty string gate)
- **Severity**: LOW
- **Category**: Correctness
- **Description**: The coder initialization gate uses `if coderBinaryPath != ""` (line 171), which is safe because `resolveCoderBinaryPath()` correctly returns `""` when no binary is found. However, this pattern differs from Argus which uses `argus.IsAvailable(argusPath)` (line 140) for a more robust check. The coder path does validate existence via `os.Stat` in each resolution step, but if an environment variable `OA_CODER_BINARY` is set to a non-existent path, `resolveCoderBinaryPath` will skip it (line 403: `os.Stat` fails) and try the next option, which is correct. The only subtle issue: if `resolveCoderBinaryPath` returns a non-empty string from `exec.LookPath("openacosmi")` (line 420), the binary exists but might not support the `coder start` subcommand (e.g., it could be an older version).
- **Evidence**:
  ```go
  // boot.go:170-171 — Coder uses simple string check
  coderBinaryPath := resolveCoderBinaryPath()
  if coderBinaryPath != "" {

  // boot.go:139-140 — Argus uses dedicated IsAvailable check
  argusPath := resolveArgusBinaryPath()
  if argus.IsAvailable(argusPath) {
  ```
- **Recommendation**: Consider adding a lightweight probe (e.g., `exec.Command(coderBinaryPath, "coder", "--help").Run()`) or a dedicated `IsCoderAvailable()` function analogous to `argus.IsAvailable()`. Low priority since the Bridge.Start() at line 187 will fail gracefully if the binary does not support the subcommand.

---

### Finding F-06
- **Location**: `server.go:178-198` (coderBridgeAdapter.AgentCallTool content extraction)
- **Severity**: MEDIUM
- **Category**: Correctness
- **Description**: The `coderBridgeAdapter.AgentCallTool` only extracts `"text"` content type from MCP results. Unlike `argusBridgeAdapter` which also handles `"image"` content (line 144-148), the coder adapter silently drops `"image"` and any future content types (e.g., `"resource"`, `"embedded_resource"` from MCP spec 2025-03-26). Coder tools may return rich content such as syntax-highlighted code blocks, diff outputs, or even diagrams. More importantly, if any content block is not `"text"`, it will be silently lost with no indication.
- **Evidence**:
  ```go
  // argusBridgeAdapter handles both text and image:
  case "text":
      sb.WriteString(c.Text)
  case "image":
      sb.WriteString(fmt.Sprintf("[image: %s, %d bytes base64]", c.MIME, len(c.Data)))

  // coderBridgeAdapter only handles text:
  case "text":
      sb.WriteString(c.Text)
  // image, resource, etc. → silently dropped
  ```
- **Recommendation**: At minimum, add a fallback case to log or include a placeholder for unrecognized content types: `default: sb.WriteString(fmt.Sprintf("[%s content: %d bytes]", c.Type, len(c.Data)+len(c.Text)))`. Consider whether coder tools might return `"image"` type (e.g., rendered diagrams) and handle accordingly.

---

### Finding F-07
- **Location**: `boot.go:174` (hardcoded workspace `"."`)
- **Severity**: LOW
- **Category**: Correctness
- **Description**: The coder bridge Args include `--workspace .` (relative current directory), while the actual workspace is determined dynamically per-request in the AttemptRunner. The `"."` value means the coder MCP subprocess will use whatever directory it was spawned from, which is the gateway process's CWD, not the user's project workspace. If the coder subprocess uses this `--workspace` argument to determine where to read/write code, it will operate on the wrong directory.
- **Evidence**:
  ```go
  coderCfg := argus.BridgeConfig{
      BinaryPath:     coderBinaryPath,
      Args:           []string{"coder", "start", "--workspace", "."},
      // "." resolves to gateway's CWD, not user's project
  ```
- **Recommendation**: This depends on how the `openacosmi coder start` command interprets `--workspace`. If the workspace is re-specified per-tool-call via the MCP tool arguments, this is benign. If it's used as the subprocess's working directory, it should be set to a meaningful default (e.g., omitted entirely to let the coder determine it, or injected dynamically). Verify against the Rust CLI's `coder start` implementation.

---

### Finding F-08
- **Location**: `attempt_runner.go:46-48` (CoderBridgeForAgent reuses ArgusToolDef)
- **Severity**: INFO
- **Category**: Integration Consistency
- **Description**: `CoderBridgeForAgent` returns `[]ArgusToolDef` from `AgentTools()`. This type reuse is pragmatic (the structs are identical: Name + Description + InputSchema), but the naming creates a conceptual coupling: coder tools are described by "Argus" tool definitions. This is not a bug, but it can confuse future maintainers who may wonder why coder tools use Argus types.
- **Evidence**:
  ```go
  type CoderBridgeForAgent interface {
      AgentTools() []ArgusToolDef  // <-- Argus-named type for Coder
      AgentCallTool(ctx context.Context, name string, args json.RawMessage, timeout time.Duration) (string, error)
  }
  ```
- **Recommendation**: Consider extracting a generic type alias: `type BridgeToolDef = ArgusToolDef` or `type MCPToolDef = ArgusToolDef` in `attempt_runner.go`, and have both interfaces return that. This is purely a readability improvement with no functional impact. INFO severity only.

---

### Finding F-09
- **Location**: `tool_executor.go:136-144` (dispatch order for prefixed tools)
- **Severity**: INFO
- **Category**: Security (Prefix Collision)
- **Description**: The dispatch checks `argus_` before `coder_` before `remote_`. Since all three prefixes are distinct (`argus_`, `coder_`, `remote_`), there is no collision risk. However, if a coder MCP server registers a tool named `argus_something`, after prefixing it becomes `coder_argus_something`, which will match `coder_` (correct). But if someone manually registers a tool with prefix overlap (e.g., a native tool named `coder_bash`), it would shadow the built-in bash tool. This is prevented by the `default:` placement - `coder_` names only reach the default branch.
- **Evidence**:
  ```go
  default:
      if strings.HasPrefix(name, "argus_") && params.ArgusBridge != nil {
          return executeArgusTool(...)
      }
      if strings.HasPrefix(name, "coder_") && params.CoderBridge != nil {
          return executeCoderTool(...)
      }
      if strings.HasPrefix(name, "remote_") && params.RemoteMCPBridge != nil {
          return executeRemoteTool(...)
      }
  ```
  The built-in tools (bash, read_file, write_file, list_dir, search, glob, lookup_skill) are matched by explicit `case` before the `default:` branch, so they cannot be shadowed by prefixed tool names.
- **Recommendation**: No action required. The dispatch order is safe. For documentation, note that the three prefixes `argus_`, `coder_`, `remote_` are reserved namespace prefixes and must never overlap with each other or with built-in tool names.

---

### Finding F-10
- **Location**: `server.go:60-107` (Close shutdown order)
- **Severity**: INFO
- **Category**: Resource Safety
- **Description**: The shutdown order in `Close()` is: Sandbox -> NativeSandbox -> Argus -> **Coder** -> RemoteMCP -> UHMS -> HTTP. This order is reasonable: bridges are stopped before the HTTP server, preventing new requests from reaching a stopped bridge. Coder is stopped after Argus but before RemoteMCP, which is consistent with their independence (no inter-bridge dependencies). However, there is no timeout on bridge Stop calls. If `bridge.Stop()` hangs (e.g., waiting for a subprocess that ignores SIGTERM), the entire shutdown blocks indefinitely.
- **Evidence**:
  ```go
  rt.State.StopSandbox()        // 1. Docker sandbox
  rt.State.StopNativeSandbox()  // 2. Native sandbox
  rt.State.StopArgus()          // 3. Argus visual
  rt.State.StopCoder()          // 4. Coder coding
  rt.State.StopRemoteMCP()      // 5. Remote MCP
  rt.State.StopUHMS()           // 6. UHMS memory
  ```
  In `argus/bridge.go:394-420`, `Stop()` sends SIGTERM, waits up to 2s, then SIGKILL + Wait(), which should bound the stop time. So this is bounded in practice.
- **Recommendation**: No immediate action. The argus.Bridge.Stop() already has a 2s timeout with SIGKILL escalation. Document that all bridge Stop methods must complete within a bounded time to prevent shutdown hangs.

---

### Finding F-11
- **Location**: `boot.go:172-185` (BridgeConfig without MaxRestartAttempts)
- **Severity**: INFO
- **Category**: Integration Consistency
- **Description**: The coder BridgeConfig at line 172-185 is constructed directly (struct literal) rather than using `argus.DefaultBridgeConfig()`. While it sets `BinaryPath`, `Args`, `HealthInterval`, and `OnStateChange`, it does not set any `MaxRestartAttempts` or similar fields (because `BridgeConfig` does not have such a field -- restart logic is hardcoded in the bridge). The Argus initialization at line 146-158 also constructs via `argus.DefaultBridgeConfig()` + overrides. The coder path bypasses the default config, but since `BridgeConfig` only has 4 fields and all are explicitly set, this is equivalent. The only difference: DefaultBridgeConfig sets `Args: []string{"-mcp"}` which coder correctly overrides.
- **Evidence**:
  ```go
  // Argus: uses DefaultBridgeConfig + overrides
  cfg := argus.DefaultBridgeConfig()
  cfg.BinaryPath = argusPath
  cfg.OnStateChange = func(...)  { ... }

  // Coder: direct struct literal
  coderCfg := argus.BridgeConfig{
      BinaryPath:     coderBinaryPath,
      Args:           []string{"coder", "start", "--workspace", "."},
      HealthInterval: 30 * time.Second,
      OnStateChange:  func(...) { ... },
  }
  ```
- **Recommendation**: Minor style inconsistency. Consider using `argus.DefaultBridgeConfig()` for coder as well, then override the fields that differ. This ensures any future default fields are inherited. INFO only.

---

### Finding F-12
- **Location**: `server.go:178-182` (coderBridgeAdapter error propagation)
- **Severity**: INFO
- **Category**: Correctness
- **Description**: When `bridge.CallTool` returns an error, `coderBridgeAdapter.AgentCallTool` returns `("", err)`. This error propagates to `executeCoderTool` at `tool_executor.go:685`, where it is caught and formatted as `[Coder tool error: %s]`. However, the Argus adapter has the same pattern. The subtle point: if the bridge is in `degraded` state, CallTool still accepts calls (line `bridge.go:386`: `state != BridgeStateReady && state != BridgeStateDegraded` returns error only if both are false). This means coder tool calls can go through in degraded state, which may have higher failure rates but is the desired behavior for graceful degradation.
- **Evidence**:
  ```go
  // bridge.go:386 — allows calls in degraded state
  if client == nil || (state != BridgeStateReady && state != BridgeStateDegraded) {
      return nil, fmt.Errorf("argus: bridge not available (state: %s)", state)
  }
  ```
- **Recommendation**: No action needed. Degraded state call-through is intentional for resilience. The error message still says "argus:" even when used for coder (the bridge is shared code), but this only appears in logs and error paths, not user-facing text. The outer `[Coder tool error: ...]` wrapper correctly identifies the origin.

---

## Summary Matrix

| ID | Severity | Category | Description | Status |
|---|---|---|---|---|
| F-01 | **HIGH** | Correctness | CoderBridge missing from permission-denied retry path | **OPEN** |
| F-02 | MEDIUM | Security | Empty tool name after prefix stripping not guarded | OPEN |
| F-03 | MEDIUM | Correctness | 30s hardcoded timeout too short for coding tasks | OPEN |
| F-06 | MEDIUM | Correctness | Only "text" content type extracted, others silently dropped | OPEN |
| F-04 | LOW | Security | Error message may leak internal details to LLM | OPEN |
| F-05 | LOW | Correctness | Binary path resolution differs from Argus pattern | OPEN |
| F-07 | LOW | Correctness | Hardcoded workspace "." may not match user project | OPEN |
| F-08 | INFO | Integration | ArgusToolDef reused for coder (naming confusion) | NOTE |
| F-09 | INFO | Security | Prefix dispatch order safe, no collision risk | NOTE |
| F-10 | INFO | Resource | Shutdown order correct, bounded by bridge Stop timeout | NOTE |
| F-11 | INFO | Integration | BridgeConfig constructed directly vs DefaultBridgeConfig | NOTE |
| F-12 | INFO | Correctness | Degraded state call-through is intentional | NOTE |

## Verdict

**CONDITIONAL PASS** -- The integration follows established patterns correctly for the most part.

**Must-fix before archive** (blocks archive gate):
- **F-01 (HIGH)**: Add `CoderBridge: r.CoderBridge` to the permission-denied retry path. This is a functional bug that causes coder tools to silently fail after permission escalation approval.

**Should-fix** (recommended but does not block archive):
- **F-02 (MEDIUM)**: Guard against empty tool name after prefix stripping.
- **F-03 (MEDIUM)**: Increase or make configurable the 30s timeout for coder tools.
- **F-06 (MEDIUM)**: Handle non-text content types in coderBridgeAdapter (at least log them).

**Nice-to-have** (LOW/INFO findings):
- F-04, F-05, F-07, F-08, F-11: Minor consistency and defense-in-depth improvements.
