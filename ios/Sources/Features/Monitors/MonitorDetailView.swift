import SwiftUI

/// Loads one monitor plus its 24h metrics. Fetched by id so the same screen works
/// from a list tap and from a push-notification deep-link.
@MainActor
@Observable
final class MonitorDetailStore {
    enum ViewState {
        case loading
        case loaded(Monitor, MonitorMetrics?)
        case failed(String)
    }

    private(set) var state: ViewState = .loading
    @ObservationIgnored private let client: APIClient
    @ObservationIgnored private let monitorID: String

    init(client: APIClient, monitorID: String) {
        self.client = client
        self.monitorID = monitorID
    }

    func load() async {
        do {
            let monitor = try await client.send(
                .init(path: "/api/v1/monitors/\(monitorID)"), as: Monitor.self)
            // Metrics are a bonus; a monitor with no history yet still renders.
            let metrics = try? await client.send(
                .init(path: "/api/v1/monitors/\(monitorID)/metrics"), as: MonitorMetrics.self)
            state = .loaded(monitor, metrics)
        } catch {
            state = .failed((error as? APIError)?.errorDescription ?? "Couldn’t load this monitor.")
        }
    }
}

struct MonitorDetailView: View {
    let monitorID: String
    @Environment(AppContainer.self) private var container
    @State private var store: MonitorDetailStore?

    var body: some View {
        Group {
            switch store?.state {
            case .loading, .none:
                ProgressView()
            case let .failed(message):
                ContentUnavailableView("Couldn’t load", systemImage: "exclamationmark.triangle",
                                       description: Text(message))
            case let .loaded(monitor, metrics):
                detail(monitor, metrics)
            }
        }
        .navigationTitle(title)
        .navigationBarTitleDisplayMode(.inline)
        .task {
            if store == nil {
                store = MonitorDetailStore(client: container.apiClient, monitorID: monitorID)
            }
            await store?.load()
        }
    }

    private var title: String {
        if case let .loaded(monitor, _)? = store?.state { return monitor.name }
        return "Monitor"
    }

    @ViewBuilder
    private func detail(_ monitor: Monitor, _ metrics: MonitorMetrics?) -> some View {
        List {
            Section {
                LabeledContent("Status") {
                    HStack(spacing: 6) {
                        StatusDot(status: monitor.status)
                        Text(monitor.status.rawValue.capitalized)
                    }
                }
                LabeledContent("Target", value: monitor.target)
                LabeledContent("Type", value: monitor.type.uppercased())
                LabeledContent("Enabled", value: monitor.enabled ? "Yes" : "No")
            }

            if let metrics {
                Section("Last 24 hours") {
                    LabeledContent("Uptime", value: String(format: "%.2f%%", metrics.uptimePercent))
                    if let avg = metrics.responseMsAvg {
                        LabeledContent("Avg response", value: String(format: "%.0f ms", avg))
                    }
                }
            }
        }
    }
}
