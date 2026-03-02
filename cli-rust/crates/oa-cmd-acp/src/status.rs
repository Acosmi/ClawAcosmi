/// ACP status command.
///
/// Show ACP bridge connection status.
///
/// Source: `src/cli/acp-cli.ts`
use anyhow::Result;

/// Show ACP bridge status.
pub fn acp_status_command(json: bool) -> Result<()> {
    if json {
        println!(r#"{{"status":"not_implemented","message":"ACP status not yet implemented"}}"#);
    } else {
        println!("📊 ACP status not yet implemented");
    }
    Ok(())
}
