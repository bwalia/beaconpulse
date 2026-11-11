package com.beacon.app

import android.content.Intent
import android.provider.Settings as AndroidSettings
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
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
import androidx.compose.material.icons.automirrored.filled.KeyboardArrowRight
import androidx.compose.material.icons.automirrored.filled.Logout
import androidx.compose.material.icons.automirrored.outlined.Send
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.outlined.Build
import androidx.compose.material.icons.outlined.Delete
import androidx.compose.material.icons.outlined.DeleteForever
import androidx.compose.material.icons.outlined.Email
import androidx.compose.material.icons.outlined.Info
import androidx.compose.material.icons.outlined.Link
import androidx.compose.material.icons.outlined.Notifications
import androidx.compose.material.icons.outlined.NotificationsActive
import androidx.compose.material.icons.outlined.NotificationsOff
import androidx.compose.material.icons.outlined.Person
import androidx.compose.material.icons.outlined.PhoneAndroid
import androidx.compose.material.icons.outlined.Policy
import androidx.compose.material.icons.outlined.Tag
import androidx.compose.material.icons.outlined.Description
import androidx.compose.material3.Checkbox
import androidx.compose.material3.FloatingActionButton
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.ListItem
import androidx.compose.material3.ListItemDefaults
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
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
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.LifecycleResumeEffect
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import kotlinx.coroutines.launch
import java.time.Duration
import java.time.Instant
import java.time.temporal.ChronoUnit

private val base = BuildConfig.API_BASE_URL.trimEnd('/')

/** Alert delivery, push status, the account, and sign-out. Mirrors iOS SettingsView. */
@Composable
fun SettingsScreen(openChannels: () -> Unit, openMaintenance: () -> Unit) {
    val app = App.container
    val user by app.session.user.collectAsStateWithLifecycle()
    val context = LocalContext.current
    val scope = rememberCoroutineScope()
    var allowed by remember { mutableStateOf(app.push.notificationsAllowed) }
    var confirmSignOut by remember { mutableStateOf(false) }
    var deletingAccount by rememberSaveable { mutableStateOf(false) }
    // Re-read on return from the system settings, in case they turned notifications on there.
    LifecycleResumeEffect(Unit) {
        allowed = app.push.notificationsAllowed
        onPauseOrDispose {}
    }

    Screen("Settings") { padding ->
        LazyColumn(Modifier.padding(padding).fillMaxSize()) {
            item { Header("Alerts") }
            item { SettingRow(Icons.Outlined.NotificationsActive, "Notification channels", onClick = openChannels, chevron = true) }
            item { SettingRow(Icons.Outlined.Build, "Maintenance windows", onClick = openMaintenance, chevron = true) }
            item {
                when {
                    !app.push.configured -> SettingRow(Icons.Outlined.PhoneAndroid, "Push notifications", "Not available in this build yet")
                    allowed -> SettingRow(Icons.Outlined.PhoneAndroid, "Push notifications", "On — alerts come to this phone")
                    else -> SettingRow(Icons.Outlined.NotificationsOff, "Push notifications", "Off — tap to turn on", onClick = {
                        context.startActivity(
                            Intent(AndroidSettings.ACTION_APP_NOTIFICATION_SETTINGS).putExtra(AndroidSettings.EXTRA_APP_PACKAGE, context.packageName),
                        )
                    })
                }
            }

            user?.let { u ->
                item { Header("Account") }
                item { SettingRow(Icons.Outlined.Person, u.name, u.role.replaceFirstChar { it.uppercase() }) }
                item { SettingRow(Icons.Outlined.Email, u.email) }
            }

            item { Header("About") }
            item { SettingRow(Icons.Outlined.Policy, "Privacy policy", onClick = { openUrl(context, "$base/privacy") }) }
            item { SettingRow(Icons.Outlined.Description, "Terms of service", onClick = { openUrl(context, "$base/terms") }) }
            item { SettingRow(Icons.Outlined.Info, "Version", "${BuildConfig.VERSION_NAME} (${BuildConfig.VERSION_CODE})") }

            item { HorizontalDivider(Modifier.padding(vertical = 8.dp)) }
            item { SettingRow(Icons.AutoMirrored.Filled.Logout, "Sign out", destructive = true, onClick = { confirmSignOut = true }) }
            user?.let { u ->
                item {
                    if (u.role == "owner") {
                        SettingRow(Icons.Outlined.DeleteForever, "Delete account", "Permanently deletes your organisation and all its data.", destructive = true, onClick = { deletingAccount = true })
                    } else {
                        SettingRow(Icons.Outlined.DeleteForever, "Delete account", "Only your organisation’s owner can delete this account.")
                    }
                }
            }
            item { Spacer(Modifier.height(24.dp)) }
        }
    }

    if (confirmSignOut) {
        ConfirmDialog("Sign out?", "This phone stops getting alerts until you sign in again.", "Sign out",
            onConfirm = { scope.launch { app.signOut() } }, onDismiss = { confirmSignOut = false })
    }
    if (deletingAccount) user?.let { DeleteAccountDialog(it.email) { deletingAccount = false } }
}

