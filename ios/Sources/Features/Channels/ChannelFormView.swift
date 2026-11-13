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
            FormScreen {
                FormCard("Channel") {
                    Field("Name") {
                        TextField("e.g. On-call team", text: $name).inputStyle()
                    }
                    Field("Type") {
                        if isCreate {
                            ChoiceChips(choices: Self.typeChoices, selection: $type, minWidth: 110)
                        } else {
                            ReadOnlyValue(type.capitalized)
                        }
                    }
                    SwitchRow(title: "Enabled", subtitle: "Send alerts to this channel", isOn: $enabled)
                }

                typeFields

                if let errorMessage { ErrorBanner(errorMessage) }
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

    /// The types the app can create (those with a working notifier server-side).
    private static let typeChoices = [
        Choice("telegram", "Telegram", icon: "paperplane.fill"),
        Choice("slack", "Slack", icon: "number"),
        Choice("email", "Email", icon: "envelope.fill"),
        Choice("webhook", "Webhook", icon: "link"),
    ]

    @ViewBuilder
    private var typeFields: some View {
        switch type {
        case "telegram":
            FormCard("Telegram", footer: "Message @BotFather to create a bot, then add it to your chat or group.") {
                Field("Chat ID", hint: "A number like 123456789, or -100… for a group.") {
                    TextField("123456789", text: $chatID).keyboardType(.numbersAndPunctuation).inputStyle()
                }
                Field("Bot token") { secretField("123456:ABC-DEF…") }
            }
        case "slack":
            FormCard("Slack") {
                Field("Incoming webhook URL", hint: "From your Slack app’s Incoming Webhooks page.") {
                    secretField("https://hooks.slack.com/services/…")
                }
            }
        case "email":
            FormCard("SMTP server") {
                Field("Host") {
                    TextField("smtp.example.com", text: $emailHost)
                        .textInputAutocapitalization(.never).autocorrectionDisabled().inputStyle()
                }
                Field("Port") {
                    TextField("587", text: $emailPort).keyboardType(.numberPad).inputStyle()
                }
                Field("Security") {
                    ChoiceChips(choices: [Choice("starttls", "STARTTLS"), Choice("tls", "TLS"), Choice("none", "None")],
                                selection: $emailSecurity, minWidth: 90)
                }
                Field("Username") {
                    TextField("Optional", text: $emailUsername)
                        .textInputAutocapitalization(.never).autocorrectionDisabled().inputStyle()
                }
                Field("Password") { secretField("Optional") }
            }
            FormCard("Message") {
                Field("From") {
                    TextField("alerts@example.com", text: $emailFrom)
                        .textInputAutocapitalization(.never).autocorrectionDisabled()
                        .keyboardType(.emailAddress).inputStyle()
                }
                Field("To") {
                    TextField("oncall@example.com", text: $emailTo)
                        .textInputAutocapitalization(.never).autocorrectionDisabled()
                        .keyboardType(.emailAddress).inputStyle()
                }
            }
        case "webhook":
            FormCard("Webhook") {
                Field("URL") {
                    TextField("https://example.com/hooks/alerts", text: $webhookURL)
                        .textInputAutocapitalization(.never).autocorrectionDisabled()
                        .keyboardType(.URL).inputStyle()
                }
                Field("Method") {
                    ChoiceChips(choices: [Choice("POST", "POST"), Choice("PUT", "PUT")],
                                selection: $webhookMethod, minWidth: 90)
                }
            }
        default:
            EmptyView()
        }
    }

    /// The type's secret. On edit it's never shown — blank keeps the stored one.
    private func secretField(_ placeholder: String) -> some View {
        SecureField(isCreate ? placeholder : "Leave blank to keep the saved value", text: $secret)
            .textInputAutocapitalization(.never)
            .autocorrectionDisabled()
            .inputStyle()
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
