/// Node install command.
use anyhow::Result;

pub fn node_install_command(host: Option<&str>, port: Option<u16>) -> Result<()> {
    let h = host.unwrap_or("127.0.0.1");
    let p = port.unwrap_or(19001);
    println!("📦 Node install ({h}:{p}) not yet implemented");
    Ok(())
}
