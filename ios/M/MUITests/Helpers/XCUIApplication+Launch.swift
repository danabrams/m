import XCTest

extension XCUIApplication {
    /// Launches the app in UI testing mode with stubbed data.
    func launchForTesting() {
        launchArguments = ["--uitesting"]
        launch()
    }

    /// Launches the app with a specific test scenario.
    func launchForTesting(scenario: TestScenario) {
        launchArguments = ["--uitesting", "--scenario", scenario.rawValue]
        launch()
    }
}

/// Pre-configured test scenarios with different stub data.
enum TestScenario: String {
    /// Empty state - no servers configured.
    case empty = "empty"

    /// Single server with repos and runs.
    case withServer = "with-server"

    /// Server with a run waiting for approval.
    case pendingApproval = "pending-approval"

    /// Server with a running task.
    case runningTask = "running-task"
}
