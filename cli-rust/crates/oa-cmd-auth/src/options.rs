/// Auth choice options and provider groups for the interactive wizard.
///
/// Builds the flat list of auth choices and organizes them into provider
/// groups for the two-level selection prompt (first pick a provider group,
/// then pick a specific auth method within that group).
///
/// Source: `src/commands/auth-choice-options.ts`

use crate::auth_choice::{AuthChoice, AuthChoiceGroupId};

/// A single auth choice option for display in the wizard.
///
/// Source: `src/commands/auth-choice-options.ts` - `AuthChoiceOption`
#[derive(Debug, Clone)]
pub struct AuthChoiceOption {
    /// The auth choice value.
    pub value: AuthChoice,
    /// Human-readable label.
    pub label: String,
    /// Optional hint text shown alongside the label.
    pub hint: Option<String>,
}

/// A group of auth choices for a provider.
///
/// Source: `src/commands/auth-choice-options.ts` - `AuthChoiceGroup`
#[derive(Debug, Clone)]
pub struct AuthChoiceGroup {
    /// The group identifier.
    pub value: AuthChoiceGroupId,
    /// Human-readable label for the group.
    pub label: String,
    /// Optional hint text for the group.
    pub hint: Option<String>,
    /// The auth choice options within this group.
    pub options: Vec<AuthChoiceOption>,
}

/// Definition for a group used to map choices to groups.
///
/// Source: `src/commands/auth-choice-options.ts` - `AUTH_CHOICE_GROUP_DEFS`
struct GroupDef {
    value: AuthChoiceGroupId,
    label: &'static str,
    hint: Option<&'static str>,
    choices: &'static [AuthChoice],
}

/// The canonical ordering of provider groups and their constituent auth choices.
///
/// Source: `src/commands/auth-choice-options.ts` - `AUTH_CHOICE_GROUP_DEFS`
const AUTH_CHOICE_GROUP_DEFS: &[GroupDef] = &[
    GroupDef {
        value: AuthChoiceGroupId::Openai,
        label: "OpenAI",
        hint: Some("Codex OAuth + API key"),
        choices: &[AuthChoice::OpenaiCodex, AuthChoice::OpenaiApiKey],
    },
    GroupDef {
        value: AuthChoiceGroupId::Anthropic,
        label: "Anthropic",
        hint: Some("setup-token + API key"),
        choices: &[AuthChoice::Token, AuthChoice::ApiKey],
    },
    GroupDef {
        value: AuthChoiceGroupId::Minimax,
        label: "MiniMax",
        hint: Some("M2.1 (recommended)"),
        choices: &[
            AuthChoice::MinimaxPortal,
            AuthChoice::MinimaxApi,
            AuthChoice::MinimaxApiLightning,
        ],
    },
    GroupDef {
        value: AuthChoiceGroupId::Moonshot,
        label: "Moonshot AI (Kimi K2.5)",
        hint: Some("Kimi K2.5 + Kimi Coding"),
        choices: &[
            AuthChoice::MoonshotApiKey,
            AuthChoice::MoonshotApiKeyCn,
            AuthChoice::KimiCodeApiKey,
        ],
    },
    GroupDef {
        value: AuthChoiceGroupId::Google,
        label: "Google",
        hint: Some("Gemini API key + OAuth"),
        choices: &[
            AuthChoice::GeminiApiKey,
            AuthChoice::GoogleAntigravity,
            AuthChoice::GoogleGeminiCli,
        ],
    },
    GroupDef {
        value: AuthChoiceGroupId::Xai,
        label: "xAI (Grok)",
        hint: Some("API key"),
        choices: &[AuthChoice::XaiApiKey],
    },
    GroupDef {
        value: AuthChoiceGroupId::Openrouter,
        label: "OpenRouter",
        hint: Some("API key"),
        choices: &[AuthChoice::OpenrouterApiKey],
    },
    GroupDef {
        value: AuthChoiceGroupId::Qwen,
        label: "Qwen",
        hint: Some("OAuth"),
        choices: &[AuthChoice::QwenPortal],
    },
    GroupDef {
        value: AuthChoiceGroupId::Zai,
        label: "Z.AI (GLM 4.7)",
        hint: Some("API key"),
        choices: &[AuthChoice::ZaiApiKey],
    },
    GroupDef {
        value: AuthChoiceGroupId::Qianfan,
        label: "Qianfan",
        hint: Some("API key"),
        choices: &[AuthChoice::QianfanApiKey],
    },
    GroupDef {
        value: AuthChoiceGroupId::Copilot,
        label: "Copilot",
        hint: Some("GitHub + local proxy"),
        choices: &[AuthChoice::GithubCopilot, AuthChoice::CopilotProxy],
    },
    GroupDef {
        value: AuthChoiceGroupId::AiGateway,
        label: "Vercel AI Gateway",
        hint: Some("API key"),
        choices: &[AuthChoice::AiGatewayApiKey],
    },
    GroupDef {
        value: AuthChoiceGroupId::OpencodeZen,
        label: "OpenCode Zen",
        hint: Some("API key"),
        choices: &[AuthChoice::OpencodeZen],
    },
    GroupDef {
        value: AuthChoiceGroupId::Xiaomi,
        label: "Xiaomi",
        hint: Some("API key"),
        choices: &[AuthChoice::XiaomiApiKey],
    },
    GroupDef {
        value: AuthChoiceGroupId::Synthetic,
        label: "Synthetic",
        hint: Some("Anthropic-compatible (multi-model)"),
        choices: &[AuthChoice::SyntheticApiKey],
    },
    GroupDef {
        value: AuthChoiceGroupId::Venice,
        label: "Venice AI",
        hint: Some("Privacy-focused (uncensored models)"),
        choices: &[AuthChoice::VeniceApiKey],
    },
    GroupDef {
        value: AuthChoiceGroupId::CloudflareAiGateway,
        label: "Cloudflare AI Gateway",
        hint: Some("Account ID + Gateway ID + API key"),
        choices: &[AuthChoice::CloudflareAiGatewayApiKey],
    },
];

