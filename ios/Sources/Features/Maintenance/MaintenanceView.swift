import SwiftUI

/// Loads and mutates the org's maintenance windows.
@MainActor
@Observable
final class MaintenanceStore {
    private(set) var state: Loadable<[MaintenanceWindow]> = .loading
    @ObservationIgnored private let service: MaintenanceService

    init(service: MaintenanceService) { self.service = service }

    func load() async {
        do {
            state = .loaded(try await service.list())
        } catch {
            state = .failed((error as? APIError)?.errorDescription ?? "Couldn’t load maintenance windows.")
        }
    }

    func delete(_ window: MaintenanceWindow) async throws {
        try await service.delete(id: window.id)
        await load()
    }
}

enum MaintenanceSheet: Identifiable {
    case create
    case edit(MaintenanceWindow)
    var id: String {
        switch self {
        case .create: return "create"
        case let .edit(window): return window.id
        }
    }
}

struct MaintenanceView: View {
    @Environment(AppContainer.self) private var container
    @State private var store: MaintenanceStore?
    @State private var sheet: MaintenanceSheet?
    @State private var banner: String?

    var body: some View {
        LoadableView(state: store?.state ?? .loading, retry: { Task { await store?.load() } }) { windows in
            if windows.isEmpty {
                ContentUnavailableView("No maintenance windows", systemImage: "wrench.and.screwdriver",
                                       description: Text("Schedule one to pause alerts during planned work."))
            } else {
                List {
                    ForEach(windows) { window in
                        MaintenanceRow(window: window)
                            .contentShape(Rectangle())
                            .onTapGesture { sheet = .edit(window) }
                            .swipeActions(edge: .trailing) {
                                Button(role: .destructive) {
                                    Task { await remove(window) }
                                } label: {
                                    Label("Delete", systemImage: "trash")
                                }
                            }
                    }
                }
                .listStyle(.plain)
                .refreshable { await store?.load() }
            }
        }
        .navigationTitle("Maintenance")
        .toolbar {
            ToolbarItem(placement: .topBarTrailing) {
                Button { sheet = .create } label: { Image(systemName: "plus") }
                    .accessibilityLabel("Schedule maintenance window")
            }
        }
        .task {
            if store == nil { store = MaintenanceStore(service: container.maintenance) }
            await store?.load()
        }
        .sheet(item: $sheet) { which in
            switch which {
            case .create:
                MaintenanceFormView(mode: .create) { Task { await store?.load() } }
            case let .edit(window):
                MaintenanceFormView(mode: .edit(window)) { Task { await store?.load() } }
            }
        }
        .alert("Maintenance", isPresented: Binding(get: { banner != nil }, set: { if !$0 { banner = nil } })) {
            Button("OK") { banner = nil }
        } message: {
            Text(banner ?? "")
        }
    }

    private func remove(_ window: MaintenanceWindow) async {
        do {
            try await store?.delete(window)
        } catch {
            banner = (error as? APIError)?.errorDescription ?? "Couldn’t delete the window."
        }
    }
}

struct MaintenanceRow: View {
    let window: MaintenanceWindow

    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            HStack {
                Text(window.title).font(.headline)
                if window.active {
                    Text("ACTIVE")
                        .font(.caption2.weight(.semibold))
                        .padding(.horizontal, 7).padding(.vertical, 2)
                        .background(Color.orange.opacity(0.18), in: .capsule)
                        .foregroundStyle(.orange)
                }
                Spacer()
            }
            Text(rangeText).font(.subheadline).foregroundStyle(.secondary)
            Text(scopeText).font(.caption).foregroundStyle(.secondary)
        }
        .padding(.vertical, 2)
        .accessibilityElement(children: .combine)
    }

    private var rangeText: String {
        let start = window.startsAt.formatted(date: .abbreviated, time: .shortened)
        let end = window.endsAt.formatted(date: .abbreviated, time: .shortened)
        return "\(start)  →  \(end)"
    }

    private var scopeText: String {
        switch window.scope {
        case "org": return "Whole organization"
        case "monitor": return "\(window.scopeIds.count) monitor(s)"
        case "project": return "\(window.scopeIds.count) project(s)"
        default: return window.scope
        }
    }
}
