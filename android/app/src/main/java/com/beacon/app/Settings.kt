package com.beacon.app

import android.content.Intent
import android.provider.Settings as AndroidSettings
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
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
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.selection.toggleable
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.Logout
import androidx.compose.material.icons.automirrored.filled.OpenInNew
import androidx.compose.material.icons.automirrored.filled.Send
import androidx.compose.material.icons.filled.Build
import androidx.compose.material.icons.filled.CheckCircle
import androidx.compose.material.icons.filled.Description
import androidx.compose.material.icons.filled.Email
import androidx.compose.material.icons.filled.Info
import androidx.compose.material.icons.filled.Link
import androidx.compose.material.icons.filled.Notifications
import androidx.compose.material.icons.filled.NotificationsActive
import androidx.compose.material.icons.filled.NotificationsOff
import androidx.compose.material.icons.filled.PhoneAndroid
import androidx.compose.material.icons.filled.PrivacyTip
import androidx.compose.material.icons.filled.Tag
import androidx.compose.material.icons.filled.Warning
import androidx.compose.material.icons.outlined.Build
import androidx.compose.material.icons.outlined.Business
import androidx.compose.material.icons.outlined.Checklist
import androidx.compose.material.icons.outlined.Delete
import androidx.compose.material.icons.outlined.NotificationsOff
import androidx.compose.material.icons.outlined.RadioButtonUnchecked
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
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.lifecycle.compose.LifecycleResumeEffect
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import kotlinx.coroutines.launch
import java.time.Duration
import java.time.Instant
import java.time.temporal.ChronoUnit

private val base = BuildConfig.API_BASE_URL.trimEnd('/')
private val version = "${BuildConfig.VERSION_NAME} (${BuildConfig.VERSION_CODE})"

/** The profile, alert delivery, push status, about, sign-out and account deletion. Mirrors iOS SettingsView. */
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
        Column(
            Modifier.padding(padding).fillMaxSize().verticalScroll(rememberScrollState()).padding(start = 16.dp, end = 16.dp, top = 4.dp, bottom = 24.dp),
            verticalArrangement = Arrangement.spacedBy(22.dp),
        ) {
            user?.let { ProfileCard(it) }

            Group("Alerts") {
                SettingsRow("Notification channels", Icons.Filled.NotificationsActive, Theme.accent, "Where your alerts are sent", onClick = openChannels) { Chevron() }
                RowDivider(Modifier.padding(start = 59.dp))
                SettingsRow("Maintenance windows", Icons.Filled.Build, Degraded, "Silence alerts during planned work", onClick = openMaintenance) { Chevron() }
                RowDivider(Modifier.padding(start = 59.dp))
                when {
                    !app.push.configured -> SettingsRow("Push notifications", Icons.Filled.Notifications, Paused, "Not set up in this build") {
                        Pill("Not set up", Paused)
                    }
                    allowed -> SettingsRow("Push notifications", Icons.Filled.PhoneAndroid, Up, "Alerts arrive on this phone") { Pill("On", Up) }
                    else -> SettingsRow(
                        "Push notifications", Icons.Filled.NotificationsOff, Degraded, "Turned off — tap to enable",
                        onClick = {
                            context.startActivity(
                                Intent(AndroidSettings.ACTION_APP_NOTIFICATION_SETTINGS).putExtra(AndroidSettings.EXTRA_APP_PACKAGE, context.packageName),
                            )
                        },
                    ) { Pill("Off", Degraded) }
                }
            }

            Group("About") {
                SettingsRow("Privacy policy", Icons.Filled.PrivacyTip, Blue, onClick = { openUrl(context, "$base/privacy") }) { ExternalMark() }
                RowDivider(Modifier.padding(start = 59.dp))
                SettingsRow("Terms of service", Icons.Filled.Description, Violet, onClick = { openUrl(context, "$base/terms") }) { ExternalMark() }
                RowDivider(Modifier.padding(start = 59.dp))
                SettingsRow("Version", Icons.Filled.Info, Paused) {
                    Text(version, style = TextStyle(fontSize = 15.sp, fontFeatureSettings = "tnum"), color = Theme.muted)
                }
            }

            Row(
                Modifier.fillMaxWidth().card().clickable { confirmSignOut = true }.padding(vertical = 14.dp),
                horizontalArrangement = Arrangement.Center,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Icon(Icons.AutoMirrored.Filled.Logout, null, Modifier.size(19.dp), tint = Down)
                Spacer(Modifier.width(8.dp))
                Text("Sign out", style = TextStyle(fontSize = 16.sp, fontWeight = FontWeight.SemiBold), color = Down)
            }

            user?.let { DangerZone(it) { deletingAccount = true } }

            Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.Center, verticalAlignment = Alignment.CenterVertically) {
                BrandTile(22.dp)
                Spacer(Modifier.width(8.dp))
                Text("${stringResource(R.string.app_name)} · $version", style = MaterialTheme.typography.bodySmall, color = Theme.muted)
            }
        }
    }

    if (confirmSignOut) {
        ConfirmDialog("Sign out?", "This phone stops getting alerts until you sign in again.", "Sign out",
            onConfirm = { scope.launch { app.signOut() } }, onDismiss = { confirmSignOut = false })
    }
    if (deletingAccount) user?.let { DeleteAccountDialog(it.email) { deletingAccount = false } }
}

