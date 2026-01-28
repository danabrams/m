import Foundation

/// A stub API client for UI testing that returns canned responses.
final class StubAPIClient: APIClientProtocol {
    private let scenario: TestScenario

    init(scenario: TestScenario) {
        self.scenario = scenario
    }

    // MARK: - Repos

    func listRepos() async throws -> [Repo] {
        switch scenario {
        case .empty:
            return []
        default:
            return StubData.repos
        }
    }

    func createRepo(name: String, gitURL: String?) async throws -> Repo {
        try await Task.sleep(nanoseconds: 300_000_000)
        return Repo(
            id: UUID().uuidString,
            name: name,
            gitURL: gitURL,
            createdAt: Date(),
            updatedAt: Date()
        )
    }

    func getRepo(id: String) async throws -> Repo {
        guard let repo = StubData.repos.first(where: { $0.id == id }) else {
            throw MError.notFound(message: "Repo not found")
        }
        return repo
    }

    func deleteRepo(id: String) async throws {
        try await Task.sleep(nanoseconds: 200_000_000)
    }

    // MARK: - Runs

    func listRuns(repoID: String) async throws -> [Run] {
        switch scenario {
        case .empty:
            return []
        case .pendingApproval:
            return StubData.runsWithPendingApproval
        case .runningTask:
            return StubData.runsWithRunning
        default:
            return StubData.runs
        }
    }

    func createRun(repoID: String, prompt: String) async throws -> Run {
        try await Task.sleep(nanoseconds: 500_000_000)
        return Run(
            id: UUID().uuidString,
            repoID: repoID,
            prompt: prompt,
            state: .running,
            createdAt: Date(),
            updatedAt: Date()
        )
    }

    func getRun(id: String) async throws -> Run {
        let allRuns = StubData.runs + StubData.runsWithPendingApproval + StubData.runsWithRunning
        guard let run = allRuns.first(where: { $0.id == id }) else {
            throw MError.notFound(message: "Run not found")
        }
        return run
    }

    func cancelRun(id: String) async throws {
        try await Task.sleep(nanoseconds: 300_000_000)
    }

    func sendInput(runID: String, text: String) async throws {
        try await Task.sleep(nanoseconds: 300_000_000)
    }

    // MARK: - Approvals

    func listPendingApprovals() async throws -> [Approval] {
        switch scenario {
        case .pendingApproval:
            return StubData.pendingApprovals
        default:
            return []
        }
    }

    func getApproval(id: String) async throws -> Approval {
        guard let approval = StubData.pendingApprovals.first(where: { $0.id == id }) else {
            throw MError.notFound(message: "Approval not found")
        }
        return approval
    }

    func resolveApproval(id: String, approved: Bool, reason: String? = nil) async throws {
        try await Task.sleep(nanoseconds: 300_000_000)
    }

    // MARK: - Device Registration

    func registerDevice(token: String) async throws {
        // No-op for testing
    }

    func unregisterDevice(token: String) async throws {
        // No-op for testing
    }
}
