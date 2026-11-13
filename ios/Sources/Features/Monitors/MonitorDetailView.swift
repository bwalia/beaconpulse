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
                ProgressView().frame(maxWidth: .infinity, maxHeight: .infinity)
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
                            .accessibilityLabel("More actions")
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
        .alert("Action failed",
               isPresented: Binding(get: { actionError != nil }, set: { if !$0 { actionError = nil } })) {
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
        let kind = MonitorKind(rawValue: monitor.type)
        ScrollView {
            VStack(alignment: .leading, spacing: 14) {
                header(monitor, kind: kind)

                if let metrics {
                    FitRow {
                        StatTile(label: "Uptime · 24h", value: String(format: "%.2f%%", metrics.uptimePercent),
                                 tint: Theme.uptimeColor(metrics.uptimePercent), icon: "checkmark.shield")
                        if let avg = metrics.responseMsAvg {
                            StatTile(label: "Avg response", value: String(format: "%.0f ms", avg), icon: "speedometer")
                        }
                        // Push-based monitors aren't probed: GitHub Actions has no interval.
                        if kind != .githubActions {
                            StatTile(label: "Check every", value: Self.interval(monitor.intervalSeconds), icon: "timer")
                        }
                    }
                    if let series = metrics.responseMs, !series.isEmpty {
                        VStack(alignment: .leading, spacing: 12) {
                            HStack {
                                Text("Response time").font(.headline)
                                Spacer()
                                Eyebrow("Last 24 hours")
                            }
                            TrendChart(series: series, color: Theme.accent, height: 190, label: "Response time")
                        }
                        .card()
                    }
                }

                if monitor.type == "heartbeat", let ping = monitor.pingUrl {
                    heartbeatCard(ping, lastPingAt: monitor.lastPingAt)
                }

                VStack(spacing: 0) {
                    detailRow("Type", monitor.type.uppercased())
                    detailRow("Enabled", monitor.enabled ? "Yes" : "No — paused")
                    if kind?.isProbed ?? true {
                        detailRow("Timeout", "\(monitor.timeoutSeconds)s", last: true)
                    }
                }
                .card(padding: 0)
            }
            .padding()
            .frame(maxWidth: 900)
            .frame(maxWidth: .infinity)
        }
        .canvas()
        .refreshable { await store?.load() }
    }

    private func header(_ monitor: Monitor, kind: MonitorKind?) -> some View {
        VStack(alignment: .leading, spacing: 10) {
            HStack(spacing: 8) {
                StatusBadge(status: monitor.status)
                TypeBadge(text: monitor.type)
            }
            Text(monitor.name).font(.title2.bold())
            if kind != .heartbeat {
                Text(monitor.target)
                    .font(.callout.monospaced())
                    .foregroundStyle(Theme.muted)
                    .textSelection(.enabled)
            }
            if let lastChecked = monitor.lastCheckedAt {
                Label("Checked \(lastChecked.formatted(.relative(presentation: .named)))", systemImage: "clock")
                    .font(.caption)
                    .foregroundStyle(Theme.muted)
            }
        }
        .card()
    }

    private func detailRow(_ label: String, _ value: String, last: Bool = false) -> some View {
        VStack(spacing: 0) {
            HStack {
                Text(label).foregroundStyle(Theme.muted)
                Spacer()
                Text(value).fontWeight(.medium)
            }
            .font(.subheadline)
            .padding(.horizontal, 16)
            .padding(.vertical, 12)
            if !last { Divider().overlay(Theme.border).padding(.leading, 16) }
        }
    }

    private static func interval(_ seconds: Int) -> String {
        seconds % 3600 == 0 ? "\(seconds / 3600) h" : seconds % 60 == 0 ? "\(seconds / 60) min" : "\(seconds) s"
    }

    /// The heartbeat ping URL is stored relative (e.g. "/api/v1/ping/<token>");
    /// resolve it against the API base so the value shown is the full URL the
    /// customer's job should call, with a one-tap copy.
    private func heartbeatCard(_ pingURL: String, lastPingAt: Date?) -> some View {
        let full = URL(string: pingURL, relativeTo: AppConfig.current.apiBaseURL)?.absoluteString ?? pingURL
        return VStack(alignment: .leading, spacing: 10) {
            HStack {
                Text("Heartbeat").font(.headline)
                Spacer()
                Button {
                    Platform.copy(full)
                } label: {
                    Label("Copy URL", systemImage: "doc.on.doc").font(.caption.weight(.semibold))
                }
                .buttonStyle(.bordered)
                .controlSize(.small)
                .accessibilityLabel("Copy ping URL")
            }
            Text(full)
                .font(.caption.monospaced())
                .padding(10)
                .frame(maxWidth: .infinity, alignment: .leading)
                .background(Theme.subtle, in: .rect(cornerRadius: 8))
                .textSelection(.enabled)
            Label("Last ping \(lastPingAt?.formatted(.relative(presentation: .named)) ?? "never")",
                  systemImage: "waveform.path.ecg")
                .font(.caption)
                .foregroundStyle(Theme.muted)
        }
        .card()
    }
}
