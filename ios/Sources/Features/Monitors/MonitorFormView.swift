import SwiftUI

/// Create or edit a monitor. On edit the form is prefilled from the fetched
/// record and the full `settings` object is round-tripped, so advanced settings
/// configured elsewhere (headers, status codes, DNS) survive an edit here. Only
/// the common fields are surfaced; the rest ride along untouched.
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
    @State private var baseSettings = MonitorSettings.empty

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
            _baseSettings = State(initialValue: m.settings)
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
                            ForEach(MonitorKind.allCases) { Text($0.rawValue.uppercased()).tag($0.rawValue) }
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
                    Stepper("Interval: \(intervalSeconds)s", value: $intervalSeconds, in: 10...86400, step: 10)
                    Stepper("Timeout: \(timeoutSeconds)s", value: $timeoutSeconds, in: 1...300)
                    if kind == .heartbeat {
                        Stepper("Grace: \(graceSeconds)s", value: $graceSeconds, in: 0...86400, step: 30)
                    }
                    Picker("Alert sensitivity", selection: $alertSensitivity) {
                        Text("Immediate").tag("immediate")
                        Text("Balanced").tag("balanced")
                        Text("Relaxed").tag("relaxed")
                    }
                    Toggle("Enabled", isOn: $enabled)
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

    private var targetHint: String {
        switch kind {
        case .http, .https: return "https://example.com"
        case .ssl: return "example.com:443"
        case .tcp: return "host:port"
        case .icmp: return "example.com"
        case .dns: return "example.com"
        case .heartbeat: return ""
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

        var settings = baseSettings
        settings.alertSensitivity = alertSensitivity

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
