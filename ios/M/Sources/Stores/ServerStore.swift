import Foundation
import Combine

/// Manages persistence and state for M servers.
/// Server metadata stored in UserDefaults; API keys in Keychain.
@MainActor
final class ServerStore: ObservableObject {
    static let shared = ServerStore()

    @Published private(set) var servers: [MServer] = []

    private let userDefaults: UserDefaults
    private let keychain: KeychainService
    private let storageKey = "com.m.client.servers"

    init(userDefaults: UserDefaults = .standard, keychain: KeychainService = .shared) {
        self.userDefaults = userDefaults
        self.keychain = keychain
        load()
    }

    // MARK: - CRUD Operations

    /// Adds a new server with its API key.
    func addServer(name: String, url: URL, apiKey: String) throws {
        let server = MServer(name: name, url: url)
        try keychain.setAPIKey(apiKey, for: server.id)
        servers.append(server)
        save()
    }

    /// Removes a server and its API key.
    func deleteServer(_ server: MServer) {
        try? keychain.deleteAPIKey(for: server.id)
        servers.removeAll { $0.id == server.id }
        save()
    }

    /// Removes a server at the given index set.
    func deleteServers(at offsets: IndexSet) {
        for index in offsets {
            let server = servers[index]
            try? keychain.deleteAPIKey(for: server.id)
        }
        servers.remove(atOffsets: offsets)
        save()
    }

    /// Retrieves the API key for a server.
    func getAPIKey(for server: MServer) -> String? {
        try? keychain.getAPIKey(for: server.id)
    }

    // MARK: - Persistence

    private func load() {
        guard let data = userDefaults.data(forKey: storageKey),
              let decoded = try? JSONDecoder().decode([MServer].self, from: data) else {
            servers = []
            return
        }
        servers = decoded
    }

    private func save() {
        guard let data = try? JSONEncoder().encode(servers) else { return }
        userDefaults.set(data, forKey: storageKey)
    }
}
