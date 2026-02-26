import Foundation

// Stable identifier used for both the macOS LaunchAgent label and Nix-managed defaults suite.
// nix-openacosmi writes app defaults into this suite to survive app bundle identifier churn.
let launchdLabel = "ai.openacosmi.mac"
let gatewayLaunchdLabel = "ai.openacosmi.gateway"
let onboardingVersionKey = "openacosmi.onboardingVersion"
let onboardingSeenKey = "openacosmi.onboardingSeen"
let currentOnboardingVersion = 7
let pauseDefaultsKey = "openacosmi.pauseEnabled"
let iconAnimationsEnabledKey = "openacosmi.iconAnimationsEnabled"
let swabbleEnabledKey = "openacosmi.swabbleEnabled"
let swabbleTriggersKey = "openacosmi.swabbleTriggers"
let voiceWakeTriggerChimeKey = "openacosmi.voiceWakeTriggerChime"
let voiceWakeSendChimeKey = "openacosmi.voiceWakeSendChime"
let showDockIconKey = "openacosmi.showDockIcon"
let defaultVoiceWakeTriggers = ["openacosmi"]
let voiceWakeMaxWords = 32
let voiceWakeMaxWordLength = 64
let voiceWakeMicKey = "openacosmi.voiceWakeMicID"
let voiceWakeMicNameKey = "openacosmi.voiceWakeMicName"
let voiceWakeLocaleKey = "openacosmi.voiceWakeLocaleID"
let voiceWakeAdditionalLocalesKey = "openacosmi.voiceWakeAdditionalLocaleIDs"
let voicePushToTalkEnabledKey = "openacosmi.voicePushToTalkEnabled"
let talkEnabledKey = "openacosmi.talkEnabled"
let iconOverrideKey = "openacosmi.iconOverride"
let connectionModeKey = "openacosmi.connectionMode"
let remoteTargetKey = "openacosmi.remoteTarget"
let remoteIdentityKey = "openacosmi.remoteIdentity"
let remoteProjectRootKey = "openacosmi.remoteProjectRoot"
let remoteCliPathKey = "openacosmi.remoteCliPath"
let canvasEnabledKey = "openacosmi.canvasEnabled"
let cameraEnabledKey = "openacosmi.cameraEnabled"
let systemRunPolicyKey = "openacosmi.systemRunPolicy"
let systemRunAllowlistKey = "openacosmi.systemRunAllowlist"
let systemRunEnabledKey = "openacosmi.systemRunEnabled"
let locationModeKey = "openacosmi.locationMode"
let locationPreciseKey = "openacosmi.locationPreciseEnabled"
let peekabooBridgeEnabledKey = "openacosmi.peekabooBridgeEnabled"
let deepLinkKeyKey = "openacosmi.deepLinkKey"
let modelCatalogPathKey = "openacosmi.modelCatalogPath"
let modelCatalogReloadKey = "openacosmi.modelCatalogReload"
let cliInstallPromptedVersionKey = "openacosmi.cliInstallPromptedVersion"
let heartbeatsEnabledKey = "openacosmi.heartbeatsEnabled"
let debugPaneEnabledKey = "openacosmi.debugPaneEnabled"
let debugFileLogEnabledKey = "openacosmi.debug.fileLogEnabled"
let appLogLevelKey = "openacosmi.debug.appLogLevel"
let voiceWakeSupported: Bool = ProcessInfo.processInfo.operatingSystemVersion.majorVersion >= 26
