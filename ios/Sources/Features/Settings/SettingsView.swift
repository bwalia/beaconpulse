import SwiftUI
import UIKit
import UserNotifications

/// The Settings tab: manage alert delivery (channels, maintenance), see push
/// permission, view the account, and sign out (which also unregisters this device
/// from push).
struct SettingsView: View {
    @Environment(AppContainer.self) private var container
    @Environment(SessionStore.self) private var session
    @Environment(PushManager.self) private var push
    @Environment(\.openURL) private var openURL

    var body: some View {
        List {
            Section("Alerts") {
                NavigationLink {
                    ChannelsView()
                } label: {
                    Label("Notification Channels", systemImage: "bell.badge")
                }
                NavigationLink {
                    MaintenanceView()
                } label: {
                    Label("Maintenance Windows", systemImage: "wrench.and.screwdriver")
                }
                pushStatusRow
            }

            if let user = session.user {
                Section("Account") {
                    LabeledContent("Name", value: user.name)
                    LabeledContent("Email", value: user.email)
                    LabeledContent("Role", value: user.role.capitalized)
                }
            }

            Section {
                Button("Sign Out", role: .destructive) {
                    Task {
                        await push.unregisterCurrentDevice()
                        await container.session.signOut()
                    }
                }
            }
        }
        .task { await push.refreshAuthorizationStatus() }
    }

    @ViewBuilder
    private var pushStatusRow: some View {
        switch push.authorizationStatus {
        case .authorized, .provisional, .ephemeral:
            LabeledContent("Push notifications") { Text("On").foregroundStyle(.secondary) }
        case .denied:
            Button {
                if let url = URL(string: UIApplication.openSettingsURLString) { openURL(url) }
            } label: {
                HStack {
                    Label("Push notifications", systemImage: "bell.slash")
                    Spacer()
                    Text("Off — tap to enable").foregroundStyle(.orange)
                }
                .contentShape(Rectangle())
            }
            .buttonStyle(.plain)
        default:
            LabeledContent("Push notifications") { Text("Not set up").foregroundStyle(.secondary) }
        }
    }
}
