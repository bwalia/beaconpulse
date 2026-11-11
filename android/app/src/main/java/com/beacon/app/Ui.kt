@file:OptIn(ExperimentalMaterial3Api::class)

package com.beacon.app

import android.content.Context
import android.content.Intent
import android.net.Uri
import android.text.format.DateUtils
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.ArrowDropDown
import androidx.compose.material.icons.filled.Close
import androidx.compose.material.icons.filled.MoreVert
import androidx.compose.material.icons.outlined.ErrorOutline
import androidx.compose.material.icons.outlined.Visibility
import androidx.compose.material.icons.outlined.VisibilityOff
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Card
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.DatePicker
import androidx.compose.material3.DatePickerDialog
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilledTonalButton
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SegmentedButton
import androidx.compose.material3.SegmentedButtonDefaults
import androidx.compose.material3.SingleChoiceSegmentedButtonRow
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TimePicker
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.pulltorefresh.PullToRefreshBox
import androidx.compose.material3.rememberDatePickerState
import androidx.compose.material3.rememberTimePickerState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.PathEffect
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.StrokeJoin
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.unit.dp
import androidx.compose.ui.window.Dialog
import androidx.compose.ui.window.DialogProperties
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import androidx.lifecycle.viewmodel.compose.viewModel
import kotlinx.coroutines.launch
import java.time.Instant
import java.time.LocalDate
import java.time.LocalTime
import java.time.ZoneId
import java.time.ZoneOffset
import java.time.format.DateTimeFormatter
import java.time.format.FormatStyle

/** The shell's snackbar, for one-line results ("Test notification sent."). */
val LocalSnack = staticCompositionLocalOf<SnackbarHostState> { error("no snackbar host") }

// ---------------- loading ----------------

/** The lifecycle of an async-loaded value; "empty" is the loaded value itself. */
sealed interface Loadable<out T> {
    data object Loading : Loadable<Nothing>
    data class Loaded<T>(val value: T) : Loadable<T>
    data class Failed(val message: String) : Loadable<Nothing>
}

/** Loads a value for one screen, survives rotation, and reloads on demand (pull to refresh, after a write). */
class Loader<T>(private val fetch: suspend () -> T) : ViewModel() {
    var state by mutableStateOf<Loadable<T>>(Loadable.Loading)
        private set
    var refreshing by mutableStateOf(false)
        private set

    init {
        reload()
    }

    fun reload() {
        viewModelScope.launch {
            refreshing = state is Loadable.Loaded
            state = attempt { fetch() }.fold(
                { Loadable.Loaded(it) },
                { Loadable.Failed(it.userMessage("Something went wrong. Please try again.")) },
            )
            refreshing = false
        }
    }
}

/** One [Loader] per [key] for the current screen. */
@Suppress("UNCHECKED_CAST")
@Composable
fun <T> rememberLoader(key: String, fetch: suspend () -> T): Loader<T> =
    viewModel<Loader<*>>(key = key) { Loader(fetch) } as Loader<T>

/** A spinner while loading, a retryable message on failure, else [content] with pull-to-refresh. */
@Composable
fun <T> LoadableContent(loader: Loader<T>, content: @Composable (T) -> Unit) {
    when (val state = loader.state) {
        Loadable.Loading -> Box(Modifier.fillMaxSize(), Alignment.Center) { CircularProgressIndicator() }
        is Loadable.Failed -> Message(Icons.Outlined.ErrorOutline, "Couldn’t load", state.message, "Try again", loader::reload)
        is Loadable.Loaded -> PullToRefreshBox(loader.refreshing, loader::reload, Modifier.fillMaxSize()) { content(state.value) }
    }
}

