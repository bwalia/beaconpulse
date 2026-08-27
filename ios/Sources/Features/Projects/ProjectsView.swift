import SwiftUI

/// Loads the org's projects.
@MainActor
@Observable
final class ProjectsStore {
    private(set) var state: Loadable<[Project]> = .loading
    @ObservationIgnored private let client: APIClient

    init(client: APIClient) { self.client = client }

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
}

struct ProjectsView: View {
    @Environment(AppContainer.self) private var container
    @State private var store: ProjectsStore?

    var body: some View {
        LoadableView(state: store?.state ?? .loading, retry: { Task { await store?.load() } }) { projects in
            if projects.isEmpty {
                ContentUnavailableView("No projects", systemImage: "folder",
                                       description: Text("Projects you create on the web appear here."))
            } else {
                List(projects) { project in
                    NavigationLink(value: project) {
                        ProjectRow(project: project)
                    }
                }
                .listStyle(.plain)
                .refreshable { await store?.load() }
            }
        }
        .task {
            if store == nil { store = ProjectsStore(client: container.apiClient) }
            await store?.load()
        }
    }
}

struct ProjectRow: View {
    let project: Project

    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            HStack {
                Text(project.name).font(.headline)
                Spacer()
                EnvironmentBadge(environment: project.environment)
            }
            if !project.description.isEmpty {
                Text(project.description)
                    .font(.subheadline)
                    .foregroundStyle(.secondary)
                    .lineLimit(2)
            }
        }
        .padding(.vertical, 4)
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
        case "production": return .red
        case "staging": return .orange
        default: return .blue
        }
    }
}
