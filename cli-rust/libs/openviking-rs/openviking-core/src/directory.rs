// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! Preset directory structure definitions for the OpenViking virtual filesystem.
//!
//! Ported from `openviking/core/directories.py`.
//!
//! Key differences from the Python original:
//! - `PRESET_DIRECTORIES` is a function returning `HashMap` rather than a
//!   module-level mutable dict.
//! - `DirectoryInitializer` is **excluded** — it couples to VikingDB and AGFS,
//!   which belong to the IO / storage layer.

use std::collections::HashMap;

use crate::context::ContextType;

// ---------------------------------------------------------------------------
// DirectoryDefinition
// ---------------------------------------------------------------------------

/// A node in the preset directory tree blueprint.
///
/// This is a **data-only** definition. Actual directory creation (writing to
/// AGFS / vector store) is performed by higher-level crates that implement the
/// necessary IO traits.
#[derive(Debug, Clone)]
pub struct DirectoryDefinition {
    /// Relative path within its scope, e.g. `"memories"` or `"memories/preferences"`.
    pub path: &'static str,
    /// L0 abstract summary.
    pub abstract_text: &'static str,
    /// L1 overview / description used for vectorization.
    pub overview: &'static str,
    /// Child directories.
    pub children: Vec<DirectoryDefinition>,
}

impl DirectoryDefinition {
    /// Create a leaf definition (no children).
    #[must_use]
    const fn leaf(path: &'static str, abstract_text: &'static str, overview: &'static str) -> Self {
        Self {
            path,
            abstract_text,
            overview,
            children: Vec::new(),
        }
    }

    /// Count total nodes in this subtree (inclusive).
    #[must_use]
    pub fn total_nodes(&self) -> usize {
        1 + self.children.iter().map(Self::total_nodes).sum::<usize>()
    }
}

// ---------------------------------------------------------------------------
// Preset directory tree
// ---------------------------------------------------------------------------

/// Build the preset directory tree.
///
/// Returns a map from scope name (`"session"`, `"user"`, `"agent"`,
/// `"resources"`, `"transactions"`) to the root `DirectoryDefinition` for that
/// scope.
///
/// This is a function (not a `static`) to avoid interior mutability / lazy_static
/// complexity — the tree is cheap to construct.
#[must_use]
pub fn preset_directories() -> HashMap<&'static str, DirectoryDefinition> {
    let mut map = HashMap::with_capacity(5);

    // ── session ────────────────────────────────────────────────────────
    map.insert(
        "session",
        DirectoryDefinition::leaf(
            "",
            "Session scope. Stores complete context for a single conversation, \
             including original messages and compressed summaries.",
            "Session-level temporary data storage, can be archived or cleaned \
             after session ends.",
        ),
    );

    // ── user ───────────────────────────────────────────────────────────
    map.insert(
        "user",
        DirectoryDefinition {
            path: "",
            abstract_text: "User scope. Stores user's long-term memory, persisted across sessions.",
            overview: "User-level persistent data storage for building user profiles \
                       and managing private memories.",
            children: vec![DirectoryDefinition {
                path: "memories",
                abstract_text: "User's long-term memory storage. Contains memory types \
                                like preferences, entities, events, managed hierarchically by type.",
                overview: "Use this directory to access user's personalized memories. \
                           Contains three main categories: 1) preferences-user preferences, \
                           2) entities-entity memories, 3) events-event records.",
                children: vec![
                    DirectoryDefinition::leaf(
                        "preferences",
                        "User's personalized preference memories. Stores preferences by topic \
                         (communication style, code standards, domain interests, etc.), \
                         one subdirectory per preference type, same-type preferences can be appended.",
                        "Access when adjusting output style, following user habits, or providing \
                         personalized services. Examples: user prefers concise communication, \
                         code needs type annotations, focus on certain tech domains. \
                         Preferences organized by topic, same-type preferences aggregated in same subdirectory.",
                    ),
                    DirectoryDefinition::leaf(
                        "entities",
                        "Entity memories from user's world. Each entity has its own subdirectory, \
                         including projects, people, concepts, etc. Entities are important objects \
                         in user's world, can append additional information.",
                        "Access when referencing user-related projects, people, concepts. \
                         Examples: OpenViking project, colleague Zhang San, certain technical concept. \
                         Each entity stored independently, can append updates.",
                    ),
                    DirectoryDefinition::leaf(
                        "events",
                        "User's event records. Each event has its own subdirectory, recording \
                         important events, decisions, milestones, etc. Events are time-independent, \
                         historical records not updated.",
                        "Access when reviewing user history, understanding event context, or \
                         tracking user progress. Examples: decided to refactor memory system, \
                         completed a project, attended an event. Events are historical records, \
                         not updated once created.",
                    ),
                ],
            }],
        },
    );

    // ── agent ──────────────────────────────────────────────────────────
    map.insert(
        "agent",
        DirectoryDefinition {
            path: "",
            abstract_text: "Agent scope. Stores Agent's learning memories, instructions, and skills.",
            overview: "Agent-level global data storage. Contains three main categories: \
                       memories-learning memories, instructions-directives, skills-capability registry.",
            children: vec![
                DirectoryDefinition {
                    path: "memories",
                    abstract_text: "Agent's long-term memory storage. Contains cases and patterns, \
                                    managed hierarchically by type.",
                    overview: "Use this directory to access Agent's learning memories. \
                               Contains two main categories: 1) cases-specific cases, \
                               2) patterns-reusable patterns.",
                    children: vec![
                        DirectoryDefinition::leaf(
                            "cases",
                            "Agent's case records. Stores specific problems and solutions, \
                             new problems and resolution processes encountered in each interaction.",
                            "Access cases when encountering similar problems, reference \
                             historical solutions. Cases are records of specific conversations, \
                             each independent and not updated.",
                        ),
                        DirectoryDefinition::leaf(
                            "patterns",
                            "Agent's effective patterns. Stores reusable processes and best \
                             practices distilled from multiple interactions, validated general solutions.",
                            "Access patterns when executing tasks requiring strategy selection \
                             or process determination. Patterns are highly distilled experiences, \
                             each independent and not updated; create new pattern if modification needed.",
                        ),
                    ],
                },
                DirectoryDefinition::leaf(
                    "instructions",
                    "Agent instruction set. Contains Agent's behavioral directives, rules, \
                     and constraints.",
                    "Access when Agent needs to follow specific rules. Examples: planner \
                     agent has specific planning process requirements, executor agent has \
                     execution standards, etc.",
                ),
                DirectoryDefinition::leaf(
                    "skills",
                    "Agent's skill registry. Uses Claude Skills protocol format, flat storage \
                     of callable skill definitions.",
                    "Access when Agent needs to execute specific tasks. Skills categorized \
                     by tags, should retrieve relevant skills before executing tasks, select \
                     most appropriate skill to execute.",
                ),
            ],
        },
    );

    // ── resources ──────────────────────────────────────────────────────
    map.insert(
        "resources",
        DirectoryDefinition::leaf(
            "",
            "Resources scope. Independent knowledge and resource storage, not bound \
             to specific account or Agent.",
            "Globally shared resource storage, organized by project/topic. No preset \
             subdirectory structure, users create project directories as needed.",
        ),
    );

    // ── transactions ───────────────────────────────────────────────────
    map.insert(
        "transactions",
        DirectoryDefinition::leaf(
            "",
            "Transaction scope. Stores transaction records.",
            "Per-account transaction storage.",
        ),
    );

    map
}

