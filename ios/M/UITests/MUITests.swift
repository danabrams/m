import XCTest

/// XCUITests for core iOS flows.
/// These tests verify the main user journeys through the M client app.
final class MUITests: XCTestCase {
    var app: XCUIApplication!

    override func setUpWithError() throws {
        continueAfterFailure = false
        app = XCUIApplication()
        app.launchArguments = ["--uitesting"]
        app.launch()
    }

    override func tearDownWithError() throws {
        app = nil
    }

    // MARK: - Add Server Flow

    /// Tests adding a new server with name, URL, and API key.
    func testAddServerFlow() throws {
        // Verify we're on the Server List screen
        XCTAssertTrue(app.navigationBars["Servers"].exists)

        // Tap the add button
        let addButton = app.navigationBars["Servers"].buttons["Add"]
        XCTAssertTrue(addButton.waitForExistence(timeout: 2))
        addButton.tap()

        // Verify Add Server sheet appears
        XCTAssertTrue(app.navigationBars["Add Server"].waitForExistence(timeout: 2))

        // Fill in server details
        let nameField = app.textFields["Name"]
        XCTAssertTrue(nameField.waitForExistence(timeout: 2))
        nameField.tap()
        nameField.typeText("Test Server")

        let urlField = app.textFields["URL"]
        urlField.tap()
        urlField.typeText("https://m.example.com")

        let apiKeyField = app.secureTextFields["API Key"]
        apiKeyField.tap()
        apiKeyField.typeText("test-api-key-12345")

        // Save the server
        let saveButton = app.navigationBars["Add Server"].buttons["Save"]
        XCTAssertTrue(saveButton.isEnabled)
        saveButton.tap()

        // Verify we're back on Server List and server appears
        XCTAssertTrue(app.navigationBars["Servers"].waitForExistence(timeout: 2))
        XCTAssertTrue(app.staticTexts["Test Server"].waitForExistence(timeout: 2))
    }

    /// Tests that the Save button is disabled until all fields are filled.
    func testAddServerValidation() throws {
        // Open Add Server sheet
        app.navigationBars["Servers"].buttons["Add"].tap()
        XCTAssertTrue(app.navigationBars["Add Server"].waitForExistence(timeout: 2))

        // Initially Save should be disabled
        let saveButton = app.navigationBars["Add Server"].buttons["Save"]
        XCTAssertFalse(saveButton.isEnabled)

        // Fill only name
        let nameField = app.textFields["Name"]
        nameField.tap()
        nameField.typeText("Test")
        XCTAssertFalse(saveButton.isEnabled)

        // Add URL
        let urlField = app.textFields["URL"]
        urlField.tap()
        urlField.typeText("https://example.com")
        XCTAssertFalse(saveButton.isEnabled)

        // Add API key - now Save should be enabled
        let apiKeyField = app.secureTextFields["API Key"]
        apiKeyField.tap()
        apiKeyField.typeText("key123")
        XCTAssertTrue(saveButton.isEnabled)

        // Cancel
        app.navigationBars["Add Server"].buttons["Cancel"].tap()
    }
}
