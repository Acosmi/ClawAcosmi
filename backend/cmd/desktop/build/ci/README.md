# Desktop CI Templates

This directory stores inactive CI templates for the desktop shell.

Current status:

- templates only
- not copied into `.github/workflows/`
- not referenced by any active build or release path

Why it is isolated here:

- the current implementation constraint is "non-destructive only"
- active workflow introduction would change repository execution behavior
- desktop runtime naming and packaging conventions are still in transition

Before activation, review at least:

- desktop binary naming consistency (`ClawAcosmi` vs legacy compatibility names)
- Wails CLI pinning strategy
- signing and packaging secrets
- platform runner prerequisites
- UI staging path stability
- artifact naming conventions in `artifact-conventions.md`

Activation is intentionally deferred until runtime-safe review is approved.