/** "Jane Doe" → "JD", for the avatar. */
internal fun initials(name: String): String =
    name.split(" ").filter { it.isNotBlank() }.take(2).map { it.first() }.joinToString("").uppercase().ifEmpty { "?" }

/** Who's signed in, on the brand gradient with faint rings. */
@Composable
private fun ProfileCard(user: AuthUser) {
    val shape = RoundedCornerShape(18.dp)
    val glow = Theme.accent.copy(alpha = 0.45f)
    Row(
        Modifier.fillMaxWidth()
            .shadow(10.dp, shape, ambientColor = glow, spotColor = glow)
            .clip(shape)
            .background(Theme.brandGradient)
            .drawBehind {
                // As on iOS: each ring's top-right sits a third of its size beyond the card's corner.
                listOf(120.dp, 190.dp, 260.dp).forEach { d ->
                    val r = d.toPx() / 2
                    drawRings(listOf(d), Offset(size.width - r / 3, r / 3), alpha = 0.09f)
                }
            }
            .padding(20.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Box(
            Modifier.size(64.dp).background(Color.White.copy(alpha = 0.18f), CircleShape).border(1.5.dp, Color.White.copy(alpha = 0.35f), CircleShape),
            contentAlignment = Alignment.Center,
        ) {
            Text(initials(user.name), style = TextStyle(fontSize = 24.sp, fontWeight = FontWeight.Bold), color = Color.White)
        }
        Spacer(Modifier.width(16.dp))
        Column(verticalArrangement = Arrangement.spacedBy(3.dp)) {
            Text(user.name, style = TextStyle(fontSize = 20.sp, fontWeight = FontWeight.Bold), color = Color.White, maxLines = 1, overflow = TextOverflow.Ellipsis)
            Text(user.email, style = TextStyle(fontSize = 14.sp), color = Color.White.copy(alpha = 0.85f), maxLines = 1, overflow = TextOverflow.Ellipsis)
            Spacer(Modifier.height(3.dp))
            Text(
                user.role.replaceFirstChar { it.uppercase() },
                Modifier.background(Color.White.copy(alpha = 0.2f), CircleShape).padding(horizontal = 9.dp, vertical = 3.dp),
                style = TextStyle(fontSize = 12.sp, fontWeight = FontWeight.SemiBold),
                color = Color.White,
            )
        }
    }
}

/** A titled card holding settings rows. */
@Composable
private fun Group(title: String, content: @Composable ColumnScope.() -> Unit) {
    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Eyebrow(title, Modifier.padding(start = 4.dp))
        Column(Modifier.fillMaxWidth().card(), content = content)
    }
}

