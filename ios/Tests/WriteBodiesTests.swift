import XCTest
@testable import SysOps

/// Guards the write-body encoding: snake_case keys, and — critically — that unset
/// optional settings are omitted rather than sent as null, so editing a monitor
/// never wipes advanced settings configured elsewhere.
final class WriteBodiesTests: XCTestCase {
    private func json<T: Encodable>(_ value: T) throws -> String {
        String(data: try APIClient.encoder().encode(value), encoding: .utf8)!
    }

    func testMonitorSettingsOmitsUnsetFields() throws {
        var settings = MonitorSettings.empty
        settings.alertSensitivity = "balanced"
        let out = try json(settings)
        XCTAssertTrue(out.contains("\"alert_sensitivity\":\"balanced\""))
        XCTAssertFalse(out.contains("method"), "unset fields must be omitted")
        XCTAssertFalse(out.contains("null"), "no nulls should be sent")
    }

    func testMonitorSettingsRoundTripPreservesAdvancedFields() throws {
        // A monitor edited on-device carries settings that were configured on the
        // web. Re-encoding must keep them.
        let incoming = """
        {"valid_status_codes":[200,201],"headers":{"X-Env":"prod"},"follow_redirects":true}
        """
        let decoded = try APIClient.decoder().decode(MonitorSettings.self, from: Data(incoming.utf8))
        var edited = decoded
        edited.alertSensitivity = "immediate"
        let out = try json(edited)
        XCTAssertTrue(out.contains("\"valid_status_codes\":[200,201]"))
        XCTAssertTrue(out.contains("\"X-Env\":\"prod\""), "header keys must pass through unchanged")
        XCTAssertTrue(out.contains("\"follow_redirects\":true"))
        XCTAssertTrue(out.contains("\"alert_sensitivity\":\"immediate\""))
    }

    func testCreateMonitorBodyUsesSnakeCaseAndOmitsNilGrace() throws {
        let body = CreateMonitorBody(
            projectId: "p1", name: "api", type: "https", target: "https://x",
            enabled: true, intervalSeconds: 60, timeoutSeconds: 10,
            graceSeconds: nil, settings: .empty)
        let out = try json(body)
        XCTAssertTrue(out.contains("\"project_id\":\"p1\""))
        XCTAssertTrue(out.contains("\"interval_seconds\":60"))
        XCTAssertFalse(out.contains("grace_seconds"), "nil grace must be omitted")
    }

    func testUpdateChannelBodyOmitsNilSecret() throws {
        let body = UpdateChannelBody(name: "Ops", enabled: true, config: ["chat_id": "1"], secret: nil)
        let out = try json(body)
        XCTAssertTrue(out.contains("\"name\":\"Ops\""))
        XCTAssertTrue(out.contains("\"chat_id\":\"1\""))
        XCTAssertFalse(out.contains("secret"), "a nil secret must be omitted to keep the stored one")
    }
}