/** A centred empty / error state. Scrollable so pull-to-refresh still works on it. */
@Composable
fun Message(icon: ImageVector, title: String, text: String, action: String? = null, onAction: () -> Unit = {}) {
    Column(
        Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(32.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp, Alignment.CenterVertically),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Spacer(Modifier.height(48.dp))
        Icon(icon, null, Modifier.size(48.dp), tint = MaterialTheme.colorScheme.onSurfaceVariant)
        Text(title, style = MaterialTheme.typography.titleMedium, textAlign = TextAlign.Center)
        Text(text, style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant, textAlign = TextAlign.Center)
        if (action != null) FilledTonalButton(onClick = onAction) { Text(action) }
    }
}

// ---------------- screens & forms ----------------

/** A screen inside the shell: top bar (with back when nested) over [content]. */
@Composable
fun Screen(
    title: String,
    onBack: (() -> Unit)? = null,
    actions: @Composable RowScope.() -> Unit = {},
    fab: @Composable () -> Unit = {},
    content: @Composable (PaddingValues) -> Unit,
) {
    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text(title, maxLines = 1, overflow = TextOverflow.Ellipsis) },
                navigationIcon = {
                    if (onBack != null) IconButton(onClick = onBack) { Icon(Icons.AutoMirrored.Filled.ArrowBack, "Back") }
                },
                actions = actions,
            )
        },
        floatingActionButton = fab,
        contentWindowInsets = WindowInsets(0),
        content = content,
    )
}

/** A full-screen create/edit form: close, title and Save in the bar; fields scroll above the keyboard. */
@Composable
fun FormDialog(
    title: String,
    onDismiss: () -> Unit,
    canSave: Boolean,
    saving: Boolean,
    error: String?,
    onSave: () -> Unit,
    saveLabel: String = "Save",
    content: @Composable ColumnScope.() -> Unit,
) {
    Dialog(
        onDismissRequest = { if (!saving) onDismiss() },
        properties = DialogProperties(usePlatformDefaultWidth = false, decorFitsSystemWindows = false),
    ) {
        Scaffold(
            topBar = {
                TopAppBar(
                    title = { Text(title) },
                    navigationIcon = { IconButton(onClick = onDismiss, enabled = !saving) { Icon(Icons.Default.Close, "Cancel") } },
                    actions = {
                        if (saving) {
                            CircularProgressIndicator(Modifier.padding(horizontal = 16.dp).size(24.dp), strokeWidth = 2.dp)
                        } else {
                            TextButton(onClick = onSave, enabled = canSave) { Text(saveLabel) }
                        }
                    },
                )
            },
        ) { padding ->
            Column(
                Modifier.padding(padding).fillMaxSize().imePadding().verticalScroll(rememberScrollState()).padding(16.dp),
                verticalArrangement = Arrangement.spacedBy(12.dp),
            ) {
                content()
                if (error != null) Text(error, color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodyMedium)
                Spacer(Modifier.height(24.dp))
            }
        }
    }
}

@Composable
fun Field(
    label: String,
    value: String,
    onChange: (String) -> Unit,
    modifier: Modifier = Modifier.fillMaxWidth(),
    keyboard: KeyboardOptions = KeyboardOptions.Default,
    placeholder: String? = null,
    singleLine: Boolean = true,
    mono: Boolean = false,
    enabled: Boolean = true,
) {
    OutlinedTextField(
        value = value,
        onValueChange = onChange,
        modifier = modifier,
        label = { Text(label) },
        placeholder = placeholder?.let { { Text(it) } },
        singleLine = singleLine,
        minLines = if (singleLine) 1 else 3,
        keyboardOptions = keyboard,
        enabled = enabled,
        textStyle = if (mono) MaterialTheme.typography.bodyMedium.copy(fontFamily = FontFamily.Monospace) else MaterialTheme.typography.bodyLarge,
    )
}

