import SwiftUI

/// Sheet for responding to an agent's question.
struct InputPromptView: View {
    let apiClient: APIClientProtocol
    let runID: String
    let question: String
    let onSent: () -> Void

    @Environment(\.dismiss) private var dismiss
    @State private var response = ""
    @State private var isSending = false
    @State private var error: MError?

    /// Key for storing draft in UserDefaults.
    private var draftKey: String { "input_draft_\(runID)" }

    var body: some View {
        NavigationStack {
            VStack(spacing: 0) {
                // Agent's question
                ScrollView {
                    Text(question)
                        .font(.body)
                        .frame(maxWidth: .infinity, alignment: .leading)
                        .padding()
                        .accessibilityIdentifier("inputPrompt.questionLabel")
                }
                .frame(maxHeight: 200)
                .background(.fill.tertiary)

                Divider()

                // Response input
                VStack(spacing: 16) {
                    TextEditor(text: $response)
                        .font(.body)
                        .scrollContentBackground(.hidden)
                        .padding(12)
                        .background(.fill.tertiary)
                        .clipShape(RoundedRectangle(cornerRadius: 8))
                        .overlay(alignment: .topLeading) {
                            if response.isEmpty {
                                Text("Type your response...")
                                    .font(.body)
                                    .foregroundStyle(.tertiary)
                                    .padding(.horizontal, 16)
                                    .padding(.vertical, 20)
                                    .allowsHitTesting(false)
                            }
                        }
                        .frame(minHeight: 100)
                        .accessibilityIdentifier("inputPrompt.responseField")

                    HStack {
                        Spacer()
                        if isSending {
                            ProgressView()
                        } else {
                            Button("Send") {
                                sendInput()
                            }
                            .buttonStyle(.borderedProminent)
                            .disabled(response.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)
                            .accessibilityIdentifier("inputPrompt.sendButton")
                        }
                    }
                }
                .padding()
            }
            .navigationTitle("Agent Question")
            #if os(iOS)
            .navigationBarTitleDisplayMode(.inline)
            #endif
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") {
                        dismiss()
                    }
                    .disabled(isSending)
                    .accessibilityIdentifier("inputPrompt.cancelButton")
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
            .interactiveDismissDisabled(isSending)
            .onAppear {
                loadDraft()
            }
            .onChange(of: response) { _, newValue in
                saveDraft(newValue)
            }
        }
    }

    private func sendInput() {
        let trimmedResponse = response.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmedResponse.isEmpty else { return }

        isSending = true
        error = nil

        Task {
            do {
                try await apiClient.sendInput(runID: runID, text: trimmedResponse)
                await MainActor.run {
                    clearDraft()
                    onSent()
                    dismiss()
                }
            } catch let mError as MError {
                await MainActor.run {
                    error = mError
                    isSending = false
                }
            } catch {
                await MainActor.run {
                    self.error = .unknown(statusCode: 0, message: error.localizedDescription)
                    isSending = false
                }
            }
        }
    }

    // MARK: - Draft Persistence

    private func loadDraft() {
        if let draft = UserDefaults.standard.string(forKey: draftKey) {
            response = draft
        }
    }

    private func saveDraft(_ text: String) {
        if text.isEmpty {
            UserDefaults.standard.removeObject(forKey: draftKey)
        } else {
            UserDefaults.standard.set(text, forKey: draftKey)
        }
    }

    private func clearDraft() {
        UserDefaults.standard.removeObject(forKey: draftKey)
    }
}
