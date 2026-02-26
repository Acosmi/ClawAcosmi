/// Subcommand routing for the OpenAcosmi CLI.
///
/// Defines the `Commands` enum connecting all command crates, defines
/// Clap `Args` wrappers for each command group, and provides the `dispatch`
/// function to route to the correct handler.
///
/// Source: `backend/cmd/openacosmi/main.go` - `registerAllCommands()`

use std::collections::HashMap;

use anyhow::Result;
use clap::{Args, Subcommand};

// ---------------------------------------------------------------------------
// Top-level subcommands
// ---------------------------------------------------------------------------

/// All top-level subcommands.
#[derive(Debug, Subcommand)]
pub enum Commands {
    /// System health check — probe gateway, channels, agents, sessions.
    Health(HealthArgs),

    /// Show system status dashboard.
    Status(StatusArgs),

    /// List session entries from the session store.
    Sessions(SessionsArgs),

    /// Channel management (list, add, remove, resolve, capabilities, logs, status).
    #[command(subcommand_required = true, arg_required_else_help = true)]
    Channels(ChannelsCommand),

    /// Model configuration (list, set, aliases, fallbacks).
    #[command(subcommand_required = true, arg_required_else_help = true)]
    Models(ModelsCommand),

    /// Agent management (list, add, delete, identity).
    #[command(subcommand_required = true, arg_required_else_help = true)]
    Agents(AgentsCommand),

    /// Sandbox container management (list, recreate, explain).
    #[command(subcommand_required = true, arg_required_else_help = true)]
    Sandbox(SandboxCommand),

    /// Coding sub-agent — MCP server with edit, read, grep, glob, bash tools.
    #[command(subcommand_required = true, arg_required_else_help = true)]
    Coder(CoderCommand),

    /// Authentication wizard — configure auth providers and API keys.
    Auth(AuthArgs),

    /// Configuration wizard — gateway, channels, daemon, workspace.
    Configure(ConfigureArgs),

    /// Initial onboarding setup wizard.
    Onboard(OnboardArgs),

    /// System diagnostics and repair.
    Doctor(DoctorArgs),

    /// Run or send a message to an AI agent.
    Agent(AgentArgs),

    /// Open the OpenAcosmi dashboard in a browser.
    Dashboard(DashboardArgs),

    /// Search OpenAcosmi documentation.
    Docs(DocsArgs),

    /// Reset OpenAcosmi state (config, sessions, workspace).
    Reset(ResetArgs),

    /// Initial workspace setup.
    Setup(SetupArgs),

    /// Uninstall OpenAcosmi components.
    Uninstall(UninstallArgs),

    /// Send a message through a channel.
    Message(MessageArgs),

    /// Comprehensive status report (debug output).
    StatusAll(StatusAllArgs),

    /// Probe gateway endpoints.
    GatewayStatus(GatewayStatusArgs),

    /// Gateway service management (run, start, stop, status, install, uninstall, call, health, probe, discover).
    #[command(subcommand_required = true, arg_required_else_help = true)]
    Gateway(GatewayCommand),

    /// Daemon service management (legacy alias for gateway).
    #[command(subcommand_required = true, arg_required_else_help = true)]
    Daemon(DaemonCommand),

    /// View and manage gateway logs.
    #[command(subcommand_required = true, arg_required_else_help = true)]
    Logs(LogsCommand),

    /// Agent memory and vector storage management.
    #[command(subcommand_required = true, arg_required_else_help = true)]
    Memory(MemoryCommand),

    /// Scheduled job management (cron).
    #[command(subcommand_required = true, arg_required_else_help = true)]
    Cron(CronCommand),

    /// Direct config file manipulation (get, set, unset).
    #[command(subcommand_required = true, arg_required_else_help = true)]
    Config(ConfigCommand),

    /// Generate shell completion scripts.
    Completion(CompletionArgs),
}

// ---------------------------------------------------------------------------
// Per-command Args structs
// ---------------------------------------------------------------------------

/// Arguments for the `health` command.
#[derive(Debug, Args)]
pub struct HealthArgs {
    /// Output result as JSON.
    #[arg(long)]
    pub json: bool,

    /// Enable verbose output.
    #[arg(long, short)]
    pub verbose: bool,

    /// Timeout in milliseconds for the gateway health call.
    #[arg(long)]
    pub timeout_ms: Option<u64>,
}

/// Arguments for the `status` command.
#[derive(Debug, Args)]
pub struct StatusArgs {
    /// Enable deep scanning.
    #[arg(long)]
    pub deep: bool,

    /// Show usage statistics.
    #[arg(long)]
    pub usage: bool,

    /// Timeout in milliseconds.
    #[arg(long)]
    pub timeout_ms: Option<u64>,

    /// Show all status details.
    #[arg(long)]
    pub all: bool,
}

/// Arguments for the `sessions` command.
#[derive(Debug, Args)]
pub struct SessionsArgs {
    /// Output result as JSON.
    #[arg(long)]
    pub json: bool,

    /// Override the session store path.
    #[arg(long)]
    pub store: Option<String>,

    /// Filter to sessions active within the last N minutes.
    #[arg(long)]
    pub active: Option<String>,
}

/// Arguments for the `status-all` command.
#[derive(Debug, Args)]
pub struct StatusAllArgs {
    /// Timeout in milliseconds.
    #[arg(long)]
    pub timeout_ms: Option<u64>,
}

/// Arguments for the `gateway-status` command.
#[derive(Debug, Args)]
pub struct GatewayStatusArgs {
    /// Gateway URL to probe.
    #[arg(long)]
    pub url: Option<String>,

    /// Auth token for the gateway.
    #[arg(long)]
    pub token: Option<String>,

    /// Auth password for the gateway.
    #[arg(long)]
    pub password: Option<String>,

    /// Timeout for the probe.
    #[arg(long)]
    pub timeout: Option<String>,

    /// Output result as JSON.
    #[arg(long)]
    pub json: bool,
}

// -- Channels subcommands ---------------------------------------------------

/// Channels subcommand group.
#[derive(Debug, Args)]
pub struct ChannelsCommand {
    #[command(subcommand)]
    pub action: ChannelsAction,
}

/// Individual channel actions.
#[derive(Debug, Subcommand)]
pub enum ChannelsAction {
    /// List configured channels.
    List(ChannelsListArgs),
    /// Add a new channel account.
    Add(ChannelsAddArgs),
    /// Remove or disable a channel account.
    Remove(ChannelsRemoveArgs),
    /// Resolve a channel contact.
    Resolve(ChannelsResolveArgs),
    /// Show channel capabilities.
    Capabilities(ChannelsCapabilitiesArgs),
    /// View channel logs.
    Logs(ChannelsLogsArgs),
    /// Check channel status.
    Status(ChannelsStatusArgs),
    /// Login to a channel account.
    Login(ChannelsLoginArgs),
    /// Logout from a channel account.
    Logout(ChannelsLogoutArgs),
}

#[derive(Debug, Args)]
pub struct ChannelsLoginArgs {
    /// Channel kind (whatsapp, telegram, discord, slack, signal, imessage).
    pub channel: String,
    /// Account identifier.
    pub account: Option<String>,
    /// Output as JSON.
    #[arg(long)]
    pub json: bool,
}

#[derive(Debug, Args)]
pub struct ChannelsLogoutArgs {
    /// Channel kind.
    pub channel: String,
    /// Account identifier.
    pub account: Option<String>,
    /// Output as JSON.
    #[arg(long)]
    pub json: bool,
}

#[derive(Debug, Args)]
pub struct ChannelsListArgs {
    /// Output as JSON.
    #[arg(long)]
    pub json: bool,
    /// Include usage information.
    #[arg(long)]
    pub usage: bool,
}

#[derive(Debug, Args)]
pub struct ChannelsAddArgs {
    /// Channel kind (whatsapp, telegram, discord, slack, signal, imessage).
    pub channel: String,
    /// Account identifier.
    pub account: Option<String>,
}

#[derive(Debug, Args)]
pub struct ChannelsRemoveArgs {
    /// Channel identifier.
    pub channel: String,
    /// Account identifier.
    pub account: Option<String>,
    /// Delete entirely (vs just disable).
    #[arg(long)]
    pub delete: bool,
}

