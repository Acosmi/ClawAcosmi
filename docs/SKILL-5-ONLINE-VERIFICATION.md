# Skill 5: Mandatory Pre-Planning Online Verification

> Plan from facts, not from memory. Verify before you build.

## Rationale

LLM training data has a knowledge cutoff and may contain outdated or incorrect
information about OS APIs, crate interfaces, and platform behaviors. For a
security-critical sandbox, building on stale assumptions can lead to broken isolation,
deprecated syscall usage, or incompatible API calls. Online verification eliminates
this class of errors.

## Rules

### 5.1 Verification Trigger

Online verification is **mandatory** before writing code when the task involves:

| Category | Examples |
|---|---|
| OS system calls | `clone3`, `pidfd_open`, `unshare`, `seccomp`, `sandbox_init` |
| Platform security APIs | macOS Seatbelt/Sandbox, Linux namespaces, Windows Job Objects |
| Kernel features | cgroups v2, eBPF, io_uring, landlock |
| Crate APIs | New crate adoption, major version upgrades, deprecated features |
| Container runtime | OCI spec, runc/crun behavior, rootless containers |
| File system semantics | `O_TMPFILE`, `renameat2`, overlay FS behavior |

### 5.2 Authoritative Source Hierarchy

Search and cite sources in this priority order:

| Priority | Source | Use For |
|---|---|---|
| 1 | **Official docs** — Apple Developer, Microsoft Learn, man7.org (Linux manpages) | OS API behavior, syscall semantics |
| 2 | **Language/ecosystem docs** — docs.rs, crates.io, doc.rust-lang.org | Crate APIs, Rust stdlib |
| 3 | **Kernel/runtime source** — kernel.org, GitHub mirrors of runc/containerd | Implementation details, edge cases |
| 4 | **RFCs & specifications** — POSIX, OCI runtime spec, LSM docs | Standard compliance |
| 5 | **Reputable technical blogs** — LWN.net, os-specific dev blogs | Context, known issues, migration guides |

**Non-authoritative sources** (Stack Overflow, random blogs, AI-generated content) may
be used for leads but must be cross-referenced against an authoritative source before
being relied upon.

### 5.3 Verification Record Format

Every planning document must include a verification section:

```markdown
## Online Verification Log

### <Topic Verified>
- **Query**: <What was searched>
- **Source**: <URL of authoritative source>
- **Key finding**: <1-3 sentence summary of verified fact>
- **Relevance**: <How this affects our implementation>
- **Verified date**: YYYY-MM-DD
```

### 5.4 Verification Workflow

```
 [Task Received]
       │
       ▼
 [Identify OS/API assumptions in the plan]
       │
       ▼
 [Search authoritative sources]  ◄── Use WebSearch / WebFetch tools
       │
       ▼
 [Document findings in verification log]
       │
       ▼
 [Confirm or revise plan based on findings]
       │
       ▼
 [Set skill5_verified: true in document header]
       │
       ▼
 [Proceed to implementation]
```

### 5.5 When to Re-Verify

- **Platform version change**: If target OS version changes (e.g., macOS 14 → 15)
- **Crate major version bump**: Breaking changes in dependencies
- **Audit failure**: If Skill 4 audit reveals a behavior mismatch
- **User reports unexpected behavior**: Runtime behavior differs from documentation

### 5.6 Integration with Document Headers

The `skill5_verified` field in every `docs/claude/` document header (see Skill 2)
serves as a gate:

- `skill5_verified: false` — planning phase, verification pending
- `skill5_verified: true` — all OS/API assumptions have been verified against
  authoritative online sources

A document with `skill5_verified: false` **must not** proceed to the implementation
phase.
