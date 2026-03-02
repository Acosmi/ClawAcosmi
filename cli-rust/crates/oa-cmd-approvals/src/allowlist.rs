/// Approvals allowlist commands.
use anyhow::Result;

/// Add a pattern to the exec allowlist.
pub fn allowlist_add_command(pattern: &str, agent: Option<&str>, node: Option<&str>) -> Result<()> {
    let agent_str = agent.unwrap_or("*");
    let target = node.map_or("local".to_string(), |n| format!("node:{n}"));
    println!(
        "➕ Allowlist add '{pattern}' (agent={agent_str}, target={target}) not yet implemented"
    );
    Ok(())
}

/// Remove a pattern from the exec allowlist.
pub fn allowlist_remove_command(pattern: &str) -> Result<()> {
    println!("➖ Allowlist remove '{pattern}' not yet implemented");
    Ok(())
}
