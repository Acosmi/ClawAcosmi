# Desktop CI Activation Checklist

This checklist defines the minimum gate before any desktop CI template becomes
an active workflow.

## 1. Naming freeze

- Confirm the user-visible product name is `ClawAcosmi`
- Confirm the desktop binary name to package
- Confirm whether legacy `openacosmi` compatibility names remain during transition

## 2. Build inputs

- Confirm `dist/control-ui` is the authoritative staged UI output
- Confirm `scripts/desktop/stage_control_ui.sh` remains the staging script
- Confirm `backend/cmd/desktop/frontend/dist` remains the desktop embed target

## 3. Toolchain pinning

- Pin Go version
- Pin Node version
- Pin Wails CLI version
- Confirm Linux runner package dependencies

## 4. Release outputs

- Define expected Windows artifact names
- Define expected Linux artifact names
- Define artifact retention and upload policy

## 5. Secret handling

- Confirm whether signing is in scope
- If yes, define secret names and runner requirements
- If no, make unsigned artifact behavior explicit

## 6. Runtime safety review

- Confirm no active workflow invokes experimental desktop runtime paths without approval
- Confirm workflow failure cannot affect the existing Gateway or macOS release path

## 7. Activation step

- Copy the chosen template into `.github/workflows/`
- Replace placeholder `echo` steps with reviewed build/package commands
- Run on a non-release branch first
