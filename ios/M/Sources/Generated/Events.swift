// Generated from api/contract/openapi.yaml
// DO NOT EDIT - regenerate with `make generate`

import Foundation

// MARK: - WebSocket Event

/// Message received from WebSocket server.
/// Note: The detailed ServerMessage enum in EventModels.swift provides
/// richer type information for event handling.
struct WebSocketEvent: Codable, Equatable {
    let type: WebSocketEventType
    let event: WebSocketRunEvent?
    let state: RunState?

    init(type: WebSocketEventType, event: WebSocketRunEvent? = nil, state: RunState? = nil) {
        self.type = type
        self.event = event
        self.state = state
    }
}

enum WebSocketEventType: String, Codable, Equatable {
    case event
    case state
    case ping
}

// MARK: - WebSocket Run Event

/// A simplified run event structure matching the OpenAPI spec.
/// For rich event handling, use RunEvent from EventModels.swift.
struct WebSocketRunEvent: Identifiable, Codable, Equatable {
    let id: String
    let seq: Int
    let type: String
    let data: [String: AnyCodable]
    let createdAt: Int

    enum CodingKeys: String, CodingKey {
        case id, seq, type, data
        case createdAt = "created_at"
    }
}
