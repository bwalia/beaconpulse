import SwiftUI

/// Top-level gate: signed-in users see the app; everyone else sees sign-in.
struct RootView: View {
    @Environment(SessionStore.self) private var session

    var body: some View {
        if session.isAuthenticated {
            HomeView()
        } else {
            LoginView()
        }
    }
}

/// The signed-in shell. Phase 1 shows the monitors list; later phases add tabs
/// (overview, alerts, settings) and an iPad split layout.
struct HomeView: View {
    @Environment(AppContainer.self) private var container
    @Environment(PushManager.self) private var push
    @State private var path = NavigationPath()

    var body: some View {
        NavigationStack(path: $path) {
            MonitorsListView()
                .navigationTitle("Monitors")
                .toolbar {
                    ToolbarItem(placement: .topBarTrailing) {
                        Button("Sign Out", role: .destructive) {
                            Task {
                                await push.unregisterCurrentDevice()
                                await container.session.signOut()
                            }
                        }
                    }
                }
                .navigationDestination(for: String.self) { monitorID in
                    MonitorDetailView(monitorID: monitorID)
                }
        }
        .task { await push.requestAuthorizationAndRegister() }
        .onChange(of: push.pendingMonitorID) { _, newValue in
            guard let id = newValue else { return }
            path.append(id)
            push.pendingMonitorID = nil
        }
    }
}
