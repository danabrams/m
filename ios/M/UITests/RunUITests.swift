import XCTest

/// Tests for run creation and management.
final class RunUITests: XCTestCase {
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

    // MARK: - Create New Run

    /// Tests creating a new run with a prompt.
    func testCreateNewRun() throws {
        // Navigate to a repo with the run list
        try navigateToRunList()

        // Tap the add button to create a new run
        let addButton = app.navigationBars.buttons["Add"]
        XCTAssertTrue(addButton.waitForExistence(timeout: 2))
        addButton.tap()

        // Verify New Run sheet appears
        XCTAssertTrue(app.navigationBars["New Run"].waitForExistence(timeout: 2))

        // Verify Start is disabled without prompt
        let startButton = app.navigationBars["New Run"].buttons["Start"]
        XCTAssertFalse(startButton.isEnabled)

        // Enter a prompt
        let textEditor = app.textViews.firstMatch
        XCTAssertTrue(textEditor.waitForExistence(timeout: 2))
        textEditor.tap()
        textEditor.typeText("Fix the authentication bug in login.swift")

        // Start should now be enabled
        XCTAssertTrue(startButton.isEnabled)

        // Create the run
        startButton.tap()

        // Verify we're back on run list
        XCTAssertTrue(app.navigationBars["Test Repo"].waitForExistence(timeout: 5))
    }

    /// Tests that the Start button is disabled with empty prompt.
    func testNewRunValidation() throws {
        try navigateToRunList()

        app.navigationBars.buttons["Add"].tap()
        XCTAssertTrue(app.navigationBars["New Run"].waitForExistence(timeout: 2))

        // Start should be disabled
        let startButton = app.navigationBars["New Run"].buttons["Start"]
        XCTAssertFalse(startButton.isEnabled)

        // Enter whitespace only
        let textEditor = app.textViews.firstMatch
        textEditor.tap()
        textEditor.typeText("   ")

        // Start should still be disabled
        XCTAssertFalse(startButton.isEnabled)

        // Cancel
        app.navigationBars["New Run"].buttons["Cancel"].tap()
    }

    /// Tests empty state when no runs exist.
    func testRunListEmptyState() throws {
        try navigateToRunList()

        // Verify empty state message
        let emptyMessage = app.staticTexts["No runs yet"]
        XCTAssertTrue(emptyMessage.waitForExistence(timeout: 5))

        // Verify "Start a Run" button exists in empty state
        let startButton = app.buttons["Start a Run"]
        XCTAssertTrue(startButton.exists)
    }

    // MARK: - Cancel Run

    /// Tests cancelling an active run.
    func testCancelRun() throws {
        // Create a run first
        try navigateToRunList()
        try createTestRun()

        // Tap on the running run to view details
        let runCell = app.cells.firstMatch
        XCTAssertTrue(runCell.waitForExistence(timeout: 5))
        runCell.tap()

        // Verify we're on Run Detail
        XCTAssertTrue(app.navigationBars["Run"].waitForExistence(timeout: 2))

        // Find and tap Cancel button (only visible for running runs)
        let cancelButton = app.buttons["Cancel"]
        if cancelButton.waitForExistence(timeout: 2) {
            cancelButton.tap()

            // Verify status changes to Cancelled
            let cancelledStatus = app.staticTexts["Cancelled"]
            XCTAssertTrue(cancelledStatus.waitForExistence(timeout: 5))
        }
    }

    // MARK: - Helpers

    private func navigateToRunList() throws {
        // Add a test server if needed
        if !app.cells.staticTexts["Test Server"].exists {
            try addTestServer()
        }

        // Navigate to server
        app.cells.staticTexts["Test Server"].tap()
        XCTAssertTrue(app.navigationBars["Test Server"].waitForExistence(timeout: 5))

        // Navigate to repo (assuming mock API returns a test repo)
        let repoCell = app.cells.firstMatch
        if repoCell.waitForExistence(timeout: 5) {
            repoCell.tap()
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

    private func createTestRun() throws {
        app.navigationBars.buttons["Add"].tap()
        XCTAssertTrue(app.navigationBars["New Run"].waitForExistence(timeout: 2))

        let textEditor = app.textViews.firstMatch
        textEditor.tap()
        textEditor.typeText("Test task")

        app.navigationBars["New Run"].buttons["Start"].tap()
    }
}
