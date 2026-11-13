import Charts
import SwiftUI

/// The org-wide dashboard: uptime + response summary, trend charts, and a
/// per-monitor breakdown, over a selectable time window.
@MainActor
@Observable
final class OverviewStore {
    private(set) var state: Loadable<Overview> = .loading
    @ObservationIgnored private let client: APIClient

    init(client: APIClient) { self.client = client }

    func load(hours: Int) async {
        do {
            let overview = try await client.send(
                .init(path: "/api/v1/overview", query: ["hours": String(hours)]),
                as: Overview.self)
            state = .loaded(overview)
        } catch {
            state = .failed((error as? APIError)?.errorDescription ?? "Couldn’t load the dashboard.")
        }
    }
}

/// The windows the API accepts (?hours=1|6|24|168|720).
enum OverviewWindow: String, CaseIterable, Identifiable {
    case hour = "1h", sixHours = "6h", day = "24h", week = "7d", month = "30d"
    var id: String { rawValue }
    var hours: Int {
        switch self {
        case .hour: return 1
        case .sixHours: return 6
        case .day: return 24
        case .week: return 168
        case .month: return 720
        }
    }
}

struct OverviewView: View {
    @Environment(AppContainer.self) private var container
    @State private var store: OverviewStore?
    @State private var window: OverviewWindow = .day

    var body: some View {
        VStack(spacing: 0) {
            Picker("Window", selection: $window) {
                ForEach(OverviewWindow.allCases) { Text($0.rawValue).tag($0) }
            }
            .pickerStyle(.segmented)
            .padding([.horizontal, .top])
            .padding(.bottom, 4)

            LoadableView(state: store?.state ?? .loading, retry: reload) { overview in
                ScrollView {
                    VStack(alignment: .leading, spacing: 14) {
                        summary(overview)
                        LazyVGrid(columns: [GridItem(.adaptive(minimum: 320), spacing: 14)], spacing: 14) {
                            chartCard("Uptime", series: overview.uptimeSeries, color: Theme.up)
                            chartCard("Response time", series: overview.responseSeries, color: Theme.accent)
                        }
                        monitorsSection(overview.monitors)
                    }
                    .padding()
                    .frame(maxWidth: 1200)
                    .frame(maxWidth: .infinity)
                }
                .refreshable { await load() }
            }
        }
        .canvas()
        .task {
            if store == nil { store = OverviewStore(client: container.apiClient) }
            await load()
        }
        .onChange(of: window) { _, _ in reload() }
    }

    // MARK: - Loading

    private func reload() { Task { await load() } }
    private func load() async { await store?.load(hours: window.hours) }

    // MARK: - Sections

    private func summary(_ overview: Overview) -> some View {
        LazyVGrid(columns: [GridItem(.adaptive(minimum: 150), spacing: 12)], spacing: 12) {
            StatTile(label: "Uptime", value: String(format: "%.2f%%", overview.uptimePercent),
                     tint: Theme.uptimeColor(overview.uptimePercent), icon: "checkmark.shield")
            StatTile(label: "Avg response", value: String(format: "%.0f ms", overview.avgResponseMs),
                     icon: "speedometer")
            StatTile(label: "Monitors", value: "\(overview.monitors.count)",
                     icon: "dot.radiowaves.left.and.right", note: "Reporting in \(window.rawValue)")
        }
    }

    @ViewBuilder
    private func chartCard(_ title: String, series: [MetricPoint], color: Color) -> some View {
        VStack(alignment: .leading, spacing: 12) {
            Text(title).font(.headline)
            if series.isEmpty {
                Text("No data for this window.")
                    .font(.footnote).foregroundStyle(Theme.muted)
                    .frame(maxWidth: .infinity, minHeight: 170)
            } else {
                TrendChart(series: series, color: color, label: title)
            }
        }
        .card()
    }

    private func monitorsSection(_ monitors: [MonitorUptime]) -> some View {
        VStack(alignment: .leading, spacing: 4) {
            HStack {
                Text("Monitors").font(.headline)
                Spacer()
                Eyebrow("Avg response")
            }
            .padding(.bottom, 6)
            if monitors.isEmpty {
                Text("No monitors reporting yet.").font(.footnote).foregroundStyle(Theme.muted)
            } else {
                ForEach(monitors) { monitor in
                    NavigationLink(value: monitor.monitorId) {
                        HStack(spacing: 12) {
                            VStack(alignment: .leading, spacing: 3) {
                                Text(monitor.monitorName).font(.subheadline.weight(.semibold))
                                Text(monitor.target)
                                    .font(.caption.monospaced())
                                    .foregroundStyle(Theme.muted)
                                    .lineLimit(1)
                            }
                            Spacer()
                            Text(String(format: "%.0f ms", monitor.avgResponseMs))
                                .font(.caption.weight(.semibold)).monospacedDigit()
                                .padding(.horizontal, 8).padding(.vertical, 4)
                                .background(Theme.subtle, in: .capsule)
                            Image(systemName: "chevron.right")
                                .font(.caption2.weight(.semibold))
                                .foregroundStyle(Theme.muted)
                        }
                        .padding(.vertical, 8)
                        .contentShape(Rectangle())
                    }
                    .buttonStyle(.plain)
                    if monitor.id != monitors.last?.id { Divider().overlay(Theme.border) }
                }
            }
        }
        .card()
    }
}
