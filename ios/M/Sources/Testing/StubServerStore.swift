import Foundation

/// Extension to provide stub servers when in UI testing mode.
extension ServerStore {
    /// Configures the store for UI testing with the given scenario.
    func configureForTesting(scenario: TestScenario) {
        switch scenario {
        case .empty:
            // Clear all servers
            while !servers.isEmpty {
                deleteServers(at: IndexSet(integer: 0))
            }
        default:
            // Add stub servers if none exist
            if servers.isEmpty {
                for stubServer in StubData.servers {
                    // Use the existing addServer method
                    try? addServer(
                        name: stubServer.name,
                        url: stubServer.url,
                        apiKey: "stub-api-key-\(stubServer.id.uuidString)"
                    )
                }
            }
        }
    }
}
