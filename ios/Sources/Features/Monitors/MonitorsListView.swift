import SwiftUI

/// Loads and holds the org's monitors for the list.
@MainActor
@Observable
final class MonitorsStore {
    enum ViewState {
        case loading
        case loaded([Monitor])
        case empty
        case failed(String)
    }

    private(set) var state: ViewState = .loading
    @ObservationIgnored private let client: APIClient

    init(client: APIClient) { self.client = client }

    func load() async {
        do {
            let page = try await client.send(
                .init(path: "/api/v1/monitors", query: ["limit": "100"]),
                as: Paginated<Monitor>.self)
            state = page.data.isEmpty ? .empty : .loaded(page.data)
        } catch {
            state = .failed((error as? APIError)?.errorDescription ?? "Couldn’t load monitors.")
        }
    }
}

struct MonitorsListView: View {
    @Environment(AppContainer.self) private var container
    @State private var store: MonitorsStore?

    var body: some View {
        Group {
            switch store?.state {
            case .loading, .none:
                ProgressView().frame(maxWidth: .infinity, maxHeight: .infinity)

            case .empty:
                ContentUnavailableView("No monitors yet",
                                       systemImage: "dot.radiowaves.left.and.right",
                                       description: Text("Monitors you add on the web appear here."))

            case let .failed(message):
                ContentUnavailableView {
                    Label("Couldn’t load", systemImage: "exclamationmark.triangle")
                } description: {
                    Text(message)
                } actions: {
                    Button("Try Again") { Task { await store?.load() } }
                }

            case let .loaded(monitors):
                List(monitors) { monitor in
                    NavigationLink(value: monitor.id) {
                        MonitorRow(monitor: monitor)
                    }
                }
                .listStyle(.plain)
                .refreshable { await store?.load() }
            }
        }
        .task {
            if store == nil { store = MonitorsStore(client: container.apiClient) }
            await store?.load()
        }
    }
}

struct MonitorRow: View {
    let monitor: Monitor

    var body: some View {
        HStack(spacing: 12) {
            StatusDot(status: monitor.status)
            VStack(alignment: .leading, spacing: 2) {
                Text(monitor.name).font(.headline)
                Text(monitor.target)
                    .font(.subheadline)
                    .foregroundStyle(.secondary)
                    .lineLimit(1)
            }
            Spacer()
            Text(monitor.type.uppercased())
                .font(.caption2)
                .foregroundStyle(.secondary)
        }
        .padding(.vertical, 4)
    }
}

/// A coloured dot encoding a monitor's health at a glance.
struct StatusDot: View {
    let status: MonitorStatus

    var body: some View {
        Circle()
            .fill(color)
            .frame(width: 10, height: 10)
            .accessibilityLabel(status.rawValue)
    }

    private var color: Color {
        switch status {
        case .up: return .green
        case .down: return .red
        case .degraded: return .orange
        case .paused: return .gray
        case .unknown: return .secondary
        }
    }
}
