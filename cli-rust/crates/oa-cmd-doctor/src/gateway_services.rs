/// Gateway service config audit, legacy cleanup, and extra-service scanning.
///
/// Detects extra gateway-like services on the host, offers to clean up
/// legacy ones, and audits the gateway service config for drift from
/// the recommended defaults.
///
/// Source: `src/commands/doctor-gateway-services.ts`

use oa_config::paths::is_nix_mode;
use oa_terminal::note::note;
use oa_types::config::OpenAcosmiConfig;
use oa_types::gateway::GatewayMode;

use crate::prompter::{DoctorOptions, DoctorPrompter};

/// Scan for extra gateway-like services running on the host.
///
/// In deep mode, performs a broader search.  Offers to clean up legacy
/// (clawdbot / moltbot) services.
///
/// Source: `src/commands/doctor-gateway-services.ts` — `maybeScanExtraGatewayServices`
pub async fn maybe_scan_extra_gateway_services(
    _options: &DoctorOptions,
    _prompter: &mut DoctorPrompter,
) {
    // Stub: the real implementation calls `findExtraGatewayServices` from `oa-daemon`.
    // When extra services are found, it offers cleanup.
}

/// Audit the installed gateway service config and offer to repair drift.
///
/// Compares the installed service's program arguments, working directory,
/// and environment against what the current install plan would produce.
/// Detects entrypoint mismatches, stale bun/node paths, and custom edits.
///
/// Source: `src/commands/doctor-gateway-services.ts` — `maybeRepairGatewayServiceConfig`
pub async fn maybe_repair_gateway_service_config(
    cfg: &OpenAcosmiConfig,
    mode: &GatewayMode,
    _prompter: &mut DoctorPrompter,
) {
    // ── Nix mode → skip ──
    if is_nix_mode() {
        note(
            "Nix mode detected; skip service updates.",
            Some("Gateway"),
        );
        return;
    }

    // ── Remote mode → skip ──
    if *mode == GatewayMode::Remote {
        note(
            "Gateway mode is remote; skipped local service audit.",
            Some("Gateway"),
        );
        return;
    }

    // Stub: the real implementation reads the installed service command
    // via `oa-daemon::service::read_command`, runs `auditGatewayServiceConfig`,
    // builds an install plan, and compares entrypoints.
    let _ = cfg;
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn remote_mode_skips_audit() {
        let cfg = OpenAcosmiConfig::default();
        let options = DoctorOptions::default();
        let mut prompter = crate::prompter::create_doctor_prompter(&options);
        // Should not panic.
        maybe_repair_gateway_service_config(&cfg, &GatewayMode::Remote, &mut prompter).await;
    }

    #[tokio::test]
    async fn scan_extra_services_noop() {
        let options = DoctorOptions::default();
        let mut prompter = crate::prompter::create_doctor_prompter(&options);
        maybe_scan_extra_gateway_services(&options, &mut prompter).await;
    }
}