#[derive(Debug, Args)]
pub struct ChannelsResolveArgs {
    /// Entries to resolve.
    pub entries: Vec<String>,
    /// Resolution kind.
    #[arg(long)]
    pub kind: Option<String>,
    /// Channel filter.
    #[arg(long)]
    pub channel: Option<String>,
    /// Output as JSON.
    #[arg(long)]
    pub json: bool,
}

#[derive(Debug, Args)]
pub struct ChannelsCapabilitiesArgs {
    /// Channel filter.
    #[arg(long)]
    pub channel: Option<String>,
    /// Account filter.
    #[arg(long)]
    pub account: Option<String>,
    /// Timeout for probes.
    #[arg(long)]
    pub timeout: Option<String>,
    /// Output as JSON.
    #[arg(long)]
    pub json: bool,
}

#[derive(Debug, Args)]
pub struct ChannelsLogsArgs {
    /// Channel filter.
    #[arg(long)]
    pub channel: Option<String>,
    /// Number of log lines to display.
    #[arg(long, short)]
    pub lines: Option<String>,
    /// Output as JSON.
    #[arg(long)]
    pub json: bool,
}

#[derive(Debug, Args)]
pub struct ChannelsStatusArgs {
    /// Output as JSON.
    #[arg(long)]
    pub json: bool,
    /// Include probe results.
    #[arg(long)]
    pub probe: bool,
    /// Timeout for probes.
    #[arg(long)]
    pub timeout: Option<String>,
}

// -- Models subcommands -----------------------------------------------------

/// Models subcommand group.
#[derive(Debug, Args)]
pub struct ModelsCommand {
    #[command(subcommand)]
    pub action: ModelsAction,
}

/// Individual model actions.
#[derive(Debug, Subcommand)]
pub enum ModelsAction {
    /// List configured models.
    List(ModelsListArgs),
    /// Set the primary model.
    Set(ModelsSetArgs),
    /// Set the primary image model.
    SetImage(ModelsSetImageArgs),
    /// Manage model aliases.
    #[command(subcommand_required = true, arg_required_else_help = true)]
    Aliases(ModelsAliasesCommand),
    /// Manage model fallbacks.
    #[command(subcommand_required = true, arg_required_else_help = true)]
    Fallbacks(ModelsFallbacksCommand),
    /// Manage image model fallbacks.
    #[command(subcommand_required = true, arg_required_else_help = true)]
    ImageFallbacks(ModelsImageFallbacksCommand),
}

#[derive(Debug, Args)]
pub struct ModelsListArgs {
    /// Output as JSON.
    #[arg(long)]
    pub json: bool,
    /// Plain output (no color/formatting).
    #[arg(long)]
    pub plain: bool,
}

#[derive(Debug, Args)]
pub struct ModelsSetArgs {
    /// Model identifier to set as primary.
    pub model: String,
}

#[derive(Debug, Args)]
pub struct ModelsSetImageArgs {
    /// Model identifier to set as primary image model.
    pub model: String,
}

/// Aliases subcommand group.
#[derive(Debug, Args)]
pub struct ModelsAliasesCommand {
    #[command(subcommand)]
    pub action: ModelsAliasesAction,
}

#[derive(Debug, Subcommand)]
pub enum ModelsAliasesAction {
    /// List all aliases.
    List(ModelsAliasesListArgs),
    /// Add an alias.
    Add(ModelsAliasesAddArgs),
    /// Remove an alias.
    Remove(ModelsAliasesRemoveArgs),
}

#[derive(Debug, Args)]
pub struct ModelsAliasesListArgs {
    /// Output as JSON.
    #[arg(long)]
    pub json: bool,
    /// Plain output.
    #[arg(long)]
    pub plain: bool,
}

#[derive(Debug, Args)]
pub struct ModelsAliasesAddArgs {
    /// Alias name.
    pub alias: String,
    /// Model identifier.
    pub model: String,
}

#[derive(Debug, Args)]
pub struct ModelsAliasesRemoveArgs {
    /// Alias name to remove.
    pub alias: String,
}

/// Fallbacks subcommand group.
#[derive(Debug, Args)]
pub struct ModelsFallbacksCommand {
    #[command(subcommand)]
    pub action: ModelsFallbacksAction,
}

#[derive(Debug, Subcommand)]
pub enum ModelsFallbacksAction {
    /// List model fallbacks.
    List(ModelsFallbacksListArgs),
    /// Add a model fallback.
    Add(ModelsFallbacksAddArgs),
    /// Remove a model fallback.
    Remove(ModelsFallbacksRemoveArgs),
    /// Clear all model fallbacks.
    Clear,
}

#[derive(Debug, Args)]
pub struct ModelsFallbacksListArgs {
    /// Output as JSON.
    #[arg(long)]
    pub json: bool,
    /// Plain output.
    #[arg(long)]
    pub plain: bool,
}

#[derive(Debug, Args)]
pub struct ModelsFallbacksAddArgs {
    /// Model identifier.
    pub model: String,
}

#[derive(Debug, Args)]
pub struct ModelsFallbacksRemoveArgs {
    /// Model identifier.
    pub model: String,
}

/// Image fallbacks subcommand group.
#[derive(Debug, Args)]
pub struct ModelsImageFallbacksCommand {
    #[command(subcommand)]
    pub action: ModelsImageFallbacksAction,
}

#[derive(Debug, Subcommand)]
pub enum ModelsImageFallbacksAction {
    /// List image model fallbacks.
    List(ModelsImageFallbacksListArgs),
    /// Add an image model fallback.
    Add(ModelsImageFallbacksAddArgs),
    /// Remove an image model fallback.
    Remove(ModelsImageFallbacksRemoveArgs),
    /// Clear all image model fallbacks.
    Clear,
}

#[derive(Debug, Args)]
pub struct ModelsImageFallbacksListArgs {
    /// Output as JSON.
    #[arg(long)]
    pub json: bool,
    /// Plain output.
    #[arg(long)]
    pub plain: bool,
}

#[derive(Debug, Args)]
pub struct ModelsImageFallbacksAddArgs {
    /// Model identifier.
    pub model: String,
}

#[derive(Debug, Args)]
pub struct ModelsImageFallbacksRemoveArgs {
    /// Model identifier.
    pub model: String,
}

// -- Agents subcommands -----------------------------------------------------

/// Agents subcommand group (plural — agent lifecycle management).
#[derive(Debug, Args)]
pub struct AgentsCommand {
    #[command(subcommand)]
    pub action: AgentsAction,
}

/// Individual agents actions.
#[derive(Debug, Subcommand)]
pub enum AgentsAction {
    /// List configured agents.
    List(AgentsListArgs),
    /// Add a new agent.
    Add(AgentsAddArgs),
    /// Delete an agent.
    Delete(AgentsDeleteArgs),
    /// Set an agent's identity (name, theme, emoji, avatar).
    SetIdentity(AgentsSetIdentityArgs),
}

#[derive(Debug, Args)]
pub struct AgentsListArgs {
    /// Output as JSON.
    #[arg(long)]
    pub json: bool,
    /// Show binding details.
    #[arg(long)]
    pub bindings: bool,
}

#[derive(Debug, Args)]
pub struct AgentsAddArgs {
    /// Agent identifier.
    pub id: String,
    /// Display name.
    #[arg(long)]
    pub name: Option<String>,
    /// Workspace path.
    #[arg(long)]
    pub workspace: Option<String>,
    /// Model override.
    #[arg(long)]
    pub model: Option<String>,
}

#[derive(Debug, Args)]
pub struct AgentsDeleteArgs {
    /// Agent identifier.
    pub id: String,
    /// Skip confirmation prompt.
    #[arg(long, short)]
    pub yes: bool,
}

#[derive(Debug, Args)]
pub struct AgentsSetIdentityArgs {
    /// Agent identifier.
    pub id: String,
    /// Display name.
    #[arg(long)]
    pub name: Option<String>,
    /// Theme.
    #[arg(long)]
    pub theme: Option<String>,
    /// Emoji.
    #[arg(long)]
    pub emoji: Option<String>,
    /// Avatar URL/path.
    #[arg(long)]
    pub avatar: Option<String>,
}

