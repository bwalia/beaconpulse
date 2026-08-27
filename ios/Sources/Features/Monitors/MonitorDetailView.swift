import Charts
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
    @Environment(\.dismiss) private var dismiss
    @State private var store: MonitorDetailStore?
    @State private var showingEdit = false
    @State private var showingDeleteConfirm = false
    @State private var actionError: String?

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
        .toolbar {
            if let monitor {
                ToolbarItem(placement: .topBarTrailing) {
                    Menu {
                        Button { showingEdit = true } label: { Label("Edit", systemImage: "pencil") }
                        Button {
                            Task { await setEnabled(monitor, !monitor.enabled) }
                        } label: {
                            Label(monitor.enabled ? "Pause" : "Resume",
                                  systemImage: monitor.enabled ? "pause" : "play")
                        }
                        Divider()
                        Button(role: .destructive) { showingDeleteConfirm = true } label: {
                            Label("Delete", systemImage: "trash")
                        }
                    } label: {
                        Image(systemName: "ellipsis.circle")
                    }
                }
            }
        }
        .sheet(isPresented: $showingEdit) {
            if let monitor {
                MonitorFormView(mode: .edit(monitor)) { Task { await store?.load() } }
            }
        }
        .confirmationDialog("Delete this monitor?", isPresented: $showingDeleteConfirm, titleVisibility: .visible) {
            Button("Delete", role: .destructive) { Task { await deleteMonitor() } }
        } message: {
            Text("This can’t be undone.")
        }
        .alert("Action failed", isPresented: .constant(actionError != nil)) {
            Button("OK") { actionError = nil }
        } message: {
            Text(actionError ?? "")
        }
    }

    private var monitor: Monitor? {
        if case let .loaded(monitor, _)? = store?.state { return monitor }
        return nil
    }

    private func setEnabled(_ monitor: Monitor, _ enabled: Bool) async {
        do {
            try await container.monitors.setEnabled(id: monitor.id, enabled)
            await store?.load()
        } catch {
            actionError = (error as? APIError)?.errorDescription ?? "Couldn’t update the monitor."
        }
    }

    private func deleteMonitor() async {
        guard let monitor else { return }
        do {
            try await container.monitors.delete(id: monitor.id)
            dismiss()
        } catch {
            actionError = (error as? APIError)?.errorDescription ?? "Couldn’t delete the monitor."
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
                if let series = metrics.responseMs, !series.isEmpty {
                    Section("Response time") {
                        Chart(series) { point in
                            LineMark(x: .value("Time", point.t), y: .value("ms", point.v))
                                .interpolationMethod(.catmullRom)
                                .foregroundStyle(AppConfig.current.accentColor)
                        }
                        .frame(height: 160)
                        .padding(.vertical, 8)
                    }
                }
            }
        }
    }
}
