import SwiftUI

/// Sheet for viewing and resolving an approval request.
struct ApprovalView: View {
    let approval: Approval
    let apiClient: APIClientProtocol
    let onResolved: () -> Void

    @Environment(\.dismiss) private var dismiss
    @State private var isResolving = false
    @State private var error: MError?
    @State private var expandedFiles: Set<String> = []

    var body: some View {
        NavigationStack {
            VStack(spacing: 0) {
                // Content based on approval type
                ScrollView {
                    VStack(alignment: .leading, spacing: 16) {
                        headerSection
                        contentSection
                    }
                    .padding()
                }
                .accessibilityIdentifier("approval.diffView")

                Divider()

                // Action buttons
                actionButtons
            }
            .navigationTitle(navigationTitle)
            #if os(iOS)
            .navigationBarTitleDisplayMode(.inline)
            #endif
            .alert("Error", isPresented: .constant(error != nil)) {
                Button("OK") {
                    error = nil
                }
            } message: {
                if let error {
                    Text(error.localizedDescription)
                }
            }
        }
    }

    private var navigationTitle: String {
        switch approval.type {
        case .diff:
            return "Apply changes?"
        case .command:
            return "Allow this action?"
        case .generic:
            return "Approve request?"
        }
    }

    private var headerSection: some View {
        VStack(alignment: .leading, spacing: 8) {
            if let message = approval.payload.message {
                Text(message)
                    .font(.body)
            }

            if let files = approval.payload.files, !files.isEmpty {
                let totalAdditions = files.reduce(0) { $0 + $1.additions }
                let totalDeletions = files.reduce(0) { $0 + $1.deletions }

                HStack {
                    Text("\(files.count) file\(files.count == 1 ? "" : "s")")
                    Text("·")
                        .foregroundStyle(.secondary)
                    Text("+\(totalAdditions)")
                        .foregroundStyle(.green)
                    Text("/")
                        .foregroundStyle(.secondary)
                    Text("-\(totalDeletions)")
                        .foregroundStyle(.red)
                }
                .font(.subheadline)
                .foregroundStyle(.secondary)
            }
        }
    }

    @ViewBuilder
    private var contentSection: some View {
        switch approval.type {
        case .diff:
            diffContent
        case .command:
            commandContent
        case .generic:
            genericContent
        }
    }

    private var diffContent: some View {
        VStack(alignment: .leading, spacing: 8) {
            if let files = approval.payload.files {
                ForEach(files, id: \.path) { file in
                    fileSection(file)
                }
            } else if let diff = approval.payload.diff {
                // Fallback to raw diff
                Text(diff)
                    .font(.system(.body, design: .monospaced))
                    .padding()
                    .background(.fill.tertiary)
                    .clipShape(RoundedRectangle(cornerRadius: 8))
            }
        }
    }

    private func fileSection(_ file: DiffFile) -> some View {
        VStack(alignment: .leading, spacing: 0) {
            // File header (expandable)
            Button {
                toggleFile(file.path)
            } label: {
                HStack {
                    Image(systemName: expandedFiles.contains(file.path) ? "chevron.down" : "chevron.right")
                        .font(.caption)
                        .foregroundStyle(.secondary)

                    Text(file.path)
                        .font(.system(.body, design: .monospaced))
                        .foregroundStyle(.primary)

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

            // File content (when expanded)
            if expandedFiles.contains(file.path) {
                Text(file.content)
                    .font(.system(.caption, design: .monospaced))
                    .padding()
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .background(.fill.quaternary)
            }
        }
        .clipShape(RoundedRectangle(cornerRadius: 8))
        .overlay(
            RoundedRectangle(cornerRadius: 8)
                .stroke(.separator, lineWidth: 1)
        )
    }

    private func toggleFile(_ path: String) {
        if expandedFiles.contains(path) {
            expandedFiles.remove(path)
        } else {
            expandedFiles.insert(path)
        }
    }

    private var commandContent: some View {
        VStack(alignment: .leading, spacing: 12) {
            if let command = approval.payload.command {
                Text(command)
                    .font(.system(.body, design: .monospaced))
                    .padding()
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .background(.fill.tertiary)
                    .clipShape(RoundedRectangle(cornerRadius: 8))
            }

            if let message = approval.payload.message {
                Text(message)
                    .font(.subheadline)
                    .foregroundStyle(.secondary)
            }
        }
    }

    private var genericContent: some View {
        VStack(alignment: .leading, spacing: 12) {
            if let message = approval.payload.message {
                Text(message)
                    .font(.body)
            }
        }
    }

    private var actionButtons: some View {
        HStack(spacing: 16) {
            Button(role: .destructive) {
                resolveApproval(approved: false)
            } label: {
                Text("Reject")
                    .frame(maxWidth: .infinity)
            }
            .buttonStyle(.bordered)
            .disabled(isResolving)
            .accessibilityIdentifier("approval.rejectButton")

            Button {
                resolveApproval(approved: true)
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
            .accessibilityIdentifier("approval.approveButton")
        }
        .padding()
    }

    private func resolveApproval(approved: Bool) {
        isResolving = true
        error = nil

        Task {
            do {
                try await apiClient.resolveApproval(id: approval.id, approved: approved, reason: nil)
                await MainActor.run {
                    onResolved()
                    dismiss()
                }
            } catch let mError as MError {
                await MainActor.run {
                    error = mError
                    isResolving = false
                }
            } catch {
                await MainActor.run {
                    self.error = .unknown(statusCode: 0, message: error.localizedDescription)
                    isResolving = false
                }
            }
        }
    }
}
