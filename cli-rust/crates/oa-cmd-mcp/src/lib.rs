//! CLI commands for MCP server management.
//!
//! Subcommands: install, list, status, update, uninstall, start, stop.

use anyhow::Result;
use clap::{Args, Subcommand};
use colored::Colorize;

use oa_mcp_install::import::{self, ConfigFormat};
use oa_mcp_install::service::{self, InstallOptions};
use oa_mcp_install::InstalledMcpServer;

/// MCP subcommand group.
#[derive(Debug, Args)]
pub struct McpCommand {
    #[command(subcommand)]
    pub action: McpAction,
}

/// Individual MCP actions.
#[derive(Debug, Subcommand)]
pub enum McpAction {
    /// Install an MCP server from a git URL or release URL.
    Install(InstallArgs),
    /// List installed MCP servers.
    List(ListArgs),
    /// Show status of an installed MCP server.
    Status(StatusArgs),
    /// Update an installed MCP server.
    Update(UpdateArgs),
    /// Uninstall an MCP server.
    Uninstall(UninstallArgs),
    /// Start an MCP server (via Gateway RPC).
    Start(StartStopArgs),
    /// Stop a running MCP server (via Gateway RPC).
    Stop(StartStopArgs),
    /// Import MCP server configs from Claude Desktop, Cursor, or VS Code.
    Import(ImportArgs),
}

// ---------- Args ----------

#[derive(Debug, Args)]
pub struct InstallArgs {
    /// Git URL, SSH URL, or GitHub Release asset URL.
    pub url: String,
    /// Pin to a specific git ref (tag/branch/commit).
    #[arg(long)]
    pub r#ref: Option<String>,
    /// Custom server name (defaults to repo name).
    #[arg(long)]
    pub name: Option<String>,
    /// Expected SHA-256 hash for pre-compiled binary verification.
    #[arg(long)]
    pub sha256: Option<String>,
    /// Force reinstall if already installed.
    #[arg(long)]
    pub force: bool,
    /// Output as JSON.
    #[arg(long)]
    pub json: bool,
}

#[derive(Debug, Args)]
pub struct ListArgs {
    /// Output as JSON.
    #[arg(long)]
    pub json: bool,
}

#[derive(Debug, Args)]
pub struct StatusArgs {
    /// Server name.
    pub name: String,
    /// Output as JSON.
    #[arg(long)]
    pub json: bool,
}

#[derive(Debug, Args)]
pub struct UpdateArgs {
    /// Server name.
    pub name: String,
    /// Force rebuild/redownload.
    #[arg(long)]
    pub force: bool,
}

#[derive(Debug, Args)]
pub struct UninstallArgs {
    /// Server name.
    pub name: String,
}

#[derive(Debug, Args)]
pub struct StartStopArgs {
    /// Server name.
    pub name: String,
}

#[derive(Debug, Args)]
pub struct ImportArgs {
    /// Path to config file (auto-detects format if omitted).
    pub path: Option<String>,
    /// Config format: claude-desktop, cursor, vscode (auto-detect if omitted).
    #[arg(long)]
    pub format: Option<String>,
    /// Output as JSON.
    #[arg(long)]
    pub json: bool,
    /// Auto-discover and import from all known config files.
    #[arg(long)]
    pub discover: bool,
}

// ---------- Dispatch ----------

/// Dispatch an MCP subcommand.
pub async fn dispatch(cmd: McpCommand, _json: bool) -> Result<()> {
    match cmd.action {
        McpAction::Install(args) => cmd_install(args).await,
        McpAction::List(args) => cmd_list(args),
        McpAction::Status(args) => cmd_status(args),
        McpAction::Update(args) => cmd_update(args).await,
        McpAction::Uninstall(args) => cmd_uninstall(args),
        McpAction::Start(args) => cmd_gateway_rpc("mcp.server.start", &args.name).await,
        McpAction::Stop(args) => cmd_gateway_rpc("mcp.server.stop", &args.name).await,
        McpAction::Import(args) => cmd_import(args).await,
    }
}

// ---------- Command Implementations ----------

