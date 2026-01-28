import SwiftUI

/// Sheet for entering a prompt to start a new run.
struct NewRunView: View {
    let apiClient: APIClientProtocol
    let repoID: String
    let onCreated: (Run) -> Void

    @Environment(\.dismiss) private var dismiss
    @State private var prompt = ""
    @State private var isCreating = false
    @State private var error: MError?

    var body: some View {
        NavigationStack {
            VStack(spacing: 16) {
                TextEditor(text: $prompt)
                    .font(.body)
                    .scrollContentBackground(.hidden)
                    .padding(12)
                    .background(.fill.tertiary)
                    .clipShape(RoundedRectangle(cornerRadius: 8))
                    .overlay(alignment: .topLeading) {
                        if prompt.isEmpty {
                            Text("What should the agent do?")
                                .font(.body)
                                .foregroundStyle(.tertiary)
                                .padding(.horizontal, 16)
                                .padding(.vertical, 20)
                                .allowsHitTesting(false)
                        }
                    }
                    .accessibilityIdentifier("newRun.promptField")

                Spacer()
            }
            .padding()
            .navigationTitle("New Run")
            #if os(iOS)
            .navigationBarTitleDisplayMode(.inline)
            #endif
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") {
                        dismiss()
                    }
                    .disabled(isCreating)
                    .accessibilityIdentifier("newRun.cancelButton")
                }

                ToolbarItem(placement: .confirmationAction) {
                    if isCreating {
                        ProgressView()
                    } else {
                        Button("Start") {
                            createRun()
                        }
                        .disabled(prompt.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)
                        .accessibilityIdentifier("newRun.startButton")
                    }
                }
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
            .interactiveDismissDisabled(isCreating)
        }
    }

    private func createRun() {
        let trimmedPrompt = prompt.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmedPrompt.isEmpty else { return }

        isCreating = true
        error = nil

        Task {
            do {
                let run = try await apiClient.createRun(repoID: repoID, prompt: trimmedPrompt)
                await MainActor.run {
                    onCreated(run)
                    dismiss()
                }
            } catch let mError as MError {
                await MainActor.run {
                    error = mError
                    isCreating = false
                }
            } catch {
                await MainActor.run {
                    self.error = .unknown(statusCode: 0, message: error.localizedDescription)
                    isCreating = false
                }
            }
        }
    }
}
