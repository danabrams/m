import Foundation

/// Centralized accessibility identifiers for UI testing.
/// These must match the identifiers set in SwiftUI views.
enum AccessibilityID {
    // MARK: - Server List
    enum ServerList {
        static let addButton = "serverList.addButton"
        static let emptyStateAddButton = "serverList.emptyState.addButton"
        static let serverRow = "serverList.serverRow"
    }

    // MARK: - Add Server
    enum AddServer {
        static let nameField = "addServer.nameField"
        static let urlField = "addServer.urlField"
        static let apiKeyField = "addServer.apiKeyField"
        static let cancelButton = "addServer.cancelButton"
        static let saveButton = "addServer.saveButton"
    }

    // MARK: - Repo List
    enum RepoList {
        static let repoRow = "repoList.repoRow"
        static let emptyState = "repoList.emptyState"
    }

    // MARK: - Run List
    enum RunList {
        static let addButton = "runList.addButton"
        static let emptyStateAddButton = "runList.emptyState.addButton"
        static let runRow = "runList.runRow"
    }

    // MARK: - New Run
    enum NewRun {
        static let promptField = "newRun.promptField"
        static let cancelButton = "newRun.cancelButton"
        static let startButton = "newRun.startButton"
    }

    // MARK: - Run Detail
    enum RunDetail {
        static let statusLabel = "runDetail.statusLabel"
        static let cancelButton = "runDetail.cancelButton"
        static let eventFeed = "runDetail.eventFeed"
        static let pendingActionCard = "runDetail.pendingActionCard"
    }

    // MARK: - Approval
    enum Approval {
        static let approveButton = "approval.approveButton"
        static let rejectButton = "approval.rejectButton"
        static let diffView = "approval.diffView"
    }

    // MARK: - Input Prompt
    enum InputPrompt {
        static let questionLabel = "inputPrompt.questionLabel"
        static let responseField = "inputPrompt.responseField"
        static let sendButton = "inputPrompt.sendButton"
        static let cancelButton = "inputPrompt.cancelButton"
    }
}
