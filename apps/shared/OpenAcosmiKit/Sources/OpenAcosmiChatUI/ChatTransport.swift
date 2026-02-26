import Foundation

public enum OpenAcosmiChatTransportEvent: Sendable {
    case health(ok: Bool)
    case tick
    case chat(OpenAcosmiChatEventPayload)
    case agent(OpenAcosmiAgentEventPayload)
    case seqGap
}

public protocol OpenAcosmiChatTransport: Sendable {
    func requestHistory(sessionKey: String) async throws -> OpenAcosmiChatHistoryPayload
    func sendMessage(
        sessionKey: String,
        message: String,
        thinking: String,
        idempotencyKey: String,
        attachments: [OpenAcosmiChatAttachmentPayload]) async throws -> OpenAcosmiChatSendResponse

    func abortRun(sessionKey: String, runId: String) async throws
    func listSessions(limit: Int?) async throws -> OpenAcosmiChatSessionsListResponse

    func requestHealth(timeoutMs: Int) async throws -> Bool
    func events() -> AsyncStream<OpenAcosmiChatTransportEvent>

    func setActiveSessionKey(_ sessionKey: String) async throws
}

extension OpenAcosmiChatTransport {
    public func setActiveSessionKey(_: String) async throws {}

    public func abortRun(sessionKey _: String, runId _: String) async throws {
        throw NSError(
            domain: "OpenAcosmiChatTransport",
            code: 0,
            userInfo: [NSLocalizedDescriptionKey: "chat.abort not supported by this transport"])
    }

    public func listSessions(limit _: Int?) async throws -> OpenAcosmiChatSessionsListResponse {
        throw NSError(
            domain: "OpenAcosmiChatTransport",
            code: 0,
            userInfo: [NSLocalizedDescriptionKey: "sessions.list not supported by this transport"])
    }
}
