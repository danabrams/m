import SwiftUI

/// Screen showing details of a single run.
/// Placeholder implementation - will be expanded with event feed, approvals, etc.
struct RunDetailView: View {
    let run: Run
    let apiClient: APIClient

    @State private var currentRun: Run
    @State private var isLoading = false
    @State private var error: MError?

    init(run: Run, apiClient: APIClient) {
        self.run = run
        self.apiClient = apiClient
        self._currentRun = State(initialValue: run)
    }

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 16) {
                statusSection
                promptSection
                timestampSection
            }
            .padding()
        }
        .navigationTitle("Run")
        #if os(iOS)
        .navigationBarTitleDisplayMode(.inline)
        #endif
        .toolbar {
            if currentRun.state == .running {
                ToolbarItem(placement: .primaryAction) {
                    Button("Cancel", role: .destructive) {
                        cancelRun()
                    }
                }
            }
        }
        .task {
            await refreshRun()
        }
        .refreshable {
            await refreshRun()
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
    }

    private var statusSection: some View {
        HStack(spacing: 12) {
            statusIcon
            VStack(alignment: .leading, spacing: 2) {
                Text(statusText)
                    .font(.headline)
                Text("Updated \(currentRun.updatedAt, style: .relative)")
                    .font(.caption)
                    .foregroundStyle(.secondary)
            }
            Spacer()
        }
        .padding()
        .background(.fill.tertiary)
        .clipShape(RoundedRectangle(cornerRadius: 8))
    }

    private var promptSection: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text("Prompt")
                .font(.subheadline)
                .foregroundStyle(.secondary)
            Text(currentRun.prompt)
                .font(.body)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding()
        .background(.fill.tertiary)
        .clipShape(RoundedRectangle(cornerRadius: 8))
    }

    private var timestampSection: some View {
        VStack(alignment: .leading, spacing: 8) {
            Text("Created")
                .font(.subheadline)
                .foregroundStyle(.secondary)
            Text(currentRun.createdAt, style: .date) +
            Text(" at ") +
            Text(currentRun.createdAt, style: .time)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding()
        .background(.fill.tertiary)
        .clipShape(RoundedRectangle(cornerRadius: 8))
    }

    @ViewBuilder
    private var statusIcon: some View {
        switch currentRun.state {
        case .running:
            ProgressView()
                .frame(width: 24, height: 24)
        case .waitingApproval, .waitingInput:
            Image(systemName: "clock.fill")
                .foregroundStyle(.orange)
                .font(.system(size: 20))
        case .completed:
            Image(systemName: "checkmark.circle.fill")
                .foregroundStyle(.green)
                .font(.system(size: 20))
        case .failed:
            Image(systemName: "xmark.circle.fill")
                .foregroundStyle(.red)
                .font(.system(size: 20))
        case .cancelled:
            Image(systemName: "minus.circle.fill")
                .foregroundStyle(.secondary)
                .font(.system(size: 20))
        }
    }

    private var statusText: String {
        switch currentRun.state {
        case .running:
            return "Running"
        case .waitingApproval:
            return "Waiting for Approval"
        case .waitingInput:
            return "Waiting for Input"
        case .completed:
            return "Completed"
        case .failed:
            return "Failed"
        case .cancelled:
            return "Cancelled"
        }
    }

    private func refreshRun() async {
        isLoading = true
        do {
            currentRun = try await apiClient.getRun(id: run.id)
        } catch let mError as MError {
            error = mError
        } catch {
            self.error = .unknown(statusCode: 0, message: error.localizedDescription)
        }
        isLoading = false
    }

    private func cancelRun() {
        Task {
            do {
                try await apiClient.cancelRun(id: run.id)
                await refreshRun()
            } catch let mError as MError {
                error = mError
            } catch {
                self.error = .unknown(statusCode: 0, message: error.localizedDescription)
            }
        }
    }
}
