import SwiftUI

/// Screen showing runs for a selected repo.
struct RunListView: View {
    let repo: Repo
    let apiClient: APIClient

    @State private var runs: [Run] = []
    @State private var isLoading = true
    @State private var error: MError?
    @State private var showingNewRun = false

    var body: some View {
        Group {
            if isLoading {
                ProgressView()
            } else if let error {
                errorView(error)
            } else if runs.isEmpty {
                emptyState
            } else {
                runList
            }
        }
        .navigationTitle(repo.name)
        .toolbar {
            ToolbarItem(placement: .primaryAction) {
                Button {
                    showingNewRun = true
                } label: {
                    Image(systemName: "plus")
                }
            }
        }
        .sheet(isPresented: $showingNewRun) {
            NewRunView(
                apiClient: apiClient,
                repoID: repo.id,
                onCreated: { newRun in
                    runs.insert(newRun, at: 0)
                }
            )
        }
        .task {
            await loadRuns()
        }
        .refreshable {
            await loadRuns()
        }
    }

    private var emptyState: some View {
        ContentUnavailableView {
            Label("No runs yet", systemImage: "terminal")
        } description: {
            Text("Start a run to begin working with the agent.")
        } actions: {
            Button("Start a Run") {
                showingNewRun = true
            }
            .buttonStyle(.borderedProminent)
        }
    }

    private var runList: some View {
        List {
            ForEach(runs) { run in
                NavigationLink(value: run) {
                    RunRowView(run: run)
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
                    await loadRuns()
                }
            }
            .buttonStyle(.borderedProminent)
        }
    }

    private func loadRuns() async {
        isLoading = runs.isEmpty
        error = nil

        do {
            runs = try await apiClient.listRuns(repoID: repo.id)
        } catch let mError as MError {
            error = mError
        } catch {
            self.error = .unknown(statusCode: 0, message: error.localizedDescription)
        }

        isLoading = false
    }
}
