/// Directory groups commands.
use anyhow::Result;

pub fn groups_list_command(channel: &str, query: Option<&str>, json: bool) -> Result<()> {
    let q = query.unwrap_or("(none)");
    if json {
        println!(
            r#"{{"channel":"{channel}","query":"{q}","groups":[],"status":"not_implemented"}}"#
        );
    } else {
        println!("👥 Directory groups list ({channel}, query={q}) not yet implemented");
    }
    Ok(())
}

pub fn groups_members_command(channel: &str, group_id: &str, json: bool) -> Result<()> {
    if json {
        println!(
            r#"{{"channel":"{channel}","group":"{group_id}","members":[],"status":"not_implemented"}}"#
        );
    } else {
        println!("👥 Directory groups members ({channel}, group={group_id}) not yet implemented");
    }
    Ok(())
}
