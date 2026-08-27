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

/// A single time-series sample: timestamp + value. Shared by overview and
/// per-monitor metrics.
struct MetricPoint: Decodable, Identifiable {
    let t: Date
    let v: Double
    var id: Date { t }
}

/// The 24h metrics for one monitor (GET /api/v1/monitors/{id}/metrics).
struct MonitorMetrics: Decodable {
    let uptimePercent: Double
    let responseMsAvg: Double?
    let responseMsCurrent: Double?
    let up: [MetricPoint]?
    let responseMs: [MetricPoint]?
}

/// The org-wide dashboard (GET /api/v1/overview?hours=1|6|24|168|720).
struct Overview: Decodable {
    let windowHours: Int
    let uptimePercent: Double
    let avgResponseMs: Double
    let uptimeSeries: [MetricPoint]
    let responseSeries: [MetricPoint]
    let monitors: [MonitorUptime]
}

/// A per-monitor row within the overview.
struct MonitorUptime: Decodable, Identifiable {
    let monitorId: String
    let monitorName: String
    let target: String
    let avgResponseMs: Double
    let points: [MetricPoint]
    var id: String { monitorId }
}

/// A currently-firing alert (GET /api/v1/alerts).
struct ActiveAlert: Decodable, Identifiable {
    let name: String
    let severity: String
    let monitorId: String
    let monitorName: String
    let monitorType: String
    let target: String
    let since: Date?
    let inMaintenance: Bool

    /// Alerts aren't uniquely keyed server-side; combine the rule and monitor.
    var id: String { "\(name):\(monitorId)" }
}

/// A project (GET /api/v1/projects). Hashable so it can drive value-based
/// navigation into its filtered monitor list.
struct Project: Decodable, Identifiable, Hashable {
    let id: String
    let name: String
    let slug: String
    let description: String
    let environment: String
    let isActive: Bool
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
