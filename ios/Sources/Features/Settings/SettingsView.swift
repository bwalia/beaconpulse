import SwiftUI
import UserNotifications

/// The Settings tab: manage alert delivery (channels, maintenance), see push
/// permission, view the account, and sign out (which also unregisters this device
/// from push).
struct SettingsView: View {
    @Environment(AppContainer.self) private var container
    @Environment(SessionStore.self) private var session
    @Environment(PushManager.self) private var push
    @Environment(\.openURL) private var openURL
    @State private var showDeleteAccount = false

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
                    HStack(spacing: 14) {
                        Text(Self.initials(user.name))
                            .font(.headline)
                            .foregroundStyle(.white)
                            .frame(width: 46, height: 46)
                            .background(LinearGradient(colors: [Theme.accent, Theme.accentDeep],
                                                       startPoint: .topLeading, endPoint: .bottomTrailing),
                                        in: .circle)
                        VStack(alignment: .leading, spacing: 3) {
                            Text(user.name).font(.headline)
                            Text(user.email).font(.subheadline).foregroundStyle(Theme.muted)
                        }
                        Spacer()
                        Text(user.role.capitalized)
                            .font(.caption.weight(.semibold))
                            .padding(.horizontal, 9).padding(.vertical, 4)
                            .foregroundStyle(Theme.accent)
                            .background(Theme.accent.opacity(0.13), in: .capsule)
                    }
                    .padding(.vertical, 4)
                    .accessibilityElement(children: .combine)
                }
            }

            Section("About") {
                Link("Privacy Policy", destination: AppConfig.current.apiBaseURL.appending(path: "privacy"))
                Link("Terms of Service", destination: AppConfig.current.apiBaseURL.appending(path: "terms"))
            }

            Section {
                Button("Sign Out", role: .destructive) {
                    Task {
                        await push.unregisterCurrentDevice()
                        await container.session.signOut()
                    }
                }
            }

            if let user = session.user {
                Section {
                    if user.role == "owner" {
                        Button("Delete Account", role: .destructive) { showDeleteAccount = true }
                    } else {
                        Text("Only your organization's owner can delete this account.")
                            .foregroundStyle(.secondary)
                    }
                } footer: {
                    Text("Permanently deletes your organization and all its data.")
                }
                .sheet(isPresented: $showDeleteAccount) {
                    DeleteAccountView(email: user.email)
                }
            }
        }
        .canvas()
        .task { await push.refreshAuthorizationStatus() }
    }

    /// "Jane Doe" → "JD", for the account avatar.
    static func initials(_ name: String) -> String {
        let letters = name.split(separator: " ").prefix(2).compactMap(\.first)
        return letters.isEmpty ? "?" : String(letters).uppercased()
    }

    @ViewBuilder
    private var pushStatusRow: some View {
        switch push.authorizationStatus {
        case .authorized, .provisional, .ephemeral:
            LabeledContent {
                Label("On", systemImage: "checkmark.circle.fill").foregroundStyle(Theme.up)
            } label: {
                Label("Push notifications", systemImage: "bell.and.waves.left.and.right")
            }
        case .denied:
            Button {
                if let url = Platform.notificationSettingsURL { openURL(url) }
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
