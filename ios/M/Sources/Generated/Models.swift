// Generated from api/contract/openapi.yaml
// DO NOT EDIT - regenerate with `make generate`

import Foundation

// MARK: - Error

struct APIError: Codable, Equatable {
    let error: APIErrorDetail
}

struct APIErrorDetail: Codable, Equatable {
    let code: APIErrorCode
    let message: String
}

enum APIErrorCode: String, Codable, Equatable {
    case invalidInput = "invalid_input"
    case unauthorized
    case notFound = "not_found"
    case invalidState = "invalid_state"
    case conflict
}

// MARK: - Repo

struct Repo: Identifiable, Codable, Equatable, Hashable {
    let id: String
    let name: String
    let gitURL: String?

    enum CodingKeys: String, CodingKey {
        case id, name
        case gitURL = "git_url"
    }
}

struct RepoCreate: Codable, Equatable {
    let name: String
    let gitURL: String?

    enum CodingKeys: String, CodingKey {
        case name
        case gitURL = "git_url"
    }
}

// MARK: - Run

enum RunState: String, Codable, Equatable {
    case running
    case waitingApproval = "waiting_approval"
    case waitingInput = "waiting_input"
    case completed
    case cancelled
    case failed
}

struct Run: Identifiable, Codable, Equatable, Hashable {
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

    init(id: String, repoID: String, prompt: String, state: RunState, createdAt: Date, updatedAt: Date) {
        self.id = id
        self.repoID = repoID
        self.prompt = prompt
        self.state = state
        self.createdAt = createdAt
        self.updatedAt = updatedAt
    }

    init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        id = try container.decode(String.self, forKey: .id)
        repoID = try container.decode(String.self, forKey: .repoID)
        prompt = try container.decode(String.self, forKey: .prompt)
        state = try container.decode(RunState.self, forKey: .state)
        let createdAtTimestamp = try container.decode(Int64.self, forKey: .createdAt)
        createdAt = Date(timeIntervalSince1970: TimeInterval(createdAtTimestamp))
        let updatedAtTimestamp = try container.decode(Int64.self, forKey: .updatedAt)
        updatedAt = Date(timeIntervalSince1970: TimeInterval(updatedAtTimestamp))
    }

    func encode(to encoder: Encoder) throws {
        var container = encoder.container(keyedBy: CodingKeys.self)
        try container.encode(id, forKey: .id)
        try container.encode(repoID, forKey: .repoID)
        try container.encode(prompt, forKey: .prompt)
        try container.encode(state, forKey: .state)
        try container.encode(Int64(createdAt.timeIntervalSince1970), forKey: .createdAt)
        try container.encode(Int64(updatedAt.timeIntervalSince1970), forKey: .updatedAt)
    }
}

struct RunCreate: Codable, Equatable {
    let prompt: String
}

struct RunInput: Codable, Equatable {
    let text: String
}

// MARK: - Approval Types

/// Approval type enum for rich type handling.
enum ApprovalType: String, Codable, Equatable {
    case diff
    case command
    case generic
}

/// Structured approval payload.
struct ApprovalPayload: Codable, Equatable {
    let message: String?
    let command: String?
    let diff: String?
    let files: [DiffFile]?
}

/// A file in a diff approval.
struct DiffFile: Codable, Equatable, Hashable {
    let path: String
    let additions: Int
    let deletions: Int
    let content: String
}

// MARK: - Approval

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

    init(id: String, runID: String, type: ApprovalType, tool: String, requestID: String, payload: ApprovalPayload, createdAt: Date) {
        self.id = id
        self.runID = runID
        self.type = type
        self.tool = tool
        self.requestID = requestID
        self.payload = payload
        self.createdAt = createdAt
    }

    init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        id = try container.decode(String.self, forKey: .id)
        runID = try container.decode(String.self, forKey: .runID)
        type = try container.decode(ApprovalType.self, forKey: .type)
        tool = try container.decode(String.self, forKey: .tool)
        requestID = try container.decode(String.self, forKey: .requestID)
        payload = try container.decode(ApprovalPayload.self, forKey: .payload)
        let createdAtTimestamp = try container.decode(Int64.self, forKey: .createdAt)
        createdAt = Date(timeIntervalSince1970: TimeInterval(createdAtTimestamp))
    }

    func encode(to encoder: Encoder) throws {
        var container = encoder.container(keyedBy: CodingKeys.self)
        try container.encode(id, forKey: .id)
        try container.encode(runID, forKey: .runID)
        try container.encode(type, forKey: .type)
        try container.encode(tool, forKey: .tool)
        try container.encode(requestID, forKey: .requestID)
        try container.encode(payload, forKey: .payload)
        try container.encode(Int64(createdAt.timeIntervalSince1970), forKey: .createdAt)
    }
}

struct ApprovalResolve: Codable, Equatable {
    let approved: Bool
    let reason: String?
}

// MARK: - Device

struct DeviceRegister: Codable, Equatable {
    let token: String
    let platform: DevicePlatform
}

enum DevicePlatform: String, Codable, Equatable {
    case ios
}

// MARK: - Interaction Request

struct InteractionRequest: Codable, Equatable {
    let runID: String
    let type: InteractionType
    let tool: String?
    let requestID: String
    let payload: [String: AnyCodable]

    enum CodingKeys: String, CodingKey {
        case runID = "run_id"
        case type, tool
        case requestID = "request_id"
        case payload
    }
}

enum InteractionType: String, Codable, Equatable {
    case approval
    case input
}

struct InteractionResponse: Codable, Equatable {
    let approved: Bool?
    let reason: String?
    let text: String?
}
