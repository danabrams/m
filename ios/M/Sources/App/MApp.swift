import SwiftUI

@main
struct MApp: App {
    #if os(iOS)
    @UIApplicationDelegateAdaptor(AppDelegate.self) private var appDelegate
    #endif
    @StateObject private var approvalStore = ApprovalStore.shared
    @StateObject private var notificationService = NotificationService.shared

    var body: some Scene {
        WindowGroup {
            RootView()
                .environmentObject(approvalStore)
                .environmentObject(notificationService)
                .task {
                    approvalStore.startPolling()
                    #if os(iOS)
                    await notificationService.checkAuthorizationStatus()
                    // Request notification permission on first launch
                    if !notificationService.isAuthorized {
                        _ = await notificationService.requestAuthorization()
                    }
                    #endif
                }
                .onOpenURL { url in
                    handleDeepLink(url: url)
                }
        }
    }

    private func handleDeepLink(url: URL) {
        guard let deepLink = DeepLink.from(url: url) else { return }
        notificationService.onDeepLink?(deepLink)
    }
}

/// Root wrapper that provides the global approval banner overlay.
struct RootView: View {
    @EnvironmentObject private var approvalStore: ApprovalStore
    @EnvironmentObject private var notificationService: NotificationService
    @State private var showingApprovalList = false
    @State private var selectedApproval: PendingApproval?
    @State private var pendingDeepLink: DeepLink?

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
        .onAppear {
            setupDeepLinkHandler()
        }
        .onChange(of: approvalStore.pendingApprovals) { _, _ in
            handlePendingDeepLink()
        }
    }

    private func setupDeepLinkHandler() {
        notificationService.onDeepLink = { deepLink in
            handleDeepLink(deepLink)
        }
    }

    private func handleDeepLink(_ deepLink: DeepLink) {
        switch deepLink {
        case .approval(let serverID, let approvalID):
            // Try to find the approval in our current list
            if let pending = approvalStore.pendingApprovals.first(where: {
                $0.approval.id == approvalID && $0.server.id.uuidString == serverID
            }) {
                selectedApproval = pending
            } else {
                // Store for later once approvals load
                pendingDeepLink = deepLink
                // Trigger a refresh to fetch latest approvals
                Task {
                    await approvalStore.refresh()
                }
            }

        case .run:
            // Run deep links would navigate to RunDetailView
            // For now, we just refresh approvals since runs may have pending approvals
            Task {
                await approvalStore.refresh()
            }
        }
    }

    private func handlePendingDeepLink() {
        guard let deepLink = pendingDeepLink else { return }

        switch deepLink {
        case .approval(let serverID, let approvalID):
            if let pending = approvalStore.pendingApprovals.first(where: {
                $0.approval.id == approvalID && $0.server.id.uuidString == serverID
            }) {
                selectedApproval = pending
                pendingDeepLink = nil
            }

        case .run:
            pendingDeepLink = nil
        }
    }
}
