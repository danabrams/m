import XCTest
@testable import M

final class MTests: XCTestCase {
    func testMServerInitialization() {
        let url = URL(string: "https://example.com")!
        let server = MServer(name: "Test Server", url: url)

        XCTAssertFalse(server.id.uuidString.isEmpty)
        XCTAssertEqual(server.name, "Test Server")
        XCTAssertEqual(server.url, url)
    }

    func testMServerWithCustomID() {
        let customID = UUID()
        let url = URL(string: "https://example.com")!
        let server = MServer(id: customID, name: "Test", url: url)

        XCTAssertEqual(server.id, customID)
    }

    func testConnectionStatusConnected() {
        let status = ConnectionStatus.connected
        XCTAssertTrue(status.isConnected)
        XCTAssertFalse(status.isError)
    }

    func testConnectionStatusError() {
        let status = ConnectionStatus.error("Connection failed")
        XCTAssertFalse(status.isConnected)
        XCTAssertTrue(status.isError)
    }

    func testConnectionStatusConnecting() {
        let status = ConnectionStatus.connecting
        XCTAssertFalse(status.isConnected)
        XCTAssertFalse(status.isError)
    }

    func testMErrorDescriptions() {
        let networkError = MError.networkError(underlying: "Connection failed")
        XCTAssertTrue(networkError.localizedDescription.contains("Network error"))

        let notFoundError = MError.notFound(message: "Repo not found")
        XCTAssertTrue(notFoundError.localizedDescription.contains("Not found"))

        let unauthorized = MError.unauthorized
        XCTAssertTrue(unauthorized.localizedDescription.contains("Authentication"))
    }

    func testMErrorEquatable() {
        let error1 = MError.unauthorized
        let error2 = MError.unauthorized
        XCTAssertEqual(error1, error2)

        let error3 = MError.invalidInput(message: "test")
        let error4 = MError.invalidInput(message: "test")
        XCTAssertEqual(error3, error4)
    }
}
