/// UI protocol freshness check and rebuild prompt.
///
/// Compares the modification times of the gateway protocol schema and the
/// built Control UI assets.  If the schema is newer, offers to rebuild
/// the UI.
///
/// Source: `src/commands/doctor-ui.ts`

use crate::prompter::DoctorPrompter;

/// Check whether the Control UI assets are present and up to date
/// relative to the protocol schema.
///
/// In the Rust CLI this is largely a no-op because the UI build step
/// is part of the TypeScript / JavaScript development workflow.
/// The Rust port notes the situation but cannot invoke `pnpm ui:build`.
///
/// Source: `src/commands/doctor-ui.ts` — `maybeRepairUiProtocolFreshness`
pub async fn maybe_repair_ui_protocol_freshness(
    _prompter: &mut DoctorPrompter,
) {
    // The Rust port does not ship bundled UI assets; skip the check.
    // In a development environment where both TS and Rust coexist,
    // the TypeScript doctor handles this.
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn ui_freshness_noop() {
        let options = crate::prompter::DoctorOptions::default();
        let mut prompter = crate::prompter::create_doctor_prompter(&options);
        maybe_repair_ui_protocol_freshness(&mut prompter).await;
        // Should complete without error.
    }
}
