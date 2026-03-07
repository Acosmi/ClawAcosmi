# Desktop QA Execution Plan

This document translates the regression matrix into an execution order for the
future live validation phase.

Current boundary:

- planning only
- no execution in the current turn
- no mutation of runtime code

## Execution order

1. `desktop.attach-existing-gateway`
2. `desktop.foreign-port-conflict`
3. `desktop.control-ui-source-required`
4. `desktop.probe-failure-runtime-close`
5. `desktop.spa-deep-link-refresh`
6. `desktop.onboarding-routing`
7. `desktop.tray-exit-runtime-close`

## Rationale

- start with port ownership because it defines whether attach/start behavior is trustworthy
- verify failure paths before UI behavior, so the process lifecycle stays controlled
- leave tray/window lifecycle last because it depends on an actual GUI-capable run

## Evidence to capture per case

- command or launch entry used
- port number
- attachedExisting state if available
- observed error or success message
- process state before and after exit
- screenshot or short recording for UI-visible cases

## Exit criteria for a future live run

- all P1 cases have evidence attached
- no case marked `uncovered` remains without an explicit follow-up owner
- tray exit behavior is verified for both owned-runtime and attached-existing modes
