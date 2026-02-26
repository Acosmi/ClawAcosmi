import CoreLocation
import Foundation
import OpenAcosmiKit
import UIKit

protocol CameraServicing: Sendable {
    func listDevices() async -> [CameraController.CameraDeviceInfo]
    func snap(params: OpenAcosmiCameraSnapParams) async throws -> (format: String, base64: String, width: Int, height: Int)
    func clip(params: OpenAcosmiCameraClipParams) async throws -> (format: String, base64: String, durationMs: Int, hasAudio: Bool)
}

protocol ScreenRecordingServicing: Sendable {
    func record(
        screenIndex: Int?,
        durationMs: Int?,
        fps: Double?,
        includeAudio: Bool?,
        outPath: String?) async throws -> String
}

@MainActor
protocol LocationServicing: Sendable {
    func authorizationStatus() -> CLAuthorizationStatus
    func accuracyAuthorization() -> CLAccuracyAuthorization
    func ensureAuthorization(mode: OpenAcosmiLocationMode) async -> CLAuthorizationStatus
    func currentLocation(
        params: OpenAcosmiLocationGetParams,
        desiredAccuracy: OpenAcosmiLocationAccuracy,
        maxAgeMs: Int?,
        timeoutMs: Int?) async throws -> CLLocation
}

protocol DeviceStatusServicing: Sendable {
    func status() async throws -> OpenAcosmiDeviceStatusPayload
    func info() -> OpenAcosmiDeviceInfoPayload
}

protocol PhotosServicing: Sendable {
    func latest(params: OpenAcosmiPhotosLatestParams) async throws -> OpenAcosmiPhotosLatestPayload
}

protocol ContactsServicing: Sendable {
    func search(params: OpenAcosmiContactsSearchParams) async throws -> OpenAcosmiContactsSearchPayload
    func add(params: OpenAcosmiContactsAddParams) async throws -> OpenAcosmiContactsAddPayload
}

protocol CalendarServicing: Sendable {
    func events(params: OpenAcosmiCalendarEventsParams) async throws -> OpenAcosmiCalendarEventsPayload
    func add(params: OpenAcosmiCalendarAddParams) async throws -> OpenAcosmiCalendarAddPayload
}

protocol RemindersServicing: Sendable {
    func list(params: OpenAcosmiRemindersListParams) async throws -> OpenAcosmiRemindersListPayload
    func add(params: OpenAcosmiRemindersAddParams) async throws -> OpenAcosmiRemindersAddPayload
}

protocol MotionServicing: Sendable {
    func activities(params: OpenAcosmiMotionActivityParams) async throws -> OpenAcosmiMotionActivityPayload
    func pedometer(params: OpenAcosmiPedometerParams) async throws -> OpenAcosmiPedometerPayload
}

extension CameraController: CameraServicing {}
extension ScreenRecordService: ScreenRecordingServicing {}
extension LocationService: LocationServicing {}