// -- Sandbox subcommands ----------------------------------------------------

/// Sandbox subcommand group.
#[derive(Debug, Args)]
pub struct SandboxCommand {
    #[command(subcommand)]
    pub action: SandboxAction,
}

/// Individual sandbox actions.
#[derive(Debug, Subcommand)]
pub enum SandboxAction {
    /// List sandbox containers.
    List(SandboxListArgs),
    /// Recreate a sandbox container.
    Recreate(SandboxRecreateArgs),
    /// Explain sandbox configuration.
    Explain(SandboxExplainArgs),
    /// Execute a command inside a sandboxed process.
    Run(SandboxRunArgs),
    /// Start a persistent sandbox Worker process (internal).
    WorkerStart(SandboxWorkerStartArgs),
}

#[derive(Debug, Args)]
pub struct SandboxListArgs {
    /// Output as JSON.
    #[arg(long)]
    pub json: bool,
    /// Show browser containers only.
    #[arg(long)]
    pub browser: bool,
}

#[derive(Debug, Args)]
pub struct SandboxRecreateArgs {
    /// Filter by agent ID.
    #[arg(long)]
    pub agent: Option<String>,
    /// Filter by session key.
    #[arg(long)]
    pub session: Option<String>,
    /// Recreate all containers.
    #[arg(long)]
    pub all: bool,
    /// Recreate browser containers.
    #[arg(long)]
    pub browser: bool,
    /// Skip confirmation prompt.
    #[arg(long)]
    pub force: bool,
}

#[derive(Debug, Args)]
pub struct SandboxExplainArgs {
    /// Agent ID to explain.
    #[arg(long)]
    pub agent: Option<String>,
    /// Session key override.
    #[arg(long)]
    pub session: Option<String>,
    /// Output as JSON.
    #[arg(long)]
    pub json: bool,
}

#[derive(Debug, Args)]
pub struct SandboxRunArgs {
    /// Security level: deny, sandbox, or full.
    #[arg(long, default_value = "sandbox")]
    pub security: String,
    /// Workspace directory (mounted into the sandbox).
    #[arg(long, default_value = ".")]
    pub workspace: String,
    /// Network policy: none, restricted, or host.
    #[arg(long)]
    pub net: Option<String>,
    /// Execution timeout in seconds.
    #[arg(long)]
    pub timeout: Option<u64>,
    /// Output format: json or text.
    #[arg(long, default_value = "json")]
    pub format: String,
    /// Backend preference: auto, native, or docker.
    #[arg(long, default_value = "auto")]
    pub backend: String,
    /// Additional bind mount (host:sandbox[:ro|rw]). Can be repeated.
    #[arg(long = "mount")]
    pub mounts: Vec<String>,
    /// Environment variable (KEY=VALUE). Can be repeated.
    #[arg(long = "env")]
    pub envs: Vec<String>,
    /// Memory limit in bytes (0 = no limit).
    #[arg(long, default_value = "0")]
    pub memory: u64,
    /// CPU limit in millicores (1000 = 1 core, 0 = no limit).
    #[arg(long, default_value = "0")]
    pub cpu: u32,
    /// Maximum number of processes (0 = no limit).
    #[arg(long, default_value = "0")]
    pub pids: u32,
    /// Show execution plan without running (also triggered automatically for L2/full).
    #[arg(long)]
    pub dry_run: bool,
    /// Command and arguments to execute.
    #[arg(trailing_var_arg = true, required = true)]
    pub command: Vec<String>,
}

/// Arguments for the `sandbox worker-start` subcommand (internal).
#[derive(Debug, Args)]
pub struct SandboxWorkerStartArgs {
    /// Workspace directory for sandboxed commands.
    #[arg(long, default_value = ".")]
    pub workspace: String,
    /// Default timeout in seconds for commands.
    #[arg(long, default_value = "120")]
    pub timeout: u64,
    /// Security level: deny (L0), sandbox (L1), full (L2).
    #[arg(long, default_value = "sandbox")]
    pub security_level: String,
    /// Idle timeout in seconds. Worker exits if no request arrives within this duration.
    /// 0 = no idle timeout (wait forever).
    #[arg(long, default_value = "0")]
    pub idle_timeout: u64,
}

// -- Coder subcommands ------------------------------------------------------

/// Coder sub-agent command group.
#[derive(Debug, Args)]
pub struct CoderCommand {
    #[command(subcommand)]
    pub action: CoderAction,
}

/// Individual coder actions.
#[derive(Debug, Subcommand)]
pub enum CoderAction {
    /// Start the MCP coding agent server (stdin/stdout JSON-RPC 2.0).
    Start(CoderStartArgs),
}

#[derive(Debug, Args)]
pub struct CoderStartArgs {
    /// Workspace directory for file operations.
    #[arg(long, default_value = ".")]
    pub workspace: String,
    /// Enable sandboxed execution for bash tool.
    #[arg(long)]
    pub sandboxed: bool,
}

// -- Auth -------------------------------------------------------------------

/// Arguments for the `auth` command.
#[derive(Debug, Args)]
pub struct AuthArgs;

// -- Configure --------------------------------------------------------------

/// Arguments for the `configure` command.
#[derive(Debug, Args)]
pub struct ConfigureArgs {
    /// Run specific sections only (comma-separated).
    #[arg(long, value_delimiter = ',')]
    pub sections: Option<Vec<String>>,
}

// -- Onboard ----------------------------------------------------------------

/// Arguments for the `onboard` command.
#[derive(Debug, Args)]
pub struct OnboardArgs {
    /// Gateway mode (local or remote).
    #[arg(long)]
    pub mode: Option<String>,

    /// Auth choice preset.
    #[arg(long)]
    pub auth_choice: Option<String>,

    /// Workspace directory.
    #[arg(long)]
    pub workspace: Option<String>,

    /// Run non-interactively.
    #[arg(long)]
    pub non_interactive: bool,

    /// Accept risk for non-interactive mode.
    #[arg(long)]
    pub accept_risk: bool,

    /// Reset existing config before onboarding.
    #[arg(long)]
    pub reset: bool,

    /// Anthropic API key.
    #[arg(long)]
    pub anthropic_api_key: Option<String>,

    /// OpenAI API key.
    #[arg(long)]
    pub openai_api_key: Option<String>,

    /// OpenRouter API key.
    #[arg(long)]
    pub openrouter_api_key: Option<String>,

    /// Gateway port.
    #[arg(long)]
    pub gateway_port: Option<u16>,

    /// Gateway bind address.
    #[arg(long)]
    pub gateway_bind: Option<String>,

    /// Gateway auth mode.
    #[arg(long)]
    pub gateway_auth: Option<String>,

    /// Gateway token.
    #[arg(long)]
    pub gateway_token: Option<String>,

    /// Gateway password.
    #[arg(long)]
    pub gateway_password: Option<String>,

    /// Install daemon service.
    #[arg(long)]
    pub install_daemon: bool,

    /// Daemon runtime (node or bun).
    #[arg(long)]
    pub daemon_runtime: Option<String>,

    /// Skip channel setup.
    #[arg(long)]
    pub skip_channels: bool,

    /// Skip skills setup.
    #[arg(long)]
    pub skip_skills: bool,

    /// Skip health check.
    #[arg(long)]
    pub skip_health: bool,

    /// Output as JSON.
    #[arg(long)]
    pub json: bool,
}

// -- Doctor -----------------------------------------------------------------

/// Arguments for the `doctor` command.
#[derive(Debug, Args)]
pub struct DoctorArgs {
    /// Accept all prompts automatically.
    #[arg(long, short)]
    pub yes: bool,

    /// Run without interactive prompts.
    #[arg(long)]
    pub non_interactive: bool,

    /// Enable deep scanning.
    #[arg(long)]
    pub deep: bool,

    /// Apply recommended repairs automatically.
    #[arg(long)]
    pub repair: bool,

    /// Allow aggressive / destructive repairs.
    #[arg(long)]
    pub force: bool,
}

// -- Agent (singular) -------------------------------------------------------

/// Arguments for the `agent` command (run a single agent).
#[derive(Debug, Args)]
pub struct AgentArgs {
    /// Message to send to the agent.
    pub message: String,

