import SwiftUI

/// Selection list for multiple pending approvals.
/// Shown as a sheet when user taps the banner with multiple approvals.
struct ApprovalListView: View {
    let approvals: [PendingApproval]
    let onSelect: (PendingApproval) -> Void
    @Environment(\.dismiss) private var dismiss

    var body: some View {
        NavigationStack {
            List {
                ForEach(approvals) { pending in
                    Button {
                        dismiss()
                        onSelect(pending)
                    } label: {
                        ApprovalRowView(pending: pending)
                    }
                    .buttonStyle(.plain)
                }
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

/// Row view for a single pending approval.
private struct ApprovalRowView: View {
    let pending: PendingApproval

    var body: some View {
        HStack(spacing: 12) {
            typeIcon
                .frame(width: 32, height: 32)

            VStack(alignment: .leading, spacing: 4) {
                Text(titleText)
                    .font(.subheadline)
                    .fontWeight(.medium)

                Text("\(pending.server.name) - \(pending.approval.createdAt, style: .relative)")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }

            Spacer()

            Image(systemName: "chevron.right")
                .font(.caption)
                .foregroundStyle(.secondary)
        }
        .padding(.vertical, 4)
    }

    @ViewBuilder
    private var typeIcon: some View {
        switch pending.approval.type {
        case .diff:
            Image(systemName: "doc.badge.plus")
                .foregroundStyle(.blue)
                .font(.title3)
        case .command:
            Image(systemName: "terminal")
                .foregroundStyle(.purple)
                .font(.title3)
        case .generic:
            Image(systemName: "questionmark.circle")
                .foregroundStyle(.orange)
                .font(.title3)
        }
    }

    private var titleText: String {
        switch pending.approval.type {
        case .diff:
            return "Apply changes"
        case .command:
            if let command = pending.approval.payload.command {
                return truncate(command, maxLength: 40)
            }
            return "Run command"
        case .generic:
            if let message = pending.approval.payload.message {
                return truncate(message, maxLength: 40)
            }
            return "Approval required"
        }
    }

    private func truncate(_ text: String, maxLength: Int) -> String {
        if text.count <= maxLength {
            return text
        }
        let endIndex = text.index(text.startIndex, offsetBy: maxLength - 3)
        return String(text[..<endIndex]) + "..."
    }
}
