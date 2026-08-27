import SwiftUI

/// Top-level gate: signed-in users see the app; everyone else sees sign-in.
struct RootView: View {
    @Environment(SessionStore.self) private var session

    var body: some View {
        if session.isAuthenticated {
            MainTabView()
        } else {
            LoginView()
        }
    }
}