/** A password-style field with a show/hide toggle. */
@Composable
fun SecretField(label: String, value: String, onChange: (String) -> Unit, keyboard: KeyboardOptions = KeyboardOptions.Default) {
    var shown by rememberSaveable { mutableStateOf(false) }
    OutlinedTextField(
        value = value,
        onValueChange = onChange,
        modifier = Modifier.fillMaxWidth(),
        label = { Text(label) },
        singleLine = true,
        keyboardOptions = keyboard,
        visualTransformation = if (shown) VisualTransformation.None else PasswordVisualTransformation(),
        trailingIcon = {
            IconButton(onClick = { shown = !shown }) {
                Icon(if (shown) Icons.Outlined.VisibilityOff else Icons.Outlined.Visibility, if (shown) "Hide" else "Show")
            }
        },
    )
}

@Composable
fun SwitchRow(label: String, checked: Boolean, onChange: (Boolean) -> Unit, supporting: String? = null) {
    Row(
        Modifier.fillMaxWidth().clickable { onChange(!checked) }.padding(vertical = 4.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Column(Modifier.weight(1f)) {
            Text(label, style = MaterialTheme.typography.bodyLarge)
            if (supporting != null) Text(supporting, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
        Switch(checked = checked, onCheckedChange = onChange)
    }
}

/** A read-only field that opens a menu of [options] (value to label). */
@Composable
fun <T> Dropdown(label: String, options: List<Pair<T, String>>, selected: T, onSelect: (T) -> Unit, enabled: Boolean = true) {
    var open by remember { mutableStateOf(false) }
    Box {
        OutlinedTextField(
            value = options.firstOrNull { it.first == selected }?.second ?: selected.toString(),
            onValueChange = {},
            readOnly = true,
            enabled = enabled,
            label = { Text(label) },
            trailingIcon = { Icon(Icons.Default.ArrowDropDown, null) },
            modifier = Modifier.fillMaxWidth(),
        )
        // Covers the field so a tap anywhere opens the menu (a read-only field eats taps).
        Box(Modifier.matchParentSize().clickable(enabled = enabled) { open = true })
        DropdownMenu(expanded = open, onDismissRequest = { open = false }) {
            options.forEach { (value, text) ->
                DropdownMenuItem(text = { Text(text) }, onClick = { onSelect(value); open = false })
            }
        }
    }
}

@Composable
fun <T> Segmented(options: List<Pair<T, String>>, selected: T, onSelect: (T) -> Unit, modifier: Modifier = Modifier.fillMaxWidth()) {
    SingleChoiceSegmentedButtonRow(modifier) {
        options.forEachIndexed { i, (value, text) ->
            SegmentedButton(
                selected = value == selected,
                onClick = { onSelect(value) },
                shape = SegmentedButtonDefaults.itemShape(i, options.size),
                icon = {},
            ) { Text(text, maxLines = 1) }
        }
    }
}

@Composable
fun SectionTitle(text: String) {
    Text(text, style = MaterialTheme.typography.titleSmall, color = MaterialTheme.colorScheme.primary, modifier = Modifier.padding(top = 8.dp))
}

/** A titled card that groups rows. */
@Composable
fun Panel(title: String? = null, content: @Composable ColumnScope.() -> Unit) {
    Card(Modifier.fillMaxWidth()) {
        Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
            if (title != null) Text(title, style = MaterialTheme.typography.titleMedium)
            content()
        }
    }
}

@Composable
fun InfoRow(label: String, value: String, trailing: @Composable (() -> Unit)? = null) {
    Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
        Text(label, style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.weight(1f))
        if (trailing != null) trailing()
        Text(value, style = MaterialTheme.typography.bodyMedium, textAlign = TextAlign.End, modifier = Modifier.weight(1.4f))
    }
}

/** An overflow (⋮) button with a menu. Each item: label to action; destructive ones render in red. */
@Composable
fun OverflowMenu(vararg items: MenuItem) {
    var open by remember { mutableStateOf(false) }
    Box {
        IconButton(onClick = { open = true }) { Icon(Icons.Default.MoreVert, "More actions") }
        DropdownMenu(expanded = open, onDismissRequest = { open = false }) {
            items.forEach { item ->
                DropdownMenuItem(
                    text = { Text(item.label, color = if (item.destructive) MaterialTheme.colorScheme.error else Color.Unspecified) },
                    leadingIcon = { Icon(item.icon, null, tint = if (item.destructive) MaterialTheme.colorScheme.error else MaterialTheme.colorScheme.onSurfaceVariant) },
                    onClick = { open = false; item.action() },
                )
            }
        }
    }
}

class MenuItem(val label: String, val icon: ImageVector, val destructive: Boolean = false, val action: () -> Unit)

@Composable
fun ConfirmDialog(title: String, text: String, confirm: String, onConfirm: () -> Unit, onDismiss: () -> Unit) {
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(title) },
        text = { Text(text) },
        confirmButton = {
            TextButton(onClick = { onDismiss(); onConfirm() }) { Text(confirm, color = MaterialTheme.colorScheme.error) }
        },
        dismissButton = { TextButton(onClick = onDismiss) { Text("Cancel") } },
    )
}

