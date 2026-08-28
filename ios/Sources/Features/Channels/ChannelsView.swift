import SwiftUI

/// Loads and mutates the org's notification channels.
@MainActor
@Observable
final class ChannelsStore {
    private(set) var state: Loadable<[Channel]> = .loading
    @ObservationIgnored private let service: ChannelService

    init(service: ChannelService) { self.service = service }

    func load() async {
        do {
            state = .loaded(try await service.list())
        } catch {
            state = .failed((error as? APIError)?.errorDescription ?? "Couldn’t load channels.")
        }
    }

    func setEnabled(_ channel: Channel, _ enabled: Bool) async throws {
        _ = try await service.update(id: channel.id, UpdateChannelBody(
            name: nil, enabled: enabled, config: nil, secret: nil))
        await load()
    }

    func delete(_ channel: Channel) async throws {
        try await service.delete(id: channel.id)
        await load()
    }

    func sendTest(_ channel: Channel) async throws {
        try await service.sendTest(id: channel.id)
    }
}

/// The channel form is presented for either creating or editing, through a single
/// sheet (SwiftUI allows only one active sheet per view).
enum ChannelSheet: Identifiable {
    case create
    case edit(Channel)
    var id: String {
        switch self {
        case .create: return "create"
        case let .edit(channel): return channel.id
        }
    }
}

struct ChannelsView: View {
    @Environment(AppContainer.self) private var container
    @State private var store: ChannelsStore?
    @State private var sheet: ChannelSheet?
    @State private var banner: String?

    var body: some View {
        LoadableView(state: store?.state ?? .loading, retry: { Task { await store?.load() } }) { channels in
            List {
                if channels.isEmpty {
                    ContentUnavailableView("No channels", systemImage: "bell.slash",
                                           description: Text("Add a channel to receive alerts."))
                } else {
                    ForEach(channels) { channel in
                        ChannelRow(channel: channel) { newValue in
                            await run { try await store?.setEnabled(channel, newValue) }
                        }
                        .contentShape(Rectangle())
                        .onTapGesture { if !channel.isManaged { sheet = .edit(channel) } }
                        .swipeActions(edge: .leading) {
                            Button { Task { await test(channel) } } label: { Label("Test", systemImage: "paperplane") }
                                .tint(.blue)
                        }
                        .swipeActions(edge: .trailing) {
                            if !channel.isManaged {
                                Button(role: .destructive) {
                                    Task { await run { try await store?.delete(channel) } }
                                } label: {
                                    Label("Delete", systemImage: "trash")
                                }
                            }
                        }
                    }
                }
            }
            .refreshable { await store?.load() }
        }
        .navigationTitle("Notification Channels")
        .toolbar {
            ToolbarItem(placement: .topBarTrailing) {
                Button { sheet = .create } label: { Image(systemName: "plus") }
                    .accessibilityLabel("Add channel")
            }
        }
        .task {
            if store == nil { store = ChannelsStore(service: container.channels) }
            await store?.load()
        }
        .sheet(item: $sheet) { which in
            switch which {
            case .create:
                ChannelFormView(mode: .create) { Task { await store?.load() } }
            case let .edit(channel):
                ChannelFormView(mode: .edit(channel)) { Task { await store?.load() } }
            }
        }
        .alert("Notifications", isPresented: Binding(get: { banner != nil }, set: { if !$0 { banner = nil } })) {
            Button("OK") { banner = nil }
        } message: {
            Text(banner ?? "")
        }
    }

    private func test(_ channel: Channel) async {
        do {
            try await store?.sendTest(channel)
            banner = "Test notification sent."
        } catch {
            banner = (error as? APIError)?.errorDescription ?? "Test failed."
        }
    }

    /// Runs a throwing action, surfacing a failure in the banner.
    private func run(_ operation: () async throws -> Void) async {
        do {
            try await operation()
        } catch {
            banner = (error as? APIError)?.errorDescription ?? "Something went wrong."
        }
    }
}

struct ChannelRow: View {
    let channel: Channel
    let onToggle: (Bool) async -> Void

    var body: some View {
        HStack(spacing: 12) {
            Image(systemName: icon).foregroundStyle(.secondary).frame(width: 24)
            VStack(alignment: .leading, spacing: 2) {
                Text(channel.name).font(.headline)
                Text(subtitle).font(.caption).foregroundStyle(.secondary)
            }
            Spacer()
            Toggle("", isOn: Binding(get: { channel.enabled }, set: { value in Task { await onToggle(value) } }))
                .labelsHidden()
                .accessibilityLabel("\(channel.name), notifications")
        }
        .padding(.vertical, 2)
    }

    private var icon: String {
        switch channel.type {
        case "telegram": return "paperplane.fill"
        case "slack": return "number"
        case "email": return "envelope.fill"
        case "webhook": return "link"
        case "apns": return "apple.logo"
        default: return "bell.fill"
        }
    }

    private var subtitle: String {
        channel.isManaged ? "Apple Push · this org’s devices" : channel.type.capitalized
    }
}
