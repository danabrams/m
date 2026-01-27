import Foundation

/// Represents an M server connection configuration.
/// API key is stored separately in Keychain for security.
struct MServer: Identifiable, Codable, Equatable, Hashable {
    let id: UUID
    var name: String
    var url: URL

    init(id: UUID = UUID(), name: String, url: URL) {
        self.id = id
        self.name = name
        self.url = url
    }
}
