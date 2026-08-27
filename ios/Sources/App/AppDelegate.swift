import UIKit

/// Bridges the UIKit remote-notification callbacks into the SwiftUI world. It
/// forwards the APNs device token to the PushManager. Because the token can
/// arrive before the App has attached the manager, an early token is buffered and
/// flushed on attach.
final class AppDelegate: NSObject, UIApplicationDelegate {
    private weak var push: PushManager?
    private var bufferedToken: Data?

    /// Wires the push manager once the SwiftUI object graph exists.
    @MainActor
    func attach(push: PushManager) {
        self.push = push
        if let token = bufferedToken {
            bufferedToken = nil
            Task { await push.didRegister(deviceToken: token) }
        }
    }

    func application(_ application: UIApplication,
                     didRegisterForRemoteNotificationsWithDeviceToken deviceToken: Data) {
        if let push {
            Task { await push.didRegister(deviceToken: deviceToken) }
        } else {
            bufferedToken = deviceToken
        }
    }

    func application(_ application: UIApplication,
                     didFailToRegisterForRemoteNotificationsWithError error: Error) {
        // Non-fatal: e.g. a build/entitlement without push, or the simulator.
        // The app runs normally; alerts simply don't arrive on this install.
    }
}