/** One settings row: a tinted icon tile, a title with an optional subtitle, and a trailing accessory. */
@Composable
private fun SettingsRow(
    title: String,
    icon: ImageVector,
    tint: Color,
    subtitle: String? = null,
    onClick: (() -> Unit)? = null,
    trailing: @Composable () -> Unit = {},
) {
    Row(
        Modifier.fillMaxWidth().then(if (onClick != null) Modifier.clickable(onClick = onClick) else Modifier).padding(horizontal = 14.dp, vertical = 11.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        IconTile(icon, tint, size = 32.dp, iconSize = 17.dp, radius = 9.dp, filled = true)
        Spacer(Modifier.width(13.dp))
        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
            Text(title, style = TextStyle(fontSize = 16.sp, fontWeight = FontWeight.Medium), color = Theme.text)
            if (subtitle != null) Text(subtitle, style = MaterialTheme.typography.bodySmall, color = Theme.muted)
        }
        Spacer(Modifier.width(8.dp))
        trailing()
    }
}

@Composable
private fun ExternalMark() {
    Icon(Icons.AutoMirrored.Filled.OpenInNew, null, Modifier.size(16.dp), tint = Theme.muted.copy(alpha = 0.7f))
}

/** The red card that holds account deletion. */
@Composable
private fun DangerZone(user: AuthUser, onDelete: () -> Unit) {
    val shape = RoundedCornerShape(14.dp)
    Column(
        Modifier.fillMaxWidth().clip(shape).background(Down.copy(alpha = 0.06f)).border(1.dp, Down.copy(alpha = 0.3f), shape).padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Icon(Icons.Filled.Warning, null, Modifier.size(18.dp), tint = Down)
            Spacer(Modifier.width(6.dp))
            Text("Danger zone", style = TextStyle(fontSize = 15.sp, fontWeight = FontWeight.SemiBold), color = Down)
        }
        Text(
            "Deleting your account permanently removes your organisation, its monitors and all their data.",
            style = TextStyle(fontSize = 13.sp, lineHeight = 18.sp),
            color = Theme.muted,
        )
        if (user.role == "owner") {
            Text(
                "Delete account",
                Modifier.padding(top = 2.dp).clip(RoundedCornerShape(10.dp)).background(Down).clickable(role = Role.Button, onClick = onDelete)
                    .padding(horizontal = 16.dp, vertical = 9.dp),
                style = TextStyle(fontSize = 14.sp, fontWeight = FontWeight.SemiBold),
                color = Color.White,
            )
        } else {
            Text("Only your organisation’s owner can delete this account.", style = TextStyle(fontSize = 13.sp, fontWeight = FontWeight.Medium))
        }
    }
}

/** Permanent deletion; typing the email confirms it, as on the web. Required in-app by the stores. */
@Composable
private fun DeleteAccountDialog(email: String, onDismiss: () -> Unit) {
    val scope = rememberCoroutineScope()
    var typed by rememberSaveable { mutableStateOf("") }
    var deleting by remember { mutableStateOf(false) }
    var error by remember { mutableStateOf<String?>(null) }
    val matches = typed.trim().equals(email, ignoreCase = true)
    FormDialog(
        title = "Delete account",
        onDismiss = onDismiss,
        canSave = matches,
        saving = deleting,
        error = error,
        onSave = null,
    ) {
        val shape = RoundedCornerShape(14.dp)
        Column(
            Modifier.fillMaxWidth().clip(shape).background(Down.copy(alpha = 0.06f)).border(1.dp, Down.copy(alpha = 0.3f), shape).padding(16.dp),
            verticalArrangement = Arrangement.spacedBy(10.dp),
        ) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Icon(Icons.Filled.Warning, null, Modifier.size(20.dp), tint = Down)
                Spacer(Modifier.width(8.dp))
                Text("This can’t be undone", style = TextStyle(fontSize = 17.sp, fontWeight = FontWeight.SemiBold), color = Down)
            }
            Text(
                "This permanently deletes your organisation and everything in it — monitors, alert history, status page, notification channels, API keys and your login. Any active subscription is cancelled immediately.",
                style = MaterialTheme.typography.bodyMedium,
            )
        }
        FormCard {
            Field("Type your email to confirm") {
                Input(typed, { typed = it }, placeholder = email, keyboard = KeyboardOptions(keyboardType = KeyboardType.Email, autoCorrectEnabled = false))
            }
            AccentButton(
                "Delete my account permanently",
                onClick = {
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
                modifier = Modifier.fillMaxWidth(),
                enabled = matches,
                busy = deleting,
                color = Down,
            )
        }
    }
}