@Composable
private fun Header(text: String) {
    Text(text, style = MaterialTheme.typography.titleSmall, color = MaterialTheme.colorScheme.primary, modifier = Modifier.padding(start = 16.dp, top = 20.dp, bottom = 4.dp))
}

@Composable
private fun SettingRow(icon: ImageVector, title: String, supporting: String? = null, destructive: Boolean = false, chevron: Boolean = false, onClick: (() -> Unit)? = null) {
    val tint = if (destructive) MaterialTheme.colorScheme.error else MaterialTheme.colorScheme.onSurfaceVariant
    ListItem(
        modifier = if (onClick != null) Modifier.clickable(onClick = onClick) else Modifier,
        leadingContent = { Icon(icon, null, tint = tint) },
        headlineContent = { Text(title, color = if (destructive) MaterialTheme.colorScheme.error else MaterialTheme.colorScheme.onSurface) },
        supportingContent = supporting?.let { { Text(it) } },
        trailingContent = if (chevron) ({ Icon(Icons.AutoMirrored.Filled.KeyboardArrowRight, null) }) else null,
        colors = ListItemDefaults.colors(),
    )
}

/** Permanent deletion; typing the email confirms it, as on the web. Required in-app by the stores. */
@Composable
private fun DeleteAccountDialog(email: String, onDismiss: () -> Unit) {
    val scope = rememberCoroutineScope()
    var typed by rememberSaveable { mutableStateOf("") }
    var deleting by remember { mutableStateOf(false) }
    var error by remember { mutableStateOf<String?>(null) }
    FormDialog(
        title = "Delete account",
        onDismiss = onDismiss,
        canSave = typed.trim().equals(email, ignoreCase = true),
        saving = deleting,
        error = error,
        saveLabel = "Delete",
        onSave = {
            deleting = true
            error = null
            scope.launch {
                attempt { App.container.api.send("POST", "/api/v1/account/delete", DeleteAccountBody(typed.trim())) }
                    // The account and this phone's registration are gone server-side; just forget the session.
                    .onSuccess { App.container.session.clear() }
                    .onFailure { error = it.userMessage("Something went wrong. Nothing was deleted — try again.") }
                deleting = false
            }
        },
    ) {
        Text(
            "This permanently deletes your organisation and everything in it — monitors, alert history, status page, notification channels, API keys and your login. Any active subscription is cancelled immediately. This can’t be undone.",
            style = MaterialTheme.typography.bodyMedium,
        )
        Field("Type $email to confirm", typed, { typed = it }, keyboard = KeyboardOptions(keyboardType = KeyboardType.Email, autoCorrectEnabled = false))
    }
}

// ---------------- notification channels ----------------

private val channelTypes = listOf("telegram" to "Telegram", "slack" to "Slack", "email" to "Email", "webhook" to "Webhook")

private fun channelIcon(type: String) = when (type) {
    "telegram" -> Icons.AutoMirrored.Outlined.Send
    "slack" -> Icons.Outlined.Tag
    "email" -> Icons.Outlined.Email
    "webhook" -> Icons.Outlined.Link
    "fcm", "apns" -> Icons.Outlined.PhoneAndroid
    else -> Icons.Outlined.Notifications
}

