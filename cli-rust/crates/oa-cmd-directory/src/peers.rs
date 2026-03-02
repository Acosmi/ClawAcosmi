/// Directory peers commands.
use anyhow::Result;

pub fn peers_list_command(channel: &str, query: Option<&str>, json: bool) -> Result<()> {
    let q = query.unwrap_or("(none)");
    if json {
        println!(
            r#"{{"channel":"{channel}","query":"{q}","peers":[],"status":"not_implemented"}}"#
        );
    } else {
        println!("👥 Directory peers list ({channel}, query={q}) not yet implemented");
    }
    Ok(())
}
