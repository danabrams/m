import Foundation

/// Canned data for UI testing.
enum StubData {
    // MARK: - Servers

    static let servers: [MServer] = [
        MServer(
            id: UUID(uuidString: "11111111-1111-1111-1111-111111111111")!,
            name: "Test Server",
            url: URL(string: "https://test.m.local")!
        )
    ]

    // MARK: - Repos

    static let repos: [Repo] = [
        Repo(
            id: "repo-1",
            name: "my-project",
            gitURL: "https://github.com/test/my-project",
            createdAt: Date().addingTimeInterval(-86400),
            updatedAt: Date().addingTimeInterval(-3600)
        ),
        Repo(
            id: "repo-2",
            name: "another-repo",
            gitURL: "https://github.com/test/another-repo",
            createdAt: Date().addingTimeInterval(-172800),
            updatedAt: Date().addingTimeInterval(-7200)
        )
    ]

    // MARK: - Runs

    static let runs: [Run] = [
        Run(
            id: "run-1",
            repoID: "repo-1",
            prompt: "Fix the authentication bug",
            state: .completed,
            createdAt: Date().addingTimeInterval(-3600),
            updatedAt: Date().addingTimeInterval(-3000)
        ),
        Run(
            id: "run-2",
            repoID: "repo-1",
            prompt: "Add user profile page",
            state: .failed,
            createdAt: Date().addingTimeInterval(-7200),
            updatedAt: Date().addingTimeInterval(-6800)
        )
    ]

    static let runsWithRunning: [Run] = [
        Run(
            id: "run-running",
            repoID: "repo-1",
            prompt: "Refactor the database layer",
            state: .running,
            createdAt: Date().addingTimeInterval(-300),
            updatedAt: Date()
        )
    ] + runs

    static let runsWithPendingApproval: [Run] = [
        Run(
            id: "run-approval",
            repoID: "repo-1",
            prompt: "Update the API endpoints",
            state: .waitingApproval,
            createdAt: Date().addingTimeInterval(-600),
            updatedAt: Date().addingTimeInterval(-60)
        )
    ] + runs

    // MARK: - Approvals

    static let pendingApprovals: [Approval] = [
        Approval(
            id: "approval-1",
            runID: "run-approval",
            type: .diff,
            tool: "Edit",
            requestID: "req-1",
            payload: ApprovalPayload(
                message: "Apply these changes to update the API endpoints",
                command: nil,
                diff: """
                diff --git a/src/api.go b/src/api.go
                --- a/src/api.go
                +++ b/src/api.go
                @@ -10,6 +10,10 @@
                 func handleRequest(w http.ResponseWriter, r *http.Request) {
                +    // Add authentication check
                +    if !isAuthenticated(r) {
                +        http.Error(w, "Unauthorized", 401)
                +        return
                +    }
                     // Handle the request
                """,
                files: [
                    DiffFile(
                        path: "src/api.go",
                        additions: 5,
                        deletions: 0,
                        content: "Authentication check added"
                    ),
                    DiffFile(
                        path: "src/auth.go",
                        additions: 20,
                        deletions: 3,
                        content: "New auth helper functions"
                    )
                ]
            ),
            createdAt: Date().addingTimeInterval(-60)
        )
    ]
}
