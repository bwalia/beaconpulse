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
                ContentUnavailableView("All clear", systemImage: "checkmark.seal",
                                       description: Text("No active alerts right now."))
            } else {
                List(alerts) { alert in
                    NavigationLink(value: alert.monitorId) {
                        AlertRow(alert: alert)
                    }
                }
                .listStyle(.plain)
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
        HStack(spacing: 12) {
            SeverityIndicator(severity: alert.severity)
            VStack(alignment: .leading, spacing: 2) {
                Text(alert.monitorName).font(.headline)
                Text(alert.name).font(.subheadline).foregroundStyle(.secondary)
                if let since = alert.since {
                    Text("Firing \(since.formatted(.relative(presentation: .named)))")
                        .font(.caption).foregroundStyle(.secondary)
                }
            }
            Spacer()
            if alert.inMaintenance {
                Image(systemName: "wrench.and.screwdriver.fill")
                    .foregroundStyle(.secondary)
                    .accessibilityLabel("Under maintenance")
            }
        }
        .padding(.vertical, 4)
    }
}

/// Severity as an icon + colour, so criticality reads at a glance.
struct SeverityIndicator: View {
    let severity: String

    var body: some View {
        Image(systemName: symbol)
            .foregroundStyle(color)
            .accessibilityLabel(severity)
    }

    private var color: Color {
        switch severity.lowercased() {
        case "critical": return .red
        case "warning": return .orange
        default: return .yellow
        }
    }

    private var symbol: String {
        severity.lowercased() == "critical" ? "exclamationmark.octagon.fill" : "exclamationmark.triangle.fill"
    }
}
