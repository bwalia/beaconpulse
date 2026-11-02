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
            Form {
                Section("Details") {
                    TextField("Name", text: $name)

                    if isCreate {
                        Picker("Type", selection: $type) {
                            ForEach(MonitorKind.creatable) { Text($0.rawValue.uppercased()).tag($0.rawValue) }
                        }
                        Picker("Project", selection: $projectID) {
                            Text("Select…").tag("")
                            ForEach(projects) { Text($0.name).tag($0.id) }
                        }
                    } else {
                        LabeledContent("Type", value: type.uppercased())
                    }

                    if kind.needsTarget {
                        TextField(targetHint, text: $target)
                            .textInputAutocapitalization(.never)
                            .autocorrectionDisabled()
                            .keyboardType(.URL)
                    }
                }

                Section("Checks") {
                    if kind != .githubActions {
                        Stepper("Interval: \(intervalSeconds)s", value: $intervalSeconds, in: 10...86400, step: 10)
                        if kind.isProbed {
                            Stepper("Timeout: \(timeoutSeconds)s", value: $timeoutSeconds, in: 1...300)
                        }
                        if kind == .heartbeat {
                            // The server sets grace only at creation.
                            if isCreate {
                                Stepper("Grace: \(graceSeconds)s", value: $graceSeconds, in: 0...86400, step: 30)
                            } else {
                                LabeledContent("Grace", value: "\(graceSeconds)s")
                            }
                        }
                        Picker("Alert sensitivity", selection: $alertSensitivity) {
                            Text("Immediate").tag("immediate")
                            Text("Balanced").tag("balanced")
                            Text("Relaxed").tag("relaxed")
                        }
                    }
                    Toggle("Enabled", isOn: $enabled)
                }

                if kind.takesRequestSettings { requestSections }

                if kind == .dns {
                    Section("DNS") {
                        Picker("Record type", selection: $settings.dnsQueryType.or("A")) {
                            ForEach(["A", "AAAA", "CNAME", "MX", "TXT", "NS", "CAA"], id: \.self) { Text($0) }
                        }
                    }
                }

                if kind == .githubActions {
                    Section {
                        TextField("Workflow filter (optional)", text: $settings.githubWorkflow.or(""))
                            .textInputAutocapitalization(.never)
                            .autocorrectionDisabled()
                        Toggle("Also notify on every successful run", isOn: $settings.githubNotifyOnSuccess.or(false))
                    } header: {
                        Text("GitHub Actions")
                    } footer: {
                        Text("Leave the filter blank to alert on any workflow. Rotate the ingest token on the web.")
                    }
                }

                if let errorMessage {
                    Section { Text(errorMessage).foregroundStyle(.red) }
                }
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
            .task { await loadProjects() }
        }
    }

    /// Method, status codes, headers and response checks for HTTP-family monitors.
    @ViewBuilder private var requestSections: some View {
        Section("Request") {
            Picker("Method", selection: $settings.method.or("GET")) {
                ForEach(["GET", "POST", "HEAD", "PUT", "PATCH", "DELETE"], id: \.self) { Text($0) }
            }
            TextField("Expected status codes, e.g. 200, 204", text: $statusCodesText)
                .keyboardType(.numbersAndPunctuation)
            Toggle("Follow redirects", isOn: $settings.followRedirects.or(false))
            Toggle("Skip TLS verification", isOn: $settings.skipTlsVerify.or(false))
            TextField("Request body (for POST/PUT/PATCH)", text: $settings.body.or(""), axis: .vertical)
                .lineLimit(1...6)
                .font(.body.monospaced())
                .textInputAutocapitalization(.never)
                .autocorrectionDisabled()
        }

        Section {
            TextField("Name: value, one per line", text: $headersText, axis: .vertical)
                .lineLimit(2...8)
                .font(.body.monospaced())
                .textInputAutocapitalization(.never)
                .autocorrectionDisabled()
        } header: {
            Text("Headers")
        } footer: {
            Text("Use for auth, e.g. Authorization: Bearer <token>. Saved values are encrypted and shown as ********: leave a line as it is to keep its value, or delete the line to remove the header.")
        }

        Section("Response") {
            TextField("Body must contain", text: $settings.bodyKeyword.or(""))
            TextField("Body must NOT contain", text: $settings.bodyNotKeyword.or(""))
            LabeledContent("Slow-response alert (ms)") {
                TextField("Off", value: $settings.responseTimeWarningMs, format: .number)
                    .multilineTextAlignment(.trailing)
                    .keyboardType(.numberPad)
            }
            LabeledContent("SSL expiry warning (days)") {
                TextField("30", value: $settings.sslExpiryWarningDays, format: .number)
                    .multilineTextAlignment(.trailing)
                    .keyboardType(.numberPad)
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

    private var canSave: Bool {
        guard !name.trimmed.isEmpty else { return false }
        if kind.needsTarget && target.trimmed.isEmpty { return false }
        if isCreate && projectID.isEmpty { return false }
        return true
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
