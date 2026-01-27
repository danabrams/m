import SwiftUI

/// Screen showing repos for a selected server.
struct RepoListView: View {
    let server: MServer
    let apiClient: APIClient

    @State private var repos: [Repo] = []
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
                    RepoRowView(repo: repo)
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
        } catch let mError as MError {
            error = mError
        } catch {
            self.error = .unknown(statusCode: 0, message: error.localizedDescription)
        }

        isLoading = false
    }
}

/// Row displaying a single repo with status indicators.
struct RepoRowView: View {
    let repo: Repo

    var body: some View {
        HStack(spacing: 12) {
            VStack(alignment: .leading, spacing: 2) {
                Text(repo.name)
                    .font(.body)
                    .foregroundStyle(.primary)

                if let gitURL = repo.gitURL {
                    Text(gitURL)
                        .font(.caption)
                        .foregroundStyle(.secondary)
                        .lineLimit(1)
                }
            }

            Spacer()
        }
        .padding(.vertical, 4)
    }
}
