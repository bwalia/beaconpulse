package com.beacon.app

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.outlined.ContentCopy
import androidx.compose.material.icons.outlined.Delete
import androidx.compose.material.icons.outlined.Edit
import androidx.compose.material.icons.outlined.MonitorHeart
import androidx.compose.material.icons.outlined.Pause
import androidx.compose.material.icons.outlined.PlayArrow
import androidx.compose.material.icons.outlined.Search
import androidx.compose.material3.FloatingActionButton
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.ListItem
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalClipboardManager
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import kotlinx.coroutines.launch
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import java.util.Locale

/** Pause or resume through the dedicated endpoints. */
private suspend fun setEnabled(id: String, enabled: Boolean) =
    App.container.api.send("POST", "/api/v1/monitors/$id/${if (enabled) "resume" else "pause"}")

private suspend fun deleteMonitor(id: String) = App.container.api.send("DELETE", "/api/v1/monitors/$id")

/** The org's monitors, or one project's. Mirrors iOS MonitorsListView. */
@Composable
fun MonitorsScreen(projectId: String?, title: String, onBack: (() -> Unit)?, openMonitor: (String) -> Unit) {
    val api = App.container.api
    val loader = rememberLoader("monitors-$projectId") {
        val query = listOfNotNull("limit" to "100", projectId?.let { "project_id" to it })
        api.get<Page<Monitor>>("/api/v1/monitors", *query.toTypedArray()).data
    }
    var search by rememberSaveable { mutableStateOf("") }
    var creating by rememberSaveable { mutableStateOf(false) }
    var deleting by remember { mutableStateOf<Monitor?>(null) }
    val snack = LocalSnack.current
    val scope = rememberCoroutineScope()

    fun act(fallback: String, block: suspend () -> Unit) {
        scope.launch {
            attempt { block() }.onSuccess { loader.reload() }.onFailure { snack.showSnackbar(it.userMessage(fallback)) }
        }
    }

    Screen(title, onBack, fab = {
        FloatingActionButton(onClick = { creating = true }) { Icon(Icons.Default.Add, "Add monitor") }
    }) { padding ->
        Column(Modifier.padding(padding)) {
            LoadableContent(loader) { monitors ->
                if (monitors.isEmpty()) {
                    Message(Icons.Outlined.MonitorHeart, "No monitors yet", "Add one to start checking your sites and servers.", "Add monitor") { creating = true }
                } else {
                    val shown = monitors.filter { search.isBlank() || it.name.contains(search.trim(), true) || it.target.contains(search.trim(), true) }
                    LazyColumn(Modifier.fillMaxSize()) {
                        item {
                            OutlinedTextField(
                                value = search,
                                onValueChange = { search = it },
                                placeholder = { Text("Search monitors") },
                                leadingIcon = { Icon(Icons.Outlined.Search, null) },
                                singleLine = true,
                                modifier = Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 8.dp),
                            )
                        }
                        if (shown.isEmpty()) {
                            item { Text("No monitors match “${search.trim()}”.", Modifier.padding(24.dp), color = MaterialTheme.colorScheme.onSurfaceVariant) }
                        }
                        items(shown, key = { it.id }) { m ->
                            ListItem(
                                modifier = Modifier.clickable { openMonitor(m.id) },
                                leadingContent = { StatusDot(if (m.enabled) m.lastStatus else "paused", 12) },
                                headlineContent = { Text(m.name, fontWeight = FontWeight.Medium, maxLines = 1, overflow = TextOverflow.Ellipsis) },
                                supportingContent = { Text(m.target.ifEmpty { "Heartbeat" }, maxLines = 1, overflow = TextOverflow.Ellipsis) },
                                trailingContent = {
                                    Row(verticalAlignment = Alignment.CenterVertically) {
                                        Text(m.type.uppercase(), style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                                        OverflowMenu(
                                            MenuItem(if (m.enabled) "Pause" else "Resume", if (m.enabled) Icons.Outlined.Pause else Icons.Outlined.PlayArrow) {
                                                act("Couldn’t update the monitor.") { setEnabled(m.id, !m.enabled) }
                                            },
                                            MenuItem("Delete", Icons.Outlined.Delete, destructive = true) { deleting = m },
                                        )
                                    }
                                },
                            )
                            HorizontalDivider()
                        }
                        item { Spacer(Modifier.height(88.dp)) } // clear of the + button
                    }
                }
            }
        }
    }

    if (creating) MonitorForm(existing = null, defaultProject = projectId, onDismiss = { creating = false }) { creating = false; loader.reload() }
    deleting?.let { m ->
        ConfirmDialog(
            "Delete ${m.name}?", "Its checks and alerts stop, and its history is removed. This can’t be undone.", "Delete",
            onConfirm = { act("Couldn’t delete the monitor.") { deleteMonitor(m.id) } },
            onDismiss = { deleting = null },
        )
    }
}

