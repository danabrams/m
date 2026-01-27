import Foundation

// MARK: - Run Event

/// An event from a run's event stream.
struct RunEvent: Identifiable, Codable, Equatable {
    let id: String
    let runID: String
    let seq: Int
    let type: EventType
    let data: EventData
    let createdAt: Date

    enum CodingKeys: String, CodingKey {
        case id
        case runID = "run_id"
        case seq, type, data
        case createdAt = "created_at"
    }
}

// MARK: - Event Types

enum EventType: String, Codable, Equatable {
    case runStarted = "run_started"
    case stdout
    case stderr
    case toolCallStart = "tool_call_start"
    case toolCallEnd = "tool_call_end"
    case approvalRequested = "approval_requested"
    case approvalResolved = "approval_resolved"
    case inputRequested = "input_requested"
    case inputReceived = "input_received"
    case runCompleted = "run_completed"
    case runFailed = "run_failed"
    case runCancelled = "run_cancelled"
}

// MARK: - Event Data

/// Union type for event data payloads.
/// Each event type has its own specific data structure.
struct EventData: Codable, Equatable {
    // stdout / stderr
    let text: String?

    // tool_call_start / tool_call_end
    let callID: String?
    let tool: String?
    let input: [String: AnyCodable]?
    let success: Bool?
    let durationMs: Int?

    // approval_requested / approval_resolved
    let approvalID: String?
    let approvalType: String?
    let approved: Bool?
    let reason: String?

    // input_requested
    let question: String?

    // run_failed
    let error: String?

    enum CodingKeys: String, CodingKey {
        case text
        case callID = "call_id"
        case tool, input, success
        case durationMs = "duration_ms"
        case approvalID = "approval_id"
        case approvalType = "type"
        case approved, reason, question, error
    }

    init(
        text: String? = nil,
        callID: String? = nil,
        tool: String? = nil,
        input: [String: AnyCodable]? = nil,
        success: Bool? = nil,
        durationMs: Int? = nil,
        approvalID: String? = nil,
        approvalType: String? = nil,
        approved: Bool? = nil,
        reason: String? = nil,
        question: String? = nil,
        error: String? = nil
    ) {
        self.text = text
        self.callID = callID
        self.tool = tool
        self.input = input
        self.success = success
        self.durationMs = durationMs
        self.approvalID = approvalID
        self.approvalType = approvalType
        self.approved = approved
        self.reason = reason
        self.question = question
        self.error = error
    }
}

// MARK: - WebSocket Messages

/// Message received from WebSocket server.
enum ServerMessage: Decodable {
    case event(RunEvent)
    case state(RunState)
    case ping

    private enum CodingKeys: String, CodingKey {
        case type, event, state
    }

    init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        let type = try container.decode(String.self, forKey: .type)

        switch type {
        case "event":
            let event = try container.decode(RunEvent.self, forKey: .event)
            self = .event(event)
        case "state":
            let state = try container.decode(RunState.self, forKey: .state)
            self = .state(state)
        case "ping":
            self = .ping
        default:
            throw DecodingError.dataCorrupted(
                DecodingError.Context(
                    codingPath: container.codingPath,
                    debugDescription: "Unknown message type: \(type)"
                )
            )
        }
    }
}

/// Message sent to WebSocket server.
enum ClientMessage: Encodable {
    case pong

    private enum CodingKeys: String, CodingKey {
        case type
    }

    func encode(to encoder: Encoder) throws {
        var container = encoder.container(keyedBy: CodingKeys.self)
        switch self {
        case .pong:
            try container.encode("pong", forKey: .type)
        }
    }
}

// MARK: - AnyCodable

/// Type-erased Codable for arbitrary JSON values.
struct AnyCodable: Codable, Equatable {
    let value: Any

    init(_ value: Any) {
        self.value = value
    }

    init(from decoder: Decoder) throws {
        let container = try decoder.singleValueContainer()

        if container.decodeNil() {
            value = NSNull()
        } else if let bool = try? container.decode(Bool.self) {
            value = bool
        } else if let int = try? container.decode(Int.self) {
            value = int
        } else if let double = try? container.decode(Double.self) {
            value = double
        } else if let string = try? container.decode(String.self) {
            value = string
        } else if let array = try? container.decode([AnyCodable].self) {
            value = array.map { $0.value }
        } else if let dict = try? container.decode([String: AnyCodable].self) {
            value = dict.mapValues { $0.value }
        } else {
            throw DecodingError.dataCorrupted(
                DecodingError.Context(
                    codingPath: container.codingPath,
                    debugDescription: "Unable to decode value"
                )
            )
        }
    }

    func encode(to encoder: Encoder) throws {
        var container = encoder.singleValueContainer()

        switch value {
        case is NSNull:
            try container.encodeNil()
        case let bool as Bool:
            try container.encode(bool)
        case let int as Int:
            try container.encode(int)
        case let double as Double:
            try container.encode(double)
        case let string as String:
            try container.encode(string)
        case let array as [Any]:
            try container.encode(array.map { AnyCodable($0) })
        case let dict as [String: Any]:
            try container.encode(dict.mapValues { AnyCodable($0) })
        default:
            throw EncodingError.invalidValue(
                value,
                EncodingError.Context(
                    codingPath: container.codingPath,
                    debugDescription: "Unable to encode value"
                )
            )
        }
    }

    static func == (lhs: AnyCodable, rhs: AnyCodable) -> Bool {
        switch (lhs.value, rhs.value) {
        case (is NSNull, is NSNull):
            return true
        case (let l as Bool, let r as Bool):
            return l == r
        case (let l as Int, let r as Int):
            return l == r
        case (let l as Double, let r as Double):
            return l == r
        case (let l as String, let r as String):
            return l == r
        default:
            return false
        }
    }
}
