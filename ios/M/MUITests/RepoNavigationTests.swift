import XCTest

final class RepoNavigationTests: XCTestCase {
    var app: XCUIApplication!

    override func setUpWithError() throws {
        continueAfterFailure = false
        app = XCUIApplication()
        app.launchForTesting(scenario: .withServer)
    }

    override func tearDownWithError() throws {
        app = nil
    }

    // MARK: - Navigation to Repo List

    func testNavigateToRepoList() throws {
        // Given: Server list with a server
        let serverRow = app.cells[AccessibilityID.ServerList.serverRow].firstMatch
        XCTAssertTrue(serverRow.waitForExistence(timeout: 5))

        // When: Tap on server
        serverRow.tap()

        // Then: Navigate to repo list
        let repoRow = app.cells[AccessibilityID.RepoList.repoRow].firstMatch
        XCTAssertTrue(repoRow.waitForExistence(timeout: 5))
    }

    func testRepoListShowsRepos() throws {
        // Given: Navigate to repo list
        let serverRow = app.cells[AccessibilityID.ServerList.serverRow].firstMatch
        XCTAssertTrue(serverRow.waitForExistence(timeout: 5))
        serverRow.tap()

        // Then: Repos are displayed
        let repoRows = app.cells.matching(identifier: AccessibilityID.RepoList.repoRow)
        XCTAssertTrue(repoRows.firstMatch.waitForExistence(timeout: 5))
        XCTAssertGreaterThan(repoRows.count, 0)
    }

    func testNavigateBackToServerList() throws {
        // Given: In repo list
        let serverRow = app.cells[AccessibilityID.ServerList.serverRow].firstMatch
        XCTAssertTrue(serverRow.waitForExistence(timeout: 5))
        serverRow.tap()

        let repoRow = app.cells[AccessibilityID.RepoList.repoRow].firstMatch
        XCTAssertTrue(repoRow.waitForExistence(timeout: 5))

        // When: Tap back button
        app.navigationBars.buttons.element(boundBy: 0).tap()

        // Then: Back to server list
        XCTAssertTrue(serverRow.waitForExistence(timeout: 3))
    }

    func testNavigateToRunList() throws {
        // Given: In repo list
        let serverRow = app.cells[AccessibilityID.ServerList.serverRow].firstMatch
        XCTAssertTrue(serverRow.waitForExistence(timeout: 5))
        serverRow.tap()

        let repoRow = app.cells[AccessibilityID.RepoList.repoRow].firstMatch
        XCTAssertTrue(repoRow.waitForExistence(timeout: 5))

        // When: Tap on repo
        repoRow.tap()

        // Then: Navigate to run list
        let addRunButton = app.buttons[AccessibilityID.RunList.addButton]
        XCTAssertTrue(addRunButton.waitForExistence(timeout: 5))
    }
}
