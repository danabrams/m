import Foundation
import Combine

/// Represents a pending approval with its source server context.
struct PendingApproval: Identifiable, Equatable {
    let approval: Approval
    let server: MServer
    let apiClient: APIClient

    var id: String { approval.id }

    static func == (lhs: PendingApproval, rhs: PendingApproval) -> Bool {
        lhs.approval == rhs.approval && lhs.server.id == rhs.server.id
    }
}

/// Tracks pending approvals across all configured servers.
/// Polls each server every 30 seconds for pending approvals.
@MainActor
final class ApprovalStore: ObservableObject {
    static let shared = ApprovalStore()

    @Published private(set) var pendingApprovals: [PendingApproval] = []
    @Published private(set) var isPolling = false

    private let serverStore: ServerStore
    private let keychain: KeychainService
    private var pollingTask: Task<Void, Never>?
    private let pollInterval: TimeInterval = 30

    init(serverStore: ServerStore = .shared, keychain: KeychainService = .shared) {
        self.serverStore = serverStore
        self.keychain = keychain
    }

    /// Starts polling all servers for pending approvals.
    func startPolling() {
        guard pollingTask == nil else { return }

        isPolling = true
        pollingTask = Task { [weak self] in
            while !Task.isCancelled {
                await self?.fetchAllPendingApprovals()
                try? await Task.sleep(for: .seconds(self?.pollInterval ?? 30))
            }
        }
    }

    /// Stops polling for approvals.
    func stopPolling() {
        pollingTask?.cancel()
        pollingTask = nil
        isPolling = false
    }

    /// Fetches pending approvals immediately.
    func refresh() async {
        await fetchAllPendingApprovals()
    }

    /// Removes an approval from the local list (after it's been resolved).
    func removeApproval(id: String) {
        pendingApprovals.removeAll { $0.id == id }
    }

    private func fetchAllPendingApprovals() async {
        let servers = serverStore.servers
        var allApprovals: [PendingApproval] = []

        await withTaskGroup(of: [PendingApproval].self) { group in
            for server in servers {
                guard let apiKey = try? keychain.getAPIKey(for: server.id) else {
                    continue
                }

                group.addTask {
                    let client = APIClient(server: server, apiKey: apiKey)
                    do {
                        let approvals = try await client.listPendingApprovals()
                        return approvals.map { approval in
                            PendingApproval(
                                approval: approval,
                                server: server,
                                apiClient: client
                            )
                        }
                    } catch {
                        // Silently fail for individual servers
                        return []
                    }
                }
            }

            for await serverApprovals in group {
                allApprovals.append(contentsOf: serverApprovals)
            }
        }

        // Sort by creation date (newest first)
        pendingApprovals = allApprovals.sorted { $0.approval.createdAt > $1.approval.createdAt }
    }
}
