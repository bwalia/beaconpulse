import Foundation

// Response models. Keys arrive snake_case and are mapped by the decoder's
// convertFromSnakeCase strategy, so properties stay camelCase with no CodingKeys.

/// The shared auth response for login / google / apple / refresh.
struct AuthResponse: Decodable {
    let accessToken: String
    let refreshToken: String
    let tokenType: String
    let expiresIn: Int
    let user: AuthUser
}

/// The authenticated user. Codable so it can also be cached in the Keychain
/// alongside the tokens.
struct AuthUser: Codable, Identifiable, Equatable {
    let id: String
    let orgId: String
    let email: String
    let name: String
    let role: String
    let isPlatformAdmin: Bool?
}

/// The standard paginated collection envelope: `{ "data": [...], "pagination": {...} }`.
struct Paginated<T: Decodable>: Decodable {
    let data: [T]
    let pagination: Pagination

    struct Pagination: Decodable {
        let total: Int
        let limit: Int
        let offset: Int
    }
}

/// A monitored resource, as returned by GET /api/v1/monitors.
struct Monitor: Decodable, Identifiable, Equatable {
    let id: String
    let name: String
    let type: String
    let target: String
    let enabled: Bool
    let lastStatus: String
    let lastCheckedAt: Date?
    let projectId: String?

    /// The health state as a UI-facing status.
    var status: MonitorStatus { MonitorStatus(rawValue: lastStatus) ?? .unknown }
}

/// The health states a monitor can be in, decoupled from the wire string.
enum MonitorStatus: String {
    case up, down, degraded, paused, unknown
}

/// The 24h metrics summary for one monitor (GET /api/v1/monitors/{id}/metrics).
struct MonitorMetrics: Decodable {
    let uptimePercent: Double
    let responseMsAvg: Double?
    let responseMsCurrent: Double?
}

// MARK: - Request bodies

struct LoginRequest: Encodable {
    let email: String
    let password: String
}

struct GoogleSignInRequest: Encodable {
    /// Maps to `id_token` via convertToSnakeCase.
    let idToken: String
}

struct AppleSignInRequest: Encodable {
    /// Maps to `identity_token`. Consumed by the (to-be-added) /api/v1/auth/apple.
    let identityToken: String
}

struct RefreshRequest: Encodable {
    let refreshToken: String
}

struct LogoutRequest: Encodable {
    let refreshToken: String
}

struct RegisterDeviceRequest: Encodable {
    let token: String
    let platform: String
}

struct UnregisterDeviceRequest: Encodable {
    let token: String
}
