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
        ScrollView {
            VStack(alignment: .leading, spacing: 22) {
                if let user = session.user { profileCard(user) }

                group("Alerts") {
                    NavigationLink { ChannelsView() } label: {
                        SettingsRow(title: "Notification channels", subtitle: "Where your alerts are sent",
                                    icon: "bell.badge.fill", tint: Theme.accent) { Chevron() }
                    }
                    RowDivider()
                    NavigationLink { MaintenanceView() } label: {
                        SettingsRow(title: "Maintenance windows", subtitle: "Silence alerts during planned work",
                                    icon: "wrench.and.screwdriver.fill", tint: Theme.degraded) { Chevron() }
                    }
                    RowDivider()
                    pushRow
                }

                group("About") {
                    Link(destination: AppConfig.current.apiBaseURL.appending(path: "privacy")) {
                        SettingsRow(title: "Privacy policy", icon: "hand.raised.fill",
                                    tint: Color(hex: "3B82F6")) { ExternalMark() }
                    }
                    RowDivider()
                    Link(destination: AppConfig.current.apiBaseURL.appending(path: "terms")) {
                        SettingsRow(title: "Terms of service", icon: "doc.text.fill",
                                    tint: Color(hex: "8B5CF6")) { ExternalMark() }
                    }
                    RowDivider()
                    SettingsRow(title: "Version", icon: "info.circle.fill", tint: Theme.paused) {
                        Text(Self.version).font(.callout.monospacedDigit()).foregroundStyle(Theme.muted)
                    }
                }

                Button {
                    Task {
                        await push.unregisterCurrentDevice()
                        await container.session.signOut()
                    }
                } label: {
                    Label("Sign out", systemImage: "rectangle.portrait.and.arrow.right")
                        .font(.body.weight(.semibold))
                        .foregroundStyle(Theme.down)
                        .frame(maxWidth: .infinity)
                        .padding(.vertical, 13)
                        .background(Theme.card, in: .rect(cornerRadius: 14))
                        .overlay(RoundedRectangle(cornerRadius: 14).strokeBorder(Theme.border))
                        .contentShape(.rect(cornerRadius: 14))
                }
                .buttonStyle(PressableStyle())

                if let user = session.user { dangerZone(user) }

                footer
            }
            .padding(20)
            .frame(maxWidth: 680)
            .frame(maxWidth: .infinity)
        }
        .canvas()
        .task { await push.refreshAuthorizationStatus() }
    }

    // MARK: - Sections

    private func profileCard(_ user: AuthUser) -> some View {
        HStack(spacing: 16) {
            Text(Self.initials(user.name))
                .font(.system(.title2, design: .rounded).weight(.bold))
                .foregroundStyle(.white)
                .frame(width: 64, height: 64)
                .background(.white.opacity(0.18), in: .circle)
                .overlay(Circle().strokeBorder(.white.opacity(0.35), lineWidth: 1.5))
            VStack(alignment: .leading, spacing: 4) {
                Text(user.name)
                    .font(.system(.title3, design: .rounded).weight(.bold))
                Text(user.email)
                    .font(.subheadline)
                    .opacity(0.85)
                    .lineLimit(1)
                    .truncationMode(.middle)
                Text(user.role.capitalized)
                    .font(.caption.weight(.semibold))
                    .padding(.horizontal, 9).padding(.vertical, 3)
                    .background(.white.opacity(0.2), in: .capsule)
                    .padding(.top, 4)
            }
            .foregroundStyle(.white)
            Spacer(minLength: 0)
        }
        .padding(20)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background {
            // The rings sit in an overlay so they can't make the background taller
            // than the card; the clip trims them to its rounded shape.
            LinearGradient(colors: [Theme.accent, Theme.accentDeep],
                           startPoint: .topLeading, endPoint: .bottomTrailing)
                .overlay(alignment: .topTrailing) {
                    // Faint rings, as on the sign-in hero.
                    ZStack {
                        ForEach([120.0, 190, 260], id: \.self) { size in
                            Circle()
                                .stroke(.white.opacity(0.09), lineWidth: 1)
                                .frame(width: size, height: size)
                        }
                    }
                    .offset(x: 90, y: -90)
                }
                .clipShape(.rect(cornerRadius: 18))
        }
        .shadow(color: Theme.accent.opacity(0.25), radius: 12, y: 6)
        .accessibilityElement(children: .combine)
    }

    @ViewBuilder
    private var pushRow: some View {
        switch push.authorizationStatus {
        case .authorized, .provisional, .ephemeral:
            SettingsRow(title: "Push notifications", subtitle: "Alerts arrive on this device",
                        icon: "bell.and.waves.left.and.right.fill", tint: Theme.up) {
                Pill(text: "On", color: Theme.up)
            }
        case .denied:
            Button {
                if let url = Platform.notificationSettingsURL { openURL(url) }
            } label: {
                SettingsRow(title: "Push notifications", subtitle: "Turned off — tap to enable",
                            icon: "bell.slash.fill", tint: Theme.degraded) {
                    Pill(text: "Off", color: Theme.degraded)
                }
            }
        default:
            SettingsRow(title: "Push notifications", subtitle: "Not set up on this device",
                        icon: "bell.fill", tint: Theme.paused) {
                Pill(text: "Not set up", color: Theme.paused)
            }
        }
    }

    private func dangerZone(_ user: AuthUser) -> some View {
        VStack(alignment: .leading, spacing: 10) {
            Label("Danger zone", systemImage: "exclamationmark.triangle.fill")
                .font(.subheadline.weight(.semibold))
                .foregroundStyle(Theme.down)
            Text("Deleting your account permanently removes your organization, its monitors and all their data.")
                .font(.footnote)
                .foregroundStyle(Theme.muted)
                .fixedSize(horizontal: false, vertical: true)
            if user.role == "owner" {
                Button { showDeleteAccount = true } label: {
                    Text("Delete account")
                        .font(.subheadline.weight(.semibold))
                        .foregroundStyle(.white)
                        .padding(.horizontal, 16).padding(.vertical, 9)
                        .background(Theme.down, in: .rect(cornerRadius: 10))
                }
                .buttonStyle(PressableStyle())
                .padding(.top, 2)
            } else {
                Text("Only your organization's owner can delete this account.")
                    .font(.footnote.weight(.medium))
            }
        }
        .padding(16)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(Theme.down.opacity(0.06), in: .rect(cornerRadius: 14))
        .overlay(RoundedRectangle(cornerRadius: 14).strokeBorder(Theme.down.opacity(0.3)))
        .sheet(isPresented: $showDeleteAccount) { DeleteAccountView(email: user.email) }
    }

    private var footer: some View {
        HStack(spacing: 8) {
            BrandTile(size: 22)
            Text("\(AppConfig.current.brandDisplayName) · \(Self.version)")
                .font(.caption)
                .foregroundStyle(Theme.muted)
        }
        .frame(maxWidth: .infinity)
        .padding(.top, 4)
    }

    /// A titled card holding settings rows.
    private func group<Content: View>(_ title: String, @ViewBuilder content: () -> Content) -> some View {
        VStack(alignment: .leading, spacing: 8) {
            Eyebrow(title).padding(.leading, 4)
            VStack(spacing: 0) { content() }
                .buttonStyle(PressableStyle())
                .background(Theme.card, in: .rect(cornerRadius: 14))
                .overlay(RoundedRectangle(cornerRadius: 14).strokeBorder(Theme.border))
                .clipShape(.rect(cornerRadius: 14))
                .shadow(color: .black.opacity(0.04), radius: 3, y: 1)
        }
    }

    // MARK: - Helpers

    /// "Jane Doe" → "JD", for the account avatar.
    static func initials(_ name: String) -> String {
        let letters = name.split(separator: " ").prefix(2).compactMap(\.first)
        return letters.isEmpty ? "?" : String(letters).uppercased()
    }

    /// "1.0.1 (6)" from the bundle.
    static var version: String {
        let info = Bundle.main.infoDictionary
        let short = info?["CFBundleShortVersionString"] as? String ?? "?"
        let build = info?["CFBundleVersion"] as? String ?? "?"
        return "\(short) (\(build))"
    }
}

