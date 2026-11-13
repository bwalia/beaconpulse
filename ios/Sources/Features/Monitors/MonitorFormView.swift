import SwiftUI

/// Create or edit a monitor. The per-type fields match the web form. On edit the
/// form is prefilled from the fetched record and the full `settings` object is
/// round-tripped, so anything this form doesn't show (e.g. DNS expected IPs)
/// survives an edit here.
struct MonitorFormView: View {
    enum Mode {
        case create
        case edit(Monitor)
    }

    let mode: Mode
    let onSaved: () -> Void

    @Environment(AppContainer.self) private var container
    @Environment(\.dismiss) private var dismiss

    @State private var name = ""
    @State private var type = "https"
    @State private var target = ""
    @State private var projectID = ""
    @State private var intervalSeconds = 60
    @State private var timeoutSeconds = 10
    @State private var graceSeconds = 60
    @State private var enabled = true
    @State private var alertSensitivity = "balanced"
    @State private var settings = MonitorSettings.empty
    // Edited as text and parsed on save, so typing "200, " isn't reformatted
    // mid-keystroke.
    @State private var statusCodesText = ""
    @State private var headersText = ""

    @State private var usage: MonitorUsage?
    @State private var projects: [Project] = []
    @State private var isSaving = false
    @State private var errorMessage: String?

    init(mode: Mode, onSaved: @escaping () -> Void) {
        self.mode = mode
        self.onSaved = onSaved
        if case let .edit(m) = mode {
            _name = State(initialValue: m.name)
            _type = State(initialValue: m.type)
            _target = State(initialValue: m.target)
            _projectID = State(initialValue: m.projectId ?? "")
            _intervalSeconds = State(initialValue: m.intervalSeconds)
            _timeoutSeconds = State(initialValue: m.timeoutSeconds)
            _graceSeconds = State(initialValue: m.graceSeconds ?? 60)
            _enabled = State(initialValue: m.enabled)
            _alertSensitivity = State(initialValue: m.settings.alertSensitivity ?? "balanced")
            _settings = State(initialValue: m.settings)
            _statusCodesText = State(initialValue: (m.settings.validStatusCodes ?? []).map(String.init).joined(separator: ", "))
            _headersText = State(initialValue: MonitorSettings.headerLines(m.settings.headers))
        }
    }

    private var isCreate: Bool { if case .create = mode { return true } else { return false } }
    private var kind: MonitorKind { MonitorKind(rawValue: type) ?? .https }