/** One monitor with its last-24h numbers. Opened by id, so a tapped alert lands here too. Mirrors iOS MonitorDetailView. */
@Composable
fun MonitorDetailScreen(id: String, onBack: () -> Unit) {
    val api = App.container.api
    val loader = rememberLoader("monitor-$id") {
        val monitor = api.get<Monitor>("/api/v1/monitors/$id")
        // Metrics are a bonus: a monitor with no history yet still shows.
        monitor to attempt { api.get<MonitorMetrics>("/api/v1/monitors/$id/metrics") }.getOrNull()
    }
    val monitor = (loader.state as? Loadable.Loaded)?.value?.first
    var editing by rememberSaveable { mutableStateOf(false) }
    var confirmDelete by rememberSaveable { mutableStateOf(false) }
    val snack = LocalSnack.current
    val scope = rememberCoroutineScope()
    val clipboard = LocalClipboardManager.current

    Screen(monitor?.name ?: "Monitor", onBack, actions = {
        if (monitor != null) {
            OverflowMenu(
                MenuItem("Edit", Icons.Outlined.Edit) { editing = true },
                MenuItem(if (monitor.enabled) "Pause" else "Resume", if (monitor.enabled) Icons.Outlined.Pause else Icons.Outlined.PlayArrow) {
                    scope.launch {
                        attempt { setEnabled(monitor.id, !monitor.enabled) }
                            .onSuccess { loader.reload() }
                            .onFailure { snack.showSnackbar(it.userMessage("Couldn’t update the monitor.")) }
                    }
                },
                MenuItem("Delete", Icons.Outlined.Delete, destructive = true) { confirmDelete = true },
            )
        }
    }) { padding ->
        Column(Modifier.padding(padding)) {
            LoadableContent(loader) { (m, metrics) ->
                val kind = m.kind
                LazyColumn(contentPadding = PaddingValues(16.dp), verticalArrangement = Arrangement.spacedBy(12.dp), modifier = Modifier.fillMaxSize()) {
                    item {
                        Panel {
                            val status = if (m.enabled) m.lastStatus else "paused"
                            InfoRow("Status", status.replaceFirstChar { it.uppercase() }) { StatusDot(status); Spacer(Modifier.width(8.dp)) }
                            if (kind != MonitorKind.HEARTBEAT) InfoRow(if (kind == MonitorKind.GITHUB_ACTIONS) "Repository" else "Target", m.target)
                            InfoRow("Type", m.type.uppercase())
                            InfoRow("Enabled", if (m.enabled) "Yes" else "No")
                            if (kind != MonitorKind.GITHUB_ACTIONS) InfoRow("Interval", duration(m.intervalSeconds))
                            if (kind?.isProbed != false) InfoRow("Timeout", "${m.timeoutSeconds}s")
                            m.lastCheckedAt?.let { InfoRow("Last checked", relative(it)) }
                        }
                    }
                    if (kind == MonitorKind.HEARTBEAT && m.pingUrl != null) {
                        // Stored relative ("/api/v1/ping/<token>"): show the full URL the job should call.
                        val full = if (m.pingUrl.startsWith("http")) m.pingUrl else BuildConfig.API_BASE_URL.trimEnd('/') + m.pingUrl
                        item {
                            Panel("Heartbeat") {
                                Text("Have your job call this URL on every run.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                                Row(verticalAlignment = Alignment.CenterVertically) {
                                    Text(full, Modifier.weight(1f), fontFamily = FontFamily.Monospace, style = MaterialTheme.typography.bodySmall)
                                    IconButton(onClick = {
                                        clipboard.setText(AnnotatedString(full))
                                        scope.launch { snack.showSnackbar("Ping URL copied") }
                                    }) { Icon(Icons.Outlined.ContentCopy, "Copy ping URL") }
                                }
                                InfoRow("Last ping", m.lastPingAt?.let(::relative) ?: "Never")
                            }
                        }
                    }
                    if (metrics != null) {
                        item {
                            Panel("Last 24 hours") {
                                InfoRow("Uptime", "%.2f%%".format(Locale.US, metrics.uptimePercent))
                                metrics.responseMsAvg?.let { InfoRow("Avg response", "%.0f ms".format(Locale.US, it)) }
                            }
                        }
                        val series = metrics.responseMs.orEmpty()
                        if (series.isNotEmpty()) {
                            item {
                                Panel("Response time") { LineChart(series, MaterialTheme.colorScheme.primary, filled = false) { "%.0f ms".format(Locale.US, it) } }
                            }
                        }
                    }
                }
            }
        }
    }

    if (editing && monitor != null) MonitorForm(existing = monitor, defaultProject = null, onDismiss = { editing = false }) { editing = false; loader.reload() }
    if (confirmDelete && monitor != null) {
        ConfirmDialog(
            "Delete ${monitor.name}?", "Its checks and alerts stop, and its history is removed. This can’t be undone.", "Delete",
            onConfirm = {
                scope.launch {
                    attempt { deleteMonitor(monitor.id) }
                        .onSuccess { onBack() }
                        .onFailure { snack.showSnackbar(it.userMessage("Couldn’t delete the monitor.")) }
                }
            },
            onDismiss = { confirmDelete = false },
        )
    }
}