// ---------------- notification channels ----------------

private val channelTypes = listOf(
    Choice("telegram", "Telegram", Icons.AutoMirrored.Filled.Send),
    Choice("slack", "Slack", Icons.Filled.Tag),
    Choice("email", "Email", Icons.Filled.Email),
    Choice("webhook", "Webhook", Icons.Filled.Link),
)

private fun channelIcon(type: String) = when (type) {
    "telegram" -> Icons.AutoMirrored.Filled.Send
    "slack" -> Icons.Filled.Tag
    "email" -> Icons.Filled.Email
    "webhook" -> Icons.Filled.Link
    "fcm", "apns" -> Icons.Filled.PhoneAndroid
    else -> Icons.Filled.Notifications
}

private fun channelTint(type: String) = when (type) {
    "telegram" -> Blue
    "slack" -> Violet
    "email" -> Degraded
    "webhook" -> Color(0xFF64748B)
    else -> Up
}

private fun channelSubtitle(c: Channel) = when (c.type) {
    "fcm" -> "Android push · this organisation’s phones"
    "apns" -> "Apple push · this organisation’s iPhones"
    "webpush" -> "Browser push"
    else -> channelTypes.firstOrNull { it.value == c.type }?.label ?: c.type.replaceFirstChar { it.uppercase() }
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

    Screen("Notification channels", onBack, fab = { AddFab("Add channel") { creating = true } }) { padding ->
        Column(Modifier.padding(padding)) {
            LoadableContent(loader) { channels ->
                if (channels.isEmpty()) {
                    Message(Icons.Outlined.NotificationsOff, "No channels", "Add a channel to receive alerts.", "Add channel") { creating = true }
                } else {
                    LazyColumn(Modifier.fillMaxSize(), contentPadding = PaddingValues(16.dp, 4.dp, 16.dp, 0.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
                        items(channels, key = { it.id }) { c ->
                            Row(
                                Modifier.fillMaxWidth().card().clickable(enabled = !c.isManaged) { editing = c }
                                    .padding(start = 14.dp, end = 10.dp, top = 12.dp, bottom = 12.dp),
                                verticalAlignment = Alignment.CenterVertically,
                            ) {
                                IconTile(channelIcon(c.type), channelTint(c.type))
                                Spacer(Modifier.width(12.dp))
                                Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
                                    Text(c.name, style = TextStyle(fontSize = 16.sp, fontWeight = FontWeight.SemiBold), maxLines = 1, overflow = TextOverflow.Ellipsis)
                                    Text(channelSubtitle(c), style = MaterialTheme.typography.bodySmall, color = Theme.muted, maxLines = 2)
                                }
                                Spacer(Modifier.width(8.dp))
                                OutlinedButton(
                                    onClick = { act("Test notification sent.", "Test failed.") { api.send("POST", "/api/v1/notification-channels/${c.id}/test") } },
                                    shape = RoundedCornerShape(8.dp),
                                    border = BorderStroke(1.dp, Theme.border),
                                    contentPadding = PaddingValues(horizontal = 12.dp),
                                    modifier = Modifier.height(32.dp),
                                ) { Text("Test", fontSize = 13.sp, fontWeight = FontWeight.SemiBold, color = Theme.accent) }
                                Spacer(Modifier.width(8.dp))
                                AccentSwitch(c.enabled) { on ->
                                    act(null, "Couldn’t update the channel.") {
                                        api.send("PATCH", "/api/v1/notification-channels/${c.id}", ChannelBody(enabled = on))
                                    }
                                }
                            }
                        }
                        item { FabClearance() }
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

    /** The type's secret: never shown on edit, where blank keeps the stored one. */
    @Composable
    fun SecretInput(placeholder: String, keyboard: KeyboardOptions = KeyboardOptions(autoCorrectEnabled = false)) =
        Input(secret, { secret = it }, placeholder = if (creating) placeholder else "Leave blank to keep the saved value", secret = true, keyboard = keyboard)

    val plain = KeyboardOptions(autoCorrectEnabled = false)
    val uri = KeyboardOptions(keyboardType = KeyboardType.Uri, autoCorrectEnabled = false)
    val mail = KeyboardOptions(keyboardType = KeyboardType.Email, autoCorrectEnabled = false)

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
        FormCard("Channel") {
            Field("Name") { Input(name, { name = it }, placeholder = "e.g. On-call team") }
            Field("Type") {
                if (creating) ChoiceChips(channelTypes, type, { type = it }, minWidth = 110.dp) else ReadOnlyValue(channelSubtitle(existing!!))
            }
            SwitchRow("Enabled", enabled, { enabled = it }, "Send alerts to this channel")
        }
        when (type) {
            "telegram" -> FormCard("Telegram", footer = "Message @BotFather to create a bot, then add it to your chat or group.") {
                Field("Chat ID", hint = "A number like 123456789, or -100… for a group.") {
                    Input(chatId, { chatId = it }, placeholder = "123456789", keyboard = plain)
                }
                Field("Bot token") { SecretInput("123456:ABC-DEF…") }
            }
            "slack" -> FormCard("Slack") {
                Field("Incoming webhook URL", hint = "From your Slack app’s Incoming Webhooks page.") {
                    SecretInput("https://hooks.slack.com/services/…", uri)
                }
            }
            "email" -> {
                FormCard("SMTP server") {
                    Field("Host") { Input(host, { host = it }, placeholder = "smtp.example.com", keyboard = uri) }
                    Field("Port") { Input(port, { port = it.filter(Char::isDigit) }, placeholder = "587", keyboard = KeyboardOptions(keyboardType = KeyboardType.Number)) }
                    Field("Security") {
                        ChoiceChips(listOf(Choice("starttls", "STARTTLS"), Choice("tls", "TLS"), Choice("none", "None")), security, { security = it }, minWidth = 90.dp)
                    }
                    Field("Username") { Input(username, { username = it }, placeholder = "Optional", keyboard = plain) }
                    Field("Password") { SecretInput("Optional", KeyboardOptions(keyboardType = KeyboardType.Password)) }
                }
                FormCard("Message") {
                    Field("From") { Input(from, { from = it }, placeholder = "alerts@example.com", keyboard = mail) }
                    Field("To") { Input(to, { to = it }, placeholder = "oncall@example.com", keyboard = mail) }
                }
            }
            "webhook" -> FormCard("Webhook") {
                Field("URL") { Input(url, { url = it }, placeholder = "https://example.com/hooks/alerts", mono = true, keyboard = uri) }
                Field("Method") { ChoiceChips(listOf(Choice("POST", "POST"), Choice("PUT", "PUT")), method, { method = it }, minWidth = 90.dp) }
            }
        }
        if (existing != null) {
            Row(
                Modifier.fillMaxWidth().card().clickable(enabled = !saving) { confirmDelete = true }.padding(vertical = 14.dp),
                horizontalArrangement = Arrangement.Center,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Icon(Icons.Outlined.Delete, null, Modifier.size(19.dp), tint = Down)
                Spacer(Modifier.width(8.dp))
                Text("Delete channel", style = TextStyle(fontSize = 16.sp, fontWeight = FontWeight.SemiBold), color = Down)
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

    Screen("Maintenance", onBack, fab = { AddFab("Schedule maintenance") { creating = true } }) { padding ->
        Column(Modifier.padding(padding)) {
            LoadableContent(loader) { windows ->
                if (windows.isEmpty()) {
                    Message(Icons.Outlined.Build, "No maintenance windows", "Schedule one to pause alerts during planned work.", "Schedule") { creating = true }
                } else {
                    LazyColumn(Modifier.fillMaxSize(), contentPadding = PaddingValues(16.dp, 4.dp, 16.dp, 0.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
                        items(windows, key = { it.id }) { w ->
                            Row(
                                Modifier.fillMaxWidth().card().clickable { editing = w }.padding(start = 14.dp, end = 2.dp, top = 14.dp, bottom = 14.dp),
                                verticalAlignment = Alignment.CenterVertically,
                            ) {
                                IconTile(Icons.Outlined.Build, if (w.active) Degraded else Paused)
                                Spacer(Modifier.width(12.dp))
                                Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(4.dp)) {
                                    Row(verticalAlignment = Alignment.CenterVertically) {
                                        Text(
                                            w.title, Modifier.weight(1f, fill = false),
                                            style = TextStyle(fontSize = 16.sp, fontWeight = FontWeight.SemiBold), maxLines = 1, overflow = TextOverflow.Ellipsis,
                                        )
                                        if (w.active) { Spacer(Modifier.width(8.dp)); Pill("Active", Degraded, uppercase = true) }
                                    }
                                    Text("${dateTime(w.startsAt)}  →  ${dateTime(w.endsAt)}", style = MaterialTheme.typography.bodySmall, color = Theme.muted)
                                    Text(
                                        when (w.scope) {
                                            "org" -> "Whole organisation"
                                            "monitor" -> "${w.scopeIds.size} monitor(s)"
                                            "project" -> "${w.scopeIds.size} project(s)"
                                            else -> w.scope
                                        },
                                        style = MaterialTheme.typography.bodySmall,
                                        color = Theme.muted,
                                    )
                                }
                                OverflowMenu(MenuItem("Delete", Icons.Outlined.Delete, destructive = true) { deleting = w })
                            }
                        }
                        item { FabClearance() }
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
        FormCard("Window") {
            Field("Title") { Input(title, { title = it }, placeholder = "e.g. Database upgrade") }
            Field("Notes") { Input(notes, { notes = it }, placeholder = "Optional", singleLine = false, minLines = 2) }
        }
        FormCard("When") {
            Field("Starts") { DateTimeField(Instant.ofEpochMilli(starts)) { starts = it.toEpochMilli() } }
            Field("Ends", hint = if (ends <= starts) "The end must be after the start." else null) {
                DateTimeField(Instant.ofEpochMilli(ends)) { ends = it.toEpochMilli() }
            }
        }
        FormCard("What it covers", footer = "Alerts for what the window covers are silenced while it’s active.") {
            ChoiceChips(
                listOf(Choice("org", "Whole organisation", Icons.Outlined.Business), Choice("monitor", "Specific monitors", Icons.Outlined.Checklist)),
                scopeKind, { scopeKind = it }, minWidth = 200.dp,
            )
        }
        if (scopeKind == "monitor") {
            FormCard("Monitors") {
                val list = monitors
                if (list == null) {
                    Text("Loading monitors…", color = Theme.muted)
                } else {
                    Column {
                        list.forEach { m ->
                            val on = m.id in selected
                            Row(
                                Modifier.fillMaxWidth().clip(RoundedCornerShape(8.dp))
                                    .toggleable(on, role = Role.Checkbox) { selected = if (it) selected + m.id else selected - m.id }
                                    .padding(vertical = 8.dp),
                                verticalAlignment = Alignment.CenterVertically,
                            ) {
                                Icon(
                                    if (on) Icons.Filled.CheckCircle else Icons.Outlined.RadioButtonUnchecked, null,
                                    Modifier.size(22.dp), tint = if (on) Theme.accent else Theme.muted.copy(alpha = 0.6f),
                                )
                                Spacer(Modifier.width(10.dp))
                                Text(m.name, Modifier.weight(1f), maxLines = 1, overflow = TextOverflow.Ellipsis)
                                TypeBadge(m.type)
                            }
                        }
                    }
                }
            }
        }
    }
}
