import SwiftUI

/// Detail view for viewing and resolving an approval request.
struct ApprovalDetailView: View {
    let pending: PendingApproval
    let onResolved: () -> Void
    @Environment(\.dismiss) private var dismiss

    @State private var isResolving = false
    @State private var showRejectReason = false
    @State private var rejectReason = ""
    @State private var error: MError?

    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(alignment: .leading, spacing: 16) {
                    headerSection
                    contentSection
                }
                .padding()
            }
            .navigationTitle(titleText)
            #if os(iOS)
            .navigationBarTitleDisplayMode(.inline)
            #endif
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") {
                        dismiss()
                    }
                }
            }
            .safeAreaInset(edge: .bottom) {
                actionButtons
            }
            .alert("Error", isPresented: .constant(error != nil)) {
                Button("OK") {
                    error = nil
                }
            } message: {
                if let error {
                    Text(error.localizedDescription)
                }
            }
            .sheet(isPresented: $showRejectReason) {
                rejectReasonSheet
            }
        }
    }

    private var titleText: String {
        switch pending.approval.type {
        case .diff:
            return "Apply changes?"
        case .command:
            return "Allow this action?"
        case .generic:
            return "Approval requested"
        }
    }

    private var headerSection: some View {
        HStack(spacing: 12) {
            typeIcon
            VStack(alignment: .leading, spacing: 2) {
                Text(pending.approval.tool)
                    .font(.headline)
                Text(pending.server.name)
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
            Spacer()
        }
        .padding()
        .background(.fill.tertiary)
        .clipShape(RoundedRectangle(cornerRadius: 8))
    }

    @ViewBuilder
    private var typeIcon: some View {
        switch pending.approval.type {
        case .diff:
            Image(systemName: "doc.badge.plus")
                .foregroundStyle(.blue)
                .font(.title2)
        case .command:
            Image(systemName: "terminal")
                .foregroundStyle(.purple)
                .font(.title2)
        case .generic:
            Image(systemName: "questionmark.circle")
                .foregroundStyle(.orange)
                .font(.title2)
        }
    }

    @ViewBuilder
    private var contentSection: some View {
        switch pending.approval.type {
        case .diff:
            diffContent
        case .command:
            commandContent
        case .generic:
            genericContent
        }
    }

    @ViewBuilder
    private var diffContent: some View {
        if let files = pending.approval.payload.files, !files.isEmpty {
            VStack(alignment: .leading, spacing: 8) {
                Text("\(files.count) file\(files.count == 1 ? "" : "s")")
                    .font(.subheadline)
                    .foregroundStyle(.secondary)

                ForEach(files, id: \.path) { file in
                    DiffFileRow(file: file)
                }
            }
        } else if let diff = pending.approval.payload.diff {
            codeBlock(diff)
        }
    }

    @ViewBuilder
    private var commandContent: some View {
        if let command = pending.approval.payload.command {
            VStack(alignment: .leading, spacing: 8) {
                Text("Command")
                    .font(.subheadline)
                    .foregroundStyle(.secondary)
                codeBlock(command)
            }
        }

        if let message = pending.approval.payload.message {
            VStack(alignment: .leading, spacing: 8) {
                Text("Details")
                    .font(.subheadline)
                    .foregroundStyle(.secondary)
                Text(message)
                    .font(.body)
            }
        }
    }

    @ViewBuilder
    private var genericContent: some View {
        if let message = pending.approval.payload.message {
            Text(message)
                .font(.body)
                .frame(maxWidth: .infinity, alignment: .leading)
                .padding()
                .background(.fill.tertiary)
                .clipShape(RoundedRectangle(cornerRadius: 8))
        }
    }

    private func codeBlock(_ code: String) -> some View {
        ScrollView(.horizontal, showsIndicators: false) {
            Text(code)
                .font(.system(.caption, design: .monospaced))
                .frame(maxWidth: .infinity, alignment: .leading)
        }
        .padding()
        .background(.fill.quaternary)
        .clipShape(RoundedRectangle(cornerRadius: 8))
    }

    private var actionButtons: some View {
        HStack(spacing: 16) {
            Button {
                showRejectReason = true
            } label: {
                Text("Reject")
                    .frame(maxWidth: .infinity)
            }
            .buttonStyle(.bordered)
            .disabled(isResolving)

            Button {
                Task {
                    await resolve(approved: true)
                }
            } label: {
                if isResolving {
                    ProgressView()
                        .frame(maxWidth: .infinity)
                } else {
                    Text("Approve")
                        .frame(maxWidth: .infinity)
                }
            }
            .buttonStyle(.borderedProminent)
            .disabled(isResolving)
        }
        .padding()
        .background(.bar)
    }

    private var rejectReasonSheet: some View {
        NavigationStack {
            VStack(spacing: 16) {
                Text("Reason (optional)")
                    .font(.headline)

                TextField("Why are you rejecting this?", text: $rejectReason, axis: .vertical)
                    .textFieldStyle(.roundedBorder)
                    .lineLimit(3...6)

                Spacer()
            }
            .padding()
            .navigationTitle("Reject")
            #if os(iOS)
            .navigationBarTitleDisplayMode(.inline)
            #endif
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") {
                        showRejectReason = false
                    }
                }
                ToolbarItem(placement: .confirmationAction) {
                    Button("Reject") {
                        showRejectReason = false
                        Task {
                            await resolve(approved: false, reason: rejectReason.isEmpty ? nil : rejectReason)
                        }
                    }
                    .foregroundStyle(.red)
                }
            }
        }
        .presentationDetents([.medium])
    }

    private func resolve(approved: Bool, reason: String? = nil) async {
        isResolving = true
        do {
            try await pending.apiClient.resolveApproval(
                id: pending.approval.id,
                approved: approved,
                reason: reason
            )
            ApprovalStore.shared.removeApproval(id: pending.approval.id)
            onResolved()
            dismiss()
        } catch let mError as MError {
            error = mError
        } catch {
            self.error = .unknown(statusCode: 0, message: error.localizedDescription)
        }
        isResolving = false
    }
}

/// Row view for a diff file in the approval detail.
private struct DiffFileRow: View {
    let file: DiffFile
    @State private var isExpanded = false

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            Button {
                withAnimation(.easeInOut(duration: 0.2)) {
                    isExpanded.toggle()
                }
            } label: {
                HStack {
                    Image(systemName: isExpanded ? "chevron.down" : "chevron.right")
                        .font(.caption)
                        .foregroundStyle(.secondary)

                    Text(file.path)
                        .font(.system(.caption, design: .monospaced))
                        .lineLimit(1)

                    Spacer()

                    Text("+\(file.additions)/-\(file.deletions)")
                        .font(.caption)
                        .foregroundStyle(.secondary)
                }
                .padding(.vertical, 8)
                .padding(.horizontal, 12)
                .background(.fill.tertiary)
            }
            .buttonStyle(.plain)

            if isExpanded {
                ScrollView(.horizontal, showsIndicators: false) {
                    Text(file.content)
                        .font(.system(.caption2, design: .monospaced))
                        .frame(maxWidth: .infinity, alignment: .leading)
                }
                .padding(12)
                .background(.fill.quaternary)
            }
        }
        .clipShape(RoundedRectangle(cornerRadius: 8))
    }
}
