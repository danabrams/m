import SwiftUI

/// Persistent banner showing pending approvals across all servers.
/// Appears below the navigation bar with subtle animation.
struct ApprovalBannerView: View {
    @ObservedObject var approvalStore: ApprovalStore
    @Binding var showingApprovalList: Bool
    @Binding var selectedApproval: PendingApproval?

    var body: some View {
        if !approvalStore.pendingApprovals.isEmpty {
            bannerContent
                .transition(.move(edge: .top).combined(with: .opacity))
                .animation(.easeInOut(duration: 0.25), value: approvalStore.pendingApprovals.isEmpty)
        }
    }

    private var bannerContent: some View {
        Button {
            handleTap()
        } label: {
            HStack(spacing: 8) {
                Image(systemName: "exclamationmark.circle.fill")
                    .foregroundStyle(.white)

                Text(bannerText)
                    .font(.subheadline)
                    .fontWeight(.medium)
                    .foregroundStyle(.white)

                Spacer()

                Image(systemName: "chevron.right")
                    .font(.caption)
                    .foregroundStyle(.white.opacity(0.7))
            }
            .padding(.horizontal, 16)
            .padding(.vertical, 12)
            .background(Color.orange)
        }
        .buttonStyle(.plain)
    }

    private var bannerText: String {
        let count = approvalStore.pendingApprovals.count
        if count == 1 {
            return "Approval needed"
        } else {
            return "\(count) approvals pending"
        }
    }

    private func handleTap() {
        let count = approvalStore.pendingApprovals.count
        if count == 1, let first = approvalStore.pendingApprovals.first {
            selectedApproval = first
        } else {
            showingApprovalList = true
        }
    }
}
