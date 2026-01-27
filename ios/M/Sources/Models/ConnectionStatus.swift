import Foundation

/// Connection status for an M server.
enum ConnectionStatus: Equatable {
    case unknown
    case connecting
    case connected
    case error(String)

    var isConnected: Bool {
        if case .connected = self { return true }
        return false
    }

    var isError: Bool {
        if case .error = self { return true }
        return false
    }
}
