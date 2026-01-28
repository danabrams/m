import Foundation

/// Detects UI testing mode from launch arguments.
enum TestingEnvironment {
    /// Whether the app is running in UI testing mode.
    static var isUITesting: Bool {
        ProcessInfo.processInfo.arguments.contains("--uitesting")
    }

    /// The current test scenario, if specified.
    static var scenario: TestScenario {
        guard let index = ProcessInfo.processInfo.arguments.firstIndex(of: "--scenario"),
              index + 1 < ProcessInfo.processInfo.arguments.count else {
            return .withServer
        }
        let scenarioName = ProcessInfo.processInfo.arguments[index + 1]
        return TestScenario(rawValue: scenarioName) ?? .withServer
    }
}

/// Pre-configured test scenarios.
enum TestScenario: String {
    case empty = "empty"
    case withServer = "with-server"
    case pendingApproval = "pending-approval"
    case runningTask = "running-task"
}
