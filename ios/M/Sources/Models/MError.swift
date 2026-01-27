import Foundation

/// API error codes returned by the M server.
/// Maps HTTP status codes to typed errors.
enum MError: Error, Equatable {
    /// 400 - Invalid request parameters
    case invalidInput(message: String)
    /// 401 - Authentication failed
    case unauthorized
    /// 404 - Resource not found
    case notFound(message: String)
    /// 409 - Invalid state for operation
    case invalidState(message: String)
    /// 409 - Resource conflict
    case conflict(message: String)
    /// Network or connection error
    case networkError(underlying: String)
    /// Failed to decode server response
    case decodingError(underlying: String)
    /// Unknown server error
    case unknown(statusCode: Int, message: String)
}

extension MError: LocalizedError {
    var errorDescription: String? {
        switch self {
        case .invalidInput(let message):
            return "Invalid input: \(message)"
        case .unauthorized:
            return "Authentication required"
        case .notFound(let message):
            return "Not found: \(message)"
        case .invalidState(let message):
            return "Invalid state: \(message)"
        case .conflict(let message):
            return "Conflict: \(message)"
        case .networkError(let underlying):
            return "Network error: \(underlying)"
        case .decodingError(let underlying):
            return "Failed to parse response: \(underlying)"
        case .unknown(let statusCode, let message):
            return "Error \(statusCode): \(message)"
        }
    }
}

/// Server error response format
struct ErrorResponse: Decodable {
    let error: ErrorDetail

    struct ErrorDetail: Decodable {
        let code: String
        let message: String
    }
}
