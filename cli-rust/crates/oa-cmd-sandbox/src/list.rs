/// `sandbox list` command: lists sandbox containers and/or browser containers.
///
/// Source: `src/commands/sandbox.ts` — `sandboxListCommand`
use anyhow::Result;

use crate::display::{display_browsers, display_containers, display_summary};
use crate::formatters::{SandboxBrowserInfo, SandboxContainerInfo};

/// Options for the `sandbox list` subcommand.
///
/// Source: `src/commands/sandbox.ts` — `SandboxListOptions`
#[derive(Debug, Clone, Default)]
pub struct SandboxListOptions {
    /// Show only browser containers.
    pub browser: bool,
    /// Output in JSON format.
    pub json: bool,
}

/// List sandbox containers using Docker.
///
/// In the full implementation this calls `docker ps` and parses the output.
/// Currently returns an empty list (stub).
///
/// Source: `src/agents/sandbox.ts` — `listSandboxContainers`
async fn list_sandbox_containers() -> Vec<SandboxContainerInfo> {
    // Stub: the full implementation would shell out to docker
    Vec::new()
}

/// List sandbox browser containers using Docker.
///
/// In the full implementation this calls `docker ps` and parses the output.
/// Currently returns an empty list (stub).
///
/// Source: `src/agents/sandbox.ts` — `listSandboxBrowsers`
async fn list_sandbox_browsers() -> Vec<SandboxBrowserInfo> {
    // Stub: the full implementation would shell out to docker
    Vec::new()
}

/// Execute the `sandbox list` command.
///
/// Source: `src/commands/sandbox.ts` — `sandboxListCommand`
pub async fn sandbox_list_command(opts: &SandboxListOptions) -> Result<()> {
    let containers = if opts.browser {
        Vec::new()
    } else {
        list_sandbox_containers().await
    };
    let browsers = if opts.browser {
        list_sandbox_browsers().await
    } else {
        Vec::new()
    };

    if opts.json {
        let payload = serde_json::json!({
            "containers": containers,
            "browsers": browsers,
        });
        println!("{}", serde_json::to_string_pretty(&payload)?);
        return Ok(());
    }

    if opts.browser {
        display_browsers(&browsers);
    } else {
        display_containers(&containers);
    }

    display_summary(&containers, &browsers);

    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn sandbox_list_json_empty() {
        let opts = SandboxListOptions {
            browser: false,
            json: true,
        };
        // Just verify it doesn't error
        let result = sandbox_list_command(&opts).await;
        assert!(result.is_ok());
    }

    #[tokio::test]
    async fn sandbox_list_browser_json_empty() {
        let opts = SandboxListOptions {
            browser: true,
            json: true,
        };
        let result = sandbox_list_command(&opts).await;
        assert!(result.is_ok());
    }

    #[tokio::test]
    async fn sandbox_list_display_empty() {
        let opts = SandboxListOptions {
            browser: false,
            json: false,
        };
        let result = sandbox_list_command(&opts).await;
        assert!(result.is_ok());
    }
}
