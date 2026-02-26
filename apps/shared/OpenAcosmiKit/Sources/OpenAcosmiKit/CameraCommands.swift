import Foundation

public enum OpenAcosmiCameraCommand: String, Codable, Sendable {
    case list = "camera.list"
    case snap = "camera.snap"
    case clip = "camera.clip"
}

public enum OpenAcosmiCameraFacing: String, Codable, Sendable {
    case back
    case front
}

public enum OpenAcosmiCameraImageFormat: String, Codable, Sendable {
    case jpg
    case jpeg
}

public enum OpenAcosmiCameraVideoFormat: String, Codable, Sendable {
    case mp4
}

public struct OpenAcosmiCameraSnapParams: Codable, Sendable, Equatable {
    public var facing: OpenAcosmiCameraFacing?
    public var maxWidth: Int?
    public var quality: Double?
    public var format: OpenAcosmiCameraImageFormat?
    public var deviceId: String?
    public var delayMs: Int?

    public init(
        facing: OpenAcosmiCameraFacing? = nil,
        maxWidth: Int? = nil,
        quality: Double? = nil,
        format: OpenAcosmiCameraImageFormat? = nil,
        deviceId: String? = nil,
        delayMs: Int? = nil)
    {
        self.facing = facing
        self.maxWidth = maxWidth
        self.quality = quality
        self.format = format
        self.deviceId = deviceId
        self.delayMs = delayMs
    }
}

public struct OpenAcosmiCameraClipParams: Codable, Sendable, Equatable {
    public var facing: OpenAcosmiCameraFacing?
    public var durationMs: Int?
    public var includeAudio: Bool?
    public var format: OpenAcosmiCameraVideoFormat?
    public var deviceId: String?

    public init(
        facing: OpenAcosmiCameraFacing? = nil,
        durationMs: Int? = nil,
        includeAudio: Bool? = nil,
        format: OpenAcosmiCameraVideoFormat? = nil,
        deviceId: String? = nil)
    {
        self.facing = facing
        self.durationMs = durationMs
        self.includeAudio = includeAudio
        self.format = format
        self.deviceId = deviceId
    }
}
