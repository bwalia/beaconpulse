package com.beacon.app

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Folder
import androidx.compose.material.icons.outlined.Bolt
import androidx.compose.material.icons.outlined.Code
import androidx.compose.material.icons.outlined.Construction
import androidx.compose.material.icons.outlined.Delete
import androidx.compose.material.icons.outlined.Edit
import androidx.compose.material.icons.outlined.Folder
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.launch

private val environments = listOf(
    Choice("production", "Production", Icons.Outlined.Bolt),
    Choice("staging", "Staging", Icons.Outlined.Construction),
    Choice("development", "Development", Icons.Outlined.Code),
)

private fun envColor(environment: String) = when (environment) {
    "production" -> Down
    "staging" -> Degraded
    else -> Blue
}

/** Projects; tapping one opens its monitors. Mirrors iOS ProjectsView. */
@Composable
fun ProjectsScreen(openProject: (Project) -> Unit) {
    val api = App.container.api
    val loader = rememberLoader("projects") { api.get<Page<Project>>("/api/v1/projects", "limit" to "200").data }
    var editing by remember { mutableStateOf<Project?>(null) }
    var creating by rememberSaveable { mutableStateOf(false) }
    var deleting by remember { mutableStateOf<Project?>(null) }
    val snack = LocalSnack.current
    val scope = rememberCoroutineScope()

    Screen("Projects", fab = { AddFab("New project") { creating = true } }) { padding ->
        Column(Modifier.padding(padding)) {
            LoadableContent(loader) { projects ->
                if (projects.isEmpty()) {
                    Message(Icons.Outlined.Folder, "No projects", "Monitors live in projects. Create one to get started.", "New project") { creating = true }
                } else {
                    LazyColumn(Modifier.fillMaxSize(), contentPadding = PaddingValues(16.dp, 4.dp, 16.dp, 0.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
                        items(projects, key = { it.id }) { p ->
                            Row(
                                Modifier.fillMaxWidth().card().clickable { openProject(p) }.padding(start = 14.dp, top = 14.dp, bottom = 14.dp, end = 2.dp),
                                verticalAlignment = Alignment.CenterVertically,
                            ) {
                                IconTile(Icons.Filled.Folder, Theme.accent)
                                Spacer(Modifier.width(12.dp))
                                Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(4.dp)) {
                                    Text(p.name, style = TextStyle(fontSize = 17.sp, fontWeight = FontWeight.SemiBold), maxLines = 1, overflow = TextOverflow.Ellipsis)
                                    if (p.description.isNotBlank()) {
                                        Text(p.description, style = MaterialTheme.typography.bodyMedium, color = Theme.muted, maxLines = 2, overflow = TextOverflow.Ellipsis)
                                    }
                                }
                                Spacer(Modifier.width(8.dp))
                                Pill(p.environment, envColor(p.environment), uppercase = true)
                                OverflowMenu(
                                    MenuItem("Edit", Icons.Outlined.Edit) { editing = p },
                                    MenuItem("Delete", Icons.Outlined.Delete, destructive = true) { deleting = p },
                                )
                            }
                        }
                        item { FabClearance() }
                    }
                }
            }
        }
    }

    if (creating) ProjectForm(null, onDismiss = { creating = false }) { creating = false; loader.reload() }
    editing?.let { p -> ProjectForm(p, onDismiss = { editing = null }) { editing = null; loader.reload() } }
    deleting?.let { p ->
        ConfirmDialog(
            "Delete ${p.name}?",
            "This also deletes every monitor in this project and stops probing them. This can’t be undone.",
            "Delete project",
            onConfirm = {
                scope.launch {
                    attempt { api.send("DELETE", "/api/v1/projects/${p.id}") }
                        .onSuccess { loader.reload() }
                        .onFailure { snack.showSnackbar(it.userMessage("Couldn’t delete the project.")) }
                }
            },
            onDismiss = { deleting = null },
        )
    }
}

/** Name, environment and an optional description: the web form's fields. */
@Composable
private fun ProjectForm(existing: Project?, onDismiss: () -> Unit, onSaved: () -> Unit) {
    val scope = rememberCoroutineScope()
    var name by rememberSaveable { mutableStateOf(existing?.name.orEmpty()) }
    var environment by rememberSaveable { mutableStateOf(existing?.environment ?: "production") }
    var description by rememberSaveable { mutableStateOf(existing?.description.orEmpty()) }
    var saving by remember { mutableStateOf(false) }
    var error by remember { mutableStateOf<String?>(null) }

    FormDialog(
        title = if (existing == null) "New project" else "Edit project",
        onDismiss = onDismiss,
        canSave = name.isNotBlank(),
        saving = saving,
        error = error,
        onSave = {
            saving = true
            error = null
            val body = ProjectBody(name.trim(), description.trim(), environment)
            scope.launch {
                attempt {
                    if (existing == null) App.container.api.send("POST", "/api/v1/projects", body)
                    else App.container.api.send("PATCH", "/api/v1/projects/${existing.id}", body)
                }.onSuccess { onSaved() }.onFailure { error = it.userMessage("Couldn’t save the project.") }
                saving = false
            }
        },
    ) {
        FormCard("Project") {
            Field("Name") { Input(name, { name = it }, placeholder = "e.g. Customer portal") }
            Field("Environment") { ChoiceChips(environments, environment, { environment = it }, minWidth = 130.dp) }
            Field("Description") { Input(description, { description = it }, placeholder = "Optional", singleLine = false, minLines = 2) }
        }
    }
}
