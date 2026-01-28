import SwiftUI

/// Global banner shown when approvals are pending.
/// Appears below navigation bar, fixed position.
struct ApprovalBannerView: View {
    @ObservedObject var store: ApprovalStore
    let onTapSingle: (PendingApproval) -> Void
    let onTapMultiple: () -> Void

    var body: some View {
        if store.hasPendingApprovals {
            Button {
                if store.count == 1, let approval = store.pendingApprovals.first {
                    onTapSingle(approval)
                } else {
                    onTapMultiple()
                }
            } label: {
                HStack(spacing: 8) {
                    Image(systemName: "exclamationmark.circle.fill")
                        .foregroundStyle(.orange)

                    Text(bannerText)
                        .font(.subheadline)
                        .fontWeight(.medium)

                    Spacer()

                    Image(systemName: "chevron.right")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
                .padding(.horizontal, 16)
                .padding(.vertical, 12)
                .background(.yellow.opacity(0.15))
                .overlay(
                    Rectangle()
                        .frame(height: 1)
                        .foregroundStyle(.yellow.opacity(0.3)),
                    alignment: .bottom
                )
            }
            .buttonStyle(.plain)
            .transition(.move(edge: .top).combined(with: .opacity))
        }
    }

    private var bannerText: String {
        if store.count == 1 {
            return "Approval needed"
        } else {
            return "\(store.count) approvals pending"
        }
    }
}

/// Container view that wraps content with the approval banner.
struct ApprovalBannerContainer<Content: View>: View {
    @StateObject private var approvalStore = ApprovalStore.shared
    @State private var showingApprovalList = false
    @State private var selectedApproval: PendingApproval?

    let content: Content

    init(@ViewBuilder content: () -> Content) {
        self.content = content()
    }

    var body: some View {
        VStack(spacing: 0) {
            ApprovalBannerView(
                store: approvalStore,
                onTapSingle: { approval in
                    selectedApproval = approval
                },
                onTapMultiple: {
                    showingApprovalList = true
                }
            )
            .animation(.easeInOut(duration: 0.25), value: approvalStore.hasPendingApprovals)

            content
        }
        .sheet(isPresented: $showingApprovalList) {
            ApprovalListView(
                approvals: approvalStore.pendingApprovals,
                onSelect: { approval in
                    showingApprovalList = false
                    selectedApproval = approval
                }
            )
        }
        .sheet(item: $selectedApproval) { approval in
            // TODO: Navigate to ApprovalDetailView when implemented
            ApprovalPlaceholderView(approval: approval)
        }
    }
}

/// List view for selecting from multiple pending approvals.
struct ApprovalListView: View {
    let approvals: [PendingApproval]
    let onSelect: (PendingApproval) -> Void
    @Environment(\.dismiss) private var dismiss

    var body: some View {
        NavigationStack {
            List(approvals) { approval in
                Button {
                    onSelect(approval)
                } label: {
                    ApprovalRowView(approval: approval)
                }
                .buttonStyle(.plain)
            }
            .listStyle(.plain)
            .navigationTitle("Pending Approvals")
            #if os(iOS)
            .navigationBarTitleDisplayMode(.inline)
            #endif
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Done") {
                        dismiss()
                    }
                }
            }
        }
    }
}

/// Row view for an approval in the list.
struct ApprovalRowView: View {
    let approval: PendingApproval

    var body: some View {
        HStack(spacing: 12) {
            approvalIcon
                .frame(width: 32, height: 32)
                .background(.orange.opacity(0.15))
                .clipShape(Circle())

            VStack(alignment: .leading, spacing: 2) {
                Text(approvalTitle)
                    .font(.body)
                    .fontWeight(.medium)

                Text(approval.approval.tool)
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }

            Spacer()

            Text(approval.approval.createdAt, style: .relative)
                .font(.caption)
                .foregroundStyle(.secondary)
        }
        .padding(.vertical, 4)
    }

    @ViewBuilder
    private var approvalIcon: some View {
        switch approval.approval.type {
        case .diff:
            Image(systemName: "doc.text.fill")
                .foregroundStyle(.orange)
        case .command:
            Image(systemName: "terminal.fill")
                .foregroundStyle(.orange)
        case .generic:
            Image(systemName: "questionmark.circle.fill")
                .foregroundStyle(.orange)
        }
    }

    private var approvalTitle: String {
        switch approval.approval.type {
        case .diff:
            return "Apply changes"
        case .command:
            return "Run command"
        case .generic:
            return "Action requested"
        }
    }
}

/// Placeholder view until ApprovalDetailView is implemented.
private struct ApprovalPlaceholderView: View {
    let approval: PendingApproval
    @Environment(\.dismiss) private var dismiss

    var body: some View {
        NavigationStack {
            VStack(spacing: 16) {
                Image(systemName: "clock.fill")
                    .font(.largeTitle)
                    .foregroundStyle(.orange)

                Text("Approval Detail")
                    .font(.headline)

                Text("Tool: \(approval.approval.tool)")
                    .foregroundStyle(.secondary)

                if let message = approval.approval.payload.message {
                    Text(message)
                        .font(.body)
                        .padding()
                        .background(.fill.tertiary)
                        .clipShape(RoundedRectangle(cornerRadius: 8))
                }
            }
            .padding()
            .navigationTitle("Approval")
            #if os(iOS)
            .navigationBarTitleDisplayMode(.inline)
            #endif
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Close") {
                        dismiss()
                    }
                }
            }
        }
    }
}
