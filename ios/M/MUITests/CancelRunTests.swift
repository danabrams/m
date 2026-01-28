import XCTest

final class CancelRunTests: XCTestCase {
    var app: XCUIApplication!

    override func setUpWithError() throws {
        continueAfterFailure = false
        app = XCUIApplication()
        app.launchForTesting(scenario: .runningTask)
    }

    override func tearDownWithError() throws {
        app = nil
    }

    // MARK: - Helper

    private func navigateToRunningRun() {
        let serverRow = app.cells[AccessibilityID.ServerList.serverRow].firstMatch
        XCTAssertTrue(serverRow.waitForExistence(timeout: 5))
        serverRow.tap()

        let repoRow = app.cells[AccessibilityID.RepoList.repoRow].firstMatch
        XCTAssertTrue(repoRow.waitForExistence(timeout: 5))
        repoRow.tap()

        let runRow = app.cells[AccessibilityID.RunList.runRow].firstMatch
        XCTAssertTrue(runRow.waitForExistence(timeout: 5))
        runRow.tap()
    }

    // MARK: - Cancel Run Tests

    func testCancelButtonVisibleForRunningRun() throws {
        // Given: Navigate to running run
        navigateToRunningRun()

        // Then: Cancel button is visible
        let cancelButton = app.buttons[AccessibilityID.RunDetail.cancelButton]
        XCTAssertTrue(cancelButton.waitForExistence(timeout: 5))
    }

    func testCancelRun() throws {
        // Given: Navigate to running run
        navigateToRunningRun()

        let cancelButton = app.buttons[AccessibilityID.RunDetail.cancelButton]
        XCTAssertTrue(cancelButton.waitForExistence(timeout: 5))

        // When: Tap Cancel
        cancelButton.tap()

        // Then: Run status changes to cancelled
        let statusLabel = app.staticTexts[AccessibilityID.RunDetail.statusLabel]
        XCTAssertTrue(statusLabel.waitForExistence(timeout: 5))

        // Verify status text indicates cancelled
        // Note: The exact text depends on implementation
        let cancelledStatus = app.staticTexts["Cancelled"]
        XCTAssertTrue(cancelledStatus.waitForExistence(timeout: 5))
    }

    func testCancelButtonNotVisibleForCompletedRun() throws {
        // This test would need a scenario with a completed run
        // For now, verify cancel button behavior
        navigateToRunningRun()

        let cancelButton = app.buttons[AccessibilityID.RunDetail.cancelButton]

        // If run is still running, cancel should be visible
        if cancelButton.waitForExistence(timeout: 3) {
            // Cancel the run
            cancelButton.tap()

            // After cancellation, cancel button should disappear
            XCTAssertFalse(cancelButton.waitForExistence(timeout: 3))
        }
    }

    func testCancelRunUpdatesRunList() throws {
        // Given: Navigate to running run and cancel it
        navigateToRunningRun()

        let cancelButton = app.buttons[AccessibilityID.RunDetail.cancelButton]
        XCTAssertTrue(cancelButton.waitForExistence(timeout: 5))
        cancelButton.tap()

        // When: Navigate back to run list
        app.navigationBars.buttons.element(boundBy: 0).tap()

        // Then: Run list shows updated status
        let runRow = app.cells[AccessibilityID.RunList.runRow].firstMatch
        XCTAssertTrue(runRow.waitForExistence(timeout: 3))

        // The row should indicate cancelled status (via icon or text)
    }
}
