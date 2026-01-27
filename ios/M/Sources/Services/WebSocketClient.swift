import Foundation

/// WebSocket client for streaming run events.
///
/// Provides automatic reconnection with exponential backoff and event replay
/// from the last received sequence number.
///
/// Usage:
/// ```swift
/// let client = WebSocketClient(server: server, apiKey: apiKey, runID: runID)
/// for try await message in client.messages {
///     switch message {
///     case .event(let event):
///         // Handle event
///     case .state(let state):
///         // Handle state change
///     case .ping:
///         // Handled automatically
///     }
/// }
/// ```
final class WebSocketClient {
    private let server: MServer
    private let apiKey: String
    private let runID: String
    private let session: URLSession

    private let decoder: JSONDecoder
    private let encoder: JSONEncoder

    private var webSocketTask: URLSessionWebSocketTask?
    private var lastSeq: Int = 0
    private var reconnectAttempt: Int = 0
    private var isConnected: Bool = false
    private var shouldReconnect: Bool = true

    private var messageContinuation: AsyncThrowingStream<ServerMessage, Error>.Continuation?

    /// Exponential backoff configuration.
    private let initialBackoff: TimeInterval = 1.0
    private let maxBackoff: TimeInterval = 30.0
    private let backoffMultiplier: Double = 2.0

    /// Creates a WebSocket client for the specified run.
    /// - Parameters:
    ///   - server: Server configuration
    ///   - apiKey: API key for authentication
    ///   - runID: The run ID to stream events from
    ///   - session: URLSession to use (defaults to shared)
    init(server: MServer, apiKey: String, runID: String, session: URLSession = .shared) {
        self.server = server
        self.apiKey = apiKey
        self.runID = runID
        self.session = session

        self.decoder = JSONDecoder()
        decoder.dateDecodingStrategy = .secondsSince1970

        self.encoder = JSONEncoder()
    }

    /// Stream of messages from the server.
    /// Automatically reconnects on disconnect and replays missed events.
    var messages: AsyncThrowingStream<ServerMessage, Error> {
        AsyncThrowingStream { continuation in
            self.messageContinuation = continuation

            continuation.onTermination = { [weak self] _ in
                self?.disconnect()
            }

            Task {
                await self.connect()
            }
        }
    }

    /// Disconnects from the WebSocket server.
    func disconnect() {
        shouldReconnect = false
        webSocketTask?.cancel(with: .goingAway, reason: nil)
        webSocketTask = nil
        isConnected = false
    }

    // MARK: - Private

    private func connect() async {
        guard let url = buildWebSocketURL() else {
            messageContinuation?.finish(throwing: MError.invalidInput(message: "Invalid WebSocket URL"))
            return
        }

        var request = URLRequest(url: url)
        request.setValue("Bearer \(apiKey)", forHTTPHeaderField: "Authorization")

        webSocketTask = session.webSocketTask(with: request)
        webSocketTask?.resume()

        isConnected = true
        reconnectAttempt = 0

        await receiveMessages()
    }

    private func buildWebSocketURL() -> URL? {
        var components = URLComponents()

        // Convert HTTP(S) to WS(S)
        if server.url.scheme == "https" {
            components.scheme = "wss"
        } else {
            components.scheme = "ws"
        }

        components.host = server.url.host
        components.port = server.url.port
        components.path = "/api/runs/\(runID)/events"

        // Include from_seq if we've received events before (for replay)
        if lastSeq > 0 {
            components.queryItems = [URLQueryItem(name: "from_seq", value: String(lastSeq))]
        }

        return components.url
    }

    private func receiveMessages() async {
        guard let task = webSocketTask else { return }

        while isConnected {
            do {
                let message = try await task.receive()
                try await handleMessage(message)
            } catch {
                isConnected = false

                if shouldReconnect {
                    await scheduleReconnect()
                } else {
                    messageContinuation?.finish(throwing: error)
                }
                return
            }
        }
    }

    private func handleMessage(_ message: URLSessionWebSocketTask.Message) async throws {
        let data: Data

        switch message {
        case .data(let d):
            data = d
        case .string(let s):
            guard let d = s.data(using: .utf8) else {
                throw MError.decodingError(underlying: "Invalid UTF-8 string")
            }
            data = d
        @unknown default:
            return
        }

        let serverMessage = try decoder.decode(ServerMessage.self, from: data)

        // Track sequence number for replay on reconnect
        if case .event(let event) = serverMessage {
            lastSeq = max(lastSeq, event.seq)
        }

        // Handle ping internally
        if case .ping = serverMessage {
            try await sendPong()
            return
        }

        messageContinuation?.yield(serverMessage)
    }

    private func sendPong() async throws {
        let message = ClientMessage.pong
        let data = try encoder.encode(message)
        guard let string = String(data: data, encoding: .utf8) else {
            throw MError.decodingError(underlying: "Failed to encode pong")
        }
        try await webSocketTask?.send(.string(string))
    }

    private func scheduleReconnect() async {
        reconnectAttempt += 1

        let backoff = min(
            initialBackoff * pow(backoffMultiplier, Double(reconnectAttempt - 1)),
            maxBackoff
        )

        try? await Task.sleep(nanoseconds: UInt64(backoff * 1_000_000_000))

        if shouldReconnect {
            await connect()
        }
    }
}
