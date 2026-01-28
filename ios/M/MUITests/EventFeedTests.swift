import XCTest

final class EventFeedTests: XCTestCase {
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

    private func navigateToRunDetail() {
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

    // MARK: - Event Feed Tests

    func testRunDetailShowsStatus() throws {
        // Given: Navigate to run detail
        navigateToRunDetail()

        // Then: Status is displayed
        let statusLabel = app.staticTexts[AccessibilityID.RunDetail.statusLabel]
        XCTAssertTrue(statusLabel.waitForExistence(timeout: 5))
    }

    func testEventFeedIsDisplayed() throws {
        // Given: Navigate to run detail
        navigateToRunDetail()

        // Then: Event feed is visible
        let eventFeed = app.scrollViews[AccessibilityID.RunDetail.eventFeed]
        XCTAssertTrue(eventFeed.waitForExistence(timeout: 5))
    }

    func testEventFeedShowsEvents() throws {
        // Given: Navigate to run detail with events
        navigateToRunDetail()

        // Then: Events are displayed in the feed
        let eventFeed = app.scrollViews[AccessibilityID.RunDetail.eventFeed]
        XCTAssertTrue(eventFeed.waitForExistence(timeout: 5))

        // Check for event content (tool calls, output, etc.)
        let eventContent = eventFeed.descendants(matching: .staticText)
        XCTAssertGreaterThan(eventContent.count, 0)
    }

    func testPendingActionCardAppears() throws {
        // Given: Scenario with pending approval
        app.terminate()
        app.launchForTesting(scenario: .pendingApproval)
        navigateToRunDetail()

        // Then: Pending action card is displayed
        let pendingCard = app.otherElements[AccessibilityID.RunDetail.pendingActionCard]
        XCTAssertTrue(pendingCard.waitForExistence(timeout: 5))
    }

    func testPendingActionCardTapOpensApproval() throws {
        // Given: Run with pending approval
        app.terminate()
        app.launchForTesting(scenario: .pendingApproval)
        navigateToRunDetail()

        let pendingCard = app.otherElements[AccessibilityID.RunDetail.pendingActionCard]
        XCTAssertTrue(pendingCard.waitForExistence(timeout: 5))

        // When: Tap pending action card
        pendingCard.tap()

        // Then: Approval sheet opens
        let approveButton = app.buttons[AccessibilityID.Approval.approveButton]
        XCTAssertTrue(approveButton.waitForExistence(timeout: 3))
    }
}
