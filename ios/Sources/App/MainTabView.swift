import SwiftUI

/// The signed-in shell: four read-parity tabs, each its own navigation stack.
/// A tapped push notification switches to the Monitors tab and deep-links to the
/// monitor. Sign-out (which also unregisters this device from push) lives on the
/// Overview tab.
struct MainTabView: View {
    @Environment(AppContainer.self) private var container
    @Environment(PushManager.self) private var push

    @State private var selection: Tab = .overview
    @State private var monitorsPath = NavigationPath()

    enum Tab: Hashable { case overview, monitors, alerts, projects }

    var body: some View {
        TabView(selection: $selection) {
            NavigationStack {
                OverviewView()
                    .navigationTitle("Overview")
                    .toolbar { signOutButton }
                    .monitorDetailDestination()
            }
            .tabItem { Label("Overview", systemImage: "chart.bar.xaxis") }
            .tag(Tab.overview)

            NavigationStack(path: $monitorsPath) {
                MonitorsListView()
                    .navigationTitle("Monitors")
                    .monitorDetailDestination()
            }
            .tabItem { Label("Monitors", systemImage: "dot.radiowaves.left.and.right") }
            .tag(Tab.monitors)

            NavigationStack {
                AlertsView()
                    .navigationTitle("Alerts")
                    .monitorDetailDestination()
            }
            .tabItem { Label("Alerts", systemImage: "bell.badge") }
            .tag(Tab.alerts)

            NavigationStack {
                ProjectsView()
                    .navigationTitle("Projects")
                    .navigationDestination(for: Project.self) { project in
                        MonitorsListView(projectID: project.id)
                            .navigationTitle(project.name)
                    }
                    .monitorDetailDestination()
            }
            .tabItem { Label("Projects", systemImage: "folder") }
            .tag(Tab.projects)
        }
        .task { await push.requestAuthorizationAndRegister() }
        .onChange(of: push.pendingMonitorID) { _, id in
            guard let id else { return }
            selection = .monitors
            monitorsPath.append(id)
            push.pendingMonitorID = nil
        }
    }

    @ToolbarContentBuilder
    private var signOutButton: some ToolbarContent {
        ToolbarItem(placement: .topBarTrailing) {
            Button("Sign Out", role: .destructive) {
                Task {
                    await push.unregisterCurrentDevice()
                    await container.session.signOut()
                }
            }
        }
    }
}

extension View {
    /// Routes a `String` navigation value (a monitor id) to the monitor detail.
    /// Applied to every tab's stack so a monitor opens the same way everywhere.
    func monitorDetailDestination() -> some View {
        navigationDestination(for: String.self) { monitorID in
            MonitorDetailView(monitorID: monitorID)
        }
    }
}
