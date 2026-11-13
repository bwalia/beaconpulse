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
    @State private var actionError: String?

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
                        // The link is invisible so the card draws its own chevron
                        // inside it rather than the List adding one outside.
                        ZStack {
                            NavigationLink(value: monitor.id) { EmptyView() }.opacity(0)
                            MonitorRow(monitor: monitor)
                        }
                        .cardRow()
                        .swipeActions(edge: .leading) {
                            Button {
                                Task { await setEnabled(monitor, !monitor.enabled) }
                            } label: {
                                Label(monitor.enabled ? "Pause" : "Resume",
                                      systemImage: monitor.enabled ? "pause" : "play")
                            }
                            .tint(monitor.enabled ? .orange : .green)
                        }
                        .swipeActions(edge: .trailing) {
                            Button(role: .destructive) {
                                Task { await deleteMonitor(monitor) }
                            } label: {
                                Label("Delete", systemImage: "trash")
                            }
                        }
                    }
                    .listStyle(.plain)
                    .canvas()
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
        .alert("Action failed",
               isPresented: Binding(get: { actionError != nil }, set: { if !$0 { actionError = nil } })) {
            Button("OK") { actionError = nil }
        } message: {
            Text(actionError ?? "")
        }
    }

    private func setEnabled(_ monitor: Monitor, _ enabled: Bool) async {
        do {
            try await container.monitors.setEnabled(id: monitor.id, enabled)
            await store?.load()
        } catch {
            actionError = (error as? APIError)?.errorDescription ?? "Couldn’t update the monitor."
        }
    }

    private func deleteMonitor(_ monitor: Monitor) async {
        do {
            try await container.monitors.delete(id: monitor.id)
            await store?.load()
        } catch {
            actionError = (error as? APIError)?.errorDescription ?? "Couldn’t delete the monitor."
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
        let color = Theme.color(for: monitor.status)
        HStack(spacing: 12) {
            Image(systemName: symbol)
                .font(.system(size: 15, weight: .semibold))
                .foregroundStyle(color)
                .frame(width: 38, height: 38)
                .background(color.opacity(0.13), in: .rect(cornerRadius: 10))
            VStack(alignment: .leading, spacing: 4) {
                Text(monitor.name).font(.headline).lineLimit(1)
                Text(monitor.target)
                    .font(.caption.monospaced())
                    .foregroundStyle(Theme.muted)
                    .lineLimit(1)
            }
            Spacer(minLength: 8)
            VStack(alignment: .trailing, spacing: 6) {
                StatusBadge(status: monitor.status)
                TypeBadge(text: monitor.type)
            }
            Image(systemName: "chevron.right")
                .font(.caption2.weight(.semibold))
                .foregroundStyle(Theme.muted)
        }
        .accessibilityElement(children: .combine)
    }

    private var symbol: String {
        switch monitor.type.lowercased() {
        case "http", "https": "globe"
        case "ssl", "tls": "lock.shield"
        case "dns": "server.rack"
        case "tcp": "network"
        case "ping", "icmp": "dot.radiowaves.left.and.right"
        case "heartbeat": "waveform.path.ecg"
        case "github_actions": "gearshape.2"
        default: "dot.radiowaves.left.and.right"
        }
    }
}

/// A coloured dot encoding a monitor's health at a glance.
struct StatusDot: View {
    let status: MonitorStatus

    var body: some View {
        Circle()
            .fill(Theme.color(for: status))
            .frame(width: 10, height: 10)
            .accessibilityLabel(status.rawValue)
    }
}
