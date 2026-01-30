import Foundation

/// Async/await API client for M server communication.
final class APIClient {
    private let server: MServer
    private let apiKey: String
    private let session: URLSession
    private let decoder: JSONDecoder
    private let encoder: JSONEncoder

    /// The server URL for WebSocket connections.
    var serverURL: URL { server.url }

    /// The API key for WebSocket authentication.
    var apiKeyForWebSocket: String { apiKey }

    /// Creates an API client for the given server.
    /// - Parameters:
    ///   - server: Server configuration
    ///   - apiKey: API key for authentication
    ///   - session: URLSession to use (defaults to shared)
    init(server: MServer, apiKey: String, session: URLSession = .shared) {
        self.server = server
        self.apiKey = apiKey
        self.session = session

        self.decoder = JSONDecoder()
        decoder.dateDecodingStrategy = .secondsSince1970

        self.encoder = JSONEncoder()
        encoder.dateEncodingStrategy = .secondsSince1970
    }

    // MARK: - Repos

    /// Lists all repos.
    func listRepos() async throws -> [Repo] {
        try await get("/api/repos")
    }

    /// Creates a new repo.
    func createRepo(name: String, gitURL: String? = nil) async throws -> Repo {
        let request = CreateRepoRequest(name: name, gitURL: gitURL)
        return try await post("/api/repos", body: request)
    }

    /// Gets a repo by ID.
    func getRepo(id: String) async throws -> Repo {
        try await get("/api/repos/\(id)")
    }

    /// Deletes a repo.
    func deleteRepo(id: String) async throws {
        try await delete("/api/repos/\(id)")
    }

    // MARK: - Runs

    /// Lists runs for a repo (newest first).
    func listRuns(repoID: String) async throws -> [Run] {
        try await get("/api/repos/\(repoID)/runs")
    }

    /// Creates a new run.
    func createRun(repoID: String, prompt: String) async throws -> Run {
        let request = CreateRunRequest(prompt: prompt)
        return try await post("/api/repos/\(repoID)/runs", body: request)
    }

    /// Gets a run by ID.
    func getRun(id: String) async throws -> Run {
        try await get("/api/runs/\(id)")
    }

    /// Cancels a run. Throws invalidState if already in terminal state.
    func cancelRun(id: String) async throws {
        try await post("/api/runs/\(id)/cancel", body: EmptyBody())
    }

    /// Sends input to a run. Throws invalidState if not in waiting_input state.
    func sendInput(runID: String, text: String) async throws {
        let request = SendInputRequest(text: text)
        try await post("/api/runs/\(runID)/input", body: request)
    }

    // MARK: - Approvals

    /// Lists all pending approvals across all runs.
    func listPendingApprovals() async throws -> [Approval] {
        try await get("/api/approvals/pending")
    }

    /// Gets approval details.
    func getApproval(id: String) async throws -> Approval {
        try await get("/api/approvals/\(id)")
    }

    /// Resolves an approval (approve or reject).
    func resolveApproval(id: String, approved: Bool, reason: String? = nil) async throws {
        let request = ResolveApprovalRequest(approved: approved, reason: reason)
        try await post("/api/approvals/\(id)/resolve", body: request)
    }

    // MARK: - Events

    /// Lists events for a run (oldest first by sequence number).
    func listEvents(runID: String) async throws -> [RunEvent] {
        try await get("/api/runs/\(runID)/events")
    }

    // MARK: - Device Registration

    /// Registers device for push notifications.
    func registerDevice(token: String) async throws {
        let request = RegisterDeviceRequest(token: token, platform: .ios)
        try await post("/api/devices", body: request)
    }

    /// Unregisters device from push notifications.
    func unregisterDevice(token: String) async throws {
        try await delete("/api/devices/\(token)")
    }

    // MARK: - Private HTTP Methods

    private func get<T: Decodable>(_ path: String) async throws -> T {
        let request = try makeRequest(path: path, method: "GET")
        return try await execute(request)
    }

    private func post<T: Decodable, B: Encodable>(_ path: String, body: B) async throws -> T {
        var request = try makeRequest(path: path, method: "POST")
        request.httpBody = try encoder.encode(body)
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        return try await execute(request)
    }

    private func post<B: Encodable>(_ path: String, body: B) async throws {
        var request = try makeRequest(path: path, method: "POST")
        request.httpBody = try encoder.encode(body)
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        try await executeVoid(request)
    }

    private func delete(_ path: String) async throws {
        let request = try makeRequest(path: path, method: "DELETE")
        try await executeVoid(request)
    }

    private func makeRequest(path: String, method: String) throws -> URLRequest {
        guard let url = URL(string: path, relativeTo: server.url) else {
            throw MError.invalidInput(message: "Invalid URL path: \(path)")
        }

        var request = URLRequest(url: url)
        request.httpMethod = method
        request.setValue("Bearer \(apiKey)", forHTTPHeaderField: "Authorization")
        return request
    }

    private func execute<T: Decodable>(_ request: URLRequest) async throws -> T {
        let (data, response) = try await performRequest(request)
        try validateResponse(response, data: data)
        do {
            return try decoder.decode(T.self, from: data)
        } catch {
            throw MError.decodingError(underlying: error.localizedDescription)
        }
    }

    private func executeVoid(_ request: URLRequest) async throws {
        let (data, response) = try await performRequest(request)
        try validateResponse(response, data: data)
    }

    private func performRequest(_ request: URLRequest) async throws -> (Data, URLResponse) {
        do {
            return try await session.data(for: request)
        } catch {
            throw MError.networkError(underlying: error.localizedDescription)
        }
    }

    private func validateResponse(_ response: URLResponse, data: Data) throws {
        guard let httpResponse = response as? HTTPURLResponse else {
            throw MError.networkError(underlying: "Invalid response type")
        }

        let statusCode = httpResponse.statusCode

        // Success range
        if 200..<300 ~= statusCode {
            return
        }

        // Parse error response
        let errorResponse = try? decoder.decode(ErrorResponse.self, from: data)
        let code = errorResponse?.error.code ?? "unknown"
        let message = errorResponse?.error.message ?? "Unknown error"

        switch statusCode {
        case 400:
            throw MError.invalidInput(message: message)
        case 401:
            throw MError.unauthorized
        case 404:
            throw MError.notFound(message: message)
        case 409:
            if code == "conflict" {
                throw MError.conflict(message: message)
            } else {
                throw MError.invalidState(message: message)
            }
        default:
            throw MError.unknown(statusCode: statusCode, message: message)
        }
    }
}

// Helper for POST requests with no response body
private struct EmptyBody: Encodable {}
