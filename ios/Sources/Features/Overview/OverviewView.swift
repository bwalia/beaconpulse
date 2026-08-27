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

            LoadableView(state: store?.state ?? .loading, retry: reload) { overview in
                ScrollView {
                    VStack(spacing: 20) {
                        summary(overview)
                        chartCard("Uptime", series: overview.uptimeSeries, filled: true, color: .green)
                        chartCard("Response time", series: overview.responseSeries, filled: false,
                                  color: AppConfig.current.accentColor)
                        monitorsSection(overview.monitors)
                    }
                    .padding()
                }
                .refreshable { await load() }
            }
        }
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
        HStack(spacing: 12) {
            metricTile("Uptime", value: String(format: "%.2f%%", overview.uptimePercent),
                       color: uptimeColor(overview.uptimePercent))
            metricTile("Avg response", value: String(format: "%.0f ms", overview.avgResponseMs),
                       color: .primary)
        }
    }

    private func metricTile(_ label: String, value: String, color: Color) -> some View {
        VStack(alignment: .leading, spacing: 4) {
            Text(label).font(.caption).foregroundStyle(.secondary)
            Text(value).font(.title2.bold()).monospacedDigit().foregroundStyle(color)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding()
        .background(.thinMaterial, in: .rect(cornerRadius: 12))
    }

    @ViewBuilder
    private func chartCard(_ title: String, series: [MetricPoint], filled: Bool, color: Color) -> some View {
        VStack(alignment: .leading, spacing: 8) {
            Text(title).font(.headline)
            if series.isEmpty {
                Text("No data for this window.")
                    .font(.footnote).foregroundStyle(.secondary)
                    .frame(maxWidth: .infinity, minHeight: 120)
            } else {
                Chart(series) { point in
                    if filled {
                        AreaMark(x: .value("Time", point.t), y: .value(title, point.v))
                            .foregroundStyle(color.opacity(0.2))
                            .interpolationMethod(.catmullRom)
                    }
                    LineMark(x: .value("Time", point.t), y: .value(title, point.v))
                        .foregroundStyle(color)
                        .interpolationMethod(.catmullRom)
                }
                .frame(height: 150)
                .chartYAxis { AxisMarks(position: .leading) }
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding()
        .background(.thinMaterial, in: .rect(cornerRadius: 12))
    }

    private func monitorsSection(_ monitors: [MonitorUptime]) -> some View {
        VStack(alignment: .leading, spacing: 8) {
            Text("Monitors").font(.headline)
            if monitors.isEmpty {
                Text("No monitors reporting yet.").font(.footnote).foregroundStyle(.secondary)
            } else {
                ForEach(monitors) { monitor in
                    NavigationLink(value: monitor.monitorId) {
                        HStack {
                            VStack(alignment: .leading, spacing: 2) {
                                Text(monitor.monitorName).font(.subheadline.weight(.medium))
                                Text(monitor.target).font(.caption).foregroundStyle(.secondary).lineLimit(1)
                            }
                            Spacer()
                            Text(String(format: "%.0f ms", monitor.avgResponseMs))
                                .font(.caption).monospacedDigit().foregroundStyle(.secondary)
                        }
                        .contentShape(Rectangle())
                    }
                    .buttonStyle(.plain)
                    if monitor.id != monitors.last?.id { Divider() }
                }
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding()
        .background(.thinMaterial, in: .rect(cornerRadius: 12))
    }

    private func uptimeColor(_ percent: Double) -> Color {
        switch percent {
        case 99.9...: return .green
        case 99...: return .yellow
        default: return .orange
        }
    }
}
