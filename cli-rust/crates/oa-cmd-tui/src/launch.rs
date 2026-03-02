/// TUI launch command.
use anyhow::Result;

pub fn tui_launch_command(
    url: Option<&str>,
    token: Option<&str>,
    session: Option<&str>,
) -> Result<()> {
    let u = url.unwrap_or("(default)");
    let s = session.unwrap_or("(default)");
    println!("🖥️ TUI launch (url={u}, session={s}) not yet implemented");
    let _ = token; // suppress unused warning
    Ok(())
}
