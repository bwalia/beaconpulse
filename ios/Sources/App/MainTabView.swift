import SwiftUI

/// The signed-in shell: five sections, each its own navigation stack (tabs on
/// iPhone/iPad, a sidebar on the Mac).
/// A tapped push notification switches to the Monitors tab and deep-links to the
/// monitor. Sign-out (which also unregisters this device from push) lives on the
/// Overview tab.
struct MainTabView: View {
    @Environment(PushManager.self) private var push

    @State private var selection: Tab = .overview
    @State private var monitorsPath = NavigationPath()

    enum Tab: Hashable, CaseIterable {
        case overview, monitors, alerts, projects, settings

        var title: String {
            switch self {
            case .overview: "Overview"
            case .monitors: "Monitors"
            case .alerts: "Alerts"
            case .projects: "Projects"
            case .settings: "Settings"
            }
        }

        var symbol: String {
            switch self {
            case .overview: "chart.bar.xaxis"
            case .monitors: "dot.radiowaves.left.and.right"
            case .alerts: "bell.badge"
            case .projects: "folder"
            case .settings: "gearshape"
            }
        }
    }

    var body: some View {
        shell
            .task { await push.requestAuthorizationAndRegister() }
            .onChange(of: push.pendingMonitorID) { _, id in
                guard let id else { return }
                selection = .monitors
                monitorsPath.append(id)
                push.pendingMonitorID = nil
            }
    }

    /// Tabs on iPhone and iPad; the Mac's usual sidebar on macOS. Same screens.
    @ViewBuilder
    private var shell: some View {
        #if os(macOS)
        NavigationSplitView {
            List(selection: Binding($selection)) {
                Section {
                    ForEach(Tab.allCases, id: \.self) { tab in
                        Label(tab.title, systemImage: tab.symbol).tag(tab)
                    }
                } header: {
                    HStack(spacing: 8) {
                        BrandTile(size: 26)
                        Text(AppConfig.current.brandDisplayName)
                            .font(.headline)
                            .foregroundStyle(.primary)
                    }
                    .padding(.bottom, 10)
                }
            }
            .navigationSplitViewColumnWidth(min: 180, ideal: 200)
        } detail: {
            stack(for: selection)
        }
        #else
        TabView(selection: $selection) {
            ForEach(Tab.allCases, id: \.self) { tab in
                stack(for: tab)
                    .tabItem { Label(tab.title, systemImage: tab.symbol) }
                    .tag(tab)
            }
        }
        #endif
    }

    @ViewBuilder
    private func stack(for tab: Tab) -> some View {
        switch tab {
        case .overview:
            NavigationStack {
                OverviewView()
                    .navigationTitle("Overview")
                    .monitorDetailDestination()
            }
        case .monitors:
            NavigationStack(path: $monitorsPath) {
                MonitorsListView()
                    .navigationTitle("Monitors")
                    .monitorDetailDestination()
            }
        case .alerts:
            NavigationStack {
                AlertsView()
                    .navigationTitle("Alerts")
                    .monitorDetailDestination()
            }
        case .projects:
            NavigationStack {
                ProjectsView()
                    .navigationTitle("Projects")
                    .navigationDestination(for: Project.self) { project in
                        MonitorsListView(projectID: project.id)
                            .navigationTitle(project.name)
                    }
                    .monitorDetailDestination()
            }
        case .settings:
            NavigationStack {
                SettingsView()
                    .navigationTitle("Settings")
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
