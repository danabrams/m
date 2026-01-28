import SwiftUI

/// Screen showing repos in a selected server.
struct RepoListView: View {
    let server: MServer
    let apiClient: APIClient

    @State private var repos: [Repo] = []
    @State private var repoRunInfo: [String: RepoRunInfo] = [:]
    @State private var isLoading = true
    @State private var error: MError?

    var body: some View {
        Group {
            if isLoading {
                ProgressView()
            } else if let error {
                errorView(error)
            } else if repos.isEmpty {
                emptyState
            } else {
                repoList
            }
        }
        .navigationTitle(server.name)
        .task {
            await loadRepos()
        }
        .refreshable {
            await loadRepos()
        }
        .navigationDestination(for: Repo.self) { repo in
            RunListView(repo: repo, apiClient: apiClient)
        }
        .onAppear {
            ApprovalStore.shared.registerClient(apiClient, for: server.id)
        }
        .onDisappear {
            ApprovalStore.shared.unregisterClient(for: server.id)
        }
    }

    private var emptyState: some View {
        ContentUnavailableView {
            Label("No repos in this server", systemImage: "folder")
        } description: {
            Text("Repos will appear here once created.")
        }
    }

    private var repoList: some View {
        List {
            ForEach(repos) { repo in
                NavigationLink(value: repo) {
                    let info = repoRunInfo[repo.id]
                    RepoRowView(
                        repo: repo,
                        hasActiveRun: info?.hasActiveRun ?? false,
                        lastRunState: info?.lastRunState
                    )
                }
            }
        }
        .listStyle(.plain)
    }

    private func errorView(_ error: MError) -> some View {
        ContentUnavailableView {
            Label("Unable to Load", systemImage: "exclamationmark.triangle")
        } description: {
            Text(error.localizedDescription)
        } actions: {
            Button("Retry") {
                Task {
                    await loadRepos()
                }
            }
            .buttonStyle(.borderedProminent)
        }
    }

    private func loadRepos() async {
        isLoading = repos.isEmpty
        error = nil

        do {
            repos = try await apiClient.listRepos()
            await loadRunInfo()
        } catch let mError as MError {
            error = mError
        } catch {
            self.error = .unknown(statusCode: 0, message: error.localizedDescription)
        }

        isLoading = false
    }

    private func loadRunInfo() async {
        // Load run info for all repos concurrently
        await withTaskGroup(of: (String, RepoRunInfo?).self) { group in
            for repo in repos {
                group.addTask {
                    do {
                        let runs = try await apiClient.listRuns(repoID: repo.id)
                        return (repo.id, RepoRunInfo(runs: runs))
                    } catch {
                        return (repo.id, nil)
                    }
                }
            }

            for await (repoID, info) in group {
                if let info {
                    repoRunInfo[repoID] = info
                }
            }
        }
    }
}

/// Summarized run information for a repo.
private struct RepoRunInfo {
    let hasActiveRun: Bool
    let lastRunState: RunState?

    init(runs: [Run]) {
        // Check if any run is active (running, waiting approval, waiting input)
        hasActiveRun = runs.contains { run in
            run.state == .running || run.state == .waitingApproval || run.state == .waitingInput
        }

        // Get the state of the most recent run (runs should be newest first)
        lastRunState = runs.first?.state
    }
}
