import XCTest

final class AddServerFlowTests: XCTestCase {
    var app: XCUIApplication!

    override func setUpWithError() throws {
        continueAfterFailure = false
        app = XCUIApplication()
        app.launchForTesting(scenario: .empty)
    }

    override func tearDownWithError() throws {
        app = nil
    }

    // MARK: - Add Server Flow

    func testAddServerFromEmptyState() throws {
        // Given: Empty server list with "Add Server" button
        let emptyStateButton = app.buttons[AccessibilityID.ServerList.emptyStateAddButton]
        XCTAssertTrue(emptyStateButton.waitForExistence(timeout: 5))

        // When: Tap "Add Server"
        emptyStateButton.tap()

        // Then: Add Server sheet appears
        let nameField = app.textFields[AccessibilityID.AddServer.nameField]
        XCTAssertTrue(nameField.waitForExistence(timeout: 2))
    }

    func testAddServerFromToolbar() throws {
        // Given: Server list with toolbar
        app.terminate()
        app.launchForTesting(scenario: .withServer)

        let addButton = app.buttons[AccessibilityID.ServerList.addButton]
        XCTAssertTrue(addButton.waitForExistence(timeout: 5))

        // When: Tap "+" button
        addButton.tap()

        // Then: Add Server sheet appears
        let nameField = app.textFields[AccessibilityID.AddServer.nameField]
        XCTAssertTrue(nameField.waitForExistence(timeout: 2))
    }

    func testAddServerFormValidation() throws {
        // Given: Add Server sheet open
        let emptyStateButton = app.buttons[AccessibilityID.ServerList.emptyStateAddButton]
        XCTAssertTrue(emptyStateButton.waitForExistence(timeout: 5))
        emptyStateButton.tap()

        let saveButton = app.buttons[AccessibilityID.AddServer.saveButton]
        XCTAssertTrue(saveButton.waitForExistence(timeout: 2))

        // Then: Save button is disabled when fields are empty
        XCTAssertFalse(saveButton.isEnabled)
    }

    func testAddServerSuccess() throws {
        // Given: Add Server sheet open
        let emptyStateButton = app.buttons[AccessibilityID.ServerList.emptyStateAddButton]
        XCTAssertTrue(emptyStateButton.waitForExistence(timeout: 5))
        emptyStateButton.tap()

        // When: Fill in all fields
        let nameField = app.textFields[AccessibilityID.AddServer.nameField]
        let urlField = app.textFields[AccessibilityID.AddServer.urlField]
        let apiKeyField = app.secureTextFields[AccessibilityID.AddServer.apiKeyField]
        let saveButton = app.buttons[AccessibilityID.AddServer.saveButton]

        XCTAssertTrue(nameField.waitForExistence(timeout: 2))

        nameField.tap()
        nameField.typeText("Test Server")

        urlField.tap()
        urlField.typeText("https://test.example.com")

        apiKeyField.tap()
        apiKeyField.typeText("test-api-key-123")

        // Then: Save button becomes enabled
        XCTAssertTrue(saveButton.isEnabled)

        // When: Tap Save
        saveButton.tap()

        // Then: Sheet dismisses and server appears in list
        let serverRow = app.cells[AccessibilityID.ServerList.serverRow].firstMatch
        XCTAssertTrue(serverRow.waitForExistence(timeout: 3))
    }

    func testAddServerCancel() throws {
        // Given: Add Server sheet open
        let emptyStateButton = app.buttons[AccessibilityID.ServerList.emptyStateAddButton]
        XCTAssertTrue(emptyStateButton.waitForExistence(timeout: 5))
        emptyStateButton.tap()

        let cancelButton = app.buttons[AccessibilityID.AddServer.cancelButton]
        XCTAssertTrue(cancelButton.waitForExistence(timeout: 2))

        // When: Tap Cancel
        cancelButton.tap()

        // Then: Sheet dismisses, back to empty state
        XCTAssertTrue(emptyStateButton.waitForExistence(timeout: 2))
    }
}
