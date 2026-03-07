# Desktop Build Assets

This directory stores the active Phase 3 build and packaging assets for the
desktop shell.

Scope of this directory:

- keep build and packaging conventions in one isolated place
- hold platform metadata, installers, and packaging templates
- provide reusable local build entry points without changing runtime behavior

Current state:

- top-level and platform `Taskfile.yml` entries are active
- macOS local packaging is already wired
- Windows NSIS packaging is already wired
- Linux now includes a reusable container build path in `build/linux/Dockerfile`
- active GitHub Actions workflows are still intentionally not added yet
