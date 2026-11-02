import SwiftUI

/// Create or edit a project: name, environment and an optional description —
/// the same fields as the web form.
struct ProjectFormView: View {
    enum Mode {
        case create
        case edit(Project)
    }

    let mode: Mode
    let onSaved: () -> Void

    @Environment(AppContainer.self) private var container
    @Environment(\.dismiss) private var dismiss

    @State private var name = ""
    @State private var environment = "production"
    @State private var details = ""

    @State private var isSaving = false
    @State private var errorMessage: String?

    init(mode: Mode, onSaved: @escaping () -> Void) {
        self.mode = mode
        self.onSaved = onSaved
        if case let .edit(project) = mode {
            _name = State(initialValue: project.name)
            _environment = State(initialValue: project.environment)
            _details = State(initialValue: project.description)
        }
    }

    private var isCreate: Bool { if case .create = mode { return true } else { return false } }

    var body: some View {
        NavigationStack {
            Form {
                Section {
                    TextField("Name", text: $name)
                    Picker("Environment", selection: $environment) {
                        Text("Production").tag("production")
                        Text("Staging").tag("staging")
                        Text("Development").tag("development")
                    }
                    TextField("Description (optional)", text: $details, axis: .vertical)
                }

                if let errorMessage {
                    Section { Text(errorMessage).foregroundStyle(.red) }
                }
            }
            .navigationTitle(isCreate ? "New Project" : "Edit Project")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel") { dismiss() }
                }
                ToolbarItem(placement: .confirmationAction) {
                    if isSaving { ProgressView() }
                    else { Button("Save") { Task { await save() } }.disabled(name.trimmed.isEmpty) }
                }
            }
        }
    }

    private func save() async {
        isSaving = true
        errorMessage = nil
        defer { isSaving = false }

        let body = ProjectBody(name: name.trimmed, description: details.trimmed, environment: environment)
        do {
            switch mode {
            case .create:
                _ = try await container.projects.create(body)
            case let .edit(project):
                _ = try await container.projects.update(id: project.id, body)
            }
            onSaved()
            dismiss()
        } catch {
            errorMessage = (error as? APIError)?.errorDescription ?? "Couldn’t save the project."
        }
    }
}