    var body: some View {
        NavigationStack {
            FormScreen {
                FormCard("Details") {
                    Field("Name") {
                        TextField("e.g. Marketing site", text: $name).inputStyle()
                    }
                    Field("Type") {
                        if isCreate {
                            ChoiceChips(choices: MonitorKind.creatable.map {
                                Choice($0.rawValue, $0.rawValue.uppercased(),
                                       icon: Theme.symbol(forMonitorType: $0.rawValue))
                            }, selection: $type, minWidth: 104)
                        } else {
                            ReadOnlyValue(type.uppercased())
                        }
                    }
                    if isCreate {
                        Field("Project") {
                            Picker("Project", selection: $projectID) {
                                Text("Select…").tag("")
                                ForEach(projects) { Text($0.name).tag($0.id) }
                            }
                            .menuInputStyle()
                        }
                    }
                    if kind.needsTarget {
                        Field("Target") {
                            TextField(targetHint, text: $target)
                                .textInputAutocapitalization(.never)
                                .autocorrectionDisabled()
                                .keyboardType(.URL)
                                .font(.body.monospaced())
                                .inputStyle()
                        }
                    }
                }

                FormCard("Checks", footer: planNote) {
                    if kind != .githubActions {
                        StepperRow(title: "Check every", value: $intervalSeconds,
                                   range: minInterval...86400, step: minInterval >= 60 ? 60 : 10,
                                   format: Self.duration)
                        if kind.isProbed {
                            StepperRow(title: "Timeout", value: $timeoutSeconds, range: 1...300) { "\($0)s" }
                        }
                        if kind == .heartbeat {
                            // The server sets grace only at creation.
                            if isCreate {
                                StepperRow(title: "Grace period", value: $graceSeconds, range: 0...86400,
                                           step: 30) { "\($0)s" }
                            } else {
                                Field("Grace period") { ReadOnlyValue("\(graceSeconds)s") }
                            }
                        }
                        Field("Alert sensitivity", hint: sensitivityHint) {
                            ChoiceChips(choices: [Choice("immediate", "Immediate", icon: "bolt.fill"),
                                                  Choice("balanced", "Balanced", icon: "scale.3d"),
                                                  Choice("relaxed", "Relaxed", icon: "leaf.fill")],
                                        selection: $alertSensitivity, minWidth: 104)
                        }
                    }
                    SwitchRow(title: "Enabled", subtitle: "Run checks and send alerts", isOn: $enabled)
                }

                if isCreate, let usage, usage.monitorsUsed >= usage.monitorsLimit {
                    ErrorBanner("You’ve reached your plan’s limit of \(usage.monitorsLimit) monitors. Delete one to add another.")
                }

                if kind.takesRequestSettings { requestSections }

                if kind == .dns {
                    FormCard("DNS") {
                        Field("Record type") {
                            ChoiceChips(choices: ["A", "AAAA", "CNAME", "MX", "TXT", "NS", "CAA"].map { Choice($0, $0) },
                                        selection: $settings.dnsQueryType.or("A"), minWidth: 70)
                        }
                    }
                }

                if kind == .githubActions {
                    FormCard("GitHub Actions",
                             footer: "Leave the filter blank to alert on any workflow. Rotate the ingest token on the web.") {
                        Field("Workflow filter") {
                            TextField("Optional, e.g. deploy.yml", text: $settings.githubWorkflow.or(""))
                                .textInputAutocapitalization(.never)
                                .autocorrectionDisabled()
                                .inputStyle()
                        }
                        SwitchRow(title: "Notify on success", subtitle: "Also send a message for every successful run",
                                  isOn: $settings.githubNotifyOnSuccess.or(false))
                    }
                }

                if let errorMessage { ErrorBanner(errorMessage) }
            }
            .navigationTitle(isCreate ? "New Monitor" : "Edit Monitor")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") { dismiss() }
                }
                ToolbarItem(placement: .confirmationAction) {
                    if isSaving {
                        ProgressView()
                    } else {
                        Button("Save") { Task { await save() } }.disabled(!canSave)
                    }
                }
            }
            .task {
                await loadUsage()
                await loadProjects()
            }
        }
    }

    private var planNote: String? {
        guard let usage, kind != .githubActions, usage.minIntervalSeconds > 10 else { return nil }
        return "Your plan checks at most every \(Self.duration(usage.minIntervalSeconds))."
    }

    private var sensitivityHint: String {
        switch alertSensitivity {
        case "immediate": "Alert on the first failed check."
        case "relaxed": "Alert only on prolonged outages (about 5 minutes)."
        default: "Alert on a sustained failure (about 2 checks), ignoring one-off blips."
        }
    }

    /// Method, status codes, headers and response checks for HTTP-family monitors.
    @ViewBuilder private var requestSections: some View {
        FormCard("Request") {
            Field("Method") {
                ChoiceChips(choices: ["GET", "POST", "HEAD", "PUT", "PATCH", "DELETE"].map { Choice($0, $0) },
                            selection: $settings.method.or("GET"), minWidth: 76)
            }
            Field("Expected status codes", hint: "Comma-separated. Blank accepts any 2xx.") {
                TextField("200, 204", text: $statusCodesText)
                    .keyboardType(.numbersAndPunctuation)
                    .inputStyle()
            }
            SwitchRow(title: "Follow redirects", isOn: $settings.followRedirects.or(false))
            SwitchRow(title: "Skip TLS verification", subtitle: "Only for self-signed certificates",
                      isOn: $settings.skipTlsVerify.or(false))
            Field("Request body", hint: "Sent with POST, PUT and PATCH.") {
                TextField("{ }", text: $settings.body.or(""), axis: .vertical)
                    .lineLimit(2...6)
                    .font(.body.monospaced())
                    .textInputAutocapitalization(.never)
                    .autocorrectionDisabled()
                    .inputStyle()
            }
        }

        FormCard("Headers",
                 footer: "Use for auth, e.g. Authorization: Bearer <token>. Saved values are encrypted and shown as ********: leave a line as it is to keep its value, or delete the line to remove the header.") {
            TextField("Name: value, one per line", text: $headersText, axis: .vertical)
                .lineLimit(3...8)
                .font(.body.monospaced())
                .textInputAutocapitalization(.never)
                .autocorrectionDisabled()
                .inputStyle()
        }

        FormCard("Response") {
            Field("Body must contain") {
                TextField("Optional", text: $settings.bodyKeyword.or("")).inputStyle()
            }
            Field("Body must not contain") {
                TextField("Optional", text: $settings.bodyNotKeyword.or("")).inputStyle()
            }
            HStack(alignment: .top, spacing: 12) {
                Field("Slow response (ms)") {
                    TextField("Off", value: $settings.responseTimeWarningMs, format: .number)
                        .keyboardType(.numberPad)
                        .inputStyle()
                }
                Field("SSL expiry warning (days)") {
                    TextField("30", value: $settings.sslExpiryWarningDays, format: .number)
                        .keyboardType(.numberPad)
                        .inputStyle()
                }
            }
        }
    }

    private var targetHint: String {
        switch kind {
        case .http, .https: return "https://example.com"
        case .ssl: return "example.com:443"
        case .tcp: return "host:port"
        case .icmp: return "example.com"
        case .dns: return "example.com"
        case .heartbeat: return ""
        case .githubActions: return "owner/repo"
        }
    }

    /// The plan's fastest allowed interval (10s until the usage call answers).
    private var minInterval: Int { usage?.minIntervalSeconds ?? 10 }

    /// "30 min", "1 hr", "45 sec".
    static func duration(_ seconds: Int) -> String {
        Duration.seconds(seconds).formatted(.units(allowed: [.hours, .minutes, .seconds], width: .abbreviated))
    }

    private var canSave: Bool {
        guard !name.trimmed.isEmpty else { return false }
        if isCreate, let usage, usage.monitorsUsed >= usage.monitorsLimit { return false }
        if kind.needsTarget && target.trimmed.isEmpty { return false }
        if isCreate && projectID.isEmpty { return false }
        return true
    }

    /// Loads the plan limits so the form never offers an interval or a new monitor
    /// the server would refuse.
    private func loadUsage() async {
        usage = try? await container.apiClient.send(.init(path: "/api/v1/monitors/usage"), as: MonitorUsage.self)
        if intervalSeconds < minInterval { intervalSeconds = minInterval }
    }

    private func loadProjects() async {
        guard isCreate else { return }
        let page = try? await container.apiClient.send(
            .init(path: "/api/v1/projects", query: ["limit": "200"]), as: Paginated<Project>.self)
        projects = page?.data ?? []
        if projectID.isEmpty { projectID = projects.first?.id ?? "" }
    }

    private func save() async {
        isSaving = true
        errorMessage = nil
        defer { isSaving = false }

        var settings = settings
        settings.alertSensitivity = alertSensitivity
        if kind.takesRequestSettings {
            settings.validStatusCodes = MonitorSettings.statusCodes(from: statusCodesText)
            settings.headers = MonitorSettings.headers(fromLines: headersText)
        }

        do {
            switch mode {
            case .create:
                _ = try await container.monitors.create(CreateMonitorBody(
                    projectId: projectID,
                    name: name.trimmed,
                    type: type,
                    target: kind.needsTarget ? target.trimmed : "",
                    enabled: enabled,
                    intervalSeconds: intervalSeconds,
                    timeoutSeconds: timeoutSeconds,
                    graceSeconds: kind == .heartbeat ? graceSeconds : nil,
                    settings: settings))
            case let .edit(monitor):
                _ = try await container.monitors.update(id: monitor.id, UpdateMonitorBody(
                    name: name.trimmed,
                    target: kind.needsTarget ? target.trimmed : nil,
                    enabled: enabled,
                    intervalSeconds: intervalSeconds,
                    timeoutSeconds: timeoutSeconds,
                    settings: settings))
            }
            onSaved()
            dismiss()
        } catch {
            errorMessage = (error as? APIError)?.errorDescription ?? "Couldn’t save the monitor."
        }
    }
}

extension String {
    /// Whitespace-trimmed copy, used across the write forms.
    var trimmed: String { trimmingCharacters(in: .whitespacesAndNewlines) }
}

extension Binding {
    /// Drives a non-optional control from an optional setting: nil shows as
    /// `fallback`, and choosing `fallback` (an empty string, false, the server
    /// default) stores nil so the field is omitted, as the web form does.
    func or<T: Equatable>(_ fallback: T) -> Binding<T> where Value == T? {
        Binding<T>(get: { wrappedValue ?? fallback },
                   set: { wrappedValue = $0 == fallback ? nil : $0 })
    }
}
