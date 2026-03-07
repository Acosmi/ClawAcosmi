# Desktop QA Assets

This directory stores non-destructive Phase 4 verification assets for the
desktop shell.

Current scope:

- regression scenario inventory
- manual acceptance checklist
- coverage gap tracking against existing unit tests
- live execution ordering for a future approved run
- evidence capture template for future live runs

Current non-goals:

- no runtime code changes
- no automatic test execution
- no CI activation
- no mutation of the live desktop bootstrap path

Existing code evidence referenced by these assets lives primarily in:

- `backend/cmd/desktop/runtime_test.go`
- `backend/internal/gateway/control_ui_test.go`
- `backend/internal/gateway/server_test.go`
- `backend/cmd/desktop/wails_app.go`

These files are intended to make Phase 4 reviewable before any live
environment validation is approved.