async fn cmd_install(args: InstallArgs) -> Result<()> {
    // Collect environment variables interactively if manifest declares them
    let env = collect_env_interactive(&args.url).await;

    let opts = InstallOptions {
        pinned_ref: args.r#ref,
        env,
        expected_sha256: args.sha256,
        name_override: args.name,
        force: args.force,
    };

    println!("{} Installing MCP server from {}...", "→".blue(), args.url);

    let result = service::install(&args.url, &opts).await?;

    if args.json {
        println!("{}", serde_json::to_string_pretty(&result.server)?);
    } else {
        let method = if result.was_downloaded {
            "downloaded"
        } else {
            "built from source"
        };
        println!(
            "{} {} ({}) — {}",
            "✓".green(),
            result.server.name.bold(),
            method,
            result.server.binary_path.display()
        );
        if let Some(ref sha) = result.server.binary_sha256 {
            println!("  SHA-256: {}…", &sha[..16]);
        }
        println!("\n  To start: {} mcp start {}", "openacosmi".dimmed(), result.server.name);
    }

    // Notify gateway to register the new server
    if let Err(e) = notify_gateway_register(&result.server).await {
        tracing::warn!("could not notify gateway: {e} (server registered locally, start manually)");
    }

    Ok(())
}

fn cmd_list(args: ListArgs) -> Result<()> {
    let servers = service::list()?;

    if args.json {
        println!("{}", serde_json::to_string_pretty(&servers)?);
        return Ok(());
    }

    if servers.is_empty() {
        println!("No MCP servers installed.");
        println!("  Install one: {} mcp install <url>", "openacosmi".dimmed());
        return Ok(());
    }

    println!("{} Installed MCP Servers:\n", "●".blue());
    for s in &servers {
        let transport = format!("{:?}", s.transport).to_lowercase();
        println!(
            "  {} {} ({}, {})",
            "•".green(),
            s.name.bold(),
            s.project_type,
            transport,
        );
        println!("    Source: {}", s.source_url.dimmed());
        println!("    Binary: {}", s.binary_path.display());
        if let Some(ref sha) = s.binary_sha256 {
            println!("    SHA-256: {}…", &sha[..16]);
        }
    }
    println!("\n  Total: {}", servers.len());
    Ok(())
}

fn cmd_status(args: StatusArgs) -> Result<()> {
    let server = service::get(&args.name)?;

    if args.json {
        println!("{}", serde_json::to_string_pretty(&server)?);
        return Ok(());
    }

    println!("{} {}", "●".blue(), server.name.bold());
    println!("  Source:    {}", server.source_url);
    println!("  Type:      {}", server.project_type);
    println!("  Transport: {:?}", server.transport);
    println!("  Binary:    {}", server.binary_path.display());
    if let Some(ref sha) = server.binary_sha256 {
        println!("  SHA-256:   {sha}");
    }
    if let Some(ref commit) = server.source_commit {
        println!("  Commit:    {commit}");
    }
    if let Some(ref pinned) = server.pinned_ref {
        println!("  Pinned:    {pinned}");
    }
    println!("  Installed: {}", server.installed_at);
    if !server.env.is_empty() {
        println!("  Env vars:  {}", server.env.keys().cloned().collect::<Vec<_>>().join(", "));
    }
    Ok(())
}

async fn cmd_update(args: UpdateArgs) -> Result<()> {
    let existing = service::get(&args.name)?;
    println!("{} Updating {}...", "→".blue(), args.name.bold());

    let opts = InstallOptions {
        pinned_ref: existing.pinned_ref.clone(),
        env: existing.env.clone(),
        expected_sha256: None,
        name_override: Some(existing.name.clone()),
        force: true,
    };

    let result = service::install(&existing.source_url, &opts).await?;

    let sha_changed = existing.binary_sha256 != result.server.binary_sha256;
    if sha_changed {
        println!(
            "{} {} updated (SHA-256 changed)",
            "✓".green(),
            result.server.name.bold()
        );
    } else {
        println!(
            "{} {} is already up to date",
            "✓".green(),
            result.server.name.bold()
        );
    }

    Ok(())
}

fn cmd_uninstall(args: UninstallArgs) -> Result<()> {
    service::uninstall(&args.name)?;
    println!("{} {} uninstalled", "✓".green(), args.name.bold());
    Ok(())
}

/// Send a simple RPC to the Gateway for start/stop operations.
async fn cmd_gateway_rpc(method: &str, name: &str) -> Result<()> {
    let opts = oa_gateway_rpc::call::CallGatewayOptions {
        method: method.to_string(),
        params: Some(serde_json::json!({ "name": name })),
        timeout_ms: Some(10_000),
        ..Default::default()
    };
    let result: serde_json::Value = oa_gateway_rpc::call::call_gateway(opts).await?;
    println!("{}", serde_json::to_string_pretty(&result)?);
    Ok(())
}

