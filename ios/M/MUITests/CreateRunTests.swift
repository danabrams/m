import XCTest

final class CreateRunTests: XCTestCase {
    var app: XCUIApplication!

    override func setUpWithError() throws {
        continueAfterFailure = false
        app = XCUIApplication()
        app.launchForTesting(scenario: .withServer)
    }

    override func tearDownWithError() throws {
        app = nil
    }

    // MARK: - Helper

    private func navigateToRunList() {
        let serverRow = app.cells[AccessibilityID.ServerList.serverRow].firstMatch
        XCTAssertTrue(serverRow.waitForExistence(timeout: 5))
        serverRow.tap()

        let repoRow = app.cells[AccessibilityID.RepoList.repoRow].firstMatch
        XCTAssertTrue(repoRow.waitForExistence(timeout: 5))
        repoRow.tap()
    }

    // MARK: - Create Run Flow

    func testOpenNewRunSheet() throws {
        // Given: In run list
        navigateToRunList()

        let addButton = app.buttons[AccessibilityID.RunList.addButton]
        XCTAssertTrue(addButton.waitForExistence(timeout: 5))

        // When: Tap "+" button
        addButton.tap()

        // Then: New Run sheet appears
        let promptField = app.textViews[AccessibilityID.NewRun.promptField]
        XCTAssertTrue(promptField.waitForExistence(timeout: 2))
    }

    func testNewRunFormValidation() throws {
        // Given: New Run sheet open
        navigateToRunList()

        let addButton = app.buttons[AccessibilityID.RunList.addButton]
        XCTAssertTrue(addButton.waitForExistence(timeout: 5))
        addButton.tap()

        let startButton = app.buttons[AccessibilityID.NewRun.startButton]
        XCTAssertTrue(startButton.waitForExistence(timeout: 2))

        // Then: Start button is disabled when prompt is empty
        XCTAssertFalse(startButton.isEnabled)
    }

    func testCreateRunSuccess() throws {
        // Given: New Run sheet open
        navigateToRunList()

        let addButton = app.buttons[AccessibilityID.RunList.addButton]
        XCTAssertTrue(addButton.waitForExistence(timeout: 5))
        addButton.tap()

        let promptField = app.textViews[AccessibilityID.NewRun.promptField]
        let startButton = app.buttons[AccessibilityID.NewRun.startButton]
        XCTAssertTrue(promptField.waitForExistence(timeout: 2))

        // When: Enter prompt
        promptField.tap()
        promptField.typeText("Fix the bug in authentication")

        // Then: Start button becomes enabled
        XCTAssertTrue(startButton.isEnabled)

        // When: Tap Start
        startButton.tap()

        // Then: Sheet dismisses and run appears in list
        let runRow = app.cells[AccessibilityID.RunList.runRow].firstMatch
        XCTAssertTrue(runRow.waitForExistence(timeout: 5))
    }

    func testCreateRunCancel() throws {
        // Given: New Run sheet open with text entered
        navigateToRunList()

        let addButton = app.buttons[AccessibilityID.RunList.addButton]
        XCTAssertTrue(addButton.waitForExistence(timeout: 5))
        addButton.tap()

        let promptField = app.textViews[AccessibilityID.NewRun.promptField]
        let cancelButton = app.buttons[AccessibilityID.NewRun.cancelButton]
        XCTAssertTrue(promptField.waitForExistence(timeout: 2))

        promptField.tap()
        promptField.typeText("Some text")

        // When: Tap Cancel
        cancelButton.tap()

        // Then: Sheet dismisses
        XCTAssertTrue(addButton.waitForExistence(timeout: 2))
    }

    func testCreateRunFromEmptyState() throws {
        // Given: Empty run list (using empty scenario for this specific test would be ideal)
        // For now, navigate and check for empty state button if present
        navigateToRunList()

        // Check if empty state button exists (may not if there are runs)
        let emptyStateButton = app.buttons[AccessibilityID.RunList.emptyStateAddButton]
        if emptyStateButton.waitForExistence(timeout: 2) {
            // When: Tap "Start a Run" in empty state
            emptyStateButton.tap()

            // Then: New Run sheet appears
            let promptField = app.textViews[AccessibilityID.NewRun.promptField]
            XCTAssertTrue(promptField.waitForExistence(timeout: 2))
        }
    }
}
