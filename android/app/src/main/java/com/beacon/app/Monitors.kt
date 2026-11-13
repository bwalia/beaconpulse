package com.beacon.app

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
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
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Balance
import androidx.compose.material.icons.outlined.Bolt
import androidx.compose.material.icons.outlined.ContentCopy
import androidx.compose.material.icons.outlined.Delete
import androidx.compose.material.icons.outlined.Edit
import androidx.compose.material.icons.outlined.GraphicEq
import androidx.compose.material.icons.outlined.Pause
import androidx.compose.material.icons.outlined.PlayArrow
import androidx.compose.material.icons.outlined.Schedule
import androidx.compose.material.icons.outlined.Search
import androidx.compose.material.icons.outlined.Spa
import androidx.compose.material.icons.outlined.Speed
import androidx.compose.material.icons.outlined.Timer
import androidx.compose.material.icons.outlined.VerifiedUser
import androidx.compose.material.icons.outlined.WifiTethering
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
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
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
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

    Screen(title, onBack, fab = { AddFab("Add monitor") { creating = true } }) { padding ->
        Column(Modifier.padding(padding)) {
            LoadableContent(loader) { monitors ->
                if (monitors.isEmpty()) {
                    Message(Icons.Outlined.WifiTethering, "No monitors yet", "Add one to start checking your sites and servers.", "Add monitor") { creating = true }
                } else {
                    val shown = monitors.filter { search.isBlank() || it.name.contains(search.trim(), true) || it.target.contains(search.trim(), true) }
                    LazyColumn(Modifier.fillMaxSize(), contentPadding = PaddingValues(16.dp, 4.dp, 16.dp, 0.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
                        item {
                            Input(
                                search, { search = it },
                                placeholder = "Search monitors",
                                leading = Icons.Outlined.Search,
                                keyboard = KeyboardOptions(imeAction = ImeAction.Search, autoCorrectEnabled = false),
                                modifier = Modifier.padding(bottom = 4.dp),
                            )
                        }
                        if (shown.isEmpty()) {
                            item { Text("No monitors match “${search.trim()}”.", Modifier.padding(24.dp), color = Theme.muted) }
                        }
                        items(shown, key = { it.id }) { m ->
                            MonitorRow(m, onClick = { openMonitor(m.id) }) {
                                OverflowMenu(
                                    MenuItem(if (m.enabled) "Pause" else "Resume", if (m.enabled) Icons.Outlined.Pause else Icons.Outlined.PlayArrow) {
                                        act("Couldn’t update the monitor.") { setEnabled(m.id, !m.enabled) }
                                    },
                                    MenuItem("Delete", Icons.Outlined.Delete, destructive = true) { deleting = m },
                                )
                            }
                        }
                        item { FabClearance() }
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

/** A monitor as a card: its type's icon in its status colour, name and target, status and type badges. */
@Composable
private fun MonitorRow(m: Monitor, onClick: () -> Unit, menu: @Composable () -> Unit) {
    val color = statusColor(m.status)
    Row(
        Modifier.fillMaxWidth().card().clickable(onClick = onClick).padding(start = 14.dp, top = 14.dp, bottom = 14.dp, end = 2.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        IconTile(monitorTypeIcon(m.type), color)
        Spacer(Modifier.width(12.dp))
        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(4.dp)) {
            Text(m.name, style = TextStyle(fontSize = 17.sp, fontWeight = FontWeight.SemiBold), maxLines = 1, overflow = TextOverflow.Ellipsis)
            Text(m.target.ifEmpty { "Heartbeat" }, style = monoCaption, color = Theme.muted, maxLines = 1, overflow = TextOverflow.Ellipsis)
        }
        Spacer(Modifier.width(8.dp))
        Column(horizontalAlignment = Alignment.End, verticalArrangement = Arrangement.spacedBy(6.dp)) {
            StatusBadge(m.status)
            TypeBadge(m.type)
        }
        menu()
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
                LazyColumn(
                    Modifier.fillMaxSize(),
                    contentPadding = PaddingValues(16.dp, 4.dp, 16.dp, 24.dp),
                    verticalArrangement = Arrangement.spacedBy(14.dp),
                ) {
                    item {
                        Card(spacing = 10.dp) {
                            Row(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = Alignment.CenterVertically) {
                                StatusBadge(m.status)
                                TypeBadge(m.type)
                            }
                            Text(m.name, style = TextStyle(fontSize = 22.sp, fontWeight = FontWeight.Bold, letterSpacing = (-0.2).sp))
                            if (kind != MonitorKind.HEARTBEAT && m.target.isNotEmpty()) {
                                SelectionContainer {
                                    Text(m.target, style = TextStyle(fontSize = 14.sp, fontFamily = FontFamily.Monospace), color = Theme.muted)
                                }
                            }
                            m.lastCheckedAt?.let { MetaLine(Icons.Outlined.Schedule, "Checked ${relative(it)}") }
                        }
                    }
                    if (metrics != null) {
                        item {
                            val tiles = buildList<@Composable () -> Unit> {
                                add {
                                    StatTile(
                                        "Uptime · 24h", "%.2f%%".format(Locale.US, metrics.uptimePercent),
                                        tint = uptimeColor(metrics.uptimePercent), icon = Icons.Outlined.VerifiedUser,
                                    )
                                }
                                metrics.responseMsAvg?.let { avg ->
                                    add { StatTile("Avg response", "%.0f ms".format(Locale.US, avg), icon = Icons.Outlined.Speed) }
                                }
                                // Push-based monitors aren't probed: GitHub Actions has no interval.
                                if (kind != MonitorKind.GITHUB_ACTIONS) {
                                    add { StatTile("Check every", duration(m.intervalSeconds), icon = Icons.Outlined.Timer) }
                                }
                            }
                            AdaptiveGrid(tiles, minWidth = 150.dp, stretchLastRow = true) { it() }
                        }
                        val series = metrics.responseMs.orEmpty()
                        if (series.isNotEmpty()) {
                            item {
                                Card(spacing = 12.dp) {
                                    Row(verticalAlignment = Alignment.CenterVertically) {
                                        Text("Response time", style = cardTitle, modifier = Modifier.weight(1f))
                                        Eyebrow("Last 24 hours")
                                    }
                                    TrendChart(series, Theme.accent, height = 190.dp) { "%.0f ms".format(Locale.US, it) }
                                }
                            }
                        }
                    }
                    if (kind == MonitorKind.HEARTBEAT && m.pingUrl != null) {
                        // Stored relative ("/api/v1/ping/<token>"): show the full URL the job should call.
                        val full = if (m.pingUrl.startsWith("http")) m.pingUrl else BuildConfig.API_BASE_URL.trimEnd('/') + m.pingUrl
                        item {
                            Card(spacing = 10.dp) {
                                Row(verticalAlignment = Alignment.CenterVertically) {
                                    Text("Heartbeat", style = cardTitle, modifier = Modifier.weight(1f))
                                    OutlinedButton(
                                        onClick = {
                                            clipboard.setText(AnnotatedString(full))
                                            scope.launch { snack.showSnackbar("Ping URL copied") }
                                        },
                                        shape = RoundedCornerShape(8.dp),
                                        border = BorderStroke(1.dp, Theme.border),
                                        contentPadding = PaddingValues(horizontal = 10.dp),
                                        modifier = Modifier.height(32.dp),
                                    ) {
                                        Icon(Icons.Outlined.ContentCopy, null, Modifier.size(15.dp), tint = Theme.accent)
                                        Spacer(Modifier.width(6.dp))
                                        Text("Copy URL", fontSize = 13.sp, fontWeight = FontWeight.SemiBold, color = Theme.accent)
                                    }
                                }
                                Text("Have your job call this URL on every run.", style = MaterialTheme.typography.bodySmall, color = Theme.muted)
                                SelectionContainer {
                                    Text(full, Modifier.fillMaxWidth().background(Theme.subtle, RoundedCornerShape(8.dp)).padding(10.dp), style = monoCaption)
                                }
                                MetaLine(Icons.Outlined.GraphicEq, "Last ping ${m.lastPingAt?.let(::relative) ?: "never"}")
                            }
                        }
                    }
                    item {
                        val rows = buildList {
                            add("Type" to m.type.uppercase())
                            add("Enabled" to if (m.enabled) "Yes" else "No — paused")
                            // Without metrics there are no tiles, so the interval shows here.
                            if (metrics == null && kind != MonitorKind.GITHUB_ACTIONS) add("Check every" to duration(m.intervalSeconds))
                            if (kind?.isProbed != false) add("Timeout" to "${m.timeoutSeconds}s")
                        }
                        Column(Modifier.fillMaxWidth().card()) {
                            rows.forEachIndexed { i, (label, value) ->
                                if (i > 0) RowDivider(Modifier.padding(start = 16.dp))
                                InfoRow(label, value, Modifier.padding(horizontal = 16.dp, vertical = 13.dp))
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

private fun sensitivityHint(sensitivity: String) = when (sensitivity) {
    "immediate" -> "Alert on the first failed check."
    "relaxed" -> "Alert only on prolonged outages (about 5 minutes)."
    else -> "Alert on a sustained failure (about 2 checks), ignoring one-off blips."
}

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

    val noCaps = KeyboardOptions(autoCorrectEnabled = false)

    FormDialog(
        title = if (creating) "New monitor" else "Edit monitor",
        onDismiss = onDismiss,
        canSave = name.isNotBlank() && !atLimit && (!kind.needsTarget || target.isNotBlank()) && (!creating || projectId.isNotEmpty()),
        saving = saving,
        error = error,
        onSave = ::save,
    ) {
        FormCard("Details") {
            Field("Name") { Input(name, { name = it }, placeholder = "e.g. Marketing site") }
            Field("Type") {
                if (creating) {
                    ChoiceChips(
                        MonitorKind.creatable.map { Choice(it.wire, it.wire.uppercase(), monitorTypeIcon(it.wire)) },
                        type, { type = it }, minWidth = 104.dp,
                    )
                } else {
                    ReadOnlyValue(type.uppercase())
                }
            }
            if (creating) {
                Field("Project", hint = if (projects.isEmpty()) "Create a project first: monitors live in projects." else null) {
                    MenuInput(projects.map { it.id to it.name }, projectId, { projectId = it }, enabled = projects.isNotEmpty())
                }
            }
            if (kind.needsTarget) {
                Field(if (kind == MonitorKind.GITHUB_ACTIONS) "Repository" else "Target") {
                    Input(
                        target, { target = it },
                        placeholder = kind.targetHint,
                        mono = true,
                        keyboard = KeyboardOptions(keyboardType = KeyboardType.Uri, autoCorrectEnabled = false),
                    )
                }
            }
        }

        FormCard(
            "Checks",
            footer = if (kind != MonitorKind.GITHUB_ACTIONS && minInterval > 10) "Your plan checks at most every ${duration(minInterval)}." else null,
        ) {
            if (kind != MonitorKind.GITHUB_ACTIONS) {
                StepperRow("Check every", choices(intervals, interval, minInterval), interval, { interval = it })
                if (kind.isProbed) StepperRow("Timeout", choices(timeouts, timeout), timeout, { timeout = it })
                if (kind == MonitorKind.HEARTBEAT) {
                    // The server sets grace only at creation.
                    if (creating) StepperRow("Grace period", choices(graces, grace), grace, { grace = it })
                    else Field("Grace period") { ReadOnlyValue(duration(grace)) }
                }
                Field("Alert sensitivity", hint = sensitivityHint(sensitivity)) {
                    ChoiceChips(
                        listOf(
                            Choice("immediate", "Immediate", Icons.Outlined.Bolt),
                            Choice("balanced", "Balanced", Icons.Outlined.Balance),
                            Choice("relaxed", "Relaxed", Icons.Outlined.Spa),
                        ),
                        sensitivity, { sensitivity = it }, minWidth = 104.dp,
                    )
                }
            }
            SwitchRow("Enabled", enabled, { enabled = it }, "Run checks and send alerts")
        }

        if (atLimit) ErrorBanner("You’ve reached your plan’s limit of ${usage?.monitorsLimit} monitors. Delete one to add another.")

        if (kind.takesRequestSettings) {
            FormCard("Request") {
                Field("Method") {
                    ChoiceChips(listOf("GET", "POST", "HEAD", "PUT", "PATCH", "DELETE").map { Choice(it, it) }, method, { method = it }, minWidth = 76.dp)
                }
                Field("Expected status codes", hint = "Comma-separated. Blank accepts any 2xx/3xx.") {
                    Input(codes, { codes = it }, placeholder = "200, 204", keyboard = noCaps)
                }
                SwitchRow("Follow redirects", follow, { follow = it })
                SwitchRow("Skip TLS verification", skipTls, { skipTls = it }, "Only for self-signed certificates")
                Field("Request body", hint = "Sent with POST, PUT and PATCH.") {
                    Input(body, { body = it }, placeholder = "{ }", singleLine = false, minLines = 2, mono = true, keyboard = noCaps)
                }
            }

            FormCard(
                "Headers",
                footer = "Use for auth, e.g. Authorization: Bearer <token>. Saved values are encrypted and shown as ********: leave a line as it is to keep its value, or delete the line to remove the header.",
            ) {
                Input(headers, { headers = it }, placeholder = "Name: value, one per line", singleLine = false, minLines = 3, mono = true, keyboard = noCaps)
            }

            FormCard("Response") {
                Field("Body must contain") { Input(mustContain, { mustContain = it }, placeholder = "Optional") }
                Field("Body must not contain") { Input(mustNotContain, { mustNotContain = it }, placeholder = "Optional") }
                Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                    Field("Slow response (ms)", modifier = Modifier.weight(1f)) {
                        Input(slowMs, { slowMs = it.filter(Char::isDigit) }, placeholder = "Off", keyboard = KeyboardOptions(keyboardType = KeyboardType.Number))
                    }
                    Field("SSL expiry (days)", modifier = Modifier.weight(1f)) {
                        Input(sslDays, { sslDays = it.filter(Char::isDigit) }, placeholder = "30", keyboard = KeyboardOptions(keyboardType = KeyboardType.Number))
                    }
                }
            }
        }

        if (kind == MonitorKind.DNS) {
            FormCard("DNS") {
                Field("Record type") {
                    ChoiceChips(listOf("A", "AAAA", "CNAME", "MX", "TXT", "NS", "CAA").map { Choice(it, it) }, dnsType, { dnsType = it }, minWidth = 70.dp)
                }
            }
        }

        if (kind == MonitorKind.GITHUB_ACTIONS) {
            FormCard("GitHub Actions", footer = "Leave the filter blank to alert on any workflow. Rotate the ingest token on the web.") {
                Field("Workflow filter") { Input(workflow, { workflow = it }, placeholder = "Optional, e.g. deploy.yml", keyboard = noCaps) }
                SwitchRow("Notify on success", notifySuccess, { notifySuccess = it }, "Also send a message for every successful run")
            }
        }
    }
}
