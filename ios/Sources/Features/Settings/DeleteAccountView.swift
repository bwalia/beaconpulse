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
            FormScreen {
                VStack(alignment: .leading, spacing: 10) {
                    Label("This can’t be undone", systemImage: "exclamationmark.triangle.fill")
                        .font(.headline)
                        .foregroundStyle(Theme.down)
                    Text("This permanently deletes your organization and everything in it — monitors, alert history, status page, notification channels, API keys and your login. Any active subscription is cancelled immediately.")
                        .font(.callout)
                        .fixedSize(horizontal: false, vertical: true)
                }
                .padding(16)
                .frame(maxWidth: .infinity, alignment: .leading)
                .background(Theme.down.opacity(0.06), in: .rect(cornerRadius: 14))
                .overlay(RoundedRectangle(cornerRadius: 14).strokeBorder(Theme.down.opacity(0.3)))

                FormCard {
                    Field("Type your email to confirm") {
                        TextField(email, text: $confirmation)
                            .textInputAutocapitalization(.never)
                            .autocorrectionDisabled()
                            .keyboardType(.emailAddress)
                            .accessibilityLabel("Type your email to confirm")
                            .inputStyle()
                    }
                    Button(role: .destructive) {
                        Task { await deleteAccount() }
                    } label: {
                        Group {
                            if isDeleting { ProgressView().tint(.white) }
                            else { Text("Delete my account permanently") }
                        }
                        .font(.body.weight(.semibold))
                        .foregroundStyle(.white)
                        .frame(maxWidth: .infinity)
                        .padding(.vertical, 12)
                        .background(Theme.down.opacity(matches ? 1 : 0.4), in: .rect(cornerRadius: 10))
                    }
                    .buttonStyle(.plain)
                    .disabled(!matches || isDeleting)
                }

                if let errorMessage { ErrorBanner(errorMessage) }
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
