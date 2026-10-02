import SwiftUI

/// Confirms and performs permanent account deletion. The person types their email
/// to confirm — the same safeguard as the web app — and on success is signed out.
struct DeleteAccountView: View {
    @Environment(AppContainer.self) private var container
    @Environment(\.dismiss) private var dismiss

    let email: String

    @State private var confirmation = ""
    @State private var isDeleting = false
    @State private var errorMessage: String?

    private var matches: Bool {
        confirmation.trimmingCharacters(in: .whitespaces).caseInsensitiveCompare(email) == .orderedSame
    }

    var body: some View {
        NavigationStack {
            Form {
                Section {
                    Text("This permanently deletes your organization and everything in it — monitors, alert history, status page, notification channels, API keys and your login. Any active subscription is cancelled immediately. This can't be undone.")
                        .font(.callout)
                }
                Section {
                    TextField(email, text: $confirmation)
                        .textInputAutocapitalization(.never)
                        .autocorrectionDisabled()
                        .keyboardType(.emailAddress)
                        .accessibilityLabel("Type your email to confirm")
                } header: {
                    Text("Type your email to confirm")
                } footer: {
                    if let errorMessage {
                        Text(errorMessage).foregroundStyle(.red)
                    }
                }
                Section {
                    Button(role: .destructive) {
                        Task { await deleteAccount() }
                    } label: {
                        if isDeleting {
                            ProgressView()
                        } else {
                            Text("Delete My Account Permanently")
                        }
                    }
                    .disabled(!matches || isDeleting)
                }
            }
            .navigationTitle("Delete Account")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") { dismiss() }.disabled(isDeleting)
                }
            }
            .interactiveDismissDisabled(isDeleting)
        }
    }

    private func deleteAccount() async {
        isDeleting = true
        errorMessage = nil
        do {
            try await container.account.deleteAccount(confirmEmail: confirmation.trimmingCharacters(in: .whitespaces))
            // The account and its device tokens are gone server-side; just clear local state.
            await container.session.signOut()
        } catch {
            errorMessage = (error as? LocalizedError)?.errorDescription ?? "Something went wrong. Nothing was deleted — try again."
            isDeleting = false
        }
    }
}
