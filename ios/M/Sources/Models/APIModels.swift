import Foundation

// MARK: - Repo Models

struct Repo: Identifiable, Codable, Equatable {
    let id: String
    let name: String
    let gitURL: String?
    let createdAt: Date
    let updatedAt: Date

    enum CodingKeys: String, CodingKey {
        case id, name
        case gitURL = "git_url"
        case createdAt = "created_at"
        case updatedAt = "updated_at"
    }
}

struct CreateRepoRequest: Encodable {
    let name: String
    let gitURL: String?

    enum CodingKeys: String, CodingKey {
        case name
        case gitURL = "git_url"
    }
}

// MARK: - Run Models

enum RunState: String, Codable, Equatable {
    case running
    case waitingApproval = "waiting_approval"
    case waitingInput = "waiting_input"
    case completed
    case failed
    case cancelled
}

struct Run: Identifiable, Codable, Equatable {
    let id: String
    let repoID: String
    let prompt: String
    let state: RunState
    let createdAt: Date
    let updatedAt: Date

    enum CodingKeys: String, CodingKey {
        case id
        case repoID = "repo_id"
        case prompt, state
        case createdAt = "created_at"
        case updatedAt = "updated_at"
    }
}

struct CreateRunRequest: Encodable {
    let prompt: String
}

struct SendInputRequest: Encodable {
    let text: String
}

// MARK: - Approval Models

enum ApprovalType: String, Codable, Equatable {
    case diff
    case command
    case generic
}

struct Approval: Identifiable, Codable, Equatable {
    let id: String
    let runID: String
    let type: ApprovalType
    let tool: String
    let requestID: String
    let payload: ApprovalPayload
    let createdAt: Date

    enum CodingKeys: String, CodingKey {
        case id
        case runID = "run_id"
        case type, tool
        case requestID = "request_id"
        case payload
        case createdAt = "created_at"
    }
}

struct ApprovalPayload: Codable, Equatable {
    let message: String?
    let command: String?
    let diff: String?
    let files: [DiffFile]?
}

struct DiffFile: Codable, Equatable {
    let path: String
    let additions: Int
    let deletions: Int
    let content: String
}

struct ResolveApprovalRequest: Encodable {
    let approved: Bool
    let reason: String?
}

// MARK: - Device Registration

struct RegisterDeviceRequest: Encodable {
    let token: String
    let platform: String
}
