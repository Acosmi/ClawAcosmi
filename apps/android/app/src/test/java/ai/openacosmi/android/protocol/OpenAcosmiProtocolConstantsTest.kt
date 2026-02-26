package ai.openacosmi.android.protocol

import org.junit.Assert.assertEquals
import org.junit.Test

class OpenAcosmiProtocolConstantsTest {
  @Test
  fun canvasCommandsUseStableStrings() {
    assertEquals("canvas.present", OpenAcosmiCanvasCommand.Present.rawValue)
    assertEquals("canvas.hide", OpenAcosmiCanvasCommand.Hide.rawValue)
    assertEquals("canvas.navigate", OpenAcosmiCanvasCommand.Navigate.rawValue)
    assertEquals("canvas.eval", OpenAcosmiCanvasCommand.Eval.rawValue)
    assertEquals("canvas.snapshot", OpenAcosmiCanvasCommand.Snapshot.rawValue)
  }

  @Test
  fun a2uiCommandsUseStableStrings() {
    assertEquals("canvas.a2ui.push", OpenAcosmiCanvasA2UICommand.Push.rawValue)
    assertEquals("canvas.a2ui.pushJSONL", OpenAcosmiCanvasA2UICommand.PushJSONL.rawValue)
    assertEquals("canvas.a2ui.reset", OpenAcosmiCanvasA2UICommand.Reset.rawValue)
  }

  @Test
  fun capabilitiesUseStableStrings() {
    assertEquals("canvas", OpenAcosmiCapability.Canvas.rawValue)
    assertEquals("camera", OpenAcosmiCapability.Camera.rawValue)
    assertEquals("screen", OpenAcosmiCapability.Screen.rawValue)
    assertEquals("voiceWake", OpenAcosmiCapability.VoiceWake.rawValue)
  }

  @Test
  fun screenCommandsUseStableStrings() {
    assertEquals("screen.record", OpenAcosmiScreenCommand.Record.rawValue)
  }
}
