import SwiftUI
import UIKit

/// Email/password signup: creates an organization and its owner, then lands on
/// the same session as login. Presented as a sheet from the sign-in screen. On
/// success the session is adopted, which swaps the root view to the app and tears
/// this sheet down — so there's no explicit dismiss on the happy path.
struct RegisterView: View {
    @Environment(AppContainer.self) private var container
    @Environment(\.dismiss) private var dismiss
    private var config: AppConfig { .current }

    @State private var orgName = ""
    @State private var name = ""
    @State private var email = ""
    @State private var password = ""
    @State private var isSubmitting = false
    @State private var errorMessage: String?

    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(spacing: 14) {
                    Text("Create your \(config.brandDisplayName) account")
                        .font(.title3.weight(.semibold))
                        .multilineTextAlignment(.center)
                        .padding(.bottom, 4)

                    field("Organization name", text: $orgName, textContentType: .organizationName)
                    field("Your name", text: $name, textContentType: .name)
                    field("Email", text: $email, textContentType: .emailAddress,
                          keyboard: .emailAddress, lowercase: true)

                    SecureField("Password (at least 8 characters)", text: $password)
                        .textContentType(.newPassword)
                        .padding()
                        .background(.thinMaterial, in: .rect(cornerRadius: 10))

                    if let errorMessage {
                        Text(errorMessage)
                            .font(.footnote)
                            .foregroundStyle(.red)
                            .frame(maxWidth: .infinity, alignment: .leading)
                    }

                    Button {
                        Task { await submit() }
                    } label: {
                        if isSubmitting {
                            ProgressView().frame(maxWidth: .infinity)
                        } else {
                            Text("Create Account").frame(maxWidth: .infinity)
                        }
                    }
                    .buttonStyle(.borderedProminent)
                    .controlSize(.large)
                    .disabled(!canSubmit)
                    .padding(.top, 4)
                }
                .padding(24)
            }
            .navigationTitle("Sign Up")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") { dismiss() }
                }
            }
        }
    }

    private func field(_ title: String, text: Binding<String>,
                       textContentType: UITextContentType,
                       keyboard: UIKeyboardType = .default,
                       lowercase: Bool = false) -> some View {
        TextField(title, text: text)
            .textContentType(textContentType)
            .keyboardType(keyboard)
            .textInputAutocapitalization(lowercase ? .never : .words)
            .autocorrectionDisabled(lowercase)
            .padding()
            .background(.thinMaterial, in: .rect(cornerRadius: 10))
    }

    private var canSubmit: Bool {
        !isSubmitting
            && !orgName.trimmed.isEmpty
            && !name.trimmed.isEmpty
            && !email.trimmed.isEmpty
            && password.count >= 8
    }

    private func submit() async {
        isSubmitting = true
        errorMessage = nil
        defer { isSubmitting = false }
        do {
            let response = try await container.authService.register(
                orgName: orgName.trimmed, name: name.trimmed,
                email: email.trimmed, password: password)
            container.session.adopt(response)
        } catch {
            errorMessage = (error as? APIError)?.errorDescription ?? "Couldn’t create your account."
        }
    }
}
