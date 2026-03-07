# Desktop Artifact Conventions

This document defines the intended artifact naming and retention conventions
for future desktop CI runs.

Current status:

- planning only
- no active workflow consumes these conventions yet

## Product naming

- User-facing product name: `ClawAcosmi`
- Desktop shell label: `ClawAcosmi Desktop`
- Legacy compatibility names may still exist in code and must be reviewed
  before workflow activation

## Windows artifact targets

- installer archive label: `ClawAcosmi-windows-x64`
- package examples:
  - `ClawAcosmi-windows-x64.zip`
  - `ClawAcosmi-desktop-windows-x64.exe`

## Linux artifact targets

- archive label: `ClawAcosmi-linux-x64`
- package examples:
  - `ClawAcosmi-linux-x64.tar.gz`
  - `ClawAcosmi-desktop-linux-x64.AppImage`
  - `ClawAcosmi-desktop-linux-x64.deb`

## Retention baseline

- branch validation artifacts: short retention
- tagged release artifacts: long retention
- unsigned artifacts must be explicitly labeled as unsigned

## Activation gate

Do not treat these names as frozen until the CI activation checklist and naming
freeze are both approved.
