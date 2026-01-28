import Foundation

/// Protocol defining the API client interface for dependency injection.
protocol APIClientProtocol {
    // MARK: - Repos
    func listRepos() async throws -> [Repo]
    func createRepo(name: String, gitURL: String?) async throws -> Repo
    func getRepo(id: String) async throws -> Repo
    func deleteRepo(id: String) async throws

    // MARK: - Runs
    func listRuns(repoID: String) async throws -> [Run]
    func createRun(repoID: String, prompt: String) async throws -> Run
    func getRun(id: String) async throws -> Run
    func cancelRun(id: String) async throws
    func sendInput(runID: String, text: String) async throws

    // MARK: - Approvals
    func listPendingApprovals() async throws -> [Approval]
    func getApproval(id: String) async throws -> Approval
    func resolveApproval(id: String, approved: Bool, reason: String?) async throws

    // MARK: - Device Registration
    func registerDevice(token: String) async throws
    func unregisterDevice(token: String) async throws
}

// Make APIClient conform to the protocol
extension APIClient: APIClientProtocol {}