/// Build the flat list of all auth choice options.
///
/// If `include_skip` is `true`, appends a "Skip for now" option at the end.
///
/// Source: `src/commands/auth-choice-options.ts` - `buildAuthChoiceOptions`
#[must_use]
pub fn build_auth_choice_options(include_skip: bool) -> Vec<AuthChoiceOption> {
    let mut options = vec![
        AuthChoiceOption {
            value: AuthChoice::Token,
            label: "Anthropic token (paste setup-token)".to_owned(),
            hint: Some(
                "run `claude setup-token` elsewhere, then paste the token here".to_owned(),
            ),
        },
        AuthChoiceOption {
            value: AuthChoice::OpenaiCodex,
            label: "OpenAI Codex (ChatGPT OAuth)".to_owned(),
            hint: None,
        },
        AuthChoiceOption {
            value: AuthChoice::Chutes,
            label: "Chutes (OAuth)".to_owned(),
            hint: None,
        },
        AuthChoiceOption {
            value: AuthChoice::OpenaiApiKey,
            label: "OpenAI API key".to_owned(),
            hint: None,
        },
        AuthChoiceOption {
            value: AuthChoice::XaiApiKey,
            label: "xAI (Grok) API key".to_owned(),
            hint: None,
        },
        AuthChoiceOption {
            value: AuthChoice::QianfanApiKey,
            label: "Qianfan API key".to_owned(),
            hint: None,
        },
        AuthChoiceOption {
            value: AuthChoice::OpenrouterApiKey,
            label: "OpenRouter API key".to_owned(),
            hint: None,
        },
        AuthChoiceOption {
            value: AuthChoice::AiGatewayApiKey,
            label: "Vercel AI Gateway API key".to_owned(),
            hint: None,
        },
        AuthChoiceOption {
            value: AuthChoice::CloudflareAiGatewayApiKey,
            label: "Cloudflare AI Gateway".to_owned(),
            hint: Some("Account ID + Gateway ID + API key".to_owned()),
        },
        AuthChoiceOption {
            value: AuthChoice::MoonshotApiKey,
            label: "Kimi API key (.ai)".to_owned(),
            hint: None,
        },
        AuthChoiceOption {
            value: AuthChoice::MoonshotApiKeyCn,
            label: "Kimi API key (.cn)".to_owned(),
            hint: None,
        },
        AuthChoiceOption {
            value: AuthChoice::KimiCodeApiKey,
            label: "Kimi Code API key (subscription)".to_owned(),
            hint: None,
        },
        AuthChoiceOption {
            value: AuthChoice::SyntheticApiKey,
            label: "Synthetic API key".to_owned(),
            hint: None,
        },
        AuthChoiceOption {
            value: AuthChoice::VeniceApiKey,
            label: "Venice AI API key".to_owned(),
            hint: Some("Privacy-focused inference (uncensored models)".to_owned()),
        },
        AuthChoiceOption {
            value: AuthChoice::GithubCopilot,
            label: "GitHub Copilot (GitHub device login)".to_owned(),
            hint: Some("Uses GitHub device flow".to_owned()),
        },
        AuthChoiceOption {
            value: AuthChoice::GeminiApiKey,
            label: "Google Gemini API key".to_owned(),
            hint: None,
        },
        AuthChoiceOption {
            value: AuthChoice::GoogleAntigravity,
            label: "Google Antigravity OAuth".to_owned(),
            hint: Some("Uses the bundled Antigravity auth plugin".to_owned()),
        },
        AuthChoiceOption {
            value: AuthChoice::GoogleGeminiCli,
            label: "Google Gemini CLI OAuth".to_owned(),
            hint: Some("Uses the bundled Gemini CLI auth plugin".to_owned()),
        },
        AuthChoiceOption {
            value: AuthChoice::ZaiApiKey,
            label: "Z.AI (GLM 4.7) API key".to_owned(),
            hint: None,
        },
        AuthChoiceOption {
            value: AuthChoice::XiaomiApiKey,
            label: "Xiaomi API key".to_owned(),
            hint: None,
        },
        AuthChoiceOption {
            value: AuthChoice::MinimaxPortal,
            label: "MiniMax OAuth".to_owned(),
            hint: Some("Oauth plugin for MiniMax".to_owned()),
        },
        AuthChoiceOption {
            value: AuthChoice::QwenPortal,
            label: "Qwen OAuth".to_owned(),
            hint: None,
        },
        AuthChoiceOption {
            value: AuthChoice::CopilotProxy,
            label: "Copilot Proxy (local)".to_owned(),
            hint: Some("Local proxy for VS Code Copilot models".to_owned()),
        },
        AuthChoiceOption {
            value: AuthChoice::ApiKey,
            label: "Anthropic API key".to_owned(),
            hint: None,
        },
        AuthChoiceOption {
            value: AuthChoice::OpencodeZen,
            label: "OpenCode Zen (multi-model proxy)".to_owned(),
            hint: Some("Claude, GPT, Gemini via opencode.ai/zen".to_owned()),
        },
        AuthChoiceOption {
            value: AuthChoice::MinimaxApi,
            label: "MiniMax M2.1".to_owned(),
            hint: None,
        },
        AuthChoiceOption {
            value: AuthChoice::MinimaxApiLightning,
            label: "MiniMax M2.1 Lightning".to_owned(),
            hint: Some("Faster, higher output cost".to_owned()),
        },
    ];

    if include_skip {
        options.push(AuthChoiceOption {
            value: AuthChoice::Skip,
            label: "Skip for now".to_owned(),
            hint: None,
        });
    }

    options
}

