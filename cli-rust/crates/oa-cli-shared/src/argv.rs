/// Argument parsing utilities.
///
/// Provides lightweight helpers for inspecting raw CLI argument vectors
/// without a full argument parser. Useful for early checks before
/// `clap` takes over.
///
/// Source: `src/cli/argv.ts`

/// Flags that indicate help was requested.
///
/// Source: `src/cli/argv.ts` – `HELP_FLAGS`
const HELP_FLAGS: &[&str] = &["-h", "--help"];

/// Flags that indicate version was requested.
///
/// Source: `src/cli/argv.ts` – `VERSION_FLAGS`
const VERSION_FLAGS: &[&str] = &["-v", "-V", "--version"];

/// Sentinel that terminates flag parsing.
///
/// Source: `src/cli/argv.ts` – `FLAG_TERMINATOR`
const FLAG_TERMINATOR: &str = "--";

/// Check whether any help or version flag appears in the argument list.
///
/// Source: `src/cli/argv.ts` – `hasHelpOrVersion`
pub fn has_help_or_version(argv: &[String]) -> bool {
    argv.iter().any(|arg| {
        HELP_FLAGS.contains(&arg.as_str()) || VERSION_FLAGS.contains(&arg.as_str())
    })
}

/// Check whether a value token is a positional argument (not a flag).
///
/// Returns `false` for `None`, the flag terminator `--`, and tokens
/// starting with `-` (unless they look like negative numbers).
///
/// Source: `src/cli/argv.ts` – `isValueToken`
fn is_value_token(arg: Option<&str>) -> bool {
    match arg {
        None => false,
        Some(s) if s == FLAG_TERMINATOR => false,
        Some(s) if !s.starts_with('-') => true,
        // Allow negative numeric values like `-1`, `-3.14`
        Some(s) => {
            let without_dash = &s[1..];
            without_dash.parse::<f64>().is_ok()
        }
    }
}

/// Check whether a flag is present in the argument list.
///
/// Skips the first two elements of `argv` (binary name and script)
/// to match the Node.js `process.argv.slice(2)` convention.
/// Stops at the flag terminator `--`.
///
/// Source: `src/cli/argv.ts` – `hasFlag`
pub fn has_flag(argv: &[String], name: &str) -> bool {
    let args = if argv.len() > 2 { &argv[2..] } else { &[] };
    for arg in args {
        if arg == FLAG_TERMINATOR {
            break;
        }
        if arg == name {
            return true;
        }
    }
    false
}

/// Get the value of a named flag from the argument list.
///
/// Supports both `--flag value` and `--flag=value` forms.
/// Returns `Some(value)` when the flag is found with a value,
/// `Some("")` when the flag is present without a value,
/// and `None` when the flag is not present.
///
/// Skips the first two elements of `argv` (binary name and script).
///
/// Source: `src/cli/argv.ts` – `getFlagValue`
pub fn get_flag_value(argv: &[String], name: &str) -> Option<String> {
    let args = if argv.len() > 2 { &argv[2..] } else { &[] };
    for (i, arg) in args.iter().enumerate() {
        if arg == FLAG_TERMINATOR {
            break;
        }
        if arg == name {
            let next = args.get(i + 1).map(|s| s.as_str());
            return if is_value_token(next) {
                Some(next.unwrap_or_default().to_string())
            } else {
                Some(String::new())
            };
        }
        let prefix = format!("{name}=");
        if let Some(rest) = arg.strip_prefix(&prefix) {
            return if rest.is_empty() {
                Some(String::new())
            } else {
                Some(rest.to_string())
            };
        }
    }
    None
}

/// Check whether the `--verbose` flag (or optionally `--debug`) is present.
///
/// Source: `src/cli/argv.ts` – `getVerboseFlag`
pub fn get_verbose_flag(argv: &[String], include_debug: bool) -> bool {
    if has_flag(argv, "--verbose") {
        return true;
    }
    if include_debug && has_flag(argv, "--debug") {
        return true;
    }
    false
}

