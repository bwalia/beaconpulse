import SwiftUI

/// Loads and deletes the org's projects.
@MainActor
@Observable
final class ProjectsStore {
    private(set) var state: Loadable<[Project]> = .loading
    @ObservationIgnored private let client: APIClient
    @ObservationIgnored private let service: ProjectService

    init(client: APIClient, service: ProjectService) {
        self.client = client
        self.service = service
    }

    func load() async {
        do {
            let page = try await client.send(
                .init(path: "/api/v1/projects", query: ["limit": "200"]),
                as: Paginated<Project>.self)
            state = .loaded(page.data)
        } catch {
            state = .failed((error as? APIError)?.errorDescription ?? "Couldn’t load projects.")
        }
    }

    func delete(_ project: Project) async throws {
        try await service.delete(id: project.id)
        await load()
    }
}

enum ProjectSheet: Identifiable {
    case create
    case edit(Project)
    var id: String {
        switch self {
        case .create: return "create"
        case let .edit(project): return project.id
        }
    }
}

struct ProjectsView: View {
    @Environment(AppContainer.self) private var container
    @State private var store: ProjectsStore?
    @State private var sheet: ProjectSheet?
    @State private var pendingDelete: Project?
    @State private var banner: String?

    var body: some View {
        LoadableView(state: store?.state ?? .loading, retry: { Task { await store?.load() } }) { projects in
            if projects.isEmpty {
                ContentUnavailableView("No projects", systemImage: "folder",
                                       description: Text("Tap + to create one. Monitors live in projects."))
            } else {
                List {
                    ForEach(projects) { project in
                        ZStack {
                            NavigationLink(value: project) { EmptyView() }.opacity(0)
                            ProjectRow(project: project)
                        }
                        .cardRow()
                        .swipeActions(edge: .trailing) {
                            Button(role: .destructive) { pendingDelete = project } label: {
                                Label("Delete", systemImage: "trash")
                            }
                            Button { sheet = .edit(project) } label: {
                                Label("Edit", systemImage: "pencil")
                            }
                        }
                    }
                }
                .listStyle(.plain)
                .canvas()
                .refreshable { await store?.load() }
            }
        }
        .toolbar {
            ToolbarItem(placement: .topBarTrailing) {
                Button { sheet = .create } label: { Image(systemName: "plus") }
                    .accessibilityLabel("New project")
            }
        }
        .task {
            if store == nil { store = ProjectsStore(client: container.apiClient, service: container.projects) }
            await store?.load()
        }
        .sheet(item: $sheet) { which in
            switch which {
            case .create:
                ProjectFormView(mode: .create) { Task { await store?.load() } }
            case let .edit(project):
                ProjectFormView(mode: .edit(project)) { Task { await store?.load() } }
            }
        }
        .confirmationDialog("Delete \(pendingDelete?.name ?? "project")?",
                            isPresented: Binding(get: { pendingDelete != nil }, set: { if !$0 { pendingDelete = nil } }),
                            titleVisibility: .visible,
                            presenting: pendingDelete) { project in
            Button("Delete Project", role: .destructive) { Task { await remove(project) } }
        } message: { _ in
            Text("This also deletes every monitor in this project and stops probing them. This can’t be undone.")
        }
        .alert("Projects", isPresented: Binding(get: { banner != nil }, set: { if !$0 { banner = nil } })) {
            Button("OK") { banner = nil }
        } message: {
            Text(banner ?? "")
        }
    }

    private func remove(_ project: Project) async {
        do {
            try await store?.delete(project)
        } catch {
            banner = (error as? APIError)?.errorDescription ?? "Couldn’t delete the project."
        }
    }
}

struct ProjectRow: View {
    let project: Project

    var body: some View {
        HStack(spacing: 12) {
            Image(systemName: "folder.fill")
                .font(.system(size: 15, weight: .semibold))
                .foregroundStyle(Theme.accent)
                .frame(width: 38, height: 38)
                .background(Theme.accent.opacity(0.13), in: .rect(cornerRadius: 10))
            VStack(alignment: .leading, spacing: 4) {
                Text(project.name).font(.headline)
                if !project.description.isEmpty {
                    Text(project.description)
                        .font(.subheadline)
                        .foregroundStyle(Theme.muted)
                        .lineLimit(2)
                }
            }
            Spacer(minLength: 8)
            EnvironmentBadge(environment: project.environment)
            Image(systemName: "chevron.right")
                .font(.caption2.weight(.semibold))
                .foregroundStyle(Theme.muted)
        }
    }
}

/// A small pill encoding a project's environment.
struct EnvironmentBadge: View {
    let environment: String

    var body: some View {
        Text(environment.uppercased())
            .font(.caption2.weight(.semibold))
            .padding(.horizontal, 8)
            .padding(.vertical, 3)
            .background(color.opacity(0.15), in: .capsule)
            .foregroundStyle(color)
    }

    private var color: Color {
        switch environment {
        case "production": return Theme.down
        case "staging": return Theme.degraded
        default: return Color(hex: "3B82F6") // blue-500
        }
    }
}
