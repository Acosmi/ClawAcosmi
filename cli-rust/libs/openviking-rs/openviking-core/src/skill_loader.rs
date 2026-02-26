// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! SKILL.md loader and parser.
//!
//! Ported from `openviking/core/skill_loader.py`.
//!
//! A SKILL.md file has the format:
//! ```text
//! ---
//! name: my-skill
//! description: Does awesome things
//! tags: [search, memory]
//! allowed-tools: [web_search]
//! ---
//!
//! Detailed instructions go here...
//! ```

use std::collections::HashMap;

use regex::Regex;
use serde::{Deserialize, Serialize};

// ---------------------------------------------------------------------------
// SkillDefinition
// ---------------------------------------------------------------------------

/// Parsed representation of a SKILL.md file.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SkillDefinition {
    /// Skill name (required).
    pub name: String,
    /// Short description (required).
    pub description: String,
    /// Markdown body (instructions).
    pub content: String,
    /// Source file path (empty if parsed from string).
    #[serde(default)]
    pub source_path: String,
    /// Allowed tool names.
    #[serde(default)]
    pub allowed_tools: Vec<String>,
    /// Classification tags.
    #[serde(default)]
    pub tags: Vec<String>,
}

// ---------------------------------------------------------------------------
// SkillLoader
// ---------------------------------------------------------------------------

/// Parses SKILL.md files with YAML frontmatter.
pub struct SkillLoader;

impl SkillLoader {
    /// Load and parse a SKILL.md file from disk.
    ///
    /// This is the Rust equivalent of Python `SkillLoader.load(path)`.
    ///
    /// # Errors
    /// Returns `Err` if the file cannot be read or parsing fails.
    pub fn load(path: &str) -> Result<SkillDefinition, String> {
        let content = std::fs::read_to_string(path)
            .map_err(|e| format!("Skill file not found or unreadable: {path}: {e}"))?;
        Self::parse(&content, path)
    }

    /// Parse SKILL.md content string into a [`SkillDefinition`].
    ///
    /// # Errors
    /// Returns `Err` if:
    /// - No YAML frontmatter is found (delimited by `---`).
    /// - YAML is invalid.
    /// - `name` or `description` fields are missing.
    pub fn parse(content: &str, source_path: &str) -> Result<SkillDefinition, String> {
        let (frontmatter, body) = Self::split_frontmatter(content);

        let fm = frontmatter.ok_or("SKILL.md must have YAML frontmatter")?;

        let meta: HashMap<String, serde_yaml::Value> =
            serde_yaml::from_str(&fm).map_err(|e| format!("Invalid YAML frontmatter: {e}"))?;

        let name = meta
            .get("name")
            .and_then(|v| v.as_str())
            .ok_or("Skill must have 'name' field")?
            .to_owned();

        let description = meta
            .get("description")
            .and_then(|v| v.as_str())
            .ok_or("Skill must have 'description' field")?
            .to_owned();

        let allowed_tools = Self::extract_string_list(&meta, "allowed-tools");
        let tags = Self::extract_string_list(&meta, "tags");

        Ok(SkillDefinition {
            name,
            description,
            content: body.trim().to_owned(),
            source_path: source_path.to_owned(),
            allowed_tools,
            tags,
        })
    }

    /// Convert a [`SkillDefinition`] back to SKILL.md format.
    #[must_use]
    pub fn to_skill_md(skill: &SkillDefinition) -> String {
        // Build minimal YAML frontmatter (name + description only).
        let mut fm = HashMap::new();
        fm.insert("name", &skill.name);
        fm.insert("description", &skill.description);

        let yaml_str = serde_yaml::to_string(&fm).unwrap_or_default();
        format!("---\n{yaml_str}---\n\n{}", skill.content)
    }

    // -----------------------------------------------------------------------
    // Internal
    // -----------------------------------------------------------------------

    /// Split `---` delimited frontmatter from the Markdown body.
    fn split_frontmatter(content: &str) -> (Option<String>, String) {
        use std::sync::OnceLock;
        static RE: OnceLock<Regex> = OnceLock::new();
        let re = RE.get_or_init(|| {
            Regex::new(r"(?s)^---\s*\n(.*?)\n---\s*\n(.*)$")
                .expect("frontmatter regex is a compile-time constant")
        });
        match re.captures(content) {
            Some(caps) => {
                let fm = caps.get(1).map(|m| m.as_str().to_owned());
                let body = caps.get(2).map_or("", |m| m.as_str()).to_owned();
                (fm, body)
            }
            None => (None, content.to_owned()),
        }
    }

    /// Extract a list of strings from a YAML value.
    fn extract_string_list(meta: &HashMap<String, serde_yaml::Value>, key: &str) -> Vec<String> {
        meta.get(key)
            .and_then(|v| v.as_sequence())
            .map(|seq| {
                seq.iter()
                    .filter_map(|item| item.as_str().map(|s| s.to_owned()))
                    .collect()
            })
            .unwrap_or_default()
    }
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;

    const SAMPLE_SKILL: &str = "\
---
name: search-skill
description: Performs web searches
tags: [search, web]
allowed-tools: [web_search, browse]
---

## Instructions

Use this skill to search the web for information.
";

    #[test]
    fn parse_valid_skill_md() {
        let skill = SkillLoader::parse(SAMPLE_SKILL, "/skills/search.md").unwrap();
        assert_eq!(skill.name, "search-skill");
        assert_eq!(skill.description, "Performs web searches");
        assert_eq!(skill.tags, vec!["search", "web"]);
        assert_eq!(skill.allowed_tools, vec!["web_search", "browse"]);
        assert!(skill.content.contains("## Instructions"));
        assert_eq!(skill.source_path, "/skills/search.md");
    }

    #[test]
    fn parse_missing_frontmatter_fails() {
        let result = SkillLoader::parse("# Just markdown\nNo frontmatter here.", "");
        assert!(result.is_err());
        assert!(result.unwrap_err().contains("frontmatter"));
    }

    #[test]
    fn parse_missing_name_fails() {
        let input = "---\ndescription: something\n---\n\nbody";
        let result = SkillLoader::parse(input, "");
        assert!(result.is_err());
        assert!(result.unwrap_err().contains("name"));
    }

    #[test]
    fn parse_missing_description_fails() {
        let input = "---\nname: test\n---\n\nbody";
        let result = SkillLoader::parse(input, "");
        assert!(result.is_err());
        assert!(result.unwrap_err().contains("description"));
    }

    #[test]
    fn roundtrip_to_skill_md() {
        let skill = SkillLoader::parse(SAMPLE_SKILL, "").unwrap();
        let output = SkillLoader::to_skill_md(&skill);

        // Re-parse the output
        let reparsed = SkillLoader::parse(&output, "").unwrap();
        assert_eq!(reparsed.name, skill.name);
        assert_eq!(reparsed.description, skill.description);
        assert!(reparsed.content.contains("## Instructions"));
    }

    #[test]
    fn skill_definition_serde_roundtrip() {
        let skill = SkillLoader::parse(SAMPLE_SKILL, "/x").unwrap();
        let json = serde_json::to_string(&skill).unwrap();
        let restored: SkillDefinition = serde_json::from_str(&json).unwrap();
        assert_eq!(restored.name, skill.name);
        assert_eq!(restored.tags, skill.tags);
    }
}
