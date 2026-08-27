import Foundation
import Observation

/// What we persist between launches: the token pair plus the cached user.
private struct StoredSession: Codable {
    var accessToken: String
    var refreshToken: String
    var user: AuthUser
}

/// The single source of truth for authentication state. Owns the tokens (in the
/// Keychain), exposes the current user to the UI, and is the `AuthProviding`
/// implementation the API client refreshes through. `@MainActor` because the UI
/// observes it directly.
@MainActor
@Observable
final class SessionStore: AuthProviding {
    private(set) var user: AuthUser?
    var isAuthenticated: Bool { user != nil }

    @ObservationIgnored private var tokens: (access: String, refresh: String)?
    @ObservationIgnored private let keychain: KeychainStore
    @ObservationIgnored private let auth: AuthService
    @ObservationIgnored private var refreshTask: Task<Bool, Never>?

    init(auth: AuthService, keychain: KeychainStore = KeychainStore()) {
        self.auth = auth
        self.keychain = keychain
        restore()
    }

    // MARK: - Session lifecycle

    /// Adopts a freshly issued session (sign-in or refresh).
    func adopt(_ response: AuthResponse) {
        tokens = (response.accessToken, response.refreshToken)
        user = response.user
        persist()
    }

    /// Signs out: best-effort revoke on the server, then wipe locally regardless.
    func signOut() async {
        if let refresh = tokens?.refresh {
            try? await auth.logout(refreshToken: refresh)
        }
        clearLocal()
    }

    private func clearLocal() {
        tokens = nil
        user = nil
        keychain.clear()
    }

    private func persist() {
        guard let tokens, let user else { return }
        let stored = StoredSession(accessToken: tokens.access, refreshToken: tokens.refresh, user: user)
        if let data = try? JSONEncoder().encode(stored) { keychain.save(data) }
    }

    private func restore() {
        guard let data = keychain.load(),
              let stored = try? JSONDecoder().decode(StoredSession.self, from: data) else { return }
        tokens = (stored.accessToken, stored.refreshToken)
        user = stored.user
    }

    // MARK: - AuthProviding

    func currentAccessToken() async -> String? { tokens?.access }

    /// Refreshes the access token, coalescing concurrent callers so a burst of
    /// 401s triggers exactly one refresh round-trip.
    func refreshAccessToken() async -> Bool {
        if let task = refreshTask { return await task.value }
        guard let refresh = tokens?.refresh else { return false }

        let task = Task { () -> Bool in
            defer { refreshTask = nil }
            do {
                adopt(try await auth.refresh(refreshToken: refresh))
                return true
            } catch {
                return false
            }
        }
        refreshTask = task
        return await task.value
    }

    func handleAuthenticationLost() async {
        clearLocal()
    }
}