    /// Agent ID override.
    #[arg(long)]
    pub agent_id: Option<String>,

    /// Delivery target (phone number, email).
    #[arg(long)]
    pub to: Option<String>,

    /// Session identifier.
    #[arg(long)]
    pub session_id: Option<String>,

    /// Session key.
    #[arg(long)]
    pub session_key: Option<String>,

    /// Thinking level (off, minimal, low, medium, high, xhigh).
    #[arg(long)]
    pub thinking: Option<String>,

    /// Output as JSON.
    #[arg(long)]
    pub json: bool,

    /// Timeout in seconds.
    #[arg(long)]
    pub timeout: Option<String>,

    /// Verbose output level (on, full, off).
    #[arg(long)]
    pub verbose: Option<String>,
}

// -- Supporting commands ----------------------------------------------------

/// Arguments for the `dashboard` command.
#[derive(Debug, Args)]
pub struct DashboardArgs {
    /// Do not open the browser.
    #[arg(long)]
    pub no_open: bool,
}

/// Arguments for the `docs` command.
#[derive(Debug, Args)]
pub struct DocsArgs {
    /// Search query.
    pub query: Vec<String>,
}

/// Arguments for the `reset` command.
#[derive(Debug, Args)]
pub struct ResetArgs {
    /// Reset scope (config, config-creds-sessions, full).
    pub scope: Option<String>,
    /// Dry run — show what would be removed.
    #[arg(long)]
    pub dry_run: bool,
    /// Accept all prompts.
    #[arg(long, short)]
    pub yes: bool,
    /// Disable interactive prompts.
    #[arg(long)]
    pub non_interactive: bool,
}

/// Arguments for the `setup` command.
#[derive(Debug, Args)]
pub struct SetupArgs {
    /// Workspace directory.
    pub workspace: Option<String>,
}

/// Arguments for the `uninstall` command.
#[derive(Debug, Args)]
pub struct UninstallArgs {
    /// Include the gateway service scope.
    #[arg(long)]
    pub service: bool,
    /// Include the state+config scope.
    #[arg(long)]
    pub state: bool,
    /// Include workspace directories scope.
    #[arg(long)]
    pub workspace: bool,
    /// Include the macOS app scope.
    #[arg(long)]
    pub app: bool,
    /// Include all scopes.
    #[arg(long)]
    pub all: bool,
    /// Accept all prompts.
    #[arg(long, short)]
    pub yes: bool,
    /// Disable interactive prompts.
    #[arg(long)]
    pub non_interactive: bool,
    /// Dry run.
    #[arg(long)]
    pub dry_run: bool,
}

/// Arguments for the `message` command.
#[derive(Debug, Args)]
pub struct MessageArgs {
    /// Action to perform (send, deliver, forward, reply, react).
    pub action: Option<String>,
    /// Output as JSON.
    #[arg(long)]
    pub json: bool,
    /// Dry run.
    #[arg(long)]
    pub dry_run: bool,
}

/// Arguments for the `completion` command.
#[derive(Debug, Args)]
pub struct CompletionArgs {
    /// Shell to generate completions for (bash, zsh, fish, powershell).
    pub shell: clap_complete::Shell,
}

// -- Gateway subcommands ----------------------------------------------------

/// Gateway subcommand group.
#[derive(Debug, Args)]
pub struct GatewayCommand {
    #[command(subcommand)]
    pub action: GatewayAction,
}

/// Individual gateway actions.
#[derive(Debug, Subcommand)]
pub enum GatewayAction {
    /// Run the gateway process in the foreground.
    Run(GatewayRunArgs),
    /// Start the gateway as a background service.
    Start(GatewayStartArgs),
    /// Stop the gateway service.
    Stop,
    /// Show gateway service status.
    Status(GatewayStatusCmdArgs),
    /// Install the gateway as a system service.
    Install(GatewayInstallArgs),
    /// Uninstall the gateway system service.
    Uninstall,
    /// Call a gateway RPC method directly.
    Call(GatewayCallArgs),
    /// Show gateway usage cost.
    UsageCost(GatewayUsageCostArgs),
    /// Show gateway health.
    Health(GatewayHealthArgs),
    /// Probe gateway endpoints (HTTP, RPC, discovery).
    Probe(GatewayProbeArgs),
    /// Discover gateway instances on the network.
    Discover(GatewayDiscoverArgs),
}

#[derive(Debug, Args)]
pub struct GatewayRunArgs {
    /// Port to listen on.
    #[arg(long)]
    pub port: Option<u16>,
    /// Path to control UI static files.
    #[arg(long)]
    pub control_ui_dir: Option<String>,
}

#[derive(Debug, Args)]
pub struct GatewayStartArgs {
    /// Port to listen on.
    #[arg(long)]
    pub port: Option<u16>,
    /// Force restart if already running.
    #[arg(long)]
    pub force: bool,
}

#[derive(Debug, Args)]
pub struct GatewayStatusCmdArgs {
    /// Output as JSON.
    #[arg(long)]
    pub json: bool,
}

#[derive(Debug, Args)]
pub struct GatewayInstallArgs {
    /// Port to listen on.
    #[arg(long)]
    pub port: Option<u16>,
}

#[derive(Debug, Args)]
pub struct GatewayCallArgs {
    /// RPC method name.
    pub method: String,
    /// JSON-encoded parameters.
    #[arg(long)]
    pub params: Option<String>,
    /// Output as JSON.
    #[arg(long)]
    pub json: bool,
}

#[derive(Debug, Args)]
pub struct GatewayUsageCostArgs {
    /// Output as JSON.
    #[arg(long)]
    pub json: bool,
}

#[derive(Debug, Args)]
pub struct GatewayHealthArgs {
    /// Output as JSON.
    #[arg(long)]
    pub json: bool,
}

#[derive(Debug, Args)]
pub struct GatewayProbeArgs {
    /// Output as JSON.
    #[arg(long)]
    pub json: bool,
}

#[derive(Debug, Args)]
pub struct GatewayDiscoverArgs {
    /// Output as JSON.
    #[arg(long)]
    pub json: bool,
}

// -- Daemon subcommands (legacy alias) --------------------------------------

/// Daemon subcommand group.
#[derive(Debug, Args)]
pub struct DaemonCommand {
    #[command(subcommand)]
    pub action: DaemonAction,
}

/// Individual daemon actions (legacy aliases for gateway).
#[derive(Debug, Subcommand)]
pub enum DaemonAction {
    /// Show daemon status.
    Status(DaemonStatusArgs),
    /// Start the daemon.
    Start,
    /// Stop the daemon.
    Stop,
    /// Restart the daemon.
    Restart,
    /// Install the daemon service.
    Install,
    /// Uninstall the daemon service.
    Uninstall,
}

#[derive(Debug, Args)]
pub struct DaemonStatusArgs {
    /// Output as JSON.
    #[arg(long)]
    pub json: bool,
}

// -- Logs subcommands -------------------------------------------------------

/// Logs subcommand group.
#[derive(Debug, Args)]
pub struct LogsCommand {
    #[command(subcommand)]
    pub action: LogsAction,
}

/// Individual log actions.
#[derive(Debug, Subcommand)]
pub enum LogsAction {
    /// Follow (tail) the most recent log file.
    Follow(LogsFollowArgs),
    /// List available log files.
    List(LogsListArgs),
    /// Show contents of a log file.
    Show(LogsShowArgs),
    /// Clear all log files.
    Clear(LogsClearArgs),
    /// Export all logs to a single file.
    Export(LogsExportArgs),
}

#[derive(Debug, Args)]
pub struct LogsFollowArgs {
    /// Number of lines to display.
    #[arg(long, short)]
    pub lines: Option<usize>,
    /// Filter by channel.
    #[arg(long)]
    pub channel: Option<String>,
}

#[derive(Debug, Args)]
pub struct LogsListArgs {
    /// Output as JSON.
    #[arg(long)]
    pub json: bool,
}

#[derive(Debug, Args)]
pub struct LogsShowArgs {
    /// Specific log file to show.
    pub file: Option<String>,
    /// Number of lines to display.
    #[arg(long, short)]
    pub lines: Option<usize>,
    /// Output as JSON.
    #[arg(long)]
    pub json: bool,
}

