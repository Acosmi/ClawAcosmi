/// Sandbox image presence checks and build helpers.
///
/// Verifies that the Docker images referenced by the sandbox config exist
/// locally, and offers to build missing images via the provided build
/// scripts.  Also warns about per-agent sandbox overrides that are
/// effectively ignored when the sandbox scope resolves to "shared".
///
/// Source: `src/commands/doctor-sandbox.ts`

use oa_terminal::note::note;
use oa_types::config::OpenAcosmiConfig;

use crate::prompter::DoctorPrompter;

/// Default sandbox Docker image name.
///
/// Source: `src/agents/sandbox.ts`
const DEFAULT_SANDBOX_IMAGE: &str = "openacosmi-sandbox:latest";

/// Default sandbox browser image name.
///
/// Source: `src/agents/sandbox.ts`
const DEFAULT_SANDBOX_BROWSER_IMAGE: &str = "openacosmi-sandbox-browser:latest";

/// Check whether Docker is available on the system.
///
/// Source: `src/commands/doctor-sandbox.ts` — `isDockerAvailable`
async fn is_docker_available() -> bool {
    tokio::process::Command::new("docker")
        .args(["version", "--format", "{{.Server.Version}}"])
        .output()
        .await
        .is_ok_and(|o| o.status.success())
}

/// Check whether a Docker image exists locally.
///
/// Source: `src/commands/doctor-sandbox.ts` — `dockerImageExists`
async fn docker_image_exists(image: &str) -> bool {
    tokio::process::Command::new("docker")
        .args(["image", "inspect", image])
        .output()
        .await
        .is_ok_and(|o| o.status.success())
}

/// Resolve the configured sandbox Docker image.
///
/// Source: `src/commands/doctor-sandbox.ts` — `resolveSandboxDockerImage`
fn resolve_sandbox_docker_image(cfg: &OpenAcosmiConfig) -> String {
    cfg.agents
        .as_ref()
        .and_then(|a| a.defaults.as_ref())
        .and_then(|d| d.sandbox.as_ref())
        .and_then(|s| s.docker.as_ref())
        .and_then(|d| d.image.as_ref())
        .map(|i| i.trim().to_string())
        .filter(|i| !i.is_empty())
        .unwrap_or_else(|| DEFAULT_SANDBOX_IMAGE.to_string())
}

/// Resolve the configured sandbox browser image.
///
/// Source: `src/commands/doctor-sandbox.ts` — `resolveSandboxBrowserImage`
fn resolve_sandbox_browser_image(cfg: &OpenAcosmiConfig) -> String {
    cfg.agents
        .as_ref()
        .and_then(|a| a.defaults.as_ref())
        .and_then(|d| d.sandbox.as_ref())
        .and_then(|s| s.browser.as_ref())
        .and_then(|b| b.image.as_ref())
        .map(|i| i.trim().to_string())
        .filter(|i| !i.is_empty())
        .unwrap_or_else(|| DEFAULT_SANDBOX_BROWSER_IMAGE.to_string())
}

/// Check sandbox images and offer to build missing ones.
///
/// Source: `src/commands/doctor-sandbox.ts` — `maybeRepairSandboxImages`
pub async fn maybe_repair_sandbox_images(
    cfg: OpenAcosmiConfig,
    _prompter: &mut DoctorPrompter,
) -> OpenAcosmiConfig {
    let sandbox = cfg
        .agents
        .as_ref()
        .and_then(|a| a.defaults.as_ref())
        .and_then(|d| d.sandbox.as_ref());

    let mode = sandbox
        .and_then(|s| s.mode.as_ref())
        .map(|m| format!("{m:?}"))
        .unwrap_or_else(|| "off".to_string());

    if sandbox.is_none() || mode == "Off" || mode == "off" {
        return cfg;
    }

    let docker_available = is_docker_available().await;
    if !docker_available {
        note(
            "Docker not available; skipping sandbox image checks.",
            Some("Sandbox"),
        );
        return cfg;
    }

    // ── Check base image ──
    let docker_image = resolve_sandbox_docker_image(&cfg);
    if !docker_image_exists(&docker_image).await {
        note(
            &format!(
                "Sandbox base image missing: {docker_image}. Build it with scripts/sandbox-setup.sh."
            ),
            Some("Sandbox"),
        );
    }

    // ── Check browser image (if enabled) ──
    let browser_enabled = sandbox
        .and_then(|s| s.browser.as_ref())
        .and_then(|b| b.enabled)
        .unwrap_or(false);
    if browser_enabled {
        let browser_image = resolve_sandbox_browser_image(&cfg);
        if !docker_image_exists(&browser_image).await {
            note(
                &format!(
                    "Sandbox browser image missing: {browser_image}. Build it with scripts/sandbox-browser-setup.sh."
                ),
                Some("Sandbox"),
            );
        }
    }

    cfg
}

/// Warn about per-agent sandbox overrides that are ignored when scope is "shared".
///
/// Source: `src/commands/doctor-sandbox.ts` — `noteSandboxScopeWarnings`
pub fn note_sandbox_scope_warnings(cfg: &OpenAcosmiConfig) {
    let agents_list = cfg
        .agents
        .as_ref()
        .and_then(|a| a.list.as_ref());

    let Some(agents) = agents_list else {
        return;
    };

    let global_scope = cfg
        .agents
        .as_ref()
        .and_then(|a| a.defaults.as_ref())
        .and_then(|d| d.sandbox.as_ref())
        .and_then(|s| s.scope.as_ref());

    let mut warnings: Vec<String> = Vec::new();

    for agent in agents {
        let agent_sandbox = match &agent.sandbox {
            Some(s) => s,
            None => continue,
        };

        // Resolve effective scope.
        let scope = agent_sandbox
            .scope
            .as_ref()
            .or(global_scope)
            .map(|s| format!("{s:?}"))
            .unwrap_or_else(|| "shared".to_string());

        if scope.to_lowercase() != "shared" {
            continue;
        }

        let mut overrides: Vec<&str> = Vec::new();
        if agent_sandbox.docker.is_some() {
            overrides.push("docker");
        }
        if agent_sandbox.browser.is_some() {
            overrides.push("browser");
        }
        if agent_sandbox.prune.is_some() {
            overrides.push("prune");
        }

        if overrides.is_empty() {
            continue;
        }

        let agent_id = &agent.id;
        warnings.push(format!(
            "- agents.list (id \"{agent_id}\") sandbox {} overrides ignored.\n  scope resolves to \"shared\".",
            overrides.join("/")
        ));
    }

    if !warnings.is_empty() {
        note(&warnings.join("\n"), Some("Sandbox"));
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn resolve_sandbox_image_default() {
        let cfg = OpenAcosmiConfig::default();
        assert_eq!(resolve_sandbox_docker_image(&cfg), DEFAULT_SANDBOX_IMAGE);
    }

    #[test]
    fn resolve_browser_image_default() {
        let cfg = OpenAcosmiConfig::default();
        assert_eq!(
            resolve_sandbox_browser_image(&cfg),
            DEFAULT_SANDBOX_BROWSER_IMAGE
        );
    }

    #[test]
    fn sandbox_scope_warnings_noop_without_agents() {
        let cfg = OpenAcosmiConfig::default();
        // Should not panic.
        note_sandbox_scope_warnings(&cfg);
    }
}
