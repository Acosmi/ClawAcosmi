/// Agent scope, model catalog, model selection, provider metadata, and defaults.
///
/// This crate provides the core agent resolution logic used by CLI commands:
/// - **scope**: Listing agents, resolving defaults, looking up per-agent config
/// - **defaults**: Fallback constants for provider, model, and context tokens
/// - **model_catalog**: Loading and querying the available model catalog
/// - **model_selection**: Parsing model references, allowlist checking, alias resolution
/// - **providers**: Known provider metadata registry
///
/// Source: `src/agents/agent-scope.ts`, `src/agents/defaults.ts`,
///         `src/agents/model-catalog.ts`, `src/agents/model-selection.ts`,
///         `src/agents/models-config.providers.ts`

pub mod defaults;
pub mod model_catalog;
pub mod model_selection;
pub mod providers;
pub mod scope;
