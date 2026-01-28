import XCTest

final class ApprovalFlowTests: XCTestCase {
    var app: XCUIApplication!

    override func setUpWithError() throws {
        continueAfterFailure = false
        app = XCUIApplication()
        app.launchForTesting(scenario: .pendingApproval)
    }

    override func tearDownWithError() throws {
        app = nil
    }

    // MARK: - Helper

    private func navigateToApproval() {
        let serverRow = app.cells[AccessibilityID.ServerList.serverRow].firstMatch
        XCTAssertTrue(serverRow.waitForExistence(timeout: 5))
        serverRow.tap()

        let repoRow = app.cells[AccessibilityID.RepoList.repoRow].firstMatch
        XCTAssertTrue(repoRow.waitForExistence(timeout: 5))
        repoRow.tap()

        let runRow = app.cells[AccessibilityID.RunList.runRow].firstMatch
        XCTAssertTrue(runRow.waitForExistence(timeout: 5))
        runRow.tap()

        let pendingCard = app.otherElements[AccessibilityID.RunDetail.pendingActionCard]
        XCTAssertTrue(pendingCard.waitForExistence(timeout: 5))
        pendingCard.tap()
    }

    // MARK: - Approval Flow Tests

    func testApprovalSheetShowsDiff() throws {
        // Given: Navigate to approval
        navigateToApproval()

        // Then: Diff view is displayed
        let diffView = app.scrollViews[AccessibilityID.Approval.diffView]
        XCTAssertTrue(diffView.waitForExistence(timeout: 3))
    }

    func testApprovalShowsApproveAndRejectButtons() throws {
        // Given: Navigate to approval
        navigateToApproval()

        // Then: Both buttons are visible
        let approveButton = app.buttons[AccessibilityID.Approval.approveButton]
        let rejectButton = app.buttons[AccessibilityID.Approval.rejectButton]

        XCTAssertTrue(approveButton.waitForExistence(timeout: 3))
        XCTAssertTrue(rejectButton.exists)
    }

    func testApproveDiff() throws {
        // Given: Navigate to approval
        navigateToApproval()

        let approveButton = app.buttons[AccessibilityID.Approval.approveButton]
        XCTAssertTrue(approveButton.waitForExistence(timeout: 3))

        // When: Tap Approve
        approveButton.tap()

        // Then: Approval sheet dismisses
        XCTAssertFalse(approveButton.waitForExistence(timeout: 2))

        // And: Run continues (status updates)
        let statusLabel = app.staticTexts[AccessibilityID.RunDetail.statusLabel]
        XCTAssertTrue(statusLabel.waitForExistence(timeout: 3))
    }

    func testRejectDiff() throws {
        // Given: Navigate to approval
        navigateToApproval()

        let rejectButton = app.buttons[AccessibilityID.Approval.rejectButton]
        XCTAssertTrue(rejectButton.waitForExistence(timeout: 3))

        // When: Tap Reject
        rejectButton.tap()

        // Then: Approval sheet dismisses
        XCTAssertFalse(rejectButton.waitForExistence(timeout: 2))
    }

    func testDiffFileExpansion() throws {
        // Given: Navigate to approval with multiple files
        navigateToApproval()

        let diffView = app.scrollViews[AccessibilityID.Approval.diffView]
        XCTAssertTrue(diffView.waitForExistence(timeout: 3))

        // Find expandable file sections
        let fileHeaders = diffView.buttons.matching(NSPredicate(format: "label CONTAINS[c] '.swift' OR label CONTAINS[c] '.go'"))

        if fileHeaders.count > 0 {
            let firstFile = fileHeaders.firstMatch

            // When: Tap to expand
            firstFile.tap()

            // Then: File content becomes visible
            // (This would show diff content - specific verification depends on implementation)
        }
    }
}