/// Extract the command path (positional arguments) from the argument list.
///
/// Returns up to `depth` positional tokens, skipping flags.
/// Stops at the flag terminator `--`.
///
/// Source: `src/cli/argv.ts` – `getCommandPath`
pub fn get_command_path(argv: &[String], depth: usize) -> Vec<String> {
    let args = if argv.len() > 2 { &argv[2..] } else { &[] };
    let mut path = Vec::new();
    for arg in args {
        if arg.is_empty() {
            continue;
        }
        if arg == "--" {
            break;
        }
        if arg.starts_with('-') {
            continue;
        }
        path.push(arg.clone());
        if path.len() >= depth {
            break;
        }
    }
    path
}

/// Get the primary (first positional) command from the argument list.
///
/// Source: `src/cli/argv.ts` – `getPrimaryCommand`
pub fn get_primary_command(argv: &[String]) -> Option<String> {
    let path = get_command_path(argv, 1);
    path.into_iter().next()
}

/// Determine whether state migration should run for the given command path.
///
/// Certain read-only or diagnostic commands skip migration to avoid
/// side effects.
///
/// Source: `src/cli/argv.ts` – `shouldMigrateStateFromPath`
pub fn should_migrate_state_from_path(path: &[String]) -> bool {
    if path.is_empty() {
        return true;
    }
    let primary = path[0].as_str();
    if primary == "health" || primary == "status" || primary == "sessions" {
        return false;
    }
    if primary == "memory" {
        if let Some(secondary) = path.get(1) {
            if secondary == "status" {
                return false;
            }
        }
    }
    if primary == "agent" {
        return false;
    }
    true
}

/// Determine whether state migration should run, given a raw argv.
///
/// Source: `src/cli/argv.ts` – `shouldMigrateState`
pub fn should_migrate_state(argv: &[String]) -> bool {
    let path = get_command_path(argv, 2);
    should_migrate_state_from_path(&path)
}

#[cfg(test)]
mod tests {
    use super::*;

    fn argv(args: &[&str]) -> Vec<String> {
        args.iter().map(|s| (*s).to_string()).collect()
    }

    #[test]
    fn has_help_or_version_detects_help() {
        assert!(has_help_or_version(&argv(&["openacosmi", "status", "-h"])));
        assert!(has_help_or_version(&argv(&["openacosmi", "--help"])));
    }

    #[test]
    fn has_help_or_version_detects_version() {
        assert!(has_help_or_version(&argv(&["--version"])));
        assert!(has_help_or_version(&argv(&["-V"])));
        assert!(has_help_or_version(&argv(&["-v"])));
    }

    #[test]
    fn has_help_or_version_negative() {
        assert!(!has_help_or_version(&argv(&["openacosmi", "status"])));
        assert!(!has_help_or_version(&argv(&[])));
    }

    #[test]
    fn has_flag_finds_flag() {
        assert!(has_flag(&argv(&["node", "cli", "--verbose"]), "--verbose"));
    }

    #[test]
    fn has_flag_stops_at_terminator() {
        assert!(!has_flag(
            &argv(&["node", "cli", "--", "--verbose"]),
            "--verbose"
        ));
    }

    #[test]
    fn has_flag_negative() {
        assert!(!has_flag(&argv(&["node", "cli", "status"]), "--verbose"));
    }

    #[test]
    fn get_flag_value_separate() {
        let result = get_flag_value(&argv(&["node", "cli", "--port", "8080"]), "--port");
        assert_eq!(result, Some("8080".to_string()));
    }

    #[test]
    fn get_flag_value_equals() {
        let result = get_flag_value(&argv(&["node", "cli", "--port=8080"]), "--port");
        assert_eq!(result, Some("8080".to_string()));
    }