#[derive(Debug, Args)]
pub struct LogsClearArgs {
    /// Skip confirmation prompt.
    #[arg(long, short)]
    pub yes: bool,
}

#[derive(Debug, Args)]
pub struct LogsExportArgs {
    /// Output file path.
    pub output: String,
}

// -- Memory subcommands -----------------------------------------------------

/// Memory subcommand group.
#[derive(Debug, Args)]
pub struct MemoryCommand {
    #[command(subcommand)]
    pub action: MemoryAction,
}

/// Individual memory actions.
#[derive(Debug, Subcommand)]
pub enum MemoryAction {
    /// Show memory system status.
    Status(MemoryStatusArgs),
    /// Trigger memory re-indexing.
    Index,
    /// Check memory system health.
    Check(MemoryCheckArgs),
    /// Search agent memory.
    Search(MemorySearchArgs),
}

#[derive(Debug, Args)]
pub struct MemoryStatusArgs {
    /// Output as JSON.
    #[arg(long)]
    pub json: bool,
}

#[derive(Debug, Args)]
pub struct MemoryCheckArgs {
    /// Output as JSON.
    #[arg(long)]
    pub json: bool,
}

#[derive(Debug, Args)]
pub struct MemorySearchArgs {
    /// Search query.
    pub query: String,
    /// Maximum results to return.
    #[arg(long)]
    pub limit: Option<usize>,
    /// Output as JSON.
    #[arg(long)]
    pub json: bool,
}

// -- Cron subcommands -------------------------------------------------------

/// Cron subcommand group.
#[derive(Debug, Args)]
pub struct CronCommand {
    #[command(subcommand)]
    pub action: CronAction,
}

/// Individual cron actions.
#[derive(Debug, Subcommand)]
pub enum CronAction {
    /// Show cron scheduler status.
    Status(CronStatusArgs),
    /// List scheduled jobs.
    List(CronListArgs),
    /// Add a scheduled job.
    Add(CronAddArgs),
    /// Edit a scheduled job.
    Edit(CronEditArgs),
    /// Remove a scheduled job.
    #[command(alias = "rm")]
    Remove(CronRemoveArgs),
    /// Enable a scheduled job.
    Enable(CronEnableArgs),
    /// Disable a scheduled job.
    Disable(CronDisableArgs),
    /// View run history for a job.
    Runs(CronRunsArgs),
    /// Trigger immediate execution of a job.
    Run(CronRunArgs),
}

#[derive(Debug, Args)]
pub struct CronStatusArgs {
    /// Output as JSON.
    #[arg(long)]
    pub json: bool,
}

#[derive(Debug, Args)]
pub struct CronListArgs {
    /// Output as JSON.
    #[arg(long)]
    pub json: bool,
}

#[derive(Debug, Args)]
pub struct CronAddArgs {
    /// Job name.
    #[arg(long)]
    pub name: String,
    /// Cron schedule expression.
    #[arg(long)]
    pub schedule: String,
    /// Agent ID to execute.
    #[arg(long)]
    pub agent_id: String,
    /// Message to send to the agent.
    #[arg(long)]
    pub message: String,
}

#[derive(Debug, Args)]
pub struct CronEditArgs {
    /// Job ID.
    pub id: String,
    /// New name.
    #[arg(long)]
    pub name: Option<String>,
    /// New schedule.
    #[arg(long)]
    pub schedule: Option<String>,
    /// New agent ID.
    #[arg(long)]
    pub agent_id: Option<String>,
    /// New message.
    #[arg(long)]
    pub message: Option<String>,
}

#[derive(Debug, Args)]
pub struct CronRemoveArgs {
    /// Job ID.
    pub id: String,
}

#[derive(Debug, Args)]
pub struct CronEnableArgs {
    /// Job ID.
    pub id: String,
}

#[derive(Debug, Args)]
pub struct CronDisableArgs {
    /// Job ID.
    pub id: String,
}

#[derive(Debug, Args)]
pub struct CronRunsArgs {
    /// Job ID.
    pub id: String,
    /// Maximum runs to display.
    #[arg(long)]
    pub limit: Option<usize>,
    /// Output as JSON.
    #[arg(long)]
    pub json: bool,
}

#[derive(Debug, Args)]
pub struct CronRunArgs {
    /// Job ID.
    pub id: String,
}

// -- Config subcommands -----------------------------------------------------

/// Config subcommand group.
#[derive(Debug, Args)]
pub struct ConfigCommand {
    #[command(subcommand)]
    pub action: ConfigAction,
}

/// Individual config actions.
#[derive(Debug, Subcommand)]
pub enum ConfigAction {
    /// Get a config value by dot-separated path.
    Get(ConfigGetArgs),
    /// Set a config value by dot-separated path.
    Set(ConfigSetArgs),
    /// Remove a config value by dot-separated path.
    Unset(ConfigUnsetArgs),
}

#[derive(Debug, Args)]
pub struct ConfigGetArgs {
    /// Dot-separated config path (e.g. "gateway.port").
    pub path: String,
    /// Output as JSON.
    #[arg(long)]
    pub json: bool,
}

#[derive(Debug, Args)]
pub struct ConfigSetArgs {
    /// Dot-separated config path.
    pub path: String,
    /// Value to set.
    pub value: String,
}

#[derive(Debug, Args)]
pub struct ConfigUnsetArgs {
    /// Dot-separated config path to remove.
    pub path: String,
}

// ---------------------------------------------------------------------------
// Dispatch
// ---------------------------------------------------------------------------

