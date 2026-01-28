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

    // MARK: - Run Detail

    /// Tests that run detail shows status section.
    func testRunDetailShowsStatus() throws {
        try navigateToRunWithDetail()

        // Verify status section exists
        let statusSection = app.otherElements["run-status-section"]
        XCTAssertTrue(statusSection.waitForExistence(timeout: 5))

        // Verify status text is visible
        let statusText = app.staticTexts["run-status-text"]
        XCTAssertTrue(statusText.exists)
    }

    /// Tests that pending approval card appears when run is waiting for approval.
    func testRunDetailPendingApprovalCard() throws {
        try navigateToRunWithPendingApproval()

        // Verify pending approval card exists
        let approvalCard = app.buttons["pending-approval-card"]
        XCTAssertTrue(approvalCard.waitForExistence(timeout: 5))

        // Verify card text
        XCTAssertTrue(app.staticTexts["Needs approval"].exists)
    }

    /// Tests that pending input card appears when run is waiting for input.
    func testRunDetailPendingInputCard() throws {
        try navigateToRunWithPendingInput()

        // Verify pending input card exists
        let inputCard = app.buttons["pending-input-card"]
        XCTAssertTrue(inputCard.waitForExistence(timeout: 5))

        // Verify card text
        XCTAssertTrue(app.staticTexts["Waiting for you"].exists)
    }

    /// Tests tapping pending approval card opens approval detail.
    func testPendingApprovalCardOpensDetail() throws {
        try navigateToRunWithPendingApproval()

        // Tap the approval card
        let approvalCard = app.buttons["pending-approval-card"]
        XCTAssertTrue(approvalCard.waitForExistence(timeout: 5))
        approvalCard.tap()

        // Verify approval detail sheet appears
        XCTAssertTrue(app.buttons["Approve"].waitForExistence(timeout: 2))
        XCTAssertTrue(app.buttons["Reject"].exists)
    }

    /// Tests tapping pending input card opens input prompt.
    func testPendingInputCardOpensPrompt() throws {
        try navigateToRunWithPendingInput()

        // Tap the input card
        let inputCard = app.buttons["pending-input-card"]
        XCTAssertTrue(inputCard.waitForExistence(timeout: 5))
        inputCard.tap()

        // Verify input prompt sheet appears
        XCTAssertTrue(app.navigationBars["Agent Question"].waitForExistence(timeout: 2))
        XCTAssertTrue(app.buttons["Send"].exists)
    }

    // MARK: - Cancel Run

    /// Tests cancelling an active run.
    func testCancelRun() throws {
        try navigateToRunWithDetail()

        // Find and tap Cancel button (only visible for running runs)
        let cancelButton = app.buttons["cancel-run"]
        if cancelButton.waitForExistence(timeout: 2) {
            cancelButton.tap()

            // Verify status changes to Cancelled
            let cancelledStatus = app.staticTexts["Cancelled"]
            XCTAssertTrue(cancelledStatus.waitForExistence(timeout: 5))
        }
    }

    // MARK: - Retry

    /// Tests retry button for completed runs.
    func testRetryCompletedRun() throws {
        try navigateToCompletedRun()

        // Verify Retry button exists
        let retryButton = app.buttons["Retry"]
        XCTAssertTrue(retryButton.waitForExistence(timeout: 5))
    }

    /// Tests edit & retry button for completed runs.
    func testEditAndRetryCompletedRun() throws {
        try navigateToCompletedRun()

        // Verify Edit & Retry button exists
        let editRetryButton = app.buttons["Edit & Retry"]
        XCTAssertTrue(editRetryButton.waitForExistence(timeout: 5))

        // Tap Edit & Retry
        editRetryButton.tap()

        // Verify Edit & Retry sheet appears
        XCTAssertTrue(app.navigationBars["Edit & Retry"].waitForExistence(timeout: 2))

        // Verify prompt is pre-filled
        let textEditor = app.textViews.firstMatch
        XCTAssertTrue(textEditor.exists)
        // The text editor should contain the original prompt
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

    private func navigateToRunWithDetail() throws {
        try navigateToRunList()
        try createTestRun()

        // Tap on the run to view details
        let runCell = app.cells.firstMatch
        XCTAssertTrue(runCell.waitForExistence(timeout: 5))
        runCell.tap()

        // Wait for detail view
        let statusSection = app.otherElements["run-status-section"]
        XCTAssertTrue(statusSection.waitForExistence(timeout: 5))
    }

    private func navigateToRunWithPendingApproval() throws {
        // This would require mock API to return a run in waiting_approval state
        // For now, use launch argument to configure mock state
        app.terminate()
        app.launchArguments = ["--uitesting", "--mock-api", "--mock-pending-approval"]
        app.launch()

        try navigateToRunList()

        // Navigate to run with pending approval
        let runCell = app.cells.firstMatch
        XCTAssertTrue(runCell.waitForExistence(timeout: 5))
        runCell.tap()
    }

    private func navigateToRunWithPendingInput() throws {
        // This would require mock API to return a run in waiting_input state
        app.terminate()
        app.launchArguments = ["--uitesting", "--mock-api", "--mock-pending-input"]
        app.launch()

        try navigateToRunList()

        // Navigate to run with pending input
        let runCell = app.cells.firstMatch
        XCTAssertTrue(runCell.waitForExistence(timeout: 5))
        runCell.tap()
    }

    private func navigateToCompletedRun() throws {
        // This would require mock API to return a completed run
        app.terminate()
        app.launchArguments = ["--uitesting", "--mock-api", "--mock-completed-run"]
        app.launch()

        try navigateToRunList()

        // Navigate to completed run
        let runCell = app.cells.firstMatch
        XCTAssertTrue(runCell.waitForExistence(timeout: 5))
        runCell.tap()
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
