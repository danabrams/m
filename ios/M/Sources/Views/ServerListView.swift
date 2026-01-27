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
                ServerRowView(
                    server: server,
                    status: connectionStatuses[server.id] ?? .unknown,
                    lastActivity: nil
                )
            }
            .onDelete(perform: store.deleteServers)
        }
        .listStyle(.plain)
    }
}