/// Route the parsed subcommand to its handler.
#[allow(clippy::too_many_lines)]
pub async fn dispatch(cmd: Commands, json: bool, verbose: bool) -> Result<()> {
    match cmd {
        // -- Tier 1: Health, Status, Sessions --------------------------------
        Commands::Health(args) => {
            let ha = oa_cmd_health::HealthArgs {
                json: json || args.json,
                verbose: verbose || args.verbose,
                timeout_ms: args.timeout_ms,
            };
            oa_cmd_health::execute(&ha).await
        }
        Commands::Status(args) => {
            oa_cmd_status::status_command(
                json,
                args.deep,
                args.usage,
                args.timeout_ms,
                verbose,
                args.all,
            )
            .await
        }
        Commands::Sessions(args) => {
            let sa = oa_cmd_sessions::SessionsArgs {
                json: json || args.json,
                store: args.store,
                active: args.active,
            };
            oa_cmd_sessions::execute(&sa).await
        }
        Commands::StatusAll(args) => {
            oa_cmd_status::status_all::status_all_command(args.timeout_ms).await
        }
        Commands::GatewayStatus(args) => {
            oa_cmd_status::gateway_status::gateway_status_command(
                args.url.as_deref(),
                args.token.as_deref(),
                args.password.as_deref(),
                args.timeout.as_deref(),
                json || args.json,
            )
            .await
        }

        // -- Tier 2: Channels, Models, Agents, Sandbox ----------------------
        Commands::Channels(cmd) => dispatch_channels(cmd, json).await,
        Commands::Models(cmd) => dispatch_models(cmd, json).await,
        Commands::Agents(cmd) => dispatch_agents(cmd, json).await,
        Commands::Sandbox(cmd) => dispatch_sandbox(cmd, json).await,
        Commands::Coder(cmd) => dispatch_coder(cmd).await,

        // -- Tier 3: Auth, Configure, Onboard --------------------------------
        Commands::Auth(_) => oa_cmd_auth::execute().await,
        Commands::Configure(args) => {
            if let Some(ref sections) = args.sections {
                let parsed: Vec<oa_cmd_configure::shared::WizardSection> = sections
                    .iter()
                    .filter_map(|s| match s.as_str() {
                        "workspace" => Some(oa_cmd_configure::shared::WizardSection::Workspace),
                        "model" => Some(oa_cmd_configure::shared::WizardSection::Model),
                        "web" => Some(oa_cmd_configure::shared::WizardSection::Web),
                        "gateway" => Some(oa_cmd_configure::shared::WizardSection::Gateway),
                        "daemon" => Some(oa_cmd_configure::shared::WizardSection::Daemon),
                        "channels" => Some(oa_cmd_configure::shared::WizardSection::Channels),
                        "skills" => Some(oa_cmd_configure::shared::WizardSection::Skills),
                        "health" => Some(oa_cmd_configure::shared::WizardSection::Health),
                        _ => None,
                    })
                    .collect();
                if parsed.is_empty() {
                    anyhow::bail!(
                        "No valid sections provided. Valid: workspace, model, web, gateway, daemon, channels, skills, health"
                    );
                }
                oa_cmd_configure::execute_with_sections(parsed).await
            } else {
                oa_cmd_configure::execute().await
            }
        }
        Commands::Onboard(args) => {
            let opts = oa_cmd_onboard::types::OnboardOptions {
                mode: args.mode,
                auth_choice: args.auth_choice,
                workspace: args.workspace,
                non_interactive: Some(args.non_interactive),
                accept_risk: Some(args.accept_risk),
                reset: Some(args.reset),
                anthropic_api_key: args.anthropic_api_key,
                openai_api_key: args.openai_api_key,
                openrouter_api_key: args.openrouter_api_key,
                gateway_port: args.gateway_port,
                gateway_bind: args.gateway_bind,
                gateway_auth: args.gateway_auth,
                gateway_token: args.gateway_token,
                gateway_password: args.gateway_password,
                install_daemon: Some(args.install_daemon),
                daemon_runtime: args.daemon_runtime,
                skip_channels: Some(args.skip_channels),
                skip_skills: Some(args.skip_skills),
                skip_health: Some(args.skip_health),
                json: Some(json || args.json),
                ..Default::default()
            };
            oa_cmd_onboard::execute(opts).await
        }

        // -- Tier 4: Doctor, Agent, Supporting -------------------------------
        Commands::Doctor(args) => {
            let opts = oa_cmd_doctor::DoctorOptions {
                yes: Some(args.yes),
                non_interactive: Some(args.non_interactive),
                deep: Some(args.deep),
                repair: Some(args.repair),
                force: Some(args.force),
                ..Default::default()
            };
            oa_cmd_doctor::execute(opts).await
        }
        Commands::Agent(args) => {
            let opts = oa_cmd_agent::types::AgentCommandOpts {
                message: args.message,
                agent_id: args.agent_id,
                to: args.to,
                session_id: args.session_id,
                session_key: args.session_key,
                thinking: args.thinking,
                json: Some(json || args.json),
                timeout: args.timeout,
                verbose: args.verbose,
                ..Default::default()
            };
            let _result = oa_cmd_agent::agent_command::agent_command(&opts).await?;
            Ok(())
        }

        // -- Supporting commands ---------------------------------------------
        Commands::Dashboard(args) => {
            let opts = oa_cmd_supporting::dashboard::DashboardOptions {
                no_open: args.no_open,
            };
            oa_cmd_supporting::dashboard::dashboard_command(opts).await
        }
        Commands::Docs(args) => {
            oa_cmd_supporting::docs::docs_search_command(&args.query).await
        }
        Commands::Reset(args) => {
            let scope = args
                .scope
                .as_deref()
                .and_then(oa_cmd_supporting::reset::ResetScope::from_str);
            let opts = oa_cmd_supporting::reset::ResetOptions {
                scope,
                dry_run: args.dry_run,
                yes: args.yes,
                non_interactive: args.non_interactive,
            };
            oa_cmd_supporting::reset::reset_command(&opts).await
        }
        Commands::Setup(args) => {
            let opts = oa_cmd_supporting::setup::SetupOptions {
                workspace: args.workspace,
            };
            oa_cmd_supporting::setup::setup_command(opts).await
        }
        Commands::Uninstall(args) => {
            let opts = oa_cmd_supporting::uninstall::UninstallOptions {
                service: args.service,
                state: args.state,
                workspace: args.workspace,
                app: args.app,
                all: args.all,
                yes: args.yes,
                non_interactive: args.non_interactive,
                dry_run: args.dry_run,
            };
            oa_cmd_supporting::uninstall::uninstall_command(&opts).await
        }
        Commands::Message(args) => {
            let opts = oa_cmd_supporting::message::MessageCommandOptions {
                action: args.action,
                json: json || args.json,
                dry_run: args.dry_run,
                params: HashMap::new(),
            };
            oa_cmd_supporting::message::message_command(&opts).await
        }

        // -- Tier 5: Gateway, Daemon, Logs, Memory, Cron, Config ---------------
        Commands::Gateway(cmd) => dispatch_gateway(cmd, json).await,
        Commands::Daemon(cmd) => dispatch_daemon(cmd, json).await,
        Commands::Logs(cmd) => dispatch_logs(cmd, json).await,
        Commands::Memory(cmd) => dispatch_memory(cmd, json).await,
        Commands::Cron(cmd) => dispatch_cron(cmd, json).await,
        Commands::Config(cmd) => dispatch_config(cmd, json).await,

        // -- Shell completions -----------------------------------------------
        Commands::Completion(args) => {
            let mut cmd = <crate::Cli as clap::CommandFactory>::command();
            clap_complete::generate(
                args.shell,
                &mut cmd,
                "openacosmi",
                &mut std::io::stdout(),
            );
            Ok(())
        }
    }
}

// ---------------------------------------------------------------------------
// Sub-dispatchers for nested subcommands
// ---------------------------------------------------------------------------

/// Dispatch channels subcommands.
async fn dispatch_channels(cmd: ChannelsCommand, json: bool) -> Result<()> {
    match cmd.action {
        ChannelsAction::List(args) => {
            let opts = oa_cmd_channels::list::ChannelsListOptions {
                json: json || args.json,
                usage: args.usage,
            };
            oa_cmd_channels::list::channels_list_command(&opts).await
        }
        ChannelsAction::Add(args) => {
            let opts = oa_cmd_channels::add::ChannelsAddOptions {
                channel: Some(args.channel),
                account: args.account,
                ..Default::default()
            };
            oa_cmd_channels::add::channels_add_command(&opts).await
        }
        ChannelsAction::Remove(args) => {
            let opts = oa_cmd_channels::remove::ChannelsRemoveOptions {
                channel: Some(args.channel),
                account: args.account,
                delete: args.delete,
            };
            oa_cmd_channels::remove::channels_remove_command(&opts).await
        }
        ChannelsAction::Resolve(args) => {
            let opts = oa_cmd_channels::resolve::ChannelsResolveOptions {
                channel: args.channel,
                kind: args.kind,
                json: json || args.json,
                entries: args.entries,
                ..Default::default()
            };
            oa_cmd_channels::resolve::channels_resolve_command(&opts).await
        }
        ChannelsAction::Capabilities(args) => {
            let opts = oa_cmd_channels::capabilities::ChannelsCapabilitiesOptions {
                channel: args.channel,
                account: args.account,
                timeout: args.timeout,
                json: json || args.json,
                ..Default::default()
            };
            oa_cmd_channels::capabilities::channels_capabilities_command(&opts).await
        }
        ChannelsAction::Logs(args) => {
            let opts = oa_cmd_channels::logs::ChannelsLogsOptions {
                channel: args.channel,
                lines: args.lines,
                json: json || args.json,
            };
            oa_cmd_channels::logs::channels_logs_command(&opts).await
        }
        ChannelsAction::Status(args) => {
            let opts = oa_cmd_channels::status::ChannelsStatusOptions {
                json: json || args.json,
                probe: args.probe,
                timeout: args.timeout,
            };
            oa_cmd_channels::status::channels_status_command(&opts).await
        }
        ChannelsAction::Login(args) => {
            let opts = oa_cmd_channels::login::ChannelsLoginOptions {
                channel: args.channel,
                account: args.account,
                json: json || args.json,
            };
            oa_cmd_channels::login::channels_login_command(&opts).await
        }
        ChannelsAction::Logout(args) => {
            let opts = oa_cmd_channels::logout::ChannelsLogoutOptions {
                channel: args.channel,
                account: args.account,
                json: json || args.json,
            };
            oa_cmd_channels::logout::channels_logout_command(&opts).await
        }
    }
}

