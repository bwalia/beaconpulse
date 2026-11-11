package com.beacon.app

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.outlined.Delete
import androidx.compose.material.icons.outlined.Edit
import androidx.compose.material.icons.outlined.Folder
import androidx.compose.material3.FloatingActionButton
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.ListItem
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
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import kotlinx.coroutines.launch

private val environments = listOf("production" to "Production", "staging" to "Staging", "development" to "Development")

private fun envColor(environment: String) = when (environment) {
    "production" -> Red
    "staging" -> Amber
    else -> Color(0xFF2563EB)
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

    Screen("Projects", fab = {
        FloatingActionButton(onClick = { creating = true }) { Icon(Icons.Default.Add, "New project") }
    }) { padding ->
        Column(Modifier.padding(padding)) {
            LoadableContent(loader) { projects ->
                if (projects.isEmpty()) {
                    Message(Icons.Outlined.Folder, "No projects", "Monitors live in projects. Create one to get started.", "New project") { creating = true }
                } else {
                    LazyColumn(Modifier.fillMaxSize()) {
                        items(projects, key = { it.id }) { p ->
                            ListItem(
                                modifier = Modifier.clickable { openProject(p) },
                                headlineContent = {
                                    Row(verticalAlignment = Alignment.CenterVertically) {
                                        Text(p.name, fontWeight = FontWeight.Medium, maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f, fill = false))
                                        Spacer(Modifier.width(8.dp))
                                        Pill(p.environment, envColor(p.environment))
                                    }
                                },
                                supportingContent = p.description.takeIf { it.isNotBlank() }?.let { { Text(it, maxLines = 2, overflow = TextOverflow.Ellipsis) } },
                                trailingContent = {
                                    OverflowMenu(
                                        MenuItem("Edit", Icons.Outlined.Edit) { editing = p },
                                        MenuItem("Delete", Icons.Outlined.Delete, destructive = true) { deleting = p },
                                    )
                                },
                            )
                            HorizontalDivider()
                        }
                        item { Spacer(Modifier.height(88.dp)) }
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
        Field("Name", name, { name = it })
        Text("Environment")
        Segmented(environments, environment, { environment = it })
        Field("Description (optional)", description, { description = it }, singleLine = false)
    }
}
