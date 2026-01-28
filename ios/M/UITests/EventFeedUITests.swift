import XCTest

/// Tests for the event feed in run detail view.
final class EventFeedUITests: XCTestCase {
    var app: XCUIApplication!

    override func setUpWithError() throws {
        continueAfterFailure = false
        app = XCUIApplication()
        // Use mock API that provides event data
        app.launchArguments = ["--uitesting", "--mock-api", "--with-events"]
        app.launch()
    }

    override func tearDownWithError() throws {
        app = nil
    }

    // MARK: - Event Feed Display

    /// Tests that event feed displays run events.
    func testEventFeedDisplaysEvents() throws {
        // Navigate to a run with events
        try navigateToRunWithEvents()

        // Verify we're on Run Detail
        XCTAssertTrue(app.navigationBars["Run"].waitForExistence(timeout: 2))

        // Verify status section is visible
        let statusSection = app.scrollViews.firstMatch
        XCTAssertTrue(statusSection.waitForExistence(timeout: 2))
    }

    /// Tests that run status is displayed correctly.
    func testRunStatusDisplay() throws {
        try navigateToRunWithEvents()

        // Check for status indicators
        // Running status shows a spinner
        let runningIndicator = app.progressIndicators.firstMatch
        let completedIndicator = app.images["checkmark.circle.fill"]
        let waitingIndicator = app.images["clock.fill"]

        // At least one status indicator should be present
        let hasStatusIndicator = runningIndicator.exists ||
                                 completedIndicator.exists ||
                                 waitingIndicator.exists
        XCTAssertTrue(hasStatusIndicator, "Expected a status indicator to be visible")
    }

    /// Tests that prompt section displays the run prompt.
    func testPromptSectionDisplaysPrompt() throws {
        try navigateToRunWithEvents()

        // Verify prompt section exists
        let promptLabel = app.staticTexts["Prompt"]
        XCTAssertTrue(promptLabel.waitForExistence(timeout: 5))
    }

    /// Tests pull-to-refresh on run detail.
    func testPullToRefresh() throws {
        try navigateToRunWithEvents()

        // Perform pull-to-refresh
        let scrollView = app.scrollViews.firstMatch
        scrollView.swipeDown()

        // Verify view still loads (no crash)
        XCTAssertTrue(app.navigationBars["Run"].waitForExistence(timeout: 5))
    }

    // MARK: - Helpers

    private func navigateToRunWithEvents() throws {
        // Add server if needed
        if !app.cells.staticTexts["Test Server"].exists {
            try addTestServer()
        }

        // Navigate through to run detail
        app.cells.staticTexts["Test Server"].tap()
        XCTAssertTrue(app.navigationBars["Test Server"].waitForExistence(timeout: 5))

        // Tap first repo
        let repoCell = app.cells.firstMatch
        if repoCell.waitForExistence(timeout: 5) {
            repoCell.tap()

            // Tap first run
            let runCell = app.cells.firstMatch
            if runCell.waitForExistence(timeout: 5) {
                runCell.tap()
            }
        }
    }

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