// ---------------------------------------------------------------------------
// URI → ContextType helper
// ---------------------------------------------------------------------------

/// Determine the [`ContextType`] from a Viking URI.
///
/// Port of `get_context_type_for_uri()` from `directories.py`.
///
/// Note: The Python original truncates to `uri[:20]` for efficiency, but that
/// causes a bug where `"/memories"` gets clipped to `"/memori"` for URIs like
/// `viking://user/memories/...`. We intentionally fix this by checking the
/// full URI — the performance difference is negligible on short URI strings.
#[must_use]
pub fn context_type_for_uri(uri: &str) -> ContextType {
    if uri.contains("/memories") {
        ContextType::Memory
    } else if uri.contains("/resources") {
        ContextType::Resource
    } else if uri.contains("/skills") {
        ContextType::Skill
    } else if uri.starts_with("viking://session") {
        ContextType::Memory
    } else {
        ContextType::Resource
    }
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn preset_has_five_scopes() {
        let dirs = preset_directories();
        assert_eq!(dirs.len(), 5);
        assert!(dirs.contains_key("session"));
        assert!(dirs.contains_key("user"));
        assert!(dirs.contains_key("agent"));
        assert!(dirs.contains_key("resources"));
        assert!(dirs.contains_key("transactions"));
    }

    #[test]
    fn user_scope_has_three_memory_children() {
        let dirs = preset_directories();
        let user = &dirs["user"];
        // user → memories
        assert_eq!(user.children.len(), 1);
        let memories = &user.children[0];
        assert_eq!(memories.path, "memories");
        // memories → preferences, entities, events
        assert_eq!(memories.children.len(), 3);
    }

    #[test]
    fn agent_scope_has_three_top_children() {
        let dirs = preset_directories();
        let agent = &dirs["agent"];
        // agent → memories, instructions, skills
        assert_eq!(agent.children.len(), 3);
    }

    #[test]
    fn total_nodes_count() {
        let dirs = preset_directories();
        // user: root(1) + memories(1) + preferences(1) + entities(1) + events(1) = 5
        assert_eq!(dirs["user"].total_nodes(), 5);
        // agent: root(1) + memories(1) + cases(1) + patterns(1) + instructions(1) + skills(1) = 6
        assert_eq!(dirs["agent"].total_nodes(), 6);
        // session: 1 (leaf)
        assert_eq!(dirs["session"].total_nodes(), 1);
    }

    #[test]
    fn context_type_for_uri_mapping() {
        assert_eq!(
            context_type_for_uri("viking://user/memories/preferences/x"),
            ContextType::Memory
        );
        assert_eq!(
            context_type_for_uri("viking://resources/docs"),
            ContextType::Resource
        );
        assert_eq!(
            context_type_for_uri("viking://agent/skills/search"),
            ContextType::Skill
        );
        assert_eq!(
            context_type_for_uri("viking://session/abc"),
            ContextType::Memory
        );
    }
}
