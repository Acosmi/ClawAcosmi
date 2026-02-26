import Foundation

public enum OpenAcosmiRemindersCommand: String, Codable, Sendable {
    case list = "reminders.list"
    case add = "reminders.add"
}

public enum OpenAcosmiReminderStatusFilter: String, Codable, Sendable {
    case incomplete
    case completed
    case all
}

public struct OpenAcosmiRemindersListParams: Codable, Sendable, Equatable {
    public var status: OpenAcosmiReminderStatusFilter?
    public var limit: Int?

    public init(status: OpenAcosmiReminderStatusFilter? = nil, limit: Int? = nil) {
        self.status = status
        self.limit = limit
    }
}

public struct OpenAcosmiRemindersAddParams: Codable, Sendable, Equatable {
    public var title: String
    public var dueISO: String?
    public var notes: String?
    public var listId: String?
    public var listName: String?

    public init(
        title: String,
        dueISO: String? = nil,
        notes: String? = nil,
        listId: String? = nil,
        listName: String? = nil)
    {
        self.title = title
        self.dueISO = dueISO
        self.notes = notes
        self.listId = listId
        self.listName = listName
    }
}

public struct OpenAcosmiReminderPayload: Codable, Sendable, Equatable {
    public var identifier: String
    public var title: String
    public var dueISO: String?
    public var completed: Bool
    public var listName: String?

    public init(
        identifier: String,
        title: String,
        dueISO: String? = nil,
        completed: Bool,
        listName: String? = nil)
    {
        self.identifier = identifier
        self.title = title
        self.dueISO = dueISO
        self.completed = completed
        self.listName = listName
    }
}

public struct OpenAcosmiRemindersListPayload: Codable, Sendable, Equatable {
    public var reminders: [OpenAcosmiReminderPayload]

    public init(reminders: [OpenAcosmiReminderPayload]) {
        self.reminders = reminders
    }
}

public struct OpenAcosmiRemindersAddPayload: Codable, Sendable, Equatable {
    public var reminder: OpenAcosmiReminderPayload

    public init(reminder: OpenAcosmiReminderPayload) {
        self.reminder = reminder
    }
}
