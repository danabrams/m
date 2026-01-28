import Foundation
import Combine

/// Tracks pending approvals across all servers for the global banner.
@MainActor
final class ApprovalStore: ObservableObject {
    static let shared = ApprovalStore()

    /// All pending approvals with their associated server info.
    @Published private(set) var pendingApprovals: [PendingApproval] = []

    /// Whether any approvals are pending.
    var hasPendingApprovals: Bool {
        !pendingApprovals.isEmpty
    }

    /// Count of pending approvals.
    var count: Int {
        pendingApprovals.count
    }

    private var refreshTasks: [UUID: Task<Void, Never>] = [:]
    private var clients: [UUID: APIClient] = [:]

    private init() {}

    /// Registers a server's API client for polling.
    func registerClient(_ client: APIClient, for serverID: UUID) {
        clients[serverID] = client
        startPolling(for: serverID)
    }

    /// Unregisters a server (stops polling).
    func unregisterClient(for serverID: UUID) {
        refreshTasks[serverID]?.cancel()
        refreshTasks[serverID] = nil
        clients[serverID] = nil
        pendingApprovals.removeAll { $0.serverID == serverID }
    }

    /// Manually refresh approvals for a server.
    func refresh(serverID: UUID) async {
        guard let client = clients[serverID] else { return }

        do {
            let approvals = try await client.listPendingApprovals()
            pendingApprovals.removeAll { $0.serverID == serverID }
            pendingApprovals.append(contentsOf: approvals.map {
                PendingApproval(serverID: serverID, approval: $0)
            })
        } catch {
            // Silently fail - banner is best-effort
        }
    }

    /// Removes an approval after it's been resolved.
    func removeApproval(id: String) {
        pendingApprovals.removeAll { $0.approval.id == id }
    }

    private func startPolling(for serverID: UUID) {
        refreshTasks[serverID]?.cancel()

        refreshTasks[serverID] = Task { [weak self] in
            while !Task.isCancelled {
                await self?.refresh(serverID: serverID)
                try? await Task.sleep(for: .seconds(30))
            }
        }
    }
}

/// A pending approval with its associated server ID.
struct PendingApproval: Identifiable {
    let serverID: UUID
    let approval: Approval

    var id: String { approval.id }
}
