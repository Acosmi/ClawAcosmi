# Linux Build Notes

This folder now contains the active Linux packaging assets for the desktop
shell.

Available components:

- `.desktop` launcher generation in `Taskfile.yml`
- artifact naming conventions in `artifact-plan.md`
- nfpm packaging resources in `nfpm/`
- AppImage helper in `appimage/`
- reusable Linux builder image in `Dockerfile`
- container-backed Wails tasks:
  - `wails3 task linux:container:image`
  - `wails3 task linux:container:build`
  - `wails3 task linux:container:package`

The native `linux:*` tasks still expect a real Linux host or equivalent toolchain.
The `linux:container:*` tasks are the host-safe path on macOS.
