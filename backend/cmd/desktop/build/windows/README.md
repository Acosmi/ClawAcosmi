# Windows Build Notes

This folder contains inactive Windows packaging templates for the desktop
shell.

Planned active assets later:

- version metadata from `info.json`
- process manifest from `wails.exe.manifest`
- artifact naming conventions documented in `artifact-plan.md`
- NSIS packaging resources in `nsis/`
- Wails Windows build/package tasks in `Taskfile.yml`

The files in this directory are now wired into `wails3 task windows:*`.
They are still not activated in a live CI workflow.
