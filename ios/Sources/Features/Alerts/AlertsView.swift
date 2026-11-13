import SwiftUI

/// Loads the org's currently-firing alerts.
@MainActor
@Observable
final class AlertsStore {
    private(set) var state: Loadable<[ActiveAlert]> = .loading
    @ObservationIgnored private let client: APIClient

    init(client: APIClient) { self.client = client }

    func load() async {
        do {
            let page = try await client.send(
                .init(path: "/api/v1/alerts", query: ["limit": "200"]),
                as: Paginated<ActiveAlert>.self)
            state = .loaded(page.data)
        } catch {
            state = .failed((error as? APIError)?.errorDescription ?? "Couldn’t load alerts.")
        }
    }
}

struct AlertsView: View {
    @Environment(AppContainer.self) private var container
    @State private var store: AlertsStore?

    var body: some View {
        LoadableView(state: store?.state ?? .loading, retry: { Task { await store?.load() } }) { alerts in
            if alerts.isEmpty {
                AllClearView()
            } else {
                List(alerts) { alert in
                    ZStack {
                        NavigationLink(value: alert.monitorId) { EmptyView() }.opacity(0)
                        AlertRow(alert: alert)
                    }
                    .cardRow()
                }
                .listStyle(.plain)
                .canvas()
                .refreshable { await store?.load() }
            }
        }
        .task {
            if store == nil { store = AlertsStore(client: container.apiClient) }
            await store?.load()
        }
    }
}

struct AlertRow: View {
    let alert: ActiveAlert

    var body: some View {
        let color = SeverityIndicator.color(alert.severity)
        HStack(spacing: 12) {
            RoundedRectangle(cornerRadius: 2)
                .fill(color)
                .frame(width: 4)
                .frame(maxHeight: .infinity)
            VStack(alignment: .leading, spacing: 4) {
                Text(alert.monitorName).font(.headline)
                Text(alert.name).font(.subheadline).foregroundStyle(Theme.muted)
                if let since = alert.since {
                    Label("Firing \(since.formatted(.relative(presentation: .named)))", systemImage: "clock")
                        .font(.caption)
                        .foregroundStyle(Theme.muted)
                }
            }
            Spacer(minLength: 8)
            VStack(alignment: .trailing, spacing: 6) {
                SeverityIndicator(severity: alert.severity)
                if alert.inMaintenance {
                    Label("Maintenance", systemImage: "wrench.and.screwdriver.fill")
                        .font(.caption2.weight(.semibold))
                        .foregroundStyle(Theme.muted)
                        .accessibilityLabel("Under maintenance")
                }
            }
            Image(systemName: "chevron.right")
                .font(.caption2.weight(.semibold))
                .foregroundStyle(Theme.muted)
        }
        .fixedSize(horizontal: false, vertical: true)
        .accessibilityElement(children: .combine)
    }
}

/// Severity as a tinted pill, so criticality reads at a glance.
struct SeverityIndicator: View {
    let severity: String

    var body: some View {
        let color = Self.color(severity)
        Label(severity.capitalized, systemImage: severity.lowercased() == "critical"
              ? "exclamationmark.octagon.fill" : "exclamationmark.triangle.fill")
            .font(.caption.weight(.semibold))
            .padding(.horizontal, 9)
            .padding(.vertical, 4)
            .foregroundStyle(color)
            .background(color.opacity(0.13), in: .capsule)
            .accessibilityLabel("Severity: \(severity)")
    }

    static func color(_ severity: String) -> Color {
        switch severity.lowercased() {
        case "critical": Theme.down
        case "warning": Theme.degraded
        default: Color(hex: "EAB308") // yellow-500
        }
    }
}

/// No firing alerts: said plainly, with the reassurance that matters.
struct AllClearView: View {
    var body: some View {
        VStack(spacing: 14) {
            Image(systemName: "checkmark.seal.fill")
                .font(.system(size: 44))
                .foregroundStyle(Theme.up)
                .frame(width: 88, height: 88)
                .background(Theme.up.opacity(0.12), in: .circle)
            Text("All clear").font(.title3.bold())
            Text("No active alerts right now. You’ll get a notification the moment something goes wrong.")
                .font(.subheadline)
                .foregroundStyle(Theme.muted)
                .multilineTextAlignment(.center)
                .frame(maxWidth: 320)
        }
        .padding()
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .background(Theme.canvas.ignoresSafeArea())
    }
}
