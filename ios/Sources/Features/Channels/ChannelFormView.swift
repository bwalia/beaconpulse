import SwiftUI

/// Create or edit a notification channel. Fields adapt to the channel type. On
/// edit the secret is never prefilled (it isn't returned) — leaving it blank
/// keeps the stored credential.
struct ChannelFormView: View {
    enum Mode {
        case create
        case edit(Channel)
    }

    let mode: Mode
    let onSaved: () -> Void

    @Environment(AppContainer.self) private var container
    @Environment(\.dismiss) private var dismiss

    @State private var name = ""
    @State private var type = "telegram"
    @State private var enabled = true
    @State private var secret = ""

    // Config fields (only the ones for the selected type are used).
    @State private var chatID = ""
    @State private var emailHost = ""
    @State private var emailPort = ""
    @State private var emailFrom = ""
    @State private var emailTo = ""
    @State private var emailUsername = ""
    @State private var emailSecurity = "starttls"
    @State private var webhookURL = ""
    @State private var webhookMethod = "POST"

    @State private var isSaving = false
    @State private var errorMessage: String?

    /// The types the app can create (those with a working notifier server-side).
    private static let creatableTypes = ["telegram", "slack", "email", "webhook"]

    init(mode: Mode, onSaved: @escaping () -> Void) {
        self.mode = mode
        self.onSaved = onSaved
        if case let .edit(channel) = mode {
            _name = State(initialValue: channel.name)
            _type = State(initialValue: channel.type)
            _enabled = State(initialValue: channel.enabled)
            let c = channel.config
            _chatID = State(initialValue: c["chat_id"] ?? "")
            _emailHost = State(initialValue: c["host"] ?? "")
            _emailPort = State(initialValue: c["port"] ?? "")
            _emailFrom = State(initialValue: c["from"] ?? "")
            _emailTo = State(initialValue: c["to"] ?? "")
            _emailUsername = State(initialValue: c["username"] ?? "")
            _emailSecurity = State(initialValue: c["security"] ?? "starttls")
            _webhookURL = State(initialValue: c["url"] ?? "")
            _webhookMethod = State(initialValue: c["method"] ?? "POST")
        }
    }

    private var isCreate: Bool { if case .create = mode { return true } else { return false } }

    var body: some View {
        NavigationStack {
            Form {
                Section {
                    TextField("Name", text: $name)
                    if isCreate {
                        Picker("Type", selection: $type) {
                            ForEach(Self.creatableTypes, id: \.self) { Text($0.capitalized).tag($0) }
                        }
                    } else {
                        LabeledContent("Type", value: type.capitalized)
                    }
                    Toggle("Enabled", isOn: $enabled)
                }

                typeFields

                if let errorMessage {
                    Section { Text(errorMessage).foregroundStyle(.red) }
                }
            }
            .navigationTitle(isCreate ? "New Channel" : "Edit Channel")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) { Button("Cancel") { dismiss() } }
                ToolbarItem(placement: .confirmationAction) {
                    if isSaving { ProgressView() }
                    else { Button("Save") { Task { await save() } }.disabled(!canSave) }
                }
            }
        }
    }

    @ViewBuilder
    private var typeFields: some View {
        switch type {
        case "telegram":
            Section("Telegram") {
                TextField("Chat ID", text: $chatID).keyboardType(.numbersAndPunctuation)
                secretField("Bot token")
            }
        case "slack":
            Section("Slack") {
                secretField("Incoming webhook URL")
            }
        case "email":
            Section("SMTP") {
                TextField("Host", text: $emailHost).textInputAutocapitalization(.never).autocorrectionDisabled()
                TextField("Port", text: $emailPort).keyboardType(.numberPad)
                Picker("Security", selection: $emailSecurity) {
                    Text("STARTTLS").tag("starttls")
                    Text("TLS").tag("tls")
                    Text("None").tag("none")
                }
                TextField("Username", text: $emailUsername).textInputAutocapitalization(.never).autocorrectionDisabled()
                secretField("Password")
            }
            Section("Message") {
                TextField("From", text: $emailFrom).textInputAutocapitalization(.never).autocorrectionDisabled().keyboardType(.emailAddress)
                TextField("To", text: $emailTo).textInputAutocapitalization(.never).autocorrectionDisabled().keyboardType(.emailAddress)
            }
        case "webhook":
            Section("Webhook") {
                TextField("URL", text: $webhookURL).textInputAutocapitalization(.never).autocorrectionDisabled().keyboardType(.URL)
                Picker("Method", selection: $webhookMethod) {
                    Text("POST").tag("POST")
                    Text("PUT").tag("PUT")
                }
            }
        default:
            EmptyView()
        }
    }

    @ViewBuilder
    private func secretField(_ label: String) -> some View {
        SecureField(isCreate ? label : "\(label) (leave blank to keep)", text: $secret)
            .textInputAutocapitalization(.never)
            .autocorrectionDisabled()
    }

    private var canSave: Bool {
        guard !name.trimmed.isEmpty else { return false }
        switch type {
        case "telegram":
            return !chatID.trimmed.isEmpty && (!isCreate || !secret.trimmed.isEmpty)
        case "slack":
            return !isCreate || !secret.trimmed.isEmpty
        case "email":
            return !emailHost.trimmed.isEmpty && !emailFrom.trimmed.isEmpty && !emailTo.trimmed.isEmpty
        case "webhook":
            return !webhookURL.trimmed.isEmpty
        default:
            return false
        }
    }

    private func configAndSecret() -> (config: [String: String], secret: String) {
        switch type {
        case "telegram":
            return (["chat_id": chatID.trimmed], secret.trimmed)
        case "slack":
            return ([:], secret.trimmed)
        case "email":
            var config = ["host": emailHost.trimmed, "from": emailFrom.trimmed,
                          "to": emailTo.trimmed, "security": emailSecurity]
            if !emailPort.trimmed.isEmpty { config["port"] = emailPort.trimmed }
            if !emailUsername.trimmed.isEmpty { config["username"] = emailUsername.trimmed }
            return (config, secret.trimmed)
        case "webhook":
            return (["url": webhookURL.trimmed, "method": webhookMethod], "")
        default:
            return ([:], "")
        }
    }

    private func save() async {
        isSaving = true
        errorMessage = nil
        defer { isSaving = false }

        let (config, secretValue) = configAndSecret()
        do {
            switch mode {
            case .create:
                _ = try await container.channels.create(CreateChannelBody(
                    name: name.trimmed, type: type, enabled: enabled, config: config, secret: secretValue))
            case let .edit(channel):
                // Send the secret only if the user entered a new one.
                let secretToSend = secretValue.isEmpty ? nil : secretValue
                _ = try await container.channels.update(id: channel.id, UpdateChannelBody(
                    name: name.trimmed, enabled: enabled, config: config, secret: secretToSend))
            }
            onSaved()
            dismiss()
        } catch {
            errorMessage = (error as? APIError)?.errorDescription ?? "Couldn’t save the channel."
        }
    }
}
