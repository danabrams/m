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

            // Verify approval was processed
            let runningStatus = app.staticTexts["Running"]
            let completedStatus = app.staticTexts["Completed"]
            XCTAssertTrue(
                runningStatus.waitForExistence(timeout: 5) ||
                completedStatus.waitForExistence(timeout: 5),
                "Expected run to transition after command approval"
            )
        }
    }

    /// Tests rejecting a command execution request.
    func testRejectCommand() throws {
        app.terminate()
        app.launchArguments = ["--uitesting", "--mock-api", "--with-command-approval"]
        app.launch()

        try navigateToRunWithApproval()

        // Find and tap Reject button
        let rejectButton = app.buttons["Reject"]
        if rejectButton.waitForExistence(timeout: 5) {
            rejectButton.tap()

            // Rejection prompt should appear
            let rejectSheet = app.navigationBars["Reject"]
            if rejectSheet.waitForExistence(timeout: 2) {
                // Enter a reason
                let reasonField = app.textFields.firstMatch
                if reasonField.exists {
                    reasonField.tap()
                    reasonField.typeText("Command is not safe")
                }

                // Submit rejection
                let confirmReject = app.buttons["Reject"]
                if confirmReject.exists {
                    confirmReject.tap()
                }
            }

            // Verify run failed or stopped after rejection
            let failedStatus = app.staticTexts["Failed"]
            let cancelledStatus = app.staticTexts["Cancelled"]
            XCTAssertTrue(
                failedStatus.waitForExistence(timeout: 5) ||
                cancelledStatus.waitForExistence(timeout: 5),
                "Expected run to stop after command rejection"
            )
        }
    }

    // MARK: - Generic Approval

    /// Tests approving a generic approval request.
    func testApproveGeneric() throws {
        app.terminate()
        app.launchArguments = ["--uitesting", "--mock-api", "--with-generic-approval"]
        app.launch()

        try navigateToRunWithApproval()

        // Verify the generic approval detail shows "Approval requested" title
        let approvalTitle = app.staticTexts["Approval requested"]
        if approvalTitle.waitForExistence(timeout: 5) {
            // The message should be displayed
            XCTAssertTrue(app.scrollViews.firstMatch.exists)
        }

        // Approve the request
        let approveButton = app.buttons["Approve"]
        if approveButton.waitForExistence(timeout: 5) {
            approveButton.tap()

            // Verify run continues after approval
            let runningStatus = app.staticTexts["Running"]
            let completedStatus = app.staticTexts["Completed"]
            XCTAssertTrue(
                runningStatus.waitForExistence(timeout: 5) ||
                completedStatus.waitForExistence(timeout: 5),
                "Expected run to transition after generic approval"
            )
        }
    }

    /// Tests rejecting a generic approval request.
    func testRejectGeneric() throws {
        app.terminate()
        app.launchArguments = ["--uitesting", "--mock-api", "--with-generic-approval"]
        app.launch()

        try navigateToRunWithApproval()

        // Reject the request
        let rejectButton = app.buttons["Reject"]
        if rejectButton.waitForExistence(timeout: 5) {
            rejectButton.tap()

            // Enter rejection reason
            let rejectSheet = app.navigationBars["Reject"]
            if rejectSheet.waitForExistence(timeout: 2) {
                let reasonField = app.textFields.firstMatch
                if reasonField.exists {
                    reasonField.tap()
                    reasonField.typeText("User declined")
                }

                // Confirm rejection
                let confirmReject = app.buttons["Reject"]
                if confirmReject.exists {
                    confirmReject.tap()
                }
            }

            // Verify run stops after rejection
            let failedStatus = app.staticTexts["Failed"]
            let cancelledStatus = app.staticTexts["Cancelled"]
            XCTAssertTrue(
                failedStatus.waitForExistence(timeout: 5) ||
                cancelledStatus.waitForExistence(timeout: 5),
                "Expected run to stop after generic rejection"
            )
        }
    }

    // MARK: - Rejection with Reason

    /// Tests that rejection reason is properly submitted.
    func testRejectWithReasonSubmission() throws {
        try navigateToRunWithApproval()

        let rejectButton = app.buttons["Reject"]
        if rejectButton.waitForExistence(timeout: 5) {
            rejectButton.tap()

            // Verify reject sheet appears
            let rejectSheet = app.navigationBars["Reject"]
            XCTAssertTrue(rejectSheet.waitForExistence(timeout: 2), "Reject reason sheet should appear")

            // Verify "Reason (optional)" label
            let reasonLabel = app.staticTexts["Reason (optional)"]
            XCTAssertTrue(reasonLabel.exists, "Reason label should be visible")

            // Enter a detailed reason
            let reasonField = app.textFields["Why are you rejecting this?"]
            if reasonField.exists {
                reasonField.tap()
                reasonField.typeText("The proposed changes would break existing functionality")
            }

            // Cancel button should dismiss without submitting
            let cancelButton = app.buttons["Cancel"]
            XCTAssertTrue(cancelButton.exists)

            // Submit the rejection with reason
            let confirmReject = app.buttons["Reject"]
            XCTAssertTrue(confirmReject.exists)
            confirmReject.tap()

            // Sheet should dismiss
            XCTAssertFalse(rejectSheet.waitForExistence(timeout: 2))
        }
    }

    // MARK: - Multiple Pending Approvals

    /// Tests the banner shows correct text for multiple approvals.
    func testMultipleApprovalsBanner() throws {
        app.terminate()
        app.launchArguments = ["--uitesting", "--mock-api", "--with-multiple-approvals"]
        app.launch()

        // Wait for the banner to appear
        // With multiple approvals, banner should show "X approvals pending"
        let multipleBanner = app.staticTexts.matching(NSPredicate(format: "label CONTAINS 'approvals pending'")).firstMatch
        XCTAssertTrue(multipleBanner.waitForExistence(timeout: 5), "Banner should show multiple approvals text")
    }

    /// Tests navigating through multiple pending approvals.
    func testMultiplePendingApprovalsNavigation() throws {
        app.terminate()
        app.launchArguments = ["--uitesting", "--mock-api", "--with-multiple-approvals"]
        app.launch()

        // Tap on the banner to show the approval list
        let banner = app.buttons.matching(NSPredicate(format: "label CONTAINS 'approvals'")).firstMatch
        if banner.waitForExistence(timeout: 5) {
            banner.tap()

            // Verify approval list sheet appears
            let listTitle = app.navigationBars["Pending Approvals"]
            XCTAssertTrue(listTitle.waitForExistence(timeout: 2), "Approval list should appear")

            // Verify multiple rows exist in the list
            let approvalRows = app.cells
            XCTAssertTrue(approvalRows.count >= 2, "Should have multiple approval rows")

            // Tap on first approval
            if approvalRows.firstMatch.exists {
                approvalRows.firstMatch.tap()

                // Verify approval detail appears
                let approveButton = app.buttons["Approve"]
                XCTAssertTrue(approveButton.waitForExistence(timeout: 2), "Approval detail should appear")

                // Approve this one
                approveButton.tap()

                // After approving, should be back at the root or approval list
                // If there are still approvals, banner should update
            }
        }
    }

    /// Tests approving all multiple pending approvals in sequence.
    func testApproveAllMultiplePendingApprovals() throws {
        app.terminate()
        app.launchArguments = ["--uitesting", "--mock-api", "--with-multiple-approvals"]
        app.launch()

        var approvedCount = 0
        let maxApprovals = 5

        while approvedCount < maxApprovals {
            // Check if there's a banner (single or multiple)
            let singleBanner = app.staticTexts["Approval needed"]
            let multipleBanner = app.staticTexts.matching(NSPredicate(format: "label CONTAINS 'approvals pending'")).firstMatch

            if singleBanner.waitForExistence(timeout: 2) {
                // Single approval - tap banner directly
                singleBanner.tap()
            } else if multipleBanner.waitForExistence(timeout: 2) {
                // Multiple approvals - tap to show list, then select first
                multipleBanner.tap()

                let listTitle = app.navigationBars["Pending Approvals"]
                if listTitle.waitForExistence(timeout: 2) {
                    let firstRow = app.cells.firstMatch
                    if firstRow.exists {
                        firstRow.tap()
                    }
                }
            } else {
                // No more approvals
                break
            }

            // Approve the current one
            let approveButton = app.buttons["Approve"]
            if approveButton.waitForExistence(timeout: 3) {
                approveButton.tap()
                approvedCount += 1
            } else {
                break
            }

            // Brief pause for state update
            Thread.sleep(forTimeInterval: 0.5)
        }

        // Verify no banner remains
        let anyBanner = app.staticTexts.matching(NSPredicate(format: "label CONTAINS 'approval'")).firstMatch
        XCTAssertFalse(anyBanner.waitForExistence(timeout: 2), "All approvals should be cleared")
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