/// One settings row: a tinted icon tile, a title with an optional subtitle, and a
/// trailing accessory (chevron, pill, value).
private struct SettingsRow<Trailing: View>: View {
    let title: String
    var subtitle: String?
    let icon: String
    let tint: Color
    @ViewBuilder var trailing: Trailing

    var body: some View {
        HStack(spacing: 13) {
            Image(systemName: icon)
                .font(.system(size: 14, weight: .semibold))
                .foregroundStyle(.white)
                .frame(width: 32, height: 32)
                .background(tint.gradient, in: .rect(cornerRadius: 9))
            VStack(alignment: .leading, spacing: 2) {
                Text(title).font(.body.weight(.medium)).foregroundStyle(.primary)
                if let subtitle {
                    Text(subtitle).font(.caption).foregroundStyle(Theme.muted)
                }
            }
            Spacer(minLength: 8)
            trailing
        }
        .padding(.horizontal, 14)
        .padding(.vertical, 11)
        .contentShape(Rectangle())
    }
}

private struct RowDivider: View {
    var body: some View { Rectangle().fill(Theme.border).frame(height: 1).padding(.leading, 59) }
}

private struct Chevron: View {
    var body: some View {
        Image(systemName: "chevron.right").font(.footnote.weight(.semibold)).foregroundStyle(Theme.muted.opacity(0.7))
    }
}

private struct ExternalMark: View {
    var body: some View {
        Image(systemName: "arrow.up.right").font(.footnote.weight(.semibold)).foregroundStyle(Theme.muted.opacity(0.7))
    }
}

private struct Pill: View {
    let text: String
    let color: Color

    var body: some View {
        Text(text)
            .font(.caption.weight(.semibold))
            .padding(.horizontal, 9).padding(.vertical, 4)
            .foregroundStyle(color)
            .background(color.opacity(0.13), in: .capsule)
    }
}

/// Plain button look with a soft highlight while pressed.
private struct PressableStyle: ButtonStyle {
    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .background(configuration.isPressed ? Theme.subtle : .clear)
            .opacity(configuration.isPressed ? 0.85 : 1)
    }
}
