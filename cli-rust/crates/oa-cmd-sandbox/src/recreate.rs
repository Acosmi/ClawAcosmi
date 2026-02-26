/// `sandbox recreate` command: stops and removes sandbox containers so they
/// are automatically recreated on next use.
///
/// Source: `src/commands/sandbox.ts` — `sandboxRecreateCommand`
use anyhow::{Result, bail};

use crate::display::{display_recreate_preview, display_recreate_result};
use crate::formatters::{SandboxBrowserInfo, SandboxContainerInfo};

/// Options for the `sandbox recreate` subcommand.
///
/// Source: `src/commands/sandbox.ts` — `SandboxRecreateOptions`
#[derive(Debug, Clone, Default)]
pub struct SandboxRecreateOptions {
    /// Recreate all containers.
    pub all: bool,
    /// Filter by session key.
    pub session: Option<String>,
    /// Filter by agent ID.
    pub agent: Option<String>,
    /// Recreate browser containers instead of sandbox containers.
    pub browser: bool,
    /// Skip confirmation prompt.
    pub force: bool,
}

/// Containers filtered for the recreate operation.
///
/// Source: `src/commands/sandbox.ts` — `FilteredContainers`
struct FilteredContainers {
    /// Sandbox compute containers to recreate.
    containers: Vec<SandboxContainerInfo>,
    /// Browser containers to recreate.
    browsers: Vec<SandboxBrowserInfo>,
}

/// Validate that exactly one of --all, --session, --agent is specified.
///
/// Source: `src/commands/sandbox.ts` — `validateRecreateOptions`
fn validate_recreate_options(opts: &SandboxRecreateOptions) -> Result<()> {
    if !opts.all && opts.session.is_none() && opts.agent.is_none() {
        bail!("Please specify --all, --session <key>, or --agent <id>");
    }

    let exclusive_count = [opts.all, opts.session.is_some(), opts.agent.is_some()]
        .iter()
        .filter(|&&v| v)
        .count();

    if exclusive_count > 1 {
        bail!("Please specify only one of: --all, --session, --agent");
    }

    Ok(())
}

/// Create a matcher function for filtering by agent ID.
///
/// Matches containers whose session key starts with `agent:<id>` or
/// `agent:<id>:`.
///
/// Source: `src/commands/sandbox.ts` — `createAgentMatcher`
fn matches_agent(session_key: &str, agent_id: &str) -> bool {
    let agent_prefix = format!("agent:{agent_id}");
    session_key == agent_prefix || session_key.starts_with(&format!("{agent_prefix}:"))
}

/// Stub: list all sandbox containers.
///
/// Source: `src/agents/sandbox.ts` — `listSandboxContainers`
async fn list_sandbox_containers() -> Vec<SandboxContainerInfo> {
    Vec::new()
}

/// Stub: list all sandbox browser containers.
///
/// Source: `src/agents/sandbox.ts` — `listSandboxBrowsers`
async fn list_sandbox_browsers() -> Vec<SandboxBrowserInfo> {
    Vec::new()
}

/// Fetch containers and filter based on options.
///
/// Source: `src/commands/sandbox.ts` — `fetchAndFilterContainers`
async fn fetch_and_filter_containers(opts: &SandboxRecreateOptions) -> FilteredContainers {
    let all_containers = list_sandbox_containers().await;
    let all_browsers = list_sandbox_browsers().await;

    let mut containers = if opts.browser {
        Vec::new()
    } else {
        all_containers
    };
    let mut browsers = if opts.browser {
        all_browsers
    } else {
        Vec::new()
    };

    if let Some(ref session) = opts.session {
        containers.retain(|c| c.session_key == *session);
        browsers.retain(|b| b.session_key == *session);
    } else if let Some(ref agent) = opts.agent {
        containers.retain(|c| matches_agent(&c.session_key, agent));
        browsers.retain(|b| matches_agent(&b.session_key, agent));
    }

    FilteredContainers {
        containers,
        browsers,
    }
}

/// Execute the `sandbox recreate` command.
///
/// Source: `src/commands/sandbox.ts` — `sandboxRecreateCommand`
pub async fn sandbox_recreate_command(opts: &SandboxRecreateOptions) -> Result<()> {
    validate_recreate_options(opts)?;

    let filtered = fetch_and_filter_containers(opts).await;

    if filtered.containers.is_empty() && filtered.browsers.is_empty() {
        println!("No containers found matching the criteria.");
        return Ok(());
    }

    display_recreate_preview(&filtered.containers, &filtered.browsers);

    if !opts.force {
        // In the full implementation this would prompt for confirmation.
        // For now we proceed if --force or bail.
        println!("Use --force to skip confirmation.");
        return Ok(());
    }

    // In the full implementation this would call docker rm -f on each
    // container. We display the result with zero counts for now.
    let success_count = 0;
    let fail_count = 0;

    display_recreate_result(success_count, fail_count);

    if fail_count > 0 {
        std::process::exit(1);
    }

    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn validate_options_all() {
        let opts = SandboxRecreateOptions {
            all: true,
            ..Default::default()
        };
        assert!(validate_recreate_options(&opts).is_ok());
    }

    #[test]
    fn validate_options_session() {
        let opts = SandboxRecreateOptions {
            session: Some("agent:main:main".to_owned()),
            ..Default::default()
        };
        assert!(validate_recreate_options(&opts).is_ok());
    }

    #[test]
    fn validate_options_agent() {
        let opts = SandboxRecreateOptions {
            agent: Some("main".to_owned()),
            ..Default::default()
        };
        assert!(validate_recreate_options(&opts).is_ok());
    }

    #[test]
    fn validate_options_none_fails() {
        let opts = SandboxRecreateOptions::default();
        assert!(validate_recreate_options(&opts).is_err());
    }

    #[test]
    fn validate_options_multiple_fails() {
        let opts = SandboxRecreateOptions {
            all: true,
            session: Some("test".to_owned()),
            ..Default::default()
        };
        assert!(validate_recreate_options(&opts).is_err());
    }

    #[test]
    fn matches_agent_exact() {
        assert!(matches_agent("agent:main", "main"));
    }

    #[test]
    fn matches_agent_with_suffix() {
        assert!(matches_agent("agent:main:discord:group:123", "main"));
    }

    #[test]
    fn matches_agent_different_id() {
        assert!(!matches_agent("agent:other:main", "main"));
    }

    #[test]
    fn matches_agent_no_prefix() {
        assert!(!matches_agent("main:key", "main"));
    }

    #[tokio::test]
    async fn recreate_empty_containers() {
        let opts = SandboxRecreateOptions {
            all: true,
            force: true,
            ..Default::default()
        };
        let result = sandbox_recreate_command(&opts).await;
        assert!(result.is_ok());
    }
}
