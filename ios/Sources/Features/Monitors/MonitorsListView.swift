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
    @ObservationIgnored private let projectID: String?

    init(client: APIClient, projectID: String? = nil) {
        self.client = client
        self.projectID = projectID
    }

    func load() async {
        do {
            var query = ["limit": "100"]
            if let projectID { query["project_id"] = projectID }
            let page = try await client.send(
                .init(path: "/api/v1/monitors", query: query),
                as: Paginated<Monitor>.self)
            state = page.data.isEmpty ? .empty : .loaded(page.data)
        } catch {
            state = .failed((error as? APIError)?.errorDescription ?? "Couldn’t load monitors.")
        }
    }
}

struct MonitorsListView: View {
    /// When set, only this project's monitors are shown.
    var projectID: String? = nil

    @Environment(AppContainer.self) private var container
    @State private var store: MonitorsStore?
    @State private var showingCreate = false
    @State private var search = ""

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
                let shown = filtered(monitors)
                if shown.isEmpty {
                    ContentUnavailableView.search(text: search)
                } else {
                    List(shown) { monitor in
                        NavigationLink(value: monitor.id) {
                            MonitorRow(monitor: monitor)
                        }
                    }
                    .listStyle(.plain)
                    .refreshable { await store?.load() }
                }
            }
        }
        .searchable(text: $search, prompt: "Search monitors")
        .task {
            if store == nil { store = MonitorsStore(client: container.apiClient, projectID: projectID) }
            await store?.load()
        }
        .toolbar {
            ToolbarItem(placement: .topBarTrailing) {
                Button {
                    showingCreate = true
                } label: {
                    Image(systemName: "plus")
                }
                .accessibilityLabel("Add monitor")
            }
        }
        .sheet(isPresented: $showingCreate) {
            MonitorFormView(mode: .create) { Task { await store?.load() } }
        }
    }

    /// Filters the loaded page by name or target. Client-side: the list is one
    /// page, so this avoids a round-trip per keystroke.
    private func filtered(_ monitors: [Monitor]) -> [Monitor] {
        let query = search.trimmed
        guard !query.isEmpty else { return monitors }
        return monitors.filter {
            $0.name.localizedCaseInsensitiveContains(query)
                || $0.target.localizedCaseInsensitiveContains(query)
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
        .accessibilityElement(children: .combine)
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