/// Result of building grouped auth choice options.
///
/// Source: `src/commands/auth-choice-options.ts` - `buildAuthChoiceGroups` return
pub struct AuthChoiceGroupsResult {
    /// The provider groups, each containing their auth method options.
    pub groups: Vec<AuthChoiceGroup>,
    /// The optional "Skip for now" option (not part of any group).
    pub skip_option: Option<AuthChoiceOption>,
}

/// Build the grouped auth choice options for the two-level wizard prompt.
///
/// Organizes the flat option list into provider groups, filtering out
/// any choices that don't have a matching option definition. Optionally
/// includes a standalone "Skip for now" option.
///
/// Source: `src/commands/auth-choice-options.ts` - `buildAuthChoiceGroups`
#[must_use]
pub fn build_auth_choice_groups(include_skip: bool) -> AuthChoiceGroupsResult {
    let options = build_auth_choice_options(false);
    let option_map: std::collections::HashMap<AuthChoice, &AuthChoiceOption> =
        options.iter().map(|opt| (opt.value, opt)).collect();

    let groups = AUTH_CHOICE_GROUP_DEFS
        .iter()
        .map(|def| {
            let group_options: Vec<AuthChoiceOption> = def
                .choices
                .iter()
                .filter_map(|choice| option_map.get(choice).map(|opt| (*opt).clone()))
                .collect();

            AuthChoiceGroup {
                value: def.value,
                label: def.label.to_owned(),
                hint: def.hint.map(str::to_owned),
                options: group_options,
            }
        })
        .collect();

    let skip_option = if include_skip {
        Some(AuthChoiceOption {
            value: AuthChoice::Skip,
            label: "Skip for now".to_owned(),
            hint: None,
        })
    } else {
        None
    };

    AuthChoiceGroupsResult {
        groups,
        skip_option,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn build_options_without_skip() {
        let options = build_auth_choice_options(false);
        assert!(!options.is_empty());
        assert!(
            !options.iter().any(|o| o.value == AuthChoice::Skip),
            "should not include Skip when include_skip is false"
        );
    }

    #[test]
    fn build_options_with_skip() {
        let options = build_auth_choice_options(true);
        assert!(
            options.iter().any(|o| o.value == AuthChoice::Skip),
            "should include Skip when include_skip is true"
        );
    }

    #[test]
    fn build_options_first_is_token() {
        let options = build_auth_choice_options(false);
        assert_eq!(options[0].value, AuthChoice::Token);
    }

    #[test]
    fn build_groups_has_all_groups() {
        let result = build_auth_choice_groups(false);
        assert_eq!(
            result.groups.len(),
            AUTH_CHOICE_GROUP_DEFS.len(),
            "should have one group per definition"
        );
    }

    #[test]
    fn build_groups_openai_has_two_options() {
        let result = build_auth_choice_groups(false);
        let openai_group = result
            .groups
            .iter()
            .find(|g| g.value == AuthChoiceGroupId::Openai)
            .expect("should have OpenAI group");
        assert_eq!(openai_group.options.len(), 2);
        assert_eq!(openai_group.options[0].value, AuthChoice::OpenaiCodex);
        assert_eq!(openai_group.options[1].value, AuthChoice::OpenaiApiKey);
    }

    #[test]
    fn build_groups_anthropic_has_two_options() {
        let result = build_auth_choice_groups(false);
        let group = result
            .groups
            .iter()
            .find(|g| g.value == AuthChoiceGroupId::Anthropic)
            .expect("should have Anthropic group");
        assert_eq!(group.options.len(), 2);
        assert_eq!(group.options[0].value, AuthChoice::Token);
        assert_eq!(group.options[1].value, AuthChoice::ApiKey);
    }

    #[test]
    fn build_groups_skip_option_when_requested() {
        let result = build_auth_choice_groups(true);
        assert!(result.skip_option.is_some());
        assert_eq!(
            result.skip_option.as_ref().map(|o| o.value),
            Some(AuthChoice::Skip)
        );
    }

    #[test]
    fn build_groups_no_skip_when_not_requested() {
        let result = build_auth_choice_groups(false);
        assert!(result.skip_option.is_none());
    }

    #[test]
    fn all_group_labels_are_nonempty() {
        let result = build_auth_choice_groups(false);
        for group in &result.groups {
            assert!(!group.label.is_empty(), "group {:?} has empty label", group.value);
        }
    }

    #[test]
    fn no_group_has_empty_options() {
        let result = build_auth_choice_groups(false);
        for group in &result.groups {
            assert!(
                !group.options.is_empty(),
                "group {:?} has no options",
                group.value
            );
        }
    }
}