/// Notify the Gateway to register a newly installed server.
async fn notify_gateway_register(server: &InstalledMcpServer) -> Result<()> {
    let opts = oa_gateway_rpc::call::CallGatewayOptions {
        method: "mcp.server.register".to_string(),
        params: Some(serde_json::json!({
            "name": server.name,
            "binary_path": server.binary_path,
            "transport": server.transport,
            "command": server.command,
            "args": server.args,
            "env": server.env,
        })),
        timeout_ms: Some(10_000),
        ..Default::default()
    };
    let _: serde_json::Value = oa_gateway_rpc::call::call_gateway(opts).await?;
    Ok(())
}

async fn cmd_import(args: ImportArgs) -> Result<()> {
    use std::path::PathBuf;

    let parse_format = |s: &str| -> Option<ConfigFormat> {
        match s.to_lowercase().as_str() {
            "claude-desktop" | "claude" => Some(ConfigFormat::ClaudeDesktop),
            "cursor" => Some(ConfigFormat::Cursor),
            "vscode" | "vs-code" => Some(ConfigFormat::VsCode),
            _ => None,
        }
    };

    // Collect servers to import
    let mut all_servers = Vec::new();

    if args.discover || args.path.is_none() {
        // Auto-discover known config files
        let discovered = import::discover_config_files();
        if discovered.is_empty() {
            println!(
                "{} No MCP config files found. Checked Claude Desktop, Cursor, and VS Code paths.",
                "!".yellow()
            );
            return Ok(());
        }

        for (path, format) in &discovered {
            println!(
                "{} Found {} config: {}",
                "→".blue(),
                format,
                path.display()
            );
            match import::import_from_file_with_format(path, *format) {
                Ok(servers) => all_servers.extend(servers),
                Err(e) => {
                    println!(
                        "  {} Could not parse {}: {e}",
                        "!".yellow(),
                        path.display()
                    );
                }
            }
        }
    } else if let Some(ref path_str) = args.path {
        let path = PathBuf::from(path_str);
        if !path.exists() {
            anyhow::bail!("config file not found: {}", path.display());
        }

        let servers = if let Some(ref fmt_str) = args.format {
            let format = parse_format(fmt_str)
                .ok_or_else(|| anyhow::anyhow!("unknown format: {fmt_str}"))?;
            import::import_from_file_with_format(&path, format)?
        } else {
            import::import_from_file(&path)?
        };
        all_servers.extend(servers);
    }

    if all_servers.is_empty() {
        println!("{} No MCP servers found in config files.", "!".yellow());
        return Ok(());
    }

    if args.json {
        let entries: Vec<serde_json::Value> = all_servers
            .iter()
            .map(|s| {
                serde_json::json!({
                    "name": s.name,
                    "command": s.command,
                    "args": s.args,
                    "env": s.env,
                    "source_format": s.source_format.to_string(),
                    "source_path": s.source_path.display().to_string(),
                })
            })
            .collect();
        println!("{}", serde_json::to_string_pretty(&entries)?);
        return Ok(());
    }

    println!(
        "\n{} Found {} MCP server(s) to import:\n",
        "●".blue(),
        all_servers.len()
    );

    for s in &all_servers {
        println!(
            "  {} {} (from {})",
            "•".green(),
            s.name.bold(),
            s.source_format,
        );
        println!("    Command: {} {}", s.command, s.args.join(" "));
        if !s.env.is_empty() {
            println!(
                "    Env: {}",
                s.env.keys().cloned().collect::<Vec<_>>().join(", ")
            );
        }
    }

    // Register each server with the Gateway
    let mut registered = 0;
    for s in &all_servers {
        let server_params = serde_json::json!({
            "name": s.name,
            "command": s.command,
            "args": s.args,
            "env": s.env,
            "transport": "stdio",
        });

        let opts = oa_gateway_rpc::call::CallGatewayOptions {
            method: "mcp.server.register".to_string(),
            params: Some(server_params),
            timeout_ms: Some(10_000),
            ..Default::default()
        };

        match oa_gateway_rpc::call::call_gateway::<serde_json::Value>(opts).await {
            Ok(_) => {
                println!("  {} {} registered", "✓".green(), s.name);
                registered += 1;
            }
            Err(e) => {
                println!("  {} {} failed: {e}", "✗".red(), s.name);
            }
        }
    }

    println!(
        "\n{} Imported {registered}/{} server(s)",
        "●".blue(),
        all_servers.len()
    );
    Ok(())
}

/// Collect environment variables interactively by checking the manifest.
async fn collect_env_interactive(url: &str) -> std::collections::HashMap<String, String> {
    let env = std::collections::HashMap::new();
    // Placeholder: interactive env collection will be enhanced in Phase 2
    // to fetch raw mcp.json from GitHub/GitLab API before cloning.
    let _ = url;
    env
}
