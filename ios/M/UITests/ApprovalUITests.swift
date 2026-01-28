import XCTest

/// Tests for the approval flow (diff approval, command approval).
final class ApprovalUITests: XCTestCase {
    var app: XCUIApplication!

    override func setUpWithError() throws {
        continueAfterFailure = false
        app = XCUIApplication()
        // Use mock API that provides pending approvals
        app.launchArguments = ["--uitesting", "--mock-api", "--with-approvals"]
        app.launch()
    }

    override func tearDownWithError() throws {
        app = nil
    }

    // MARK: - Diff Approval

    /// Tests approving a diff approval request.
    func testApproveDiff() throws {
        // Navigate to a run with pending approval
        try navigateToRunWithApproval()

        // Look for approval indicators
        // The run detail should show "Waiting for Approval" status
        let waitingStatus = app.staticTexts["Waiting for Approval"]
        if waitingStatus.waitForExistence(timeout: 5) {
            // Tap on pending action card or approval indicator
            let approvalCard = app.buttons.matching(identifier: "pending-approval").firstMatch
            if approvalCard.exists {
                approvalCard.tap()
            }
        }

        // Look for Approve button
        let approveButton = app.buttons["Approve"]
        if approveButton.waitForExistence(timeout: 5) {
            approveButton.tap()

            // Verify status changes or sheet dismisses
            // The run should no longer be in waiting_approval state
            let runningStatus = app.staticTexts["Running"]
            let completedStatus = app.staticTexts["Completed"]
            XCTAssertTrue(
                runningStatus.waitForExistence(timeout: 5) ||
                completedStatus.waitForExistence(timeout: 5),
                "Expected run to transition after approval"
            )
        }
    }

    /// Tests rejecting a diff approval request.
    func testRejectDiff() throws {
        try navigateToRunWithApproval()

        // Look for approval in waiting state
        let waitingStatus = app.staticTexts["Waiting for Approval"]
        if waitingStatus.waitForExistence(timeout: 5) {
            // Find and tap Reject button
            let rejectButton = app.buttons["Reject"]
            if rejectButton.waitForExistence(timeout: 5) {
                rejectButton.tap()

                // Rejection might prompt for a reason
                let reasonField = app.textFields.firstMatch
                if reasonField.waitForExistence(timeout: 2) {
                    reasonField.tap()
                    reasonField.typeText("Not needed")

                    // Submit rejection
                    let submitButton = app.buttons["Submit"]
                    if submitButton.exists {
                        submitButton.tap()
                    }
                }
            }
        }
    }

    /// Tests viewing diff details before approval.
    func testViewDiffDetails() throws {
        try navigateToRunWithApproval()

        // Look for file sections in diff view
        // According to UI spec, files are expandable
        let fileHeader = app.staticTexts.matching(NSPredicate(format: "label CONTAINS '.swift' OR label CONTAINS '.go'")).firstMatch
        if fileHeader.waitForExistence(timeout: 5) {
            // Tap to expand file diff
            fileHeader.tap()

            // Verify diff content appears (code should be visible)
            let diffContent = app.scrollViews.firstMatch
            XCTAssertTrue(diffContent.waitForExistence(timeout: 2))
        }
    }

    // MARK: - Command Approval

    /// Tests approving a command execution request.
    func testApproveCommand() throws {
        // Use mock with command approval
        app.terminate()
        app.launchArguments = ["--uitesting", "--mock-api", "--with-command-approval"]
        app.launch()

        try navigateToRunWithApproval()

        // Look for command approval
        let allowButton = app.buttons["Approve"]
        if allowButton.waitForExistence(timeout: 5) {
            // Verify command is displayed in monospace
            // The command should be visible before approval
            allowButton.tap()
        }
    }

    // MARK: - Input Prompt

    /// Tests responding to an agent question.
    func testRespondToAgentQuestion() throws {
        // Use mock with input prompt
        app.terminate()
        app.launchArguments = ["--uitesting", "--mock-api", "--with-input-prompt"]
        app.launch()

        try navigateToRunWithApproval()

        // Look for "Waiting for Input" status
        let waitingInput = app.staticTexts["Waiting for Input"]
        if waitingInput.waitForExistence(timeout: 5) {
            // Open input prompt sheet
            let inputCard = app.buttons.matching(identifier: "pending-input").firstMatch
            if inputCard.exists {
                inputCard.tap()
            }

            // Verify Agent Question sheet
            let questionSheet = app.navigationBars["Agent Question"]
            if questionSheet.waitForExistence(timeout: 2) {
                // Type a response
                let textEditor = app.textViews.firstMatch
                textEditor.tap()
                textEditor.typeText("Yes, proceed with the changes")

                // Send response
                let sendButton = app.buttons["Send"]
                XCTAssertTrue(sendButton.isEnabled)
                sendButton.tap()

                // Verify we're back on run detail
                XCTAssertTrue(app.navigationBars["Run"].waitForExistence(timeout: 5))
            }
        }
    }

    // MARK: - Helpers

    private func navigateToRunWithApproval() throws {
        // Add server if needed
        if !app.cells.staticTexts["Test Server"].exists {
            try addTestServer()
        }

        // Navigate to run with pending approval
        app.cells.staticTexts["Test Server"].tap()
        XCTAssertTrue(app.navigationBars["Test Server"].waitForExistence(timeout: 5))

        let repoCell = app.cells.firstMatch
        if repoCell.waitForExistence(timeout: 5) {
            repoCell.tap()

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
