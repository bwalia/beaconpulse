import Foundation

/// The unauthenticated auth calls: sign in (three ways), refresh, sign out. Uses
/// a plain APIClient with no bearer — these endpoints carry their credentials in
/// the request body, and refresh must work precisely when the access token is
/// already invalid.
struct AuthService {
    let client: APIClient

    func login(email: String, password: String) async throws -> AuthResponse {
        try await client.send(
            .init(method: "POST", path: "/api/v1/auth/login",
                  body: LoginRequest(email: email, password: password), authenticated: false),
            as: AuthResponse.self)
    }

    /// Creates an organization + owner and signs the new user in.
    func register(orgName: String, name: String, email: String, password: String) async throws -> AuthResponse {
        try await client.send(
            .init(method: "POST", path: "/api/v1/auth/register",
                  body: RegisterRequest(orgName: orgName, name: name, email: email, password: password),
                  authenticated: false),
            as: AuthResponse.self)
    }

    func signInWithGoogle(idToken: String) async throws -> AuthResponse {
        try await client.send(
            .init(method: "POST", path: "/api/v1/auth/google",
                  body: GoogleSignInRequest(idToken: idToken), authenticated: false),
            as: AuthResponse.self)
    }

    /// Requires the backend endpoint POST /api/v1/auth/apple (verifies Apple's
    /// identity token). See the iOS README — it is the one paired backend task
    /// for Phase 1, mirroring the existing Google verifier.
    func signInWithApple(identityToken: String) async throws -> AuthResponse {
        try await client.send(
            .init(method: "POST", path: "/api/v1/auth/apple",
                  body: AppleSignInRequest(identityToken: identityToken), authenticated: false),
            as: AuthResponse.self)
    }

    func refresh(refreshToken: String) async throws -> AuthResponse {
        try await client.send(
            .init(method: "POST", path: "/api/v1/auth/refresh",
                  body: RefreshRequest(refreshToken: refreshToken), authenticated: false),
            as: AuthResponse.self)
    }

    func logout(refreshToken: String) async throws {
        try await client.send(
            .init(method: "POST", path: "/api/v1/auth/logout",
                  body: LogoutRequest(refreshToken: refreshToken), authenticated: false))
    }
}
