import Foundation

public enum OpenAcosmiDeviceCommand: String, Codable, Sendable {
    case status = "device.status"
    case info = "device.info"
}

public enum OpenAcosmiBatteryState: String, Codable, Sendable {
    case unknown
    case unplugged
    case charging
    case full
}

public enum OpenAcosmiThermalState: String, Codable, Sendable {
    case nominal
    case fair
    case serious
    case critical
}

public enum OpenAcosmiNetworkPathStatus: String, Codable, Sendable {
    case satisfied
    case unsatisfied
    case requiresConnection
}

public enum OpenAcosmiNetworkInterfaceType: String, Codable, Sendable {
    case wifi
    case cellular
    case wired
    case other
}

public struct OpenAcosmiBatteryStatusPayload: Codable, Sendable, Equatable {
    public var level: Double?
    public var state: OpenAcosmiBatteryState
    public var lowPowerModeEnabled: Bool

    public init(level: Double?, state: OpenAcosmiBatteryState, lowPowerModeEnabled: Bool) {
        self.level = level
        self.state = state
        self.lowPowerModeEnabled = lowPowerModeEnabled
    }
}

public struct OpenAcosmiThermalStatusPayload: Codable, Sendable, Equatable {
    public var state: OpenAcosmiThermalState

    public init(state: OpenAcosmiThermalState) {
        self.state = state
    }
}

public struct OpenAcosmiStorageStatusPayload: Codable, Sendable, Equatable {
    public var totalBytes: Int64
    public var freeBytes: Int64
    public var usedBytes: Int64

    public init(totalBytes: Int64, freeBytes: Int64, usedBytes: Int64) {
        self.totalBytes = totalBytes
        self.freeBytes = freeBytes
        self.usedBytes = usedBytes
    }
}

public struct OpenAcosmiNetworkStatusPayload: Codable, Sendable, Equatable {
    public var status: OpenAcosmiNetworkPathStatus
    public var isExpensive: Bool
    public var isConstrained: Bool
    public var interfaces: [OpenAcosmiNetworkInterfaceType]

    public init(
        status: OpenAcosmiNetworkPathStatus,
        isExpensive: Bool,
        isConstrained: Bool,
        interfaces: [OpenAcosmiNetworkInterfaceType])
    {
        self.status = status
        self.isExpensive = isExpensive
        self.isConstrained = isConstrained
        self.interfaces = interfaces
    }
}

public struct OpenAcosmiDeviceStatusPayload: Codable, Sendable, Equatable {
    public var battery: OpenAcosmiBatteryStatusPayload
    public var thermal: OpenAcosmiThermalStatusPayload
    public var storage: OpenAcosmiStorageStatusPayload
    public var network: OpenAcosmiNetworkStatusPayload
    public var uptimeSeconds: Double

    public init(
        battery: OpenAcosmiBatteryStatusPayload,
        thermal: OpenAcosmiThermalStatusPayload,
        storage: OpenAcosmiStorageStatusPayload,
        network: OpenAcosmiNetworkStatusPayload,
        uptimeSeconds: Double)
    {
        self.battery = battery
        self.thermal = thermal
        self.storage = storage
        self.network = network
        self.uptimeSeconds = uptimeSeconds
    }
}

public struct OpenAcosmiDeviceInfoPayload: Codable, Sendable, Equatable {
    public var deviceName: String
    public var modelIdentifier: String
    public var systemName: String
    public var systemVersion: String
    public var appVersion: String
    public var appBuild: String
    public var locale: String

    public init(
        deviceName: String,
        modelIdentifier: String,
        systemName: String,
        systemVersion: String,
        appVersion: String,
        appBuild: String,
        locale: String)
    {
        self.deviceName = deviceName
        self.modelIdentifier = modelIdentifier
        self.systemName = systemName
        self.systemVersion = systemVersion
        self.appVersion = appVersion
        self.appBuild = appBuild
        self.locale = locale
    }
}
