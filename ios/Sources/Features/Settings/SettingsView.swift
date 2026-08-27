import SwiftUI

/// The Settings tab: manage notification channels, view the account, and sign out
/// (which also unregisters this device from push).
struct SettingsView: View {
    @Environment(AppContainer.self) private var container
    @Environment(SessionStore.self) private var session
    @Environment(PushManager.self) private var push

    var body: some View {
        List {
            Section("Notifications") {
                NavigationLink {
                    ChannelsView()
                } label: {
                    Label("Notification Channels", systemImage: "bell.badge")
                }
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
    }
}
