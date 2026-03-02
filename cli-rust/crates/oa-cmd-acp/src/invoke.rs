/// ACP invoke command.
///
/// Invoke an ACP method on the bridge.
///
/// Source: `src/cli/acp-cli.ts`
use anyhow::Result;

/// Invoke an ACP method.
pub fn acp_invoke_command(method: &str, json: bool) -> Result<()> {
    if json {
        println!(
            r#"{{"status":"not_implemented","method":"{}","message":"ACP invoke not yet implemented"}}"#,
            method
        );
    } else {
        println!("🔧 ACP invoke '{method}' not yet implemented");
    }
    Ok(())
}