    #[test]
    fn get_flag_value_missing() {
        let result = get_flag_value(&argv(&["node", "cli", "status"]), "--port");
        assert_eq!(result, None);
    }

    #[test]
    fn get_flag_value_no_value() {
        let result = get_flag_value(&argv(&["node", "cli", "--json"]), "--json");
        assert_eq!(result, Some(String::new()));
    }

    #[test]
    fn get_flag_value_flag_before_another_flag() {
        let result = get_flag_value(&argv(&["node", "cli", "--json", "--verbose"]), "--json");
        assert_eq!(result, Some(String::new()));
    }

    #[test]
    fn get_primary_command_extracts_first() {
        let result = get_primary_command(&argv(&["node", "cli", "status", "--json"]));
        assert_eq!(result, Some("status".to_string()));
    }

    #[test]
    fn get_primary_command_skips_flags() {
        let result = get_primary_command(&argv(&["node", "cli", "--verbose", "health"]));
        assert_eq!(result, Some("health".to_string()));
    }

    #[test]
    fn get_primary_command_none_when_empty() {
        let result = get_primary_command(&argv(&["node", "cli"]));
        assert_eq!(result, None);
    }

    #[test]
    fn get_command_path_depth() {
        let result = get_command_path(&argv(&["node", "cli", "gateway", "status", "--json"]), 2);
        assert_eq!(result, vec!["gateway".to_string(), "status".to_string()]);
    }

    #[test]
    fn get_command_path_stops_at_depth() {
        let result = get_command_path(&argv(&["node", "cli", "a", "b", "c"]), 2);
        assert_eq!(result, vec!["a".to_string(), "b".to_string()]);
    }

    #[test]
    fn get_verbose_flag_verbose() {
        assert!(get_verbose_flag(&argv(&["node", "cli", "--verbose"]), false));
    }

    #[test]
    fn get_verbose_flag_debug() {
        assert!(get_verbose_flag(
            &argv(&["node", "cli", "--debug"]),
            true
        ));
        assert!(!get_verbose_flag(
            &argv(&["node", "cli", "--debug"]),
            false
        ));
    }

    #[test]
    fn is_value_token_positional() {
        assert!(is_value_token(Some("hello")));
        assert!(is_value_token(Some("8080")));
    }

    #[test]
    fn is_value_token_negative_number() {
        assert!(is_value_token(Some("-42")));
        assert!(is_value_token(Some("-3.14")));
    }

    #[test]
    fn is_value_token_flag() {
        assert!(!is_value_token(Some("--verbose")));
        assert!(!is_value_token(Some("-v")));
    }

    #[test]
    fn is_value_token_none() {
        assert!(!is_value_token(None));
    }

    #[test]
    fn is_value_token_terminator() {
        assert!(!is_value_token(Some("--")));
    }

    #[test]
    fn should_migrate_state_from_path_empty() {
        assert!(should_migrate_state_from_path(&[]));
    }

    #[test]
    fn should_migrate_state_from_path_skips_health() {
        assert!(!should_migrate_state_from_path(&argv(&["health"])));
        assert!(!should_migrate_state_from_path(&argv(&["status"])));
        assert!(!should_migrate_state_from_path(&argv(&["sessions"])));
    }

    #[test]
    fn should_migrate_state_from_path_skips_memory_status() {
        assert!(!should_migrate_state_from_path(&argv(&["memory", "status"])));
    }

    #[test]
    fn should_migrate_state_from_path_skips_agent() {
        assert!(!should_migrate_state_from_path(&argv(&["agent"])));
    }

    #[test]
    fn should_migrate_state_from_path_allows_deploy() {
        assert!(should_migrate_state_from_path(&argv(&["deploy"])));
    }

    #[test]
    fn should_migrate_state_uses_argv() {
        assert!(!should_migrate_state(&argv(&["node", "cli", "health"])));
        assert!(should_migrate_state(&argv(&["node", "cli", "deploy"])));
    }
}
