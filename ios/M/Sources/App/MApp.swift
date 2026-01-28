import SwiftUI

@main
struct MApp: App {
    @StateObject private var approvalStore = ApprovalStore.shared

    var body: some Scene {
        WindowGroup {
            RootView()
                .environmentObject(approvalStore)
                .task {
                    approvalStore.startPolling()
                }
        }
    }
}

/// Root wrapper that provides the global approval banner overlay.
struct RootView: View {
    @EnvironmentObject private var approvalStore: ApprovalStore
    @State private var showingApprovalList = false
    @State private var selectedApproval: PendingApproval?

    var body: some View {
        VStack(spacing: 0) {
            ApprovalBannerView(
                approvalStore: approvalStore,
                showingApprovalList: $showingApprovalList,
                selectedApproval: $selectedApproval
            )

            ServerListView()
        }
        .sheet(isPresented: $showingApprovalList) {
            ApprovalListView(approvals: approvalStore.pendingApprovals) { pending in
                selectedApproval = pending
            }
        }
        .sheet(item: $selectedApproval) { pending in
            ApprovalDetailView(pending: pending) {
                // Approval resolved, refresh the store
                Task {
                    await approvalStore.refresh()
                }
            }
        }
    }
}
