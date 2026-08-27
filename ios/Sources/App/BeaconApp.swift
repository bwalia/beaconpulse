import GoogleSignIn
import SwiftUI

@main
struct BeaconApp: App {
    @UIApplicationDelegateAdaptor(AppDelegate.self) private var appDelegate
    @State private var container = AppContainer()

    var body: some Scene {
        WindowGroup {
            RootView()
                .environment(container)
                .environment(container.session)
                .environment(container.push)
                .tint(AppConfig.current.accentColor)
                .onAppear { appDelegate.attach(push: container.push) }
                .onOpenURL { GIDSignIn.sharedInstance.handle($0) }
        }
    }
}

/// Composition root: builds the object graph once and hands it to the views. Two
/// API clients — an unauthenticated one for the auth endpoints, and an
/// authenticated one (bearer + transparent refresh) for everything else.
@MainActor
@Observable
final class AppContainer {
    let session: SessionStore
    let authService: AuthService
    let apiClient: APIClient
    let push: PushManager
    let monitors: MonitorService
    let channels: ChannelService

    init() {
        let config = AppConfig.current
        let unauthClient = APIClient(baseURL: config.apiBaseURL)
        let authService = AuthService(client: unauthClient)
        let session = SessionStore(auth: authService)
        let apiClient = APIClient(baseURL: config.apiBaseURL, auth: session)

        self.session = session
        self.authService = authService
        self.apiClient = apiClient
        self.push = PushManager(client: apiClient, session: session)
        self.monitors = MonitorService(client: apiClient)
        self.channels = ChannelService(client: apiClient)

        // Configure Google Sign-In only when this brand ships a client id.
        if let clientID = config.googleClientID {
            GIDSignIn.sharedInstance.configuration = GIDConfiguration(clientID: clientID)
        }
    }
}