// ---------------- status ----------------

val Green = Color(0xFF16A34A)
val Red = Color(0xFFDC2626)
val Amber = Color(0xFFF59E0B)
val Slate = Color(0xFF94A3B8)

fun statusColor(status: String) = when (status) {
    "up" -> Green
    "down" -> Red
    "degraded" -> Amber
    else -> Slate
}

@Composable
fun StatusDot(status: String, size: Int = 10) {
    Box(Modifier.size(size.dp).background(statusColor(status), CircleShape))
}

@Composable
fun Pill(text: String, color: Color) {
    Text(
        text.uppercase(),
        style = MaterialTheme.typography.labelSmall,
        fontWeight = FontWeight.SemiBold,
        color = color,
        modifier = Modifier.background(color.copy(alpha = 0.14f), RoundedCornerShape(50)).padding(horizontal = 8.dp, vertical = 2.dp),
    )
}

// ---------------- chart ----------------

/** A time series as a line (optionally filled), with min/max and start/end labels. */
@Composable
fun LineChart(points: List<MetricPoint>, color: Color, filled: Boolean, label: (Double) -> String) {
    if (points.size < 2) {
        Text(
            "No data for this window.",
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
            modifier = Modifier.fillMaxWidth().padding(vertical = 40.dp),
            textAlign = TextAlign.Center,
        )
        return
    }
    val lo = points.minOf { it.v }
    val hi = points.maxOf { it.v }
    val span = (hi - lo).takeIf { it > 0 } ?: 1.0
    val t0 = points.first().t.toEpochMilli()
    val tSpan = (points.last().t.toEpochMilli() - t0).coerceAtLeast(1).toFloat()
    val grid = MaterialTheme.colorScheme.outlineVariant
    val muted = MaterialTheme.colorScheme.onSurfaceVariant
    val axis = MaterialTheme.typography.labelSmall
    val timeFormat = DateTimeFormatter.ofPattern(if (tSpan <= 86_400_000f) "HH:mm" else "d MMM").withZone(ZoneId.systemDefault())

    Column {
        Row(Modifier.fillMaxWidth().height(150.dp)) {
            Column(Modifier.width(52.dp).fillMaxHeight(), verticalArrangement = Arrangement.SpaceBetween) {
                Text(label(hi), style = axis, color = muted)
                Text(label(lo), style = axis, color = muted)
            }
            Canvas(Modifier.weight(1f).fillMaxHeight().padding(vertical = 6.dp)) {
                val dash = PathEffect.dashPathEffect(floatArrayOf(6f, 6f))
                listOf(0f, 0.5f, 1f).forEach { f ->
                    drawLine(grid, androidx.compose.ui.geometry.Offset(0f, size.height * f), androidx.compose.ui.geometry.Offset(size.width, size.height * f), 1f, pathEffect = dash)
                }
                fun x(p: MetricPoint) = (p.t.toEpochMilli() - t0) / tSpan * size.width
                fun y(p: MetricPoint) = size.height - ((p.v - lo) / span).toFloat() * size.height
                val line = Path().apply {
                    moveTo(x(points.first()), y(points.first()))
                    points.drop(1).forEach { lineTo(x(it), y(it)) }
                }
                if (filled) {
                    val area = Path().apply {
                        addPath(line)
                        lineTo(x(points.last()), size.height)
                        lineTo(x(points.first()), size.height)
                        close()
                    }
                    drawPath(area, color.copy(alpha = 0.15f))
                }
                drawPath(line, color, style = Stroke(width = 2.dp.toPx(), cap = StrokeCap.Round, join = StrokeJoin.Round))
            }
        }
        Row(Modifier.fillMaxWidth().padding(start = 52.dp)) {
            Text(timeFormat.format(points.first().t), style = axis, color = muted, modifier = Modifier.weight(1f))
            Text(timeFormat.format(points.last().t), style = axis, color = muted)
        }
    }
}

