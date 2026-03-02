/// Voicecall call/continue/end commands.
use anyhow::Result;

pub fn voicecall_call_command(to: &str, message: &str) -> Result<()> {
    println!("📞 Voicecall call to='{to}' message='{message}' not yet implemented");
    Ok(())
}

pub fn voicecall_continue_command(call_id: &str, message: &str) -> Result<()> {
    println!("📞 Voicecall continue call_id='{call_id}' message='{message}' not yet implemented");
    Ok(())
}

pub fn voicecall_end_command(call_id: &str) -> Result<()> {
    println!("📞 Voicecall end call_id='{call_id}' not yet implemented");
    Ok(())
}
