import SwiftUI

/// Root screen showing all configured M servers.
struct ServerListView: View {
    @StateObject private var store = ServerStore.shared
    @State private var showingAddServer = false
    @State private var connectionStatuses: [UUID: ConnectionStatus] = [:]

    var body: some View {
        NavigationStack {
            Group {
                if store.servers.isEmpty {
                    emptyState
                } else {
                    serverList
                }
            }
            .navigationTitle("Servers")
            .toolbar {
                ToolbarItem(placement: .primaryAction) {
                    Button {
                        showingAddServer = true
                    } label: {
                        Image(systemName: "plus")
                    }
                }
            }
            .sheet(isPresented: $showingAddServer) {
                AddServerView(store: store)
            }
        }
    }

    private var emptyState: some View {
        ContentUnavailableView {
            Label("No servers yet", systemImage: "server.rack")
        } description: {
            Text("Add an M server to get started.")
        } actions: {
            Button("Add Server") {
                showingAddServer = true
            }
            .buttonStyle(.borderedProminent)
        }
    }

    private var serverList: some View {
        List {
            ForEach(store.servers) { server in
                NavigationLink(value: server) {
                    ServerRowView(
                        server: server,
                        status: connectionStatuses[server.id] ?? .unknown,
                        lastActivity: nil
                    )
                }
            }
            .onDelete(perform: store.deleteServers)
        }
        .listStyle(.plain)
        .navigationDestination(for: MServer.self) { server in
            ServerDestinationView(server: server)
        }
    }
}

/// Wrapper view that creates APIClient for server navigation.
private struct ServerDestinationView: View {
    let server: MServer
    @State private var apiKey: String?
    @State private var error: Error?

    var body: some View {
        Group {
            if let apiKey, !apiKey.isEmpty {
                let apiClient = APIClient(server: server, apiKey: apiKey)
                RepoListView(server: server, apiClient: apiClient)
            } else if let error {
                ContentUnavailableView {
                    Label("Unable to Connect", systemImage: "exclamationmark.triangle")
                } description: {
                    Text(error.localizedDescription)
                }
            } else {
                ProgressView()
            }
        }
        .task {
            loadAPIKey()
        }
    }

    private func loadAPIKey() {
        do {
            apiKey = try KeychainService.shared.getAPIKey(for: server.id)
            if apiKey == nil {
                error = MError.unauthorized
            }
        } catch {
            self.error = error
        }
    }
}