private val intervals = listOf(10, 30, 60, 120, 300, 600, 900, 1800, 3600, 21600, 43200, 86400)
private val timeouts = listOf(5, 10, 15, 20, 30, 45, 60, 90, 120, 180, 300)
private val graces = listOf(0, 30, 60, 120, 300, 600, 1800, 3600, 21600, 86400)

/** Presets plus whatever is set now (it may have come from the web), as value to label. */
private fun choices(presets: List<Int>, current: Int, min: Int = 0) =
    (presets.filter { it >= min } + current).distinct().sorted().map { it to duration(it) }

/**
 * Create or edit a monitor, with the web form's per-type fields. Editing sends back the
 * whole settings object with only the shown fields changed, so web-only settings
 * survive. Mirrors iOS MonitorFormView.
 */
@Composable
private fun MonitorForm(existing: Monitor?, defaultProject: String?, onDismiss: () -> Unit, onSaved: () -> Unit) {
    val api = App.container.api
    val scope = rememberCoroutineScope()
    val creating = existing == null
    val s = existing?.settings ?: JsonObject(emptyMap())

    var name by rememberSaveable { mutableStateOf(existing?.name.orEmpty()) }
    var type by rememberSaveable { mutableStateOf(existing?.type ?: "https") }
    var target by rememberSaveable { mutableStateOf(existing?.target.orEmpty()) }
    var projectId by rememberSaveable { mutableStateOf(existing?.projectId ?: defaultProject.orEmpty()) }
    var interval by rememberSaveable { mutableStateOf(existing?.intervalSeconds ?: 60) }
    var timeout by rememberSaveable { mutableStateOf(existing?.timeoutSeconds ?: 10) }
    var grace by rememberSaveable { mutableStateOf(existing?.graceSeconds ?: 60) }
    var enabled by rememberSaveable { mutableStateOf(existing?.enabled ?: true) }
    var sensitivity by rememberSaveable { mutableStateOf(s.str("alert_sensitivity") ?: "balanced") }
    var method by rememberSaveable { mutableStateOf(s.str("method") ?: "GET") }
    var codes by rememberSaveable { mutableStateOf(statusCodesText(s)) }
    var follow by rememberSaveable { mutableStateOf(s.bool("follow_redirects") ?: false) }
    var skipTls by rememberSaveable { mutableStateOf(s.bool("skip_tls_verify") ?: false) }
    var body by rememberSaveable { mutableStateOf(s.str("body").orEmpty()) }
    var headers by rememberSaveable { mutableStateOf(headerLines(s)) }
    var mustContain by rememberSaveable { mutableStateOf(s.str("body_keyword").orEmpty()) }
    var mustNotContain by rememberSaveable { mutableStateOf(s.str("body_not_keyword").orEmpty()) }
    var slowMs by rememberSaveable { mutableStateOf(s.int("response_time_warning_ms")?.toString().orEmpty()) }
    var sslDays by rememberSaveable { mutableStateOf(s.int("ssl_expiry_warning_days")?.toString().orEmpty()) }
    var dnsType by rememberSaveable { mutableStateOf(s.str("dns_query_type") ?: "A") }
    var workflow by rememberSaveable { mutableStateOf(s.str("github_workflow").orEmpty()) }
    var notifySuccess by rememberSaveable { mutableStateOf(s.bool("github_notify_on_success") ?: false) }

    var usage by remember { mutableStateOf<MonitorUsage?>(null) }
    var projects by remember { mutableStateOf<List<Project>>(emptyList()) }
    var saving by remember { mutableStateOf(false) }
    var error by remember { mutableStateOf<String?>(null) }

    val kind = MonitorKind.entries.firstOrNull { it.wire == type } ?: MonitorKind.HTTPS
    val minInterval = usage?.minIntervalSeconds ?: 10
    val atLimit = creating && usage?.let { it.monitorsUsed >= it.monitorsLimit } == true

    // The plan's limits, so the form never offers an interval or a monitor the server would refuse.
    LaunchedEffect(Unit) {
        usage = attempt { api.get<MonitorUsage>("/api/v1/monitors/usage") }.getOrNull()
        if (interval < minInterval) interval = minInterval
        if (creating) {
            projects = attempt { api.get<Page<Project>>("/api/v1/projects", "limit" to "200").data }.getOrDefault(emptyList())
            if (projectId.isEmpty()) projectId = projects.firstOrNull()?.id.orEmpty()
        }
    }

    fun text(value: String): JsonElement? = value.trim().takeIf { it.isNotEmpty() }?.let { JsonPrimitive(it) }
    fun flag(on: Boolean): JsonElement? = if (on) JsonPrimitive(true) else null
    fun number(value: String): JsonElement? = value.trim().toIntOrNull()?.let { JsonPrimitive(it) }

    fun save() {
        saving = true
        error = null
        // An unset value is left out, as the web form does; everything not shown is kept.
        val edits = mutableMapOf<String, JsonElement?>("alert_sensitivity" to JsonPrimitive(sensitivity))
        if (kind.takesRequestSettings) {
            edits["method"] = method.takeIf { it != "GET" }?.let { JsonPrimitive(it) }
            edits["valid_status_codes"] = parseStatusCodes(codes)
            edits["follow_redirects"] = flag(follow)
            edits["skip_tls_verify"] = flag(skipTls)
            edits["body"] = text(body)
            edits["headers"] = parseHeaders(headers)
            edits["body_keyword"] = text(mustContain)
            edits["body_not_keyword"] = text(mustNotContain)
            edits["response_time_warning_ms"] = number(slowMs)
            edits["ssl_expiry_warning_days"] = number(sslDays)
        }
        if (kind == MonitorKind.DNS) edits["dns_query_type"] = dnsType.takeIf { it != "A" }?.let { JsonPrimitive(it) }
        if (kind == MonitorKind.GITHUB_ACTIONS) {
            edits["github_workflow"] = text(workflow)
            edits["github_notify_on_success"] = flag(notifySuccess)
        }
        val settings = s.with(edits)
        scope.launch {
            attempt {
                if (existing == null) {
                    api.send("POST", "/api/v1/monitors", CreateMonitorBody(
                        projectId = projectId, name = name.trim(), type = type,
                        target = if (kind.needsTarget) target.trim() else "", enabled = enabled,
                        intervalSeconds = interval, timeoutSeconds = timeout,
                        graceSeconds = if (kind == MonitorKind.HEARTBEAT) grace else null, settings = settings,
                    ))
                } else {
                    api.send("PATCH", "/api/v1/monitors/${existing.id}", UpdateMonitorBody(
                        name = name.trim(), target = if (kind.needsTarget) target.trim() else null, enabled = enabled,
                        intervalSeconds = interval, timeoutSeconds = timeout, settings = settings,
                    ))
                }
            }.onSuccess { onSaved() }.onFailure { error = it.userMessage("Couldn’t save the monitor.") }
            saving = false
        }
    }

    FormDialog(
        title = if (creating) "New monitor" else "Edit monitor",
        onDismiss = onDismiss,
        canSave = name.isNotBlank() && !atLimit && (!kind.needsTarget || target.isNotBlank()) && (!creating || projectId.isNotEmpty()),
        saving = saving,
        error = error,
        onSave = ::save,
    ) {
        SectionTitle("Details")
        Field("Name", name, { name = it })
        if (creating) {
            Dropdown("Type", MonitorKind.creatable.map { it.wire to it.wire.uppercase() }, type, { type = it })
            if (projects.isEmpty()) {
                Text("Create a project first: monitors live in projects.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            } else {
                Dropdown("Project", projects.map { it.id to it.name }, projectId, { projectId = it })
            }
        } else {
            InfoRow("Type", type.uppercase())
        }
        if (kind.needsTarget) {
            Field(
                if (kind == MonitorKind.GITHUB_ACTIONS) "Repository" else "Target", target, { target = it },
                placeholder = kind.targetHint,
                keyboard = KeyboardOptions(keyboardType = KeyboardType.Uri, autoCorrectEnabled = false),
            )
        }

        SectionTitle("Checks")
        if (kind != MonitorKind.GITHUB_ACTIONS) {
            Dropdown("Check every", choices(intervals, interval, minInterval), interval, { interval = it })
            if (kind.isProbed) Dropdown("Timeout", choices(timeouts, timeout), timeout, { timeout = it })
            if (kind == MonitorKind.HEARTBEAT) {
                // The server sets grace only at creation.
                if (creating) Dropdown("Grace period", choices(graces, grace), grace, { grace = it }) else InfoRow("Grace period", duration(grace))
            }
            Dropdown("Alert sensitivity", listOf("immediate" to "Immediate", "balanced" to "Balanced", "relaxed" to "Relaxed"), sensitivity, { sensitivity = it })
            if (minInterval > 10) {
                Text("Your plan checks at most every ${duration(minInterval)}.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
        }
        SwitchRow("Enabled", enabled, { enabled = it })
        if (atLimit) {
            Text(
                "You’ve reached your plan’s limit of ${usage?.monitorsLimit} monitors. Delete one to add another.",
                style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.error,
            )
        }

        if (kind.takesRequestSettings) {
            SectionTitle("Request")
            Dropdown("Method", listOf("GET", "POST", "HEAD", "PUT", "PATCH", "DELETE").map { it to it }, method, { method = it })
            Field("Expected status codes", codes, { codes = it }, placeholder = "e.g. 200, 204 (blank = any 2xx/3xx)")
            SwitchRow("Follow redirects", follow, { follow = it })
            SwitchRow("Skip TLS verification", skipTls, { skipTls = it })
            Field("Request body (for POST/PUT/PATCH)", body, { body = it }, singleLine = false, mono = true)

            SectionTitle("Headers")
            Field("Name: value, one per line", headers, { headers = it }, singleLine = false, mono = true,
                keyboard = KeyboardOptions(autoCorrectEnabled = false))
            Text(
                "Use for auth, e.g. Authorization: Bearer <token>. Saved values are encrypted and shown as ********: leave a line as it is to keep its value, or delete the line to remove the header.",
                style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant,
            )

            SectionTitle("Response")
            Field("Body must contain", mustContain, { mustContain = it })
            Field("Body must NOT contain", mustNotContain, { mustNotContain = it })
            Field("Slow-response alert (ms)", slowMs, { slowMs = it.filter(Char::isDigit) }, placeholder = "Off",
                keyboard = KeyboardOptions(keyboardType = KeyboardType.Number))
            Field("SSL expiry warning (days)", sslDays, { sslDays = it.filter(Char::isDigit) }, placeholder = "30",
                keyboard = KeyboardOptions(keyboardType = KeyboardType.Number))
        }

        if (kind == MonitorKind.DNS) {
            SectionTitle("DNS")
            Dropdown("Record type", listOf("A", "AAAA", "CNAME", "MX", "TXT", "NS", "CAA").map { it to it }, dnsType, { dnsType = it })
        }

        if (kind == MonitorKind.GITHUB_ACTIONS) {
            SectionTitle("GitHub Actions")
            Field("Workflow filter (optional)", workflow, { workflow = it })
            SwitchRow("Also notify on every successful run", notifySuccess, { notifySuccess = it })
            Text(
                "Leave the filter blank to alert on any workflow. Rotate the ingest token on the web.",
                style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
    }
}