private fun channelSubtitle(c: Channel) = when (c.type) {
    "fcm" -> "Android push · this organisation’s phones"
    "apns" -> "Apple push · this organisation’s iPhones"
    "webpush" -> "Browser push"
    else -> channelTypes.firstOrNull { it.first == c.type }?.second ?: c.type.replaceFirstChar { it.uppercase() }
}

/** Where alerts go, with a visible Test button. Mirrors iOS ChannelsView. */
@Composable
fun ChannelsScreen(onBack: () -> Unit) {
    val api = App.container.api
    val loader = rememberLoader("channels") { api.get<Page<Channel>>("/api/v1/notification-channels", "limit" to "200").data }
    var creating by rememberSaveable { mutableStateOf(false) }
    var editing by remember { mutableStateOf<Channel?>(null) }
    val snack = LocalSnack.current
    val scope = rememberCoroutineScope()

    fun act(done: String?, fallback: String, block: suspend () -> Unit) {
        scope.launch {
            attempt { block() }
                .onSuccess { loader.reload(); done?.let { snack.showSnackbar(it) } }
                .onFailure { snack.showSnackbar(it.userMessage(fallback)) }
        }
    }

    Screen("Notification channels", onBack, fab = {
        FloatingActionButton(onClick = { creating = true }) { Icon(Icons.Default.Add, "Add channel") }
    }) { padding ->
        Column(Modifier.padding(padding)) {
            LoadableContent(loader) { channels ->
                if (channels.isEmpty()) {
                    Message(Icons.Outlined.NotificationsOff, "No channels", "Add a channel to receive alerts.", "Add channel") { creating = true }
                } else {
                    LazyColumn(Modifier.fillMaxSize()) {
                        items(channels, key = { it.id }) { c ->
                            ListItem(
                                modifier = if (c.isManaged) Modifier else Modifier.clickable { editing = c },
                                leadingContent = { Icon(channelIcon(c.type), null) },
                                headlineContent = { Text(c.name, fontWeight = FontWeight.Medium) },
                                supportingContent = { Text(channelSubtitle(c)) },
                                trailingContent = {
                                    Row(verticalAlignment = Alignment.CenterVertically) {
                                        OutlinedButton(onClick = {
                                            act("Test notification sent.", "Test failed.") { api.send("POST", "/api/v1/notification-channels/${c.id}/test") }
                                        }) { Text("Test") }
                                        Spacer(Modifier.width(8.dp))
                                        Switch(c.enabled, onCheckedChange = { on ->
                                            act(null, "Couldn’t update the channel.") {
                                                api.send("PATCH", "/api/v1/notification-channels/${c.id}", ChannelBody(enabled = on))
                                            }
                                        })
                                    }
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

    if (creating) ChannelForm(null, onDismiss = { creating = false }) { creating = false; loader.reload() }
    editing?.let { c -> ChannelForm(c, onDismiss = { editing = null }) { editing = null; loader.reload() } }
}

/** Create or edit a channel; fields follow the type. A blank secret on edit keeps the stored one. */
@Composable
private fun ChannelForm(existing: Channel?, onDismiss: () -> Unit, onSaved: () -> Unit) {
    val api = App.container.api
    val scope = rememberCoroutineScope()
    val creating = existing == null
    val c = existing?.config.orEmpty()
    var name by rememberSaveable { mutableStateOf(existing?.name.orEmpty()) }
    var type by rememberSaveable { mutableStateOf(existing?.type ?: "telegram") }
    var enabled by rememberSaveable { mutableStateOf(existing?.enabled ?: true) }
    var secret by rememberSaveable { mutableStateOf("") }
    var chatId by rememberSaveable { mutableStateOf(c["chat_id"].orEmpty()) }
    var host by rememberSaveable { mutableStateOf(c["host"].orEmpty()) }
    var port by rememberSaveable { mutableStateOf(c["port"].orEmpty()) }
    var security by rememberSaveable { mutableStateOf(c["security"] ?: "starttls") }
    var username by rememberSaveable { mutableStateOf(c["username"].orEmpty()) }
    var from by rememberSaveable { mutableStateOf(c["from"].orEmpty()) }
    var to by rememberSaveable { mutableStateOf(c["to"].orEmpty()) }
    var url by rememberSaveable { mutableStateOf(c["url"].orEmpty()) }
    var method by rememberSaveable { mutableStateOf(c["method"] ?: "POST") }
    var saving by remember { mutableStateOf(false) }
    var error by remember { mutableStateOf<String?>(null) }
    var confirmDelete by remember { mutableStateOf(false) }

    val hasSecret = !creating || secret.isNotBlank()
    val canSave = name.isNotBlank() && when (type) {
        "telegram" -> chatId.isNotBlank() && hasSecret
        "slack" -> hasSecret
        "email" -> host.isNotBlank() && from.isNotBlank() && to.isNotBlank()
        "webhook" -> url.isNotBlank()
        else -> false
    }

    fun config(): Map<String, String> = when (type) {
        "telegram" -> mapOf("chat_id" to chatId.trim())
        "email" -> buildMap {
            put("host", host.trim()); put("from", from.trim()); put("to", to.trim()); put("security", security)
            port.trim().takeIf { it.isNotEmpty() }?.let { put("port", it) }
            username.trim().takeIf { it.isNotEmpty() }?.let { put("username", it) }
        }
        "webhook" -> mapOf("url" to url.trim(), "method" to method)
        else -> emptyMap()
    }

    fun run(fallback: String, block: suspend () -> Unit) {
        saving = true
        error = null
        scope.launch {
            attempt { block() }.onSuccess { onSaved() }.onFailure { error = it.userMessage(fallback) }
            saving = false
        }
    }

    val secretLabel = { label: String -> if (creating) label else "$label (leave blank to keep)" }

    FormDialog(
        title = if (creating) "New channel" else "Edit channel",
        onDismiss = onDismiss,
        canSave = canSave,
        saving = saving,
        error = error,
        onSave = {
            run("Couldn’t save the channel.") {
                if (existing == null) {
                    api.send("POST", "/api/v1/notification-channels", ChannelBody(name.trim(), type, enabled, config(), secret.trim()))
                } else {
                    api.send("PATCH", "/api/v1/notification-channels/${existing.id}",
                        ChannelBody(name = name.trim(), enabled = enabled, config = config(), secret = secret.trim().ifEmpty { null }))
                }
            }
        },
    ) {
        Field("Name", name, { name = it })
        if (creating) Dropdown("Type", channelTypes, type, { type = it }) else InfoRow("Type", channelSubtitle(existing!!))
        SwitchRow("Enabled", enabled, { enabled = it })
        when (type) {
            "telegram" -> {
                SectionTitle("Telegram")
                Field("Chat ID", chatId, { chatId = it })
                SecretField(secretLabel("Bot token"), secret, { secret = it })
            }
            "slack" -> {
                SectionTitle("Slack")
                SecretField(secretLabel("Incoming webhook URL"), secret, { secret = it }, KeyboardOptions(keyboardType = KeyboardType.Uri))
            }
            "email" -> {
                SectionTitle("SMTP server")
                Field("Host", host, { host = it }, keyboard = KeyboardOptions(keyboardType = KeyboardType.Uri, autoCorrectEnabled = false))
                Field("Port", port, { port = it.filter(Char::isDigit) }, keyboard = KeyboardOptions(keyboardType = KeyboardType.Number))
                Dropdown("Security", listOf("starttls" to "STARTTLS", "tls" to "TLS", "none" to "None"), security, { security = it })
                Field("Username", username, { username = it }, keyboard = KeyboardOptions(autoCorrectEnabled = false))
                SecretField(secretLabel("Password"), secret, { secret = it }, KeyboardOptions(keyboardType = KeyboardType.Password))
                SectionTitle("Message")
                Field("From", from, { from = it }, keyboard = KeyboardOptions(keyboardType = KeyboardType.Email, autoCorrectEnabled = false))
                Field("To", to, { to = it }, keyboard = KeyboardOptions(keyboardType = KeyboardType.Email, autoCorrectEnabled = false))
            }
            "webhook" -> {
                SectionTitle("Webhook")
                Field("URL", url, { url = it }, keyboard = KeyboardOptions(keyboardType = KeyboardType.Uri, autoCorrectEnabled = false))
                Dropdown("Method", listOf("POST" to "POST", "PUT" to "PUT"), method, { method = it })
            }
        }
        if (existing != null) {
            Spacer(Modifier.height(8.dp))
            TextButton(onClick = { confirmDelete = true }, enabled = !saving) {
                Icon(Icons.Outlined.Delete, null, tint = MaterialTheme.colorScheme.error)
                Spacer(Modifier.width(8.dp))
                Text("Delete channel", color = MaterialTheme.colorScheme.error)
            }
        }
    }

    if (confirmDelete && existing != null) {
        ConfirmDialog("Delete ${existing.name}?", "Alerts stop going to this channel.", "Delete",
            onConfirm = { run("Couldn’t delete the channel.") { api.send("DELETE", "/api/v1/notification-channels/${existing.id}") } },
            onDismiss = { confirmDelete = false })
    }
}

// ---------------- maintenance windows ----------------

/** Planned work that silences alerts. Mirrors iOS MaintenanceView. */
@Composable
fun MaintenanceScreen(onBack: () -> Unit) {
    val api = App.container.api
    val loader = rememberLoader("maintenance") { api.get<Page<MaintenanceWindow>>("/api/v1/maintenance-windows", "limit" to "200").data }
    var creating by rememberSaveable { mutableStateOf(false) }
    var editing by remember { mutableStateOf<MaintenanceWindow?>(null) }
    var deleting by remember { mutableStateOf<MaintenanceWindow?>(null) }
    val snack = LocalSnack.current
    val scope = rememberCoroutineScope()

    Screen("Maintenance", onBack, fab = {
        FloatingActionButton(onClick = { creating = true }) { Icon(Icons.Default.Add, "Schedule maintenance") }
    }) { padding ->
        Column(Modifier.padding(padding)) {
            LoadableContent(loader) { windows ->
                if (windows.isEmpty()) {
                    Message(Icons.Outlined.Build, "No maintenance windows", "Schedule one to pause alerts during planned work.", "Schedule") { creating = true }
                } else {
                    LazyColumn(Modifier.fillMaxSize()) {
                        items(windows, key = { it.id }) { w ->
                            ListItem(
                                modifier = Modifier.clickable { editing = w },
                                headlineContent = {
                                    Row(verticalAlignment = Alignment.CenterVertically) {
                                        Text(w.title, fontWeight = FontWeight.Medium, modifier = Modifier.weight(1f, fill = false))
                                        if (w.active) { Spacer(Modifier.width(8.dp)); Pill("Active", Amber) }
                                    }
                                },
                                supportingContent = {
                                    Column {
                                        Text("${dateTime(w.startsAt)}  →  ${dateTime(w.endsAt)}")
                                        Text(
                                            when (w.scope) {
                                                "org" -> "Whole organisation"
                                                "monitor" -> "${w.scopeIds.size} monitor(s)"
                                                "project" -> "${w.scopeIds.size} project(s)"
                                                else -> w.scope
                                            },
                                            style = MaterialTheme.typography.bodySmall,
                                        )
                                    }
                                },
                                trailingContent = { OverflowMenu(MenuItem("Delete", Icons.Outlined.Delete, destructive = true) { deleting = w }) },
                            )
                            HorizontalDivider()
                        }
                        item { Spacer(Modifier.height(88.dp)) }
                    }
                }
            }
        }
    }

    if (creating) MaintenanceForm(null, onDismiss = { creating = false }) { creating = false; loader.reload() }
    editing?.let { w -> MaintenanceForm(w, onDismiss = { editing = null }) { editing = null; loader.reload() } }
    deleting?.let { w ->
        ConfirmDialog("Delete ${w.title}?", "Alerts for what it covers resume as normal.", "Delete",
            onConfirm = {
                scope.launch {
                    attempt { api.send("DELETE", "/api/v1/maintenance-windows/${w.id}") }
                        .onSuccess { loader.reload() }
                        .onFailure { snack.showSnackbar(it.userMessage("Couldn’t delete the window.")) }
                }
            },
            onDismiss = { deleting = null })
    }
}

/** Whole organisation or chosen monitors (project scope stays on the web, as on iOS). */
@Composable
private fun MaintenanceForm(existing: MaintenanceWindow?, onDismiss: () -> Unit, onSaved: () -> Unit) {
    val api = App.container.api
    val scope = rememberCoroutineScope()
    val now = Instant.now().truncatedTo(ChronoUnit.MINUTES)
    var title by rememberSaveable { mutableStateOf(existing?.title.orEmpty()) }
    var notes by rememberSaveable { mutableStateOf(existing?.description.orEmpty()) }
    var starts by rememberSaveable { mutableStateOf((existing?.startsAt ?: now).toEpochMilli()) }
    var ends by rememberSaveable { mutableStateOf((existing?.endsAt ?: now.plus(Duration.ofHours(1))).toEpochMilli()) }
    var scopeKind by rememberSaveable { mutableStateOf(if (existing?.scope == "monitor") "monitor" else "org") }
    var selected by remember { mutableStateOf(if (existing?.scope == "monitor") existing.scopeIds.toSet() else emptySet()) }
    var monitors by remember { mutableStateOf<List<Monitor>?>(null) }
    var saving by remember { mutableStateOf(false) }
    var error by remember { mutableStateOf<String?>(null) }

    LaunchedEffect(Unit) {
        monitors = attempt { api.get<Page<Monitor>>("/api/v1/monitors", "limit" to "200").data }.getOrDefault(emptyList())
    }

    FormDialog(
        title = if (existing == null) "New window" else "Edit window",
        onDismiss = onDismiss,
        canSave = title.isNotBlank() && ends > starts && (scopeKind == "org" || selected.isNotEmpty()),
        saving = saving,
        error = error,
        onSave = {
            saving = true
            error = null
            val body = WindowBody(
                title.trim(), notes.trim(), Instant.ofEpochMilli(starts), Instant.ofEpochMilli(ends),
                scopeKind, if (scopeKind == "monitor") selected.toList() else emptyList(),
            )
            scope.launch {
                attempt {
                    if (existing == null) api.send("POST", "/api/v1/maintenance-windows", body)
                    else api.send("PATCH", "/api/v1/maintenance-windows/${existing.id}", body)
                }.onSuccess { onSaved() }.onFailure { error = it.userMessage("Couldn’t save the window.") }
                saving = false
            }
        },
    ) {
        Field("Title", title, { title = it })
        Field("Notes (optional)", notes, { notes = it }, singleLine = false)
        SectionTitle("When")
        DateTimeField("Starts", Instant.ofEpochMilli(starts)) { starts = it.toEpochMilli() }
        DateTimeField("Ends", Instant.ofEpochMilli(ends)) { ends = it.toEpochMilli() }
        if (ends <= starts) Text("The end must be after the start.", color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodySmall)
        SectionTitle("What it covers")
        Segmented(listOf("org" to "Whole organisation", "monitor" to "Specific monitors"), scopeKind, { scopeKind = it })
        if (scopeKind == "monitor") {
            val list = monitors
            if (list == null) {
                Text("Loading monitors…", color = MaterialTheme.colorScheme.onSurfaceVariant)
            } else {
                Column(verticalArrangement = Arrangement.spacedBy(0.dp)) {
                    list.forEach { m ->
                        Row(
                            Modifier.fillMaxWidth().clickable { selected = if (m.id in selected) selected - m.id else selected + m.id },
                            verticalAlignment = Alignment.CenterVertically,
                        ) {
                            Checkbox(checked = m.id in selected, onCheckedChange = { selected = if (it) selected + m.id else selected - m.id })
                            Text(m.name)
                        }
                    }
                }
            }
        }
    }
}
