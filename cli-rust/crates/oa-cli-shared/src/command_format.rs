/// Command output formatting.
///
/// Formats CLI command strings for display, injecting the active
/// profile `--profile` flag when the `OPENACOSMI_PROFILE` environment
/// variable is set.
///
/// Source: `src/cli/command-format.ts`

use regex::Regex;
use std::sync::LazyLock;

/// Regex matching a CLI invocation prefix (e.g. `npx openacosmi`, `openacosmi`).
///
/// Source: `src/cli/command-format.ts` – `CLI_PREFIX_RE`
static CLI_PREFIX_RE: LazyLock<Regex> = LazyLock::new(|| {
    Regex::new(r"^(?:pnpm|npm|bunx|npx)\s+openacosmi\b|^openacosmi\b")
        .expect("CLI_PREFIX_RE is a valid regex")
});

/// Regex detecting an existing `--profile` flag.
///
/// Source: `src/cli/command-format.ts` – `PROFILE_FLAG_RE`
static PROFILE_FLAG_RE: LazyLock<Regex> = LazyLock::new(|| {
    Regex::new(r"(?:^|\s)--profile(?:\s|=|$)").expect("PROFILE_FLAG_RE is a valid regex")
});

/// Regex detecting an existing `--dev` flag.
///
/// Source: `src/cli/command-format.ts` – `DEV_FLAG_RE`
static DEV_FLAG_RE: LazyLock<Regex> = LazyLock::new(|| {
    Regex::new(r"(?:^|\s)--dev(?:\s|$)").expect("DEV_FLAG_RE is a valid regex")
});

/// Normalize a profile name: trim whitespace, return `None` for empty strings.
///
/// Source: `src/cli/profile-utils.ts` – `normalizeProfileName`
fn normalize_profile_name(raw: Option<&str>) -> Option<String> {
    let trimmed = raw?.trim();
    if trimmed.is_empty() {
        return None;
    }
    Some(trimmed.to_string())
}

/// Format a CLI command string for display.
///
/// When the `OPENACOSMI_PROFILE` environment variable is set and the
/// command does not already include `--profile` or `--dev`, the
/// `--profile <name>` flag is injected immediately after the CLI prefix.
///
/// Source: `src/cli/command-format.ts` – `formatCliCommand`
pub fn format_cli_command(command: &str) -> String {
    format_cli_command_with_env(command, std::env::var("OPENACOSMI_PROFILE").ok().as_deref())
}

/// Format a CLI command string with an explicit profile value.
///
/// This is the testable inner implementation of [`format_cli_command`].
///
/// Source: `src/cli/command-format.ts` – `formatCliCommand`
pub fn format_cli_command_with_env(command: &str, profile_env: Option<&str>) -> String {
    let profile = match normalize_profile_name(profile_env) {
        Some(p) => p,
        None => return command.to_string(),
    };

    if !CLI_PREFIX_RE.is_match(command) {
        return command.to_string();
    }

    if PROFILE_FLAG_RE.is_match(command) || DEV_FLAG_RE.is_match(command) {
        return command.to_string();
    }

    CLI_PREFIX_RE
        .replace(command, |caps: &regex::Captures<'_>| {
            format!("{} --profile {profile}", &caps[0])
        })
        .into_owned()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn injects_profile_into_bare_command() {
        let result = format_cli_command_with_env("openacosmi doctor --fix", Some("staging"));
        assert_eq!(result, "openacosmi --profile staging doctor --fix");
    }

    #[test]
    fn injects_profile_into_npx_command() {
        let result = format_cli_command_with_env("npx openacosmi status", Some("dev"));
        assert_eq!(result, "npx openacosmi --profile dev status");
    }

    #[test]
    fn no_injection_without_profile() {
        let result = format_cli_command_with_env("openacosmi status", None);
        assert_eq!(result, "openacosmi status");
    }

    #[test]
    fn no_injection_with_empty_profile() {
        let result = format_cli_command_with_env("openacosmi status", Some("  "));
        assert_eq!(result, "openacosmi status");
    }

    #[test]
    fn no_injection_when_profile_flag_present() {
        let result =
            format_cli_command_with_env("openacosmi --profile prod status", Some("staging"));
        assert_eq!(result, "openacosmi --profile prod status");
    }

    #[test]
    fn no_injection_when_dev_flag_present() {
        let result = format_cli_command_with_env("openacosmi --dev status", Some("staging"));
        assert_eq!(result, "openacosmi --dev status");
    }

    #[test]
    fn no_injection_for_non_cli_command() {
        let result = format_cli_command_with_env("ls -la", Some("staging"));
        assert_eq!(result, "ls -la");
    }

    #[test]
    fn normalize_profile_name_trims() {
        assert_eq!(normalize_profile_name(Some("  prod  ")), Some("prod".to_string()));
    }

    #[test]
    fn normalize_profile_name_none_for_empty() {
        assert_eq!(normalize_profile_name(Some("")), None);
        assert_eq!(normalize_profile_name(None), None);
    }
}