/// Dispatch models subcommands.
async fn dispatch_models(cmd: ModelsCommand, json: bool) -> Result<()> {
    match cmd.action {
        ModelsAction::List(args) => {
            let cfg = oa_config::io::load_config().unwrap_or_default();
            let entries = oa_cmd_models::list_configured::resolve_configured_entries(&cfg);
            let output = if json || args.json {
                serde_json::to_string_pretty(&entries)?
            } else {
                entries
                    .iter()
                    .map(|e| {
                        let tags: Vec<&String> = e.tags.iter().collect();
                        format!(
                            "{} ({}/{}) [{}]",
                            e.key,
                            e.ref_provider,
                            e.ref_model,
                            tags.iter()
                                .map(|s| s.as_str())
                                .collect::<Vec<_>>()
                                .join(", ")
                        )
                    })
                    .collect::<Vec<_>>()
                    .join("\n")
            };
            println!("{output}");
            Ok(())
        }
        ModelsAction::Set(args) => {
            let msg = oa_cmd_models::set::models_set_command(&args.model).await?;
            println!("{msg}");
            Ok(())
        }
        ModelsAction::SetImage(args) => {
            let msg = oa_cmd_models::set_image::models_set_image_command(&args.model).await?;
            println!("{msg}");
            Ok(())
        }
        ModelsAction::Aliases(cmd) => dispatch_models_aliases(cmd, json).await,
        ModelsAction::Fallbacks(cmd) => dispatch_models_fallbacks(cmd, json).await,
        ModelsAction::ImageFallbacks(cmd) => dispatch_models_image_fallbacks(cmd, json).await,
    }
}

/// Dispatch model alias subcommands.
async fn dispatch_models_aliases(cmd: ModelsAliasesCommand, json: bool) -> Result<()> {
    match cmd.action {
        ModelsAliasesAction::List(args) => {
            let msg = oa_cmd_models::aliases::models_aliases_list_command(
                json || args.json,
                args.plain,
            )?;
            println!("{msg}");
            Ok(())
        }
        ModelsAliasesAction::Add(args) => {
            let msg = oa_cmd_models::aliases::models_aliases_add_command(
                &args.alias,
                &args.model,
            )
            .await?;
            println!("{msg}");
            Ok(())
        }
        ModelsAliasesAction::Remove(args) => {
            let msg = oa_cmd_models::aliases::models_aliases_remove_command(&args.alias).await?;
            println!("{msg}");
            Ok(())
        }
    }
}

/// Dispatch model fallback subcommands.
async fn dispatch_models_fallbacks(cmd: ModelsFallbacksCommand, json: bool) -> Result<()> {
    match cmd.action {
        ModelsFallbacksAction::List(args) => {
            let msg = oa_cmd_models::fallbacks::models_fallbacks_list_command(
                json || args.json,
                args.plain,
            )?;
            println!("{msg}");
            Ok(())
        }
        ModelsFallbacksAction::Add(args) => {
            let msg = oa_cmd_models::fallbacks::models_fallbacks_add_command(&args.model).await?;
            println!("{msg}");
            Ok(())
        }
        ModelsFallbacksAction::Remove(args) => {
            let msg =
                oa_cmd_models::fallbacks::models_fallbacks_remove_command(&args.model).await?;
            println!("{msg}");
            Ok(())
        }
        ModelsFallbacksAction::Clear => {
            let msg = oa_cmd_models::fallbacks::models_fallbacks_clear_command().await?;
            println!("{msg}");
            Ok(())
        }
    }
}

/// Dispatch image fallback subcommands.
async fn dispatch_models_image_fallbacks(
    cmd: ModelsImageFallbacksCommand,
    json: bool,
) -> Result<()> {
    match cmd.action {
        ModelsImageFallbacksAction::List(args) => {
            let msg = oa_cmd_models::image_fallbacks::models_image_fallbacks_list_command(
                json || args.json,
                args.plain,
            )?;
            println!("{msg}");
            Ok(())
        }
        ModelsImageFallbacksAction::Add(args) => {
            let msg =
                oa_cmd_models::image_fallbacks::models_image_fallbacks_add_command(&args.model)
                    .await?;
            println!("{msg}");
            Ok(())
        }
        ModelsImageFallbacksAction::Remove(args) => {
            let msg =
                oa_cmd_models::image_fallbacks::models_image_fallbacks_remove_command(&args.model)
                    .await?;
            println!("{msg}");
            Ok(())
        }
        ModelsImageFallbacksAction::Clear => {
            let msg =
                oa_cmd_models::image_fallbacks::models_image_fallbacks_clear_command().await?;
            println!("{msg}");
            Ok(())
        }
    }
}

/// Dispatch agents subcommands.
async fn dispatch_agents(cmd: AgentsCommand, json: bool) -> Result<()> {
    match cmd.action {
        AgentsAction::List(args) => {
            let cfg = oa_config::io::load_config().unwrap_or_default();
            let output = oa_cmd_agents::list::agents_list_command(
                &cfg,
                json || args.json,
                args.bindings,
            )?;
            println!("{output}");
            Ok(())
        }
        AgentsAction::Add(args) => {
            let opts = oa_cmd_agents::add::AgentsAddOptions {
                id: &args.id,
                name: args.name.as_deref(),
                workspace: args.workspace.as_deref(),
                model: args.model.as_deref(),
            };
            let msg = oa_cmd_agents::add::agents_add_command(&opts).await?;
            println!("{msg}");
            Ok(())
        }
        AgentsAction::Delete(args) => {
            let opts = oa_cmd_agents::delete::AgentsDeleteOptions {
                id: &args.id,
                yes: args.yes,
            };
            let msg = oa_cmd_agents::delete::agents_delete_command(&opts).await?;
            println!("{msg}");
            Ok(())
        }
        AgentsAction::SetIdentity(args) => {
            let opts = oa_cmd_agents::set_identity::AgentsSetIdentityOptions {
                id: &args.id,
                name: args.name.as_deref(),
                theme: args.theme.as_deref(),
                emoji: args.emoji.as_deref(),
                avatar: args.avatar.as_deref(),
            };
            let msg = oa_cmd_agents::set_identity::agents_set_identity_command(&opts).await?;
            println!("{msg}");
            Ok(())
        }
    }
}

/// Dispatch sandbox subcommands.
async fn dispatch_sandbox(cmd: SandboxCommand, json: bool) -> Result<()> {
    match cmd.action {
        SandboxAction::List(args) => {
            let opts = oa_cmd_sandbox::list::SandboxListOptions {
                json: json || args.json,
                browser: args.browser,
            };
            oa_cmd_sandbox::list::sandbox_list_command(&opts).await
        }
        SandboxAction::Recreate(args) => {
            let opts = oa_cmd_sandbox::recreate::SandboxRecreateOptions {
                agent: args.agent,
                session: args.session,
                all: args.all,
                browser: args.browser,
                force: args.force,
            };
            oa_cmd_sandbox::recreate::sandbox_recreate_command(&opts).await
        }
        SandboxAction::Explain(args) => {
            let opts = oa_cmd_sandbox::explain::SandboxExplainOptions {
                agent: args.agent,
                session: args.session,
                json: json || args.json,
            };
            oa_cmd_sandbox::explain::sandbox_explain_command(&opts).await
        }
        SandboxAction::Run(args) => {
            let security = match args.security.as_str() {
                "deny" => oa_sandbox::config::SecurityLevel::L0Deny,
                "sandbox" => oa_sandbox::config::SecurityLevel::L1Sandbox,
                "full" => oa_sandbox::config::SecurityLevel::L2Full,
                other => anyhow::bail!("invalid security level '{other}': expected deny, sandbox, or full"),
            };
            let network = args.net.as_deref().map(|n| match n {
                "none" => Ok(oa_sandbox::config::NetworkPolicy::None),
                "restricted" => Ok(oa_sandbox::config::NetworkPolicy::Restricted),
                "host" => Ok(oa_sandbox::config::NetworkPolicy::Host),
                other => Err(anyhow::anyhow!("invalid network policy '{other}': expected none, restricted, or host")),
            }).transpose()?;
            let format = match args.format.as_str() {
                "json" => oa_sandbox::config::OutputFormat::Json,
                "text" => oa_sandbox::config::OutputFormat::Text,
                other => anyhow::bail!("invalid format '{other}': expected json or text"),
            };
            let backend = match args.backend.as_str() {
                "auto" => oa_sandbox::config::BackendPreference::Auto,
                "native" => oa_sandbox::config::BackendPreference::Native,
                "docker" => oa_sandbox::config::BackendPreference::Docker,
                other => anyhow::bail!("invalid backend '{other}': expected auto, native, or docker"),
            };
            let workspace = std::path::Path::new(&args.workspace)
                .canonicalize()
                .unwrap_or_else(|_| std::path::PathBuf::from(&args.workspace));

            let (command, cmd_args) = if args.command.is_empty() {
                anyhow::bail!("no command specified");
            } else {
                (args.command[0].clone(), args.command[1..].to_vec())
            };

            let opts = oa_cmd_sandbox::run::SandboxRunOptions {
                security,
                workspace,
                network,
                timeout: args.timeout,
                format,
                backend,
                mounts: args.mounts,
                env: args.envs,
                memory: args.memory,
                cpu: args.cpu,
                pids: args.pids,
                dry_run: args.dry_run,
                command,
                args: cmd_args,
            };
            oa_cmd_sandbox::run::sandbox_run_command(&opts)
        }
        SandboxAction::WorkerStart(args) => {
            let workspace = std::path::Path::new(&args.workspace)
                .canonicalize()
                .unwrap_or_else(|_| std::path::PathBuf::from(&args.workspace));
            let opts = oa_cmd_sandbox::worker_cmd::WorkerStartOptions {
                workspace,
                timeout: args.timeout,
                security_level: args.security_level,
                idle_timeout: args.idle_timeout,
            };
            oa_cmd_sandbox::worker_cmd::sandbox_worker_start_command(&opts)
        }
    }
}

