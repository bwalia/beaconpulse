import AuthenticationServices
import GoogleSignIn
import SwiftUI

/// Sign-in screen: email/password, Sign in with Apple, and (when the brand ships
/// a Google client id) Continue with Google. Every path lands on the same
/// AuthResponse, which the session store adopts.
struct LoginView: View {
    @Environment(AppContainer.self) private var container
    private var config: AppConfig { .current }

    @State private var email = ""
    @State private var password = ""
    @State private var isSubmitting = false
    @State private var errorMessage: String?
    @State private var showingSignUp = false

    var body: some View {
        VStack(spacing: 24) {
            Spacer()
            header
            emailPasswordForm
            thirdPartyButtons
            signUpPrompt
            Spacer()
        }
        .padding(24)
        .sheet(isPresented: $showingSignUp) { RegisterView() }
    }

    private var signUpPrompt: some View {
        Button("New here? Create an account") { showingSignUp = true }
            .font(.subheadline)
            .disabled(isSubmitting)
    }

    private var header: some View {
        VStack(spacing: 8) {
            Image(systemName: "dot.radiowaves.left.and.right")
                .font(.system(size: 44))
                .foregroundStyle(config.accentColor)
            Text(config.brandDisplayName).font(.largeTitle.bold())
            Text("Sign in to your monitors").foregroundStyle(.secondary)
        }
    }

    private var emailPasswordForm: some View {
        VStack(spacing: 12) {
            TextField("Email", text: $email)
                .textContentType(.emailAddress)
                .keyboardType(.emailAddress)
                .textInputAutocapitalization(.never)
                .autocorrectionDisabled()
                .padding()
                .background(.thinMaterial, in: .rect(cornerRadius: 10))

            SecureField("Password", text: $password)
                .textContentType(.password)
                .padding()
                .background(.thinMaterial, in: .rect(cornerRadius: 10))

            if let errorMessage {
                Text(errorMessage)
                    .font(.footnote)
                    .foregroundStyle(.red)
                    .frame(maxWidth: .infinity, alignment: .leading)
            }

            Button {
                Task { await run { try await container.authService.login(email: email, password: password) } }
            } label: {
                if isSubmitting {
                    ProgressView().frame(maxWidth: .infinity)
                } else {
                    Text("Sign In").frame(maxWidth: .infinity)
                }
            }
            .buttonStyle(.borderedProminent)
            .controlSize(.large)
            .disabled(isSubmitting || email.isEmpty || password.isEmpty)
        }
    }

    private var thirdPartyButtons: some View {
        VStack(spacing: 12) {
            SignInWithAppleButton(.signIn,
                                  onRequest: { $0.requestedScopes = [.fullName, .email] },
                                  onCompletion: handleApple)
                .signInWithAppleButtonStyle(.black)
                .frame(height: 48)
                .clipShape(.rect(cornerRadius: 10))

            if config.googleEnabled {
                Button {
                    Task { await signInWithGoogle() }
                } label: {
                    Label("Continue with Google", systemImage: "g.circle.fill")
                        .frame(maxWidth: .infinity)
                }
                .buttonStyle(.bordered)
                .controlSize(.large)
            }
        }
        .disabled(isSubmitting)
    }

    // MARK: - Actions

    private func signInWithGoogle() async {
        guard let presenter = UIApplication.shared.topViewController else { return }
        do {
            let result = try await GIDSignIn.sharedInstance.signIn(withPresenting: presenter)
            guard let idToken = result.user.idToken?.tokenString else {
                errorMessage = "Google sign-in did not return a token."
                return
            }
            await run { try await container.authService.signInWithGoogle(idToken: idToken) }
        } catch {
            // A user-cancelled flow is not worth surfacing as an error.
        }
    }

    private func handleApple(_ result: Result<ASAuthorization, Error>) {
        guard case let .success(authorization) = result,
              let credential = authorization.credential as? ASAuthorizationAppleIDCredential,
              let tokenData = credential.identityToken,
              let identityToken = String(data: tokenData, encoding: .utf8) else { return }
        Task { await run { try await container.authService.signInWithApple(identityToken: identityToken) } }
    }

    /// Runs an auth call, adopting the session on success and surfacing a message
    /// on failure.
    private func run(_ operation: () async throws -> AuthResponse) async {
        isSubmitting = true
        errorMessage = nil
        defer { isSubmitting = false }
        do {
            container.session.adopt(try await operation())
        } catch {
            errorMessage = (error as? APIError)?.errorDescription ?? "Sign in failed. Please try again."
        }
    }
}
