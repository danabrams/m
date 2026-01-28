import SwiftUI

/// Sheet for adding a new M server configuration.
struct AddServerView: View {
    @Environment(\.dismiss) private var dismiss
    @ObservedObject var store: ServerStore

    @State private var name = ""
    @State private var urlString = ""
    @State private var apiKey = ""
    @State private var error: String?

    private var isValid: Bool {
        !name.trimmingCharacters(in: .whitespaces).isEmpty &&
        !urlString.trimmingCharacters(in: .whitespaces).isEmpty &&
        !apiKey.trimmingCharacters(in: .whitespaces).isEmpty &&
        URL(string: urlString) != nil
    }

    var body: some View {
        NavigationStack {
            Form {
                Section {
                    TextField("Name", text: $name)
                        .textContentType(.organizationName)
                        .autocorrectionDisabled()
                        .accessibilityIdentifier("addServer.nameField")

                    TextField("URL", text: $urlString)
                        .textContentType(.URL)
                        #if os(iOS)
                        .keyboardType(.URL)
                        .textInputAutocapitalization(.never)
                        #endif
                        .autocorrectionDisabled()
                        .accessibilityIdentifier("addServer.urlField")

                    SecureField("API Key", text: $apiKey)
                        .textContentType(.password)
                        #if os(iOS)
                        .textInputAutocapitalization(.never)
                        #endif
                        .autocorrectionDisabled()
                        .accessibilityIdentifier("addServer.apiKeyField")
                }

                if let error {
                    Section {
                        Text(error)
                            .foregroundStyle(.red)
                            .font(.footnote)
                    }
                }
            }
            .navigationTitle("Add Server")
            #if os(iOS)
            .navigationBarTitleDisplayMode(.inline)
            #endif
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") {
                        dismiss()
                    }
                    .accessibilityIdentifier("addServer.cancelButton")
                }
                ToolbarItem(placement: .confirmationAction) {
                    Button("Save") {
                        save()
                    }
                    .disabled(!isValid)
                    .accessibilityIdentifier("addServer.saveButton")
                }
            }
        }
    }

    private func save() {
        guard let url = URL(string: urlString) else {
            error = "Invalid URL"
            return
        }

        do {
            try store.addServer(
                name: name.trimmingCharacters(in: .whitespaces),
                url: url,
                apiKey: apiKey
            )
            dismiss()
        } catch {
            self.error = error.localizedDescription
        }
    }
}
