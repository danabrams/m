import XCTest

/// Tests for navigating through the repo list.
final class RepoNavigationUITests: XCTestCase {
    var app: XCUIApplication!

    override func setUpWithError() throws {
        continueAfterFailure = false
        app = XCUIApplication()
        app.launchArguments = ["--uitesting", "--mock-api"]
        app.launch()
    }

    override func tearDownWithError() throws {
        app = nil
    }

    // MARK: - Navigation to Repo List

    /// Tests navigating from server list to repo list.
    func testNavigateToRepoList() throws {
        // Given a configured server exists
        try addTestServer()

        // When tapping on the server
        let serverCell = app.cells.staticTexts["Test Server"]
        XCTAssertTrue(serverCell.waitForExistence(timeout: 2))
        serverCell.tap()

        // Then repo list is shown with server name as title
        XCTAssertTrue(app.navigationBars["Test Server"].waitForExistence(timeout: 5))
    }

    /// Tests empty state when no repos exist.
    func testRepoListEmptyState() throws {
        // Given a server with no repos
        try addTestServer()
        app.cells.staticTexts["Test Server"].tap()

        // Then empty state message is shown
        let emptyMessage = app.staticTexts["No repos in this server"]
        XCTAssertTrue(emptyMessage.waitForExistence(timeout: 5))
    }

    /// Tests navigating back from repo list to server list.
    func testNavigateBackToServerList() throws {
        try addTestServer()
        app.cells.staticTexts["Test Server"].tap()
        XCTAssertTrue(app.navigationBars["Test Server"].waitForExistence(timeout: 5))

        // Tap back button
        app.navigationBars.buttons.element(boundBy: 0).tap()

        // Verify we're back on Servers
        XCTAssertTrue(app.navigationBars["Servers"].waitForExistence(timeout: 2))
    }

    // MARK: - Helpers

    private func addTestServer() throws {
        app.navigationBars["Servers"].buttons["Add"].tap()
        XCTAssertTrue(app.navigationBars["Add Server"].waitForExistence(timeout: 2))

        app.textFields["Name"].tap()
        app.textFields["Name"].typeText("Test Server")

        app.textFields["URL"].tap()
        app.textFields["URL"].typeText("https://m.example.com")

        app.secureTextFields["API Key"].tap()
        app.secureTextFields["API Key"].typeText("test-key")

        app.navigationBars["Add Server"].buttons["Save"].tap()
        XCTAssertTrue(app.navigationBars["Servers"].waitForExistence(timeout: 2))
    }
}
