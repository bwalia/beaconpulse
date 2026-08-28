import SwiftUI

/// Create or edit a maintenance window. Scope is either the whole organization or
/// a chosen set of monitors (project scope is web-only for now). While a window is
/// active, alerts for what it covers are suppressed.
struct MaintenanceFormView: View {
    enum Mode {
        case create
        case edit(MaintenanceWindow)
    }

    let mode: Mode
    let onSaved: () -> Void

    @Environment(AppContainer.self) private var container
    @Environment(\.dismiss) private var dismiss

    @State private var title = ""
    @State private var details = ""
    @State private var startsAt = Date()
    @State private var endsAt = Date().addingTimeInterval(3600)
    @State private var scope = "org"
    @State private var selectedMonitorIDs: Set<String> = []

    @State private var monitors: [Monitor] = []
    @State private var isSaving = false
    @State private var errorMessage: String?

    init(mode: Mode, onSaved: @escaping () -> Void) {
        self.mode = mode
        self.onSaved = onSaved
        if case let .edit(window) = mode {
            _title = State(initialValue: window.title)
            _details = State(initialValue: window.description)
            _startsAt = State(initialValue: window.startsAt)
            _endsAt = State(initialValue: window.endsAt)
            // A window scoped to specific projects can't be edited to monitor scope
            // here; keep whatever scope it had, defaulting the picker sensibly.
            _scope = State(initialValue: window.scope == "monitor" ? "monitor" : "org")
            _selectedMonitorIDs = State(initialValue: Set(window.scope == "monitor" ? window.scopeIds : []))
        }
    }

    private var isCreate: Bool { if case .create = mode { return true } else { return false } }

    var body: some View {
        NavigationStack {
            Form {
                Section {
                    TextField("Title", text: $title)
                    TextField("Notes (optional)", text: $details, axis: .vertical)
                }

                Section("When") {
                    DatePicker("Starts", selection: $startsAt)
                    DatePicker("Ends", selection: $endsAt, in: startsAt...)
                }

                Section("What it covers") {
                    Picker("Scope", selection: $scope) {
                        Text("Whole organization").tag("org")
                        Text("Specific monitors").tag("monitor")
                    }
                    .pickerStyle(.segmented)
                }

                if scope == "monitor" {
                    Section("Monitors") {
                        if monitors.isEmpty {
                            Text("Loading…").foregroundStyle(.secondary)
                        } else {
                            ForEach(monitors) { monitor in
                                Button {
                                    toggle(monitor.id)
                                } label: {
                                    HStack {
                                        Text(monitor.name)
                                        Spacer()
                                        if selectedMonitorIDs.contains(monitor.id) {
                                            Image(systemName: "checkmark").foregroundStyle(.tint)
                                        }
                                    }
                                    .contentShape(Rectangle())
                                }
                                .buttonStyle(.plain)
                            }
                        }
                    }
                }

                if let errorMessage {
                    Section { Text(errorMessage).foregroundStyle(.red) }
                }
            }
            .navigationTitle(isCreate ? "New Window" : "Edit Window")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) { Button("Cancel") { dismiss() } }
                ToolbarItem(placement: .confirmationAction) {
                    if isSaving { ProgressView() }
                    else { Button("Save") { Task { await save() } }.disabled(!canSave) }
                }
            }
            .task { await loadMonitors() }
        }
    }

    private var canSave: Bool {
        guard !title.trimmed.isEmpty, endsAt > startsAt else { return false }
        if scope == "monitor" && selectedMonitorIDs.isEmpty { return false }
        return true
    }

    private func toggle(_ id: String) {
        if selectedMonitorIDs.contains(id) { selectedMonitorIDs.remove(id) }
        else { selectedMonitorIDs.insert(id) }
    }

    private func loadMonitors() async {
        guard monitors.isEmpty else { return }
        let page = try? await container.apiClient.send(
            .init(path: "/api/v1/monitors", query: ["limit": "200"]), as: Paginated<Monitor>.self)
        monitors = page?.data ?? []
    }

    private func save() async {
        isSaving = true
        errorMessage = nil
        defer { isSaving = false }

        let scopeIDs = scope == "monitor" ? Array(selectedMonitorIDs) : []
        do {
            switch mode {
            case .create:
                _ = try await container.maintenance.create(CreateWindowBody(
                    title: title.trimmed, description: details.trimmed,
                    startsAt: startsAt, endsAt: endsAt, scope: scope, scopeIds: scopeIDs))
            case let .edit(window):
                _ = try await container.maintenance.update(id: window.id, UpdateWindowBody(
                    title: title.trimmed, description: details.trimmed,
                    startsAt: startsAt, endsAt: endsAt, scope: scope, scopeIds: scopeIDs))
            }
            onSaved()
            dismiss()
        } catch {
            errorMessage = (error as? APIError)?.errorDescription ?? "Couldn’t save the window."
        }
    }
}
