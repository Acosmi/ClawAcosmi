// Copyright (c) 2026 UHMS Team. Licensed under Apache-2.0.
//! Centralized prompt templates for LLM interactions.
//!
//! Replaces hardcoded `format!()` strings scattered across modules.
//! Each template is a `const &str` with `{placeholder}` slots replaced via
//! `.replace()` at call site, matching the Python Jinja template approach.

// ---------------------------------------------------------------------------
// Intent Analysis
// ---------------------------------------------------------------------------

/// Intent analysis prompt template.
///
/// Placeholders: `{summary}`, `{recent}`, `{current}`, `{ctx_type}`, `{target_abstract}`
pub const INTENT_ANALYZE: &str = "\
You are a session intent analyzer. Analyze the conversation context and generate \
a search query plan.

## Session Context
- Compression summary: {summary}
- Recent messages:
{recent}
- Current message: {current}

## Constraints
- Context type filter: {ctx_type}
- Target abstract: {target_abstract}

## Output Format
Respond with valid JSON only:
```json
{
  \"queries\": [
    {
      \"query\": \"semantic search query text\",
      \"context_type\": \"memory|resource|skill\",
      \"intent\": \"why this query is needed\",
      \"priority\": 1
    }
  ],
  \"reasoning\": \"overall reasoning for this query plan\"
}
```

Rules:
- Priority: 1 (highest) to 5 (lowest)
- Generate 1-5 queries covering different aspects
- Each query should be a natural language search term
- context_type must be one of: memory, resource, skill";

// ---------------------------------------------------------------------------
// Memory Extraction
// ---------------------------------------------------------------------------

/// Memory extraction prompt template.
///
/// Placeholders: `{user_id}`, `{output_language}`, `{messages}`
pub const MEMORY_EXTRACT: &str = "\
You are a memory extraction specialist. Analyze the conversation and extract \
long-term memories worth preserving.

## User
{user_id}

## Output Language
{output_language}

## Conversation
{messages}

## Memory Categories
- **profile**: User identity, background, roles, expertise
- **preferences**: User preferences, coding style, tool preferences
- **entities**: Projects, organizations, people, technologies mentioned
- **events**: Significant events, milestones, decisions
- **cases**: Problem-solving patterns, debugging cases, solutions
- **patterns**: Recurring behaviors, workflows, communication patterns

## Output Format
Respond with valid JSON only:
```json
{
  \"memories\": [
    {
      \"category\": \"profile|preferences|entities|events|cases|patterns\",
      \"abstract\": \"One-line summary of this memory\",
      \"overview\": \"2-3 sentence overview with key details\",
      \"content\": \"Full structured content of the memory in markdown\"
    }
  ]
}
```

Rules:
- Only extract genuinely useful long-term information
- Each memory should be self-contained and understandable without context
- Use the specified output language for all text content
- Avoid extracting trivial or ephemeral information";

// ---------------------------------------------------------------------------
// Memory Merge
// ---------------------------------------------------------------------------

/// Memory merge prompt template.
///
/// Placeholders: `{category}`, `{output_language}`,
/// `{existing_abstract}`, `{existing_overview}`, `{existing_content}`,
/// `{new_abstract}`, `{new_overview}`, `{new_content}`
pub const MEMORY_MERGE: &str = "\
You are a memory merge specialist. Combine two memory entries into a single, \
coherent memory entry.

## Category: {category}
## Output Language: {output_language}

## Existing Memory
- Abstract: {existing_abstract}
- Overview: {existing_overview}
- Content:
{existing_content}

## New Memory
- Abstract: {new_abstract}
- Overview: {new_overview}
- Content:
{new_content}

## Output Format
Respond with valid JSON only:
```json
{
  \"abstract\": \"Merged one-line summary\",
  \"overview\": \"Merged 2-3 sentence overview\",
  \"content\": \"Full merged content in markdown\",
  \"reason\": \"Brief explanation of merge strategy\"
}
```

Rules:
- Preserve all unique information from both entries
- Resolve conflicts by preferring newer information
- Keep the merged entry concise but complete
- Use the specified output language";

// ---------------------------------------------------------------------------
// Deduplication
// ---------------------------------------------------------------------------

/// Deduplication decision prompt template.
///
/// Placeholders: `{candidate_abstract}`, `{candidate_overview}`,
/// `{candidate_content}`, `{existing_memories}`
pub const DEDUP_DECIDE: &str = "\
You are a memory deduplication specialist. Decide how to handle a candidate \
memory given existing similar memories.

## Candidate Memory
- Abstract: {candidate_abstract}
- Overview: {candidate_overview}
- Content: {candidate_content}

## Existing Similar Memories
{existing_memories}

## Decision Options
- **skip**: Candidate is a duplicate; discard it entirely
- **create**: Candidate is sufficiently novel; create as new memory
- **none**: No new memory needed, but may require actions on existing memories

## Output Format
Respond with valid JSON only:
```json
{
  \"decision\": \"skip|create|none\",
  \"reason\": \"Brief explanation\",
  \"list\": [
    {
      \"uri\": \"viking://...\",
      \"decide\": \"merge|delete\",
      \"reason\": \"Why this action on the existing memory\"
    }
  ]
}
```

Rules:
- 'list' contains actions for existing memories (merge into or delete)
- 'skip' means the candidate adds no value
- 'create' means the candidate is genuinely new information
- 'none' with 'list' actions means update existing memories instead";

// ---------------------------------------------------------------------------
// Archive Summary
// ---------------------------------------------------------------------------

/// Archive summary prompt template.
///
/// Placeholder: `{messages}`
pub const ARCHIVE_SUMMARY: &str = "\
Summarize the following conversation concisely in structured markdown.

## Conversation
{messages}

## Output Format
Use this structure:
```markdown
**Overview**: One-sentence summary of the conversation topic

**Key Points**:
- Point 1
- Point 2

**Decisions Made**: (if any)
- Decision 1

**Action Items**: (if any)
- Action 1
```

Rules:
- Keep the summary concise (under 200 words)
- Focus on information worth remembering long-term
- Use the same language as the conversation";

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

/// Apply template by replacing `{key}` placeholders.
///
/// # Example
/// ```
/// use openviking_session::prompts::apply;
/// let result = apply("Hello {name}!", &[("name", "world")]);
/// assert_eq!(result, "Hello world!");
/// ```
pub fn apply(template: &str, vars: &[(&str, &str)]) -> String {
    let mut result = template.to_owned();
    for (key, val) in vars {
        result = result.replace(&format!("{{{key}}}"), val);
    }
    result
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn apply_replaces_placeholders() {
        let result = apply(
            "Hello {name}, you are {role}.",
            &[("name", "Alice"), ("role", "admin")],
        );
        assert_eq!(result, "Hello Alice, you are admin.");
    }

    #[test]
    fn apply_preserves_unmatched() {
        let result = apply("Hello {name}, {unknown}", &[("name", "Bob")]);
        assert_eq!(result, "Hello Bob, {unknown}");
    }

    #[test]
    fn templates_have_placeholders() {
        assert!(INTENT_ANALYZE.contains("{summary}"));
        assert!(MEMORY_EXTRACT.contains("{user_id}"));
        assert!(MEMORY_MERGE.contains("{category}"));
        assert!(DEDUP_DECIDE.contains("{candidate_abstract}"));
        assert!(ARCHIVE_SUMMARY.contains("{messages}"));
    }
}