/// Dispatch coder subcommands.
async fn dispatch_coder(cmd: CoderCommand) -> Result<()> {
    match cmd.action {
        CoderAction::Start(args) => {
            let workspace = std::path::Path::new(&args.workspace)
                .canonicalize()
                .unwrap_or_else(|_| std::path::PathBuf::from(&args.workspace));
            let opts = oa_cmd_coder::CoderStartOptions {
                workspace,
                sandboxed: args.sandboxed,
            };
            oa_cmd_coder::coder_start_command(&opts)
        }
    }
}

/// Dispatch gateway subcommands.
async fn dispatch_gateway(cmd: GatewayCommand, json: bool) -> Result<()> {
    match cmd.action {
        GatewayAction::Run(args) => {
            oa_cmd_gateway::run::gateway_run_command(
                args.port,
                args.control_ui_dir.as_deref(),
            )
            .await
        }
        GatewayAction::Start(args) => {
            oa_cmd_gateway::start::gateway_start_command(args.port, args.force).await
        }
        GatewayAction::Stop => oa_cmd_gateway::stop::gateway_stop_command().await,
        GatewayAction::Status(args) => {
            oa_cmd_gateway::status::gateway_status_command(json || args.json).await
        }
        GatewayAction::Install(args) => {
            oa_cmd_gateway::install::gateway_install_command(args.port).await
        }
        GatewayAction::Uninstall => oa_cmd_gateway::uninstall::gateway_uninstall_command().await,
        GatewayAction::Call(args) => {
            oa_cmd_gateway::call::gateway_call_command(
                &args.method,
                args.params.as_deref(),
                json || args.json,
            )
            .await
        }
        GatewayAction::UsageCost(args) => {
            oa_cmd_gateway::usage_cost::gateway_usage_cost_command(json || args.json).await
        }
        GatewayAction::Health(args) => {
            oa_cmd_gateway::health::gateway_health_command(json || args.json).await
        }
        GatewayAction::Probe(args) => {
            oa_cmd_gateway::probe::gateway_probe_command(json || args.json).await
        }
        GatewayAction::Discover(args) => {
            oa_cmd_gateway::discover::gateway_discover_command(json || args.json).await
        }
    }
}

/// Dispatch daemon subcommands (legacy aliases for gateway).
async fn dispatch_daemon(cmd: DaemonCommand, json: bool) -> Result<()> {
    match cmd.action {
        DaemonAction::Status(args) => {
            oa_cmd_daemon::commands::daemon_status_command(json || args.json).await
        }
        DaemonAction::Start => oa_cmd_daemon::commands::daemon_start_command().await,
        DaemonAction::Stop => oa_cmd_daemon::commands::daemon_stop_command().await,
        DaemonAction::Restart => oa_cmd_daemon::commands::daemon_restart_command().await,
        DaemonAction::Install => oa_cmd_daemon::commands::daemon_install_command().await,
        DaemonAction::Uninstall => oa_cmd_daemon::commands::daemon_uninstall_command().await,
    }
}

/// Dispatch logs subcommands.
async fn dispatch_logs(cmd: LogsCommand, json: bool) -> Result<()> {
    match cmd.action {
        LogsAction::Follow(args) => {
            oa_cmd_logs::follow::logs_follow_command(args.lines, args.channel.as_deref()).await
        }
        LogsAction::List(args) => {
            oa_cmd_logs::list::logs_list_command(json || args.json).await
        }
        LogsAction::Show(args) => {
            oa_cmd_logs::show::logs_show_command(
                args.file.as_deref(),
                args.lines,
                json || args.json,
            )
            .await
        }
        LogsAction::Clear(args) => oa_cmd_logs::clear::logs_clear_command(args.yes).await,
        LogsAction::Export(args) => {
            oa_cmd_logs::export::logs_export_command(&args.output).await
        }
    }
}

/// Dispatch memory subcommands.
async fn dispatch_memory(cmd: MemoryCommand, json: bool) -> Result<()> {
    match cmd.action {
        MemoryAction::Status(args) => {
            oa_cmd_memory::status::memory_status_command(json || args.json).await
        }
        MemoryAction::Index => oa_cmd_memory::index::memory_index_command().await,
        MemoryAction::Check(args) => {
            oa_cmd_memory::check::memory_check_command(json || args.json).await
        }
        MemoryAction::Search(args) => {
            oa_cmd_memory::search::memory_search_command(
                &args.query,
                args.limit,
                json || args.json,
            )
            .await
        }
    }
}

/// Dispatch cron subcommands.
async fn dispatch_cron(cmd: CronCommand, json: bool) -> Result<()> {
    match cmd.action {
        CronAction::Status(args) => {
            oa_cmd_cron::status::cron_status_command(json || args.json).await
        }
        CronAction::List(args) => {
            oa_cmd_cron::list::cron_list_command(json || args.json).await
        }
        CronAction::Add(args) => {
            oa_cmd_cron::add::cron_add_command(
                &args.name,
                &args.schedule,
                &args.agent_id,
                &args.message,
            )
            .await
        }
        CronAction::Edit(args) => {
            oa_cmd_cron::edit::cron_edit_command(
                &args.id,
                args.name.as_deref(),
                args.schedule.as_deref(),
                args.agent_id.as_deref(),
                args.message.as_deref(),
            )
            .await
        }
        CronAction::Remove(args) => oa_cmd_cron::remove::cron_remove_command(&args.id).await,
        CronAction::Enable(args) => oa_cmd_cron::enable::cron_enable_command(&args.id).await,
        CronAction::Disable(args) => oa_cmd_cron::disable::cron_disable_command(&args.id).await,
        CronAction::Runs(args) => {
            oa_cmd_cron::runs::cron_runs_command(&args.id, args.limit, json || args.json).await
        }
        CronAction::Run(args) => oa_cmd_cron::run::cron_run_command(&args.id).await,
    }
}

/// Dispatch config subcommands.
async fn dispatch_config(cmd: ConfigCommand, json: bool) -> Result<()> {
    match cmd.action {
        ConfigAction::Get(args) => {
            oa_cmd_config::get::config_get_command(&args.path, json || args.json)
        }
        ConfigAction::Set(args) => {
            oa_cmd_config::set::config_set_command(&args.path, &args.value).await
        }
        ConfigAction::Unset(args) => {
            oa_cmd_config::unset::config_unset_command(&args.path).await
        }
    }
}
