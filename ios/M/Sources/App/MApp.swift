import SwiftUI

@main
struct MApp: App {
    init() {
        // Configure for UI testing if launched with --uitesting flag
        if TestingEnvironment.isUITesting {
            ServerStore.shared.configureForTesting(scenario: TestingEnvironment.scenario)
        }
    }

    var body: some Scene {
        WindowGroup {
            ServerListView()
        }
    }
}