// ---------------- dates & formatting ----------------

/** "5 minutes ago". */
fun relative(instant: Instant): String =
    DateUtils.getRelativeTimeSpanString(instant.toEpochMilli(), System.currentTimeMillis(), DateUtils.MINUTE_IN_MILLIS).toString()

fun dateTime(instant: Instant): String =
    DateTimeFormatter.ofLocalizedDateTime(FormatStyle.MEDIUM, FormatStyle.SHORT).withZone(ZoneId.systemDefault()).format(instant)

/** "45 sec", "5 min", "1 hr 30 min". */
fun duration(seconds: Int): String {
    val h = seconds / 3600
    val m = seconds % 3600 / 60
    val s = seconds % 60
    return listOfNotNull(
        h.takeIf { it > 0 }?.let { "$it hr" },
        m.takeIf { it > 0 }?.let { "$it min" },
        s.takeIf { it > 0 || seconds == 0 }?.let { "$it sec" },
    ).joinToString(" ")
}

fun openUrl(context: Context, url: String) {
    context.startActivity(Intent(Intent.ACTION_VIEW, Uri.parse(url)))
}

/** A date and time picked in the phone's time zone: tap opens the date picker, then the time. */
@Composable
fun DateTimeField(label: String, value: Instant, onChange: (Instant) -> Unit) {
    var step by remember { mutableStateOf(0) } // 0 closed, 1 date, 2 time
    var date by remember { mutableStateOf(LocalDate.now()) }
    val zone = ZoneId.systemDefault()
    Box {
        OutlinedTextField(value = dateTime(value), onValueChange = {}, readOnly = true, label = { Text(label) }, modifier = Modifier.fillMaxWidth())
        Box(Modifier.matchParentSize().clickable { step = 1 })
    }
    if (step == 1) {
        val local = value.atZone(zone)
        val state = rememberDatePickerState(initialSelectedDateMillis = local.toLocalDate().atStartOfDay(ZoneOffset.UTC).toInstant().toEpochMilli())
        DatePickerDialog(
            onDismissRequest = { step = 0 },
            confirmButton = {
                TextButton(onClick = {
                    date = state.selectedDateMillis?.let { Instant.ofEpochMilli(it).atZone(ZoneOffset.UTC).toLocalDate() } ?: local.toLocalDate()
                    step = 2
                }) { Text("Next") }
            },
            dismissButton = { TextButton(onClick = { step = 0 }) { Text("Cancel") } },
        ) { DatePicker(state) }
    }
    if (step == 2) {
        val local = value.atZone(zone)
        val state = rememberTimePickerState(local.hour, local.minute)
        AlertDialog(
            onDismissRequest = { step = 0 },
            confirmButton = {
                TextButton(onClick = {
                    onChange(date.atTime(LocalTime.of(state.hour, state.minute)).atZone(zone).toInstant())
                    step = 0
                }) { Text("OK") }
            },
            dismissButton = { TextButton(onClick = { step = 0 }) { Text("Cancel") } },
            text = { TimePicker(state) },
        )
    }
}
