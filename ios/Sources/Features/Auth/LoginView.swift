import AuthenticationServices
import GoogleSignIn
import SwiftUI
#if os(macOS)
import AppKit
#endif

/// Sign-in screen: email/password, Sign in with Apple, and (when the brand ships
/// a Google client id) Continue with Google. Every path lands on the same
/// AuthResponse, which the session store adopts.
struct LoginView: View {
    @Environment(AppContainer.self) private var container
    @Environment(\.colorScheme) private var colorScheme
    private var config: AppConfig { .current }

    @State private var email = ""
    @State private var password = ""
    @State private var isSubmitting = false
    @State private var errorMessage: String?
    @State private var showingSignUp = false

    var body: some View {
        ScrollView {
            VStack(spacing: 22) {
                header.padding(.top, 44)
                VStack(spacing: 16) {
                    emailPasswordForm
                    orDivider
                    thirdPartyButtons
                }
                .padding(22)
                .background(Theme.card, in: .rect(cornerRadius: 20))
                .overlay(RoundedRectangle(cornerRadius: 20).strokeBorder(Theme.border))
                .shadow(color: .black.opacity(0.08), radius: 18, y: 8)
                signUpPrompt
            }
            .padding(20)
            .frame(maxWidth: 440)
            .frame(maxWidth: .infinity)
        }
        .scrollBounceBehavior(.basedOnSize)
        .background(alignment: .top) { heroBackground }
        .background(Theme.canvas.ignoresSafeArea())
        .sheet(isPresented: $showingSignUp) { RegisterView() }
    }

    /// The brand gradient behind the header, with faint rings echoing the mark.
    private var heroBackground: some View {
        LinearGradient(colors: [Theme.accent, Theme.accentDeep], startPoint: .topLeading, endPoint: .bottomTrailing)
            .overlay {
                ZStack {
                    ForEach([220.0, 340.0, 460.0], id: \.self) { size in
                        Circle().stroke(.white.opacity(0.08), lineWidth: 1).frame(width: size, height: size)
                    }
                }
                .offset(x: 120, y: -40)
            }
            .clipped()
            .frame(height: 400)
            .ignoresSafeArea()
    }

    private var signUpPrompt: some View {
        HStack(spacing: 4) {
            Text("New here?").foregroundStyle(Theme.muted)
            Button("Create an account") { showingSignUp = true }
                .fontWeight(.semibold)
                .disabled(isSubmitting)
        }
        .font(.subheadline)
    }

    private var header: some View {
        VStack(spacing: 12) {
            RoundedRectangle(cornerRadius: 18)
                .fill(.white.opacity(0.18))
                .frame(width: 68, height: 68)
                .overlay(RoundedRectangle(cornerRadius: 18).strokeBorder(.white.opacity(0.35)))
                .overlay(BrandMark().stroke(.white, style: StrokeStyle(lineWidth: 3, lineCap: .round)).padding(12))
                .accessibilityHidden(true)
            Text(config.brandDisplayName).font(.largeTitle.bold()).foregroundStyle(.white)
            Text("Sign in to your monitors").foregroundStyle(.white.opacity(0.85))
        }
    }

    private var orDivider: some View {
        HStack(spacing: 10) {
            Rectangle().fill(Theme.border).frame(height: 1)
            Text("or").font(.caption).foregroundStyle(Theme.muted)
            Rectangle().fill(Theme.border).frame(height: 1)
        }
    }

    private func field<F: View>(_ content: F) -> some View {
        content
            .padding(14)
            .background(Theme.canvas, in: .rect(cornerRadius: 10))
            .overlay(RoundedRectangle(cornerRadius: 10).strokeBorder(Theme.border))
    }

    private var emailPasswordForm: some View {
        VStack(spacing: 12) {
            field(
                TextField("Email", text: $email)
                    .textContentType(.emailAddress)
                    .keyboardType(.emailAddress)
                    .textInputAutocapitalization(.never)
                    .autocorrectionDisabled()
            )

            field(
                SecureField("Password", text: $password)
                    .textContentType(.password)
            )

            // The reset email links to the web app anyway, so the flow lives there.
            Link("Forgot password?", destination: config.apiBaseURL.appending(path: "forgot-password"))
                .font(.footnote.weight(.medium))
                .frame(maxWidth: .infinity, alignment: .trailing)

            if let errorMessage {
                Label(errorMessage, systemImage: "exclamationmark.circle.fill")
                    .font(.footnote)
                    .foregroundStyle(Theme.down)
                    .frame(maxWidth: .infinity, alignment: .leading)
            }

            Button {
                Task { await run { try await container.authService.login(email: email, password: password) } }
            } label: {
                Group {
                    if isSubmitting {
                        ProgressView().tint(.white)
                    } else {
                        Text("Sign in").fontWeight(.semibold)
                    }
                }
                .frame(maxWidth: .infinity, minHeight: 26)
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
                .signInWithAppleButtonStyle(colorScheme == .dark ? .white : .black)
                .frame(height: 46)
                .clipShape(.rect(cornerRadius: 10))

            if config.googleEnabled {
                Button {
                    Task { await signInWithGoogle() }
                } label: {
                    Label("Continue with Google", systemImage: "g.circle.fill")
                        .font(.body.weight(.medium))
                        .frame(maxWidth: .infinity, minHeight: 46)
                        .foregroundStyle(.primary)
                        .background(Theme.card, in: .rect(cornerRadius: 10))
                        .overlay(RoundedRectangle(cornerRadius: 10).strokeBorder(Theme.border))
                        .contentShape(.rect(cornerRadius: 10))
                }
                .buttonStyle(.plain)
            }
        }
        .disabled(isSubmitting)
    }

    // MARK: - Actions

    private func signInWithGoogle() async {
        #if os(iOS)
        guard let presenter = UIApplication.shared.topViewController else { return }
        #else
        guard let presenter = NSApp.keyWindow else { return }
        #endif
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
