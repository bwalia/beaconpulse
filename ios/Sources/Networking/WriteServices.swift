import Foundation

/// Monitor write operations (create, edit, delete, pause/resume). Thin wrappers
/// over the authenticated client so views don't hand-build requests.
struct MonitorService {
    let client: APIClient

    func create(_ body: CreateMonitorBody) async throws -> Monitor {
        try await client.send(.init(method: "POST", path: "/api/v1/monitors", body: body), as: Monitor.self)
    }

    func update(id: String, _ body: UpdateMonitorBody) async throws -> Monitor {
        try await client.send(.init(method: "PATCH", path: "/api/v1/monitors/\(id)", body: body), as: Monitor.self)
    }

    func delete(id: String) async throws {
        try await client.send(.init(method: "DELETE", path: "/api/v1/monitors/\(id)"))
    }

    /// Pauses or resumes a monitor via the dedicated endpoints.
    func setEnabled(id: String, _ enabled: Bool) async throws {
        let action = enabled ? "resume" : "pause"
        try await client.send(.init(method: "POST", path: "/api/v1/monitors/\(id)/\(action)"))
    }
}

/// Notification-channel operations, including the "send test" action.
struct ChannelService {
    let client: APIClient

    func list() async throws -> [Channel] {
        try await client.send(
            .init(path: "/api/v1/notification-channels", query: ["limit": "200"]),
            as: Paginated<Channel>.self).data
    }

    func create(_ body: CreateChannelBody) async throws -> Channel {
        try await client.send(.init(method: "POST", path: "/api/v1/notification-channels", body: body), as: Channel.self)
    }

    func update(id: String, _ body: UpdateChannelBody) async throws -> Channel {
        try await client.send(.init(method: "PATCH", path: "/api/v1/notification-channels/\(id)", body: body), as: Channel.self)
    }

    func delete(id: String) async throws {
        try await client.send(.init(method: "DELETE", path: "/api/v1/notification-channels/\(id)"))
    }

    func sendTest(id: String) async throws {
        try await client.send(.init(method: "POST", path: "/api/v1/notification-channels/\(id)/test"))
    }
}

/// Maintenance-window operations. An active window suppresses alerts for the
/// monitors it covers.
struct MaintenanceService {
    let client: APIClient

    func list() async throws -> [MaintenanceWindow] {
        try await client.send(
            .init(path: "/api/v1/maintenance-windows", query: ["limit": "200"]),
            as: Paginated<MaintenanceWindow>.self).data
    }

    func create(_ body: CreateWindowBody) async throws -> MaintenanceWindow {
        try await client.send(.init(method: "POST", path: "/api/v1/maintenance-windows", body: body), as: MaintenanceWindow.self)
    }

    func update(id: String, _ body: UpdateWindowBody) async throws -> MaintenanceWindow {
        try await client.send(.init(method: "PATCH", path: "/api/v1/maintenance-windows/\(id)", body: body), as: MaintenanceWindow.self)
    }

    func delete(id: String) async throws {
        try await client.send(.init(method: "DELETE", path: "/api/v1/maintenance-windows/\(id)"))
    }
}
