import Foundation
import Observation
import UIKit
import UserNotifications

/// Owns the push lifecycle on the device side: ask permission, register with
/// APNs, hand the token to the backend, drop it on sign-out, and surface a tapped
/// alert's monitor id so the UI can deep-link. The alert fan-out itself lives in
/// the Go apns notifier — this side only registers and receives.
@MainActor
@Observable
final class PushManager: NSObject {
    /// The monitor id from the most recently tapped notification. The UI consumes
    /// it to deep-link, then clears it.
    var pendingMonitorID: String?

    /// The system notification permission, surfaced in Settings so a user who
    /// denied it understands why no alerts arrive.
    var authorizationStatus: UNAuthorizationStatus = .notDetermined

    @ObservationIgnored private let client: APIClient
    @ObservationIgnored private weak var session: SessionStore?
    @ObservationIgnored private var lastRegisteredToken: String?

    init(client: APIClient, session: SessionStore) {
        self.client = client
        self.session = session
        super.init()
        UNUserNotificationCenter.current().delegate = self
    }

    /// Requests authorization and, if granted, registers for remote notifications.
    /// The APNs device token arrives asynchronously via the AppDelegate.
    func requestAuthorizationAndRegister() async {
        let center = UNUserNotificationCenter.current()
        let granted = (try? await center.requestAuthorization(options: [.alert, .badge, .sound])) ?? false
        authorizationStatus = await center.notificationSettings().authorizationStatus
        guard granted else { return }
        UIApplication.shared.registerForRemoteNotifications()
    }

    /// Re-reads the current permission (e.g. when Settings appears, in case the
    /// user changed it in the iOS Settings app).
    func refreshAuthorizationStatus() async {
        authorizationStatus = await UNUserNotificationCenter.current().notificationSettings().authorizationStatus
    }

    /// Called by the AppDelegate with the raw APNs token. Sends it to the backend
    /// so this org's alerts reach this device. Skips a redundant re-send of an
    /// unchanged token.
    func didRegister(deviceToken: Data) async {
        let token = deviceToken.map { String(format: "%02x", $0) }.joined()
        guard token != lastRegisteredToken, session?.isAuthenticated == true else { return }
        do {
            try await client.send(.init(method: "POST", path: "/api/v1/devices",
                                        body: RegisterDeviceRequest(token: token, platform: "ios")))
            lastRegisteredToken = token
        } catch {
            // Non-fatal: retried on next launch/foreground. Clear so we try again.
            lastRegisteredToken = nil
        }
    }

    /// Removes this device's token from the backend on sign-out.
    func unregisterCurrentDevice() async {
        guard let token = lastRegisteredToken else { return }
        try? await client.send(.init(method: "DELETE", path: "/api/v1/devices",
                                     body: UnregisterDeviceRequest(token: token)))
        lastRegisteredToken = nil
    }
}

extension PushManager: UNUserNotificationCenterDelegate {
    /// Show alerts as banners while the app is foregrounded.
    func userNotificationCenter(_ center: UNUserNotificationCenter,
                                willPresent notification: UNNotification) async
        -> UNNotificationPresentationOptions {
        [.banner, .sound, .list]
    }

    /// A tapped notification: pull the monitor id for deep-linking.
    func userNotificationCenter(_ center: UNUserNotificationCenter,
                                didReceive response: UNNotificationResponse) async {
        if let monitorID = response.notification.request.content.userInfo["monitor_id"] as? String {
            pendingMonitorID = monitorID
        }
    }
}
