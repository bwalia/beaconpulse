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

/// Type-specific probe settings. Every field is optional and encodes only when
/// set (synthesized `encodeIfPresent`), so editing a monitor round-trips whatever
/// was configured on the web instead of wiping it.
struct MonitorSettings: Codable, Equatable {
    var method: String? = nil
    var validStatusCodes: [Int]? = nil
    var bodyKeyword: String? = nil
    var bodyNotKeyword: String? = nil
    var followRedirects: Bool? = nil
    var headers: [String: String]? = nil
    var skipTlsVerify: Bool? = nil
    var sslExpiryWarningDays: Int? = nil
    var responseTimeWarningMs: Int? = nil
    var alertSensitivity: String? = nil
    var dnsQueryName: String? = nil
    var dnsQueryType: String? = nil
    var dnsExpectedIps: [String]? = nil

    static let empty = MonitorSettings()
}

/// A monitored resource, as returned by GET /api/v1/monitors. Decodes the full
/// record (not just the list view) so the same model drives the edit form.
struct Monitor: Decodable, Identifiable, Equatable {
    let id: String
    let name: String
    let type: String
    let target: String
    let enabled: Bool
    let intervalSeconds: Int
    let timeoutSeconds: Int
    let graceSeconds: Int?
    let settings: MonitorSettings
    let lastStatus: String
    let lastCheckedAt: Date?
    let projectId: String?
    let pingUrl: String?

    /// The health state as a UI-facing status.
    var status: MonitorStatus { MonitorStatus(rawValue: lastStatus) ?? .unknown }
}

/// The monitor types the API accepts.
enum MonitorKind: String, CaseIterable, Identifiable {
    case http, https, ssl, tcp, icmp, dns, heartbeat
    var id: String { rawValue }
    /// A heartbeat is pinged by the customer's job, so it has no probe target.
    var needsTarget: Bool { self != .heartbeat }
}

/// A notification channel (GET /api/v1/notification-channels). The secret is never
/// returned — only `hasSecret`.
struct Channel: Decodable, Identifiable, Equatable {
    let id: String
    let name: String
    let type: String
    let enabled: Bool
    let config: [String: String]
    let hasSecret: Bool

    /// The Apple Push channel is created and managed automatically; the app only
    /// toggles it, never edits config/secret.
    var isManaged: Bool { type == "apns" }
}

/// A maintenance window (GET /api/v1/maintenance-windows). While active, alerts
/// for the covered monitors are suppressed. `scope` is org | project | monitor.
struct MaintenanceWindow: Decodable, Identifiable, Hashable {
    let id: String
    let title: String
    let description: String
    let startsAt: Date
    let endsAt: Date
    let scope: String
    let scopeIds: [String]
    let active: Bool
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

/// Body for POST /api/v1/monitors.
struct CreateMonitorBody: Encodable {
    let projectId: String
    let name: String
    let type: String
    let target: String
    let enabled: Bool
    let intervalSeconds: Int
    let timeoutSeconds: Int
    let graceSeconds: Int?
    let settings: MonitorSettings
}

/// Body for PATCH /api/v1/monitors/{id}. Type is immutable server-side, so it is
/// absent. `settings` is the full round-tripped object to avoid clobbering.
struct UpdateMonitorBody: Encodable {
    let name: String
    let target: String?
    let enabled: Bool
    let intervalSeconds: Int
    let timeoutSeconds: Int
    let settings: MonitorSettings
}

/// Body for POST /api/v1/notification-channels.
struct CreateChannelBody: Encodable {
    let name: String
    let type: String
    let enabled: Bool
    let config: [String: String]
    let secret: String
}

/// Body for PATCH /api/v1/notification-channels/{id}. A nil `secret` leaves the
/// stored credential unchanged.
struct UpdateChannelBody: Encodable {
    let name: String?
    let enabled: Bool?
    let config: [String: String]?
    let secret: String?
}

/// Body for POST /api/v1/maintenance-windows.
struct CreateWindowBody: Encodable {
    let title: String
    let description: String
    let startsAt: Date
    let endsAt: Date
    let scope: String
    let scopeIds: [String]
}

/// Body for PATCH /api/v1/maintenance-windows/{id}.
struct UpdateWindowBody: Encodable {
    let title: String?
    let description: String?
    let startsAt: Date?
    let endsAt: Date?
    let scope: String?
    let scopeIds: [String]?
}

struct RegisterDeviceRequest: Encodable {
    let token: String
    let platform: String
}

struct UnregisterDeviceRequest: Encodable {
    let token: String
}
