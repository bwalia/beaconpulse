@file:OptIn(ExperimentalMaterial3Api::class)

package com.beacon.app

import android.content.Context
import android.content.Intent
import android.net.Uri
import android.text.format.DateUtils
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.collectIsFocusedAsState
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.layout.wrapContentHeight
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.selection.selectable
import androidx.compose.foundation.selection.toggleable
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.Check
import androidx.compose.material.icons.filled.Close
import androidx.compose.material.icons.filled.MoreVert
import androidx.compose.material.icons.filled.Warning
import androidx.compose.material.icons.outlined.CalendarMonth
import androidx.compose.material.icons.outlined.ErrorOutline
import androidx.compose.material.icons.outlined.Remove
import androidx.compose.material.icons.outlined.UnfoldMore
import androidx.compose.material.icons.outlined.Visibility
import androidx.compose.material.icons.outlined.VisibilityOff
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.DatePicker
import androidx.compose.material3.DatePickerDialog
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FloatingActionButton
import androidx.compose.material3.FloatingActionButtonDefaults
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Switch
import androidx.compose.material3.SwitchDefaults
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TimePicker
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.material3.pulltorefresh.PullToRefreshBox
import androidx.compose.material3.rememberDatePickerState
import androidx.compose.material3.rememberTimePickerState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.SideEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableFloatStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.PathEffect
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.StrokeJoin
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.drawText
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.text.rememberTextMeasurer
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.platform.LocalView
import androidx.compose.ui.window.Dialog
import androidx.compose.ui.window.DialogWindowProvider
import androidx.compose.ui.window.DialogProperties
import androidx.core.view.WindowCompat
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
import kotlin.math.abs
import kotlin.math.ceil
import kotlin.math.floor
import kotlin.math.ln
import kotlin.math.log10
import kotlin.math.pow

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
        Loadable.Loading -> Box(Modifier.fillMaxSize(), Alignment.Center) {
            CircularProgressIndicator(Modifier.size(32.dp), color = Theme.accent, strokeWidth = 3.dp, trackColor = Theme.border)
        }
        is Loadable.Failed -> Message(Icons.Outlined.ErrorOutline, "Couldn’t load", state.message, "Try again", tint = Down, onAction = loader::reload)
        is Loadable.Loaded -> PullToRefreshBox(loader.refreshing, loader::reload, Modifier.fillMaxSize()) { content(state.value) }
    }
}

/** A centred empty / error state: the icon on a tinted disc, a title and a line. Scrollable so pull-to-refresh still works. */
@Composable
fun Message(icon: ImageVector, title: String, text: String, action: String? = null, tint: Color = Theme.accent, onAction: () -> Unit = {}) {
    Column(
        Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(32.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp, Alignment.CenterVertically),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Spacer(Modifier.height(48.dp))
        Box(Modifier.size(88.dp).background(tint.copy(alpha = 0.12f), CircleShape), contentAlignment = Alignment.Center) {
            Icon(icon, null, Modifier.size(42.dp), tint = tint)
        }
        Spacer(Modifier.height(2.dp))
        Text(title, style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.Bold, textAlign = TextAlign.Center)
        Text(
            text,
            Modifier.widthIn(max = 320.dp),
            style = MaterialTheme.typography.bodyMedium,
            color = Theme.muted,
            textAlign = TextAlign.Center,
        )
        if (action != null) {
            Spacer(Modifier.height(4.dp))
            AccentButton(action, onAction)
        }
    }
}

// ---------------- screens ----------------

/** A screen inside the shell: a canvas-coloured top bar (with back when nested) over [content]. */
@Composable
fun Screen(
    title: String,
    onBack: (() -> Unit)? = null,
    actions: @Composable RowScope.() -> Unit = {},
    fab: @Composable () -> Unit = {},
    content: @Composable (PaddingValues) -> Unit,
) {
    Scaffold(
        containerColor = Theme.canvas,
        topBar = {
            TopAppBar(
                title = {
                    // A tab's root gets the large bold title; a pushed screen a quieter one.
                    Text(
                        title,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                        style = if (onBack == null) {
                            TextStyle(fontSize = 26.sp, fontWeight = FontWeight.Bold, letterSpacing = (-0.4).sp)
                        } else {
                            TextStyle(fontSize = 18.sp, fontWeight = FontWeight.SemiBold)
                        },
                    )
                },
                navigationIcon = {
                    if (onBack != null) IconButton(onClick = onBack) { Icon(Icons.AutoMirrored.Filled.ArrowBack, "Back") }
                },
                actions = actions,
                colors = TopAppBarDefaults.topAppBarColors(containerColor = Theme.canvas, scrolledContainerColor = Theme.canvas),
            )
        },
        floatingActionButton = fab,
        contentWindowInsets = WindowInsets(0),
        content = content,
    )
}

/** The add button over a list. */
@Composable
fun AddFab(label: String, onClick: () -> Unit) {
    FloatingActionButton(
        onClick = onClick,
        containerColor = Theme.accent,
        contentColor = Color.White,
        shape = RoundedCornerShape(16.dp),
        elevation = FloatingActionButtonDefaults.elevation(defaultElevation = 3.dp, pressedElevation = 5.dp),
    ) { Icon(Icons.Default.Add, label) }
}

/** The filled accent button: Sign in, Try again. */
@Composable
fun AccentButton(text: String, onClick: () -> Unit, modifier: Modifier = Modifier, enabled: Boolean = true, busy: Boolean = false, color: Color = Theme.accent) {
    Button(
        onClick = onClick,
        enabled = enabled && !busy,
        modifier = modifier,
        shape = RoundedCornerShape(12.dp),
        colors = ButtonDefaults.buttonColors(
            containerColor = color,
            contentColor = Color.White,
            disabledContainerColor = color.copy(alpha = 0.4f),
            disabledContentColor = Color.White.copy(alpha = 0.9f),
        ),
        contentPadding = PaddingValues(horizontal = 20.dp, vertical = 12.dp),
    ) {
        if (busy) CircularProgressIndicator(Modifier.size(20.dp), color = Color.White, strokeWidth = 2.dp)
        else Text(text, fontWeight = FontWeight.SemiBold, fontSize = 16.sp)
    }
}

// ---------------- forms ----------------

/**
 * A full-screen create/edit form: close, title and Save in the bar; titled cards of
 * fields scroll above the keyboard. Without [onSave] the form carries its own action.
 */
@Composable
fun FormDialog(
    title: String,
    onDismiss: () -> Unit,
    canSave: Boolean,
    saving: Boolean,
    error: String?,
    onSave: (() -> Unit)?,
    saveLabel: String = "Save",
    content: @Composable ColumnScope.() -> Unit,
) {
    Dialog(
        onDismissRequest = { if (!saving) onDismiss() },
        properties = DialogProperties(usePlatformDefaultWidth = false, decorFitsSystemWindows = false),
    ) {
        // The dialog has its own window: give its system bars dark icons on the light canvas.
        val view = LocalView.current
        val dark = LocalPalette.current.dark
        SideEffect {
            (view.parent as? DialogWindowProvider)?.window?.let { window ->
                WindowCompat.getInsetsController(window, view).apply {
                    isAppearanceLightStatusBars = !dark
                    isAppearanceLightNavigationBars = !dark
                }
            }
        }
        Scaffold(
            containerColor = Theme.canvas,
            topBar = {
                TopAppBar(
                    title = { Text(title, style = TextStyle(fontSize = 18.sp, fontWeight = FontWeight.SemiBold)) },
                    navigationIcon = { IconButton(onClick = onDismiss, enabled = !saving) { Icon(Icons.Default.Close, "Cancel") } },
                    actions = {
                        if (onSave != null) {
                            if (saving) {
                                CircularProgressIndicator(Modifier.padding(horizontal = 24.dp).size(22.dp), color = Theme.accent, strokeWidth = 2.dp)
                            } else {
                                Button(
                                    onClick = onSave,
                                    enabled = canSave,
                                    modifier = Modifier.padding(end = 12.dp).height(38.dp),
                                    shape = RoundedCornerShape(10.dp),
                                    colors = ButtonDefaults.buttonColors(
                                        containerColor = Theme.accent,
                                        contentColor = Color.White,
                                        disabledContainerColor = Theme.subtle,
                                        disabledContentColor = Theme.muted,
                                    ),
                                    contentPadding = PaddingValues(horizontal = 18.dp),
                                ) { Text(saveLabel, fontWeight = FontWeight.SemiBold) }
                            }
                        }
                    },
                    colors = TopAppBarDefaults.topAppBarColors(containerColor = Theme.canvas, scrolledContainerColor = Theme.canvas),
                )
            },
        ) { padding ->
            Column(
                Modifier.padding(padding).fillMaxSize().imePadding().verticalScroll(rememberScrollState())
                    .padding(horizontal = 16.dp, vertical = 8.dp),
                verticalArrangement = Arrangement.spacedBy(22.dp),
            ) {
                content()
                if (error != null) ErrorBanner(error)
                Spacer(Modifier.height(24.dp))
            }
        }
    }
}

/** A titled card of fields, with an optional note under it. */
@Composable
fun FormCard(title: String? = null, footer: String? = null, content: @Composable ColumnScope.() -> Unit) {
    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        if (title != null) Eyebrow(title, Modifier.padding(start = 4.dp))
        Column(Modifier.fillMaxWidth().card().padding(18.dp), verticalArrangement = Arrangement.spacedBy(18.dp), content = content)
        if (footer != null) Text(footer, Modifier.padding(horizontal = 4.dp), style = MaterialTheme.typography.bodySmall, color = Theme.muted)
    }
}

/** A label above its control, with an optional hint below. */
@Composable
fun Field(label: String, hint: String? = null, modifier: Modifier = Modifier, content: @Composable () -> Unit) {
    Column(modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(7.dp)) {
        Text(label, style = TextStyle(fontSize = 14.sp, fontWeight = FontWeight.SemiBold), color = Theme.text)
        content()
        if (hint != null) Text(hint, style = MaterialTheme.typography.bodySmall, color = Theme.muted)
    }
}

private val inputShape = RoundedCornerShape(10.dp)

/** The input chrome: soft fill, hairline border, and an accent border with a soft ring while [focused]. */
@Composable
private fun Modifier.inputChrome(focused: Boolean): Modifier {
    val accent = Theme.accent
    return this
        .drawBehind {
            if (focused) {
                val ring = 3.dp.toPx()
                drawRoundRect(
                    accent.copy(alpha = 0.18f),
                    topLeft = Offset(-ring, -ring),
                    size = Size(size.width + 2 * ring, size.height + 2 * ring),
                    cornerRadius = CornerRadius(10.dp.toPx() + ring),
                )
            }
        }
        .background(Theme.canvas, inputShape)
        .border(if (focused) 1.5.dp else 1.dp, if (focused) accent else Theme.border, inputShape)
}

/** The web's text input. [secret] adds a show/hide eye; [mono] is for URLs, headers and bodies. */
@Composable
fun Input(
    value: String,
    onChange: (String) -> Unit,
    modifier: Modifier = Modifier,
    placeholder: String = "",
    keyboard: KeyboardOptions = KeyboardOptions.Default,
    actions: KeyboardActions = KeyboardActions.Default,
    singleLine: Boolean = true,
    minLines: Int = 1,
    mono: Boolean = false,
    secret: Boolean = false,
    enabled: Boolean = true,
    leading: ImageVector? = null,
) {
    val interaction = remember { MutableInteractionSource() }
    val focused by interaction.collectIsFocusedAsState()
    var shown by rememberSaveable { mutableStateOf(false) }
    val style = TextStyle(
        fontSize = if (mono) 15.sp else 16.sp,
        fontFamily = if (mono) FontFamily.Monospace else FontFamily.Default,
        color = Theme.text,
    )
    BasicTextField(
        value = value,
        onValueChange = onChange,
        modifier = modifier.fillMaxWidth().alpha(if (enabled) 1f else 0.6f).inputChrome(focused),
        enabled = enabled,
        singleLine = singleLine,
        minLines = if (singleLine) 1 else minLines,
        maxLines = if (singleLine) 1 else 10,
        textStyle = style,
        keyboardOptions = keyboard,
        keyboardActions = actions,
        visualTransformation = if (secret && !shown) PasswordVisualTransformation() else VisualTransformation.None,
        interactionSource = interaction,
        cursorBrush = SolidColor(Theme.accent),
        decorationBox = { inner ->
            Row(
                Modifier.padding(start = 12.dp, end = 12.dp, top = 12.dp, bottom = 12.dp),
                verticalAlignment = if (singleLine) Alignment.CenterVertically else Alignment.Top,
            ) {
                if (leading != null) {
                    Icon(leading, null, Modifier.size(19.dp), tint = Theme.muted)
                    Spacer(Modifier.width(8.dp))
                }
                Box(Modifier.weight(1f)) {
                    if (value.isEmpty() && placeholder.isNotEmpty()) {
                        Text(placeholder, style = style.copy(color = Theme.muted.copy(alpha = 0.75f)), maxLines = if (singleLine) 1 else 3)
                    }
                    inner()
                }
                if (secret) {
                    Spacer(Modifier.width(8.dp))
                    Icon(
                        if (shown) Icons.Outlined.VisibilityOff else Icons.Outlined.Visibility,
                        if (shown) "Hide" else "Show",
                        Modifier.size(20.dp).clip(CircleShape).clickable { shown = !shown },
                        tint = Theme.muted,
                    )
                }
            }
        },
    )
}

/** A value that can't be changed here, in the input's shape. */
@Composable
fun ReadOnlyValue(text: String) {
    Text(
        text,
        Modifier.fillMaxWidth().background(Theme.subtle, inputShape).padding(horizontal = 12.dp, vertical = 12.dp),
        color = Theme.muted,
        style = TextStyle(fontSize = 16.sp),
    )
}

/** A drop-down for long or dynamic lists (projects), in the input's look. */
@Composable
fun <T> MenuInput(options: List<Pair<T, String>>, selected: T, onSelect: (T) -> Unit, enabled: Boolean = true, placeholder: String = "Select…") {
    var open by remember { mutableStateOf(false) }
    val current = options.firstOrNull { it.first == selected }?.second
    Box {
        Row(
            Modifier.fillMaxWidth().inputChrome(open).clip(inputShape).clickable(enabled = enabled) { open = true }
                .padding(horizontal = 12.dp, vertical = 12.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text(
                current ?: placeholder,
                Modifier.weight(1f),
                style = TextStyle(fontSize = 16.sp),
                color = if (current != null) Theme.text else Theme.muted,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            Icon(Icons.Outlined.UnfoldMore, null, Modifier.size(20.dp), tint = Theme.muted)
        }
        DropdownMenu(
            expanded = open,
            onDismissRequest = { open = false },
            shape = RoundedCornerShape(12.dp),
            containerColor = Theme.card,
        ) {
            options.forEach { (value, text) ->
                DropdownMenuItem(
                    text = { Text(text, fontWeight = if (value == selected) FontWeight.SemiBold else FontWeight.Normal) },
                    trailingIcon = if (value == selected) ({ Icon(Icons.Default.Check, null, tint = Theme.accent) }) else null,
                    onClick = { onSelect(value); open = false },
                )
            }
        }
    }
}

/** One option for [ChoiceChips]. */
class Choice<T>(val value: T, val label: String, val icon: ImageVector? = null)

/** A short list of options as tappable chips; the selected one is outlined in the accent. */
@Composable
fun <T> ChoiceChips(choices: List<Choice<T>>, selected: T, onSelect: (T) -> Unit, minWidth: Dp = 96.dp) {
    AdaptiveGrid(choices, minWidth, spacing = 8.dp) { choice ->
        val on = choice.value == selected
        val color = if (on) Theme.accent else Theme.text
        val shape = RoundedCornerShape(10.dp)
        Row(
            Modifier.fillMaxWidth().height(44.dp).clip(shape)
                .background(if (on) Theme.accent.copy(alpha = 0.1f) else Theme.canvas)
                .border(if (on) 1.5.dp else 1.dp, if (on) Theme.accent else Theme.border, shape)
                .selectable(on, role = Role.RadioButton) { onSelect(choice.value) }
                .padding(horizontal = 8.dp),
            horizontalArrangement = Arrangement.Center,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            if (choice.icon != null) {
                Icon(choice.icon, null, Modifier.size(16.dp), tint = color)
                Spacer(Modifier.width(6.dp))
            }
            FitText(choice.label, TextStyle(fontSize = 14.sp, fontWeight = if (on) FontWeight.SemiBold else FontWeight.Medium, color = color))
        }
    }
}

/** One line of text that shrinks to 80% before it ellipsizes, like SwiftUI's minimumScaleFactor(0.8). */
@Composable
private fun FitText(text: String, style: TextStyle) {
    var scale by remember(text) { mutableFloatStateOf(1f) }
    Text(
        text,
        style = style.copy(fontSize = style.fontSize * scale),
        maxLines = 1,
        softWrap = false,
        overflow = if (scale <= 0.8f) TextOverflow.Ellipsis else TextOverflow.Clip,
        onTextLayout = { if (it.didOverflowWidth && scale > 0.8f) scale = (scale - 0.05f).coerceAtLeast(0.8f) },
    )
}

/** An on/off setting: title and explanation, with a switch on the right. */
@Composable
fun SwitchRow(label: String, checked: Boolean, onChange: (Boolean) -> Unit, supporting: String? = null) {
    Row(
        Modifier.fillMaxWidth().toggleable(checked, role = Role.Switch, onValueChange = onChange),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
            Text(label, style = TextStyle(fontSize = 16.sp, fontWeight = FontWeight.Medium), color = Theme.text)
            if (supporting != null) Text(supporting, style = MaterialTheme.typography.bodySmall, color = Theme.muted)
        }
        Spacer(Modifier.width(12.dp))
        AccentSwitch(checked, null)
    }
}

/** The switch in the accent. */
@Composable
fun AccentSwitch(checked: Boolean, onChange: ((Boolean) -> Unit)?) {
    Switch(
        checked = checked,
        onCheckedChange = onChange,
        colors = SwitchDefaults.colors(
            checkedTrackColor = Theme.accent,
            checkedThumbColor = Color.White,
            checkedBorderColor = Theme.accent,
            uncheckedTrackColor = Theme.subtle,
            uncheckedThumbColor = Theme.muted,
            uncheckedBorderColor = MaterialTheme.colorScheme.outline,
        ),
    )
}

/** A value picked from [options] with − / +, shown large; tapping the value lists them all. */
@Composable
fun <T> StepperRow(title: String, options: List<Pair<T, String>>, selected: T, onSelect: (T) -> Unit) {
    val index = options.indexOfFirst { it.first == selected }.coerceAtLeast(0)
    var open by remember { mutableStateOf(false) }
    Row(verticalAlignment = Alignment.CenterVertically) {
        Box(Modifier.weight(1f)) {
            Column(Modifier.clip(RoundedCornerShape(8.dp)).clickable { open = true }, verticalArrangement = Arrangement.spacedBy(2.dp)) {
                Text(title, style = TextStyle(fontSize = 14.sp, fontWeight = FontWeight.SemiBold), color = Theme.text)
                Text(
                    options.getOrNull(index)?.second.orEmpty(),
                    style = TextStyle(fontSize = 21.sp, fontWeight = FontWeight.Bold, fontFeatureSettings = "tnum", letterSpacing = (-0.2).sp),
                    color = Theme.text,
                )
            }
            DropdownMenu(expanded = open, onDismissRequest = { open = false }, shape = RoundedCornerShape(12.dp), containerColor = Theme.card) {
                options.forEach { (value, text) ->
                    DropdownMenuItem(
                        text = { Text(text, fontWeight = if (value == selected) FontWeight.SemiBold else FontWeight.Normal) },
                        trailingIcon = if (value == selected) ({ Icon(Icons.Default.Check, null, tint = Theme.accent) }) else null,
                        onClick = { onSelect(value); open = false },
                    )
                }
            }
        }
        Row(Modifier.clip(RoundedCornerShape(10.dp)).background(Theme.subtle), verticalAlignment = Alignment.CenterVertically) {
            StepButton(Icons.Outlined.Remove, "Decrease $title", index > 0) { onSelect(options[index - 1].first) }
            Box(Modifier.width(1.dp).height(20.dp).background(MaterialTheme.colorScheme.outline))
            StepButton(Icons.Default.Add, "Increase $title", index < options.lastIndex) { onSelect(options[index + 1].first) }
        }
    }
}

@Composable
private fun StepButton(icon: ImageVector, label: String, enabled: Boolean, onClick: () -> Unit) {
    Box(Modifier.size(width = 48.dp, height = 38.dp).clickable(enabled = enabled, onClick = onClick), contentAlignment = Alignment.Center) {
        Icon(icon, label, Modifier.size(20.dp), tint = if (enabled) Theme.text else Theme.muted.copy(alpha = 0.4f))
    }
}

/** A save/delete failure, shown above the form's end. */
@Composable
fun ErrorBanner(message: String) {
    Row(
        Modifier.fillMaxWidth().clip(inputShape).background(Down.copy(alpha = 0.08f)).border(1.dp, Down.copy(alpha = 0.3f), inputShape).padding(12.dp),
        horizontalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        Icon(Icons.Default.Warning, null, Modifier.size(18.dp), tint = Down)
        Text(message, style = MaterialTheme.typography.bodyMedium, color = Down)
    }
}

/** A row in a details card: muted label, value on the right. */
@Composable
fun InfoRow(label: String, value: String, modifier: Modifier = Modifier, trailing: @Composable (() -> Unit)? = null) {
    Row(modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
        Text(label, style = MaterialTheme.typography.bodyMedium, color = Theme.muted, modifier = Modifier.weight(1f))
        Row(Modifier.weight(1.4f), horizontalArrangement = Arrangement.End, verticalAlignment = Alignment.CenterVertically) {
            if (trailing != null) trailing()
            Text(value, style = MaterialTheme.typography.bodyMedium, fontWeight = FontWeight.Medium, textAlign = TextAlign.End)
        }
    }
}

/** An iOS-style segmented control: a soft track with the chosen segment raised. */
@Composable
fun <T> Segmented(options: List<Pair<T, String>>, selected: T, onSelect: (T) -> Unit, modifier: Modifier = Modifier.fillMaxWidth()) {
    val raised = if (LocalPalette.current.dark) Color(0xFF334155) else Theme.card
    Row(modifier.clip(RoundedCornerShape(10.dp)).background(Theme.subtle).padding(3.dp)) {
        options.forEach { (value, text) ->
            val on = value == selected
            val shape = RoundedCornerShape(8.dp)
            Box(
                Modifier.weight(1f).height(32.dp)
                    .then(if (on) Modifier.shadow(1.dp, shape).background(raised, shape) else Modifier)
                    .clip(shape)
                    .selectable(on, role = Role.Tab) { onSelect(value) },
                contentAlignment = Alignment.Center,
            ) {
                Text(
                    text,
                    color = if (on) Theme.text else Theme.muted,
                    style = TextStyle(fontSize = 13.sp, fontWeight = if (on) FontWeight.SemiBold else FontWeight.Medium),
                    maxLines = 1,
                )
            }
        }
    }
}

/** An overflow (⋮) button with a menu. Each item: label to action; destructive ones render in red. */
@Composable
fun OverflowMenu(vararg items: MenuItem) {
    var open by remember { mutableStateOf(false) }
    Box {
        IconButton(onClick = { open = true }) { Icon(Icons.Default.MoreVert, "More actions", tint = Theme.muted) }
        DropdownMenu(expanded = open, onDismissRequest = { open = false }, shape = RoundedCornerShape(12.dp), containerColor = Theme.card) {
            items.forEach { item ->
                DropdownMenuItem(
                    text = { Text(item.label, color = if (item.destructive) Down else Color.Unspecified) },
                    leadingIcon = { Icon(item.icon, null, tint = if (item.destructive) Down else Theme.muted) },
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
        containerColor = Theme.card,
        shape = RoundedCornerShape(20.dp),
        title = { Text(title, fontWeight = FontWeight.SemiBold) },
        text = { Text(text, color = Theme.muted) },
        confirmButton = {
            TextButton(onClick = { onDismiss(); onConfirm() }) { Text(confirm, color = Down, fontWeight = FontWeight.SemiBold) }
        },
        dismissButton = { TextButton(onClick = onDismiss) { Text("Cancel", color = Theme.text) } },
    )
}

// ---------------- chart ----------------

/** A trend line with a soft gradient fill and quiet axes, like the iOS TrendChart. */
@Composable
fun TrendChart(points: List<MetricPoint>, color: Color, height: Dp = 170.dp, label: (Double) -> String) {
    if (points.size < 2) {
        Text(
            "No data for this window.",
            Modifier.fillMaxWidth().height(height).wrapContentHeight(),
            style = MaterialTheme.typography.bodySmall,
            color = Theme.muted,
            textAlign = TextAlign.Center,
        )
        return
    }
    // From zero, like Swift Charts on iOS: a flat 100% sits at the top.
    val ticks = remember(points) { niceTicks(minOf(0.0, points.minOf { it.v }), points.maxOf { it.v }) }
    val lo = ticks.first()
    val hi = ticks.last()
    val t0 = points.first().t.toEpochMilli()
    val tSpan = (points.last().t.toEpochMilli() - t0).coerceAtLeast(1)
    val grid = Theme.border
    val measurer = rememberTextMeasurer()
    val axis = TextStyle(fontSize = 10.sp, color = Theme.muted, fontFeatureSettings = "tnum")
    // A 24h window starts and ends at the same clock time, so it needs the day too.
    val timeFormat = DateTimeFormatter.ofPattern(
        when {
            tSpan <= 21_600_000 -> "HH:mm"
            tSpan <= 172_800_000 -> "EEE HH:mm"
            else -> "d MMM"
        },
    ).withZone(ZoneId.systemDefault())

    Canvas(Modifier.fillMaxWidth().height(height)) {
        val yLabels = ticks.map { measurer.measure(label(it), axis) }
        val gutter = yLabels.maxOf { it.size.width } + 8.dp.toPx()
        val labelHeight = yLabels.first().size.height.toFloat()
        val top = labelHeight / 2
        val bottom = size.height - labelHeight - 6.dp.toPx()
        val plotWidth = size.width - gutter
        fun y(v: Double) = bottom - ((v - lo) / (hi - lo)).toFloat() * (bottom - top)
        fun x(p: MetricPoint) = gutter + (p.t.toEpochMilli() - t0).toFloat() / tSpan * plotWidth

        val dash = PathEffect.dashPathEffect(floatArrayOf(3.dp.toPx(), 3.dp.toPx()))
        ticks.forEachIndexed { i, v ->
            val yy = y(v)
            drawLine(grid, Offset(gutter, yy), Offset(size.width, yy), 1f, pathEffect = dash)
            drawText(yLabels[i], topLeft = Offset(gutter - 8.dp.toPx() - yLabels[i].size.width, yy - labelHeight / 2))
        }

        val xs = FloatArray(points.size) { x(points[it]) }
        val ys = FloatArray(points.size) { y(points[it].v) }
        val line = monotonePath(xs, ys)
        val area = Path().apply {
            addPath(line)
            lineTo(xs.last(), bottom)
            lineTo(xs.first(), bottom)
            close()
        }
        drawPath(area, Brush.verticalGradient(listOf(color.copy(alpha = 0.28f), color.copy(alpha = 0.02f)), startY = top, endY = bottom))
        drawPath(line, color, style = Stroke(width = 2.dp.toPx(), cap = StrokeCap.Round, join = StrokeJoin.Round))

        for (i in 0..3) {
            val text = measurer.measure(timeFormat.format(Instant.ofEpochMilli(t0 + tSpan * i / 3)), axis)
            val left = (gutter + plotWidth * i / 3 - text.size.width / 2f).coerceIn(gutter, size.width - text.size.width)
            drawText(text, topLeft = Offset(left, size.height - labelHeight))
        }
    }
}

/** Round axis values from [lo] to at least [hi]: 3–6 gridlines at 1, 2, 2.5 or 5 × 10ⁿ apart. */
internal fun niceTicks(lo: Double, hi: Double): List<Double> {
    val span = (hi - lo).takeIf { it > 0 } ?: abs(hi).takeIf { it > 0 } ?: 1.0
    val raw = span / 4
    val magnitude = 10.0.pow(floor(log10(raw)))
    var step = listOf(1.0, 2.0, 2.5, 5.0, 10.0).map { it * magnitude }.minBy { abs(ln(it / raw)) }
    if (ceil(span / step) > 5) step *= 2
    val start = floor(lo / step) * step
    val end = maxOf(ceil(hi / step) * step, start + step)
    return generateSequence(start) { it + step }.takeWhile { it <= end + step / 2 }.toList()
}

/** A smooth line through the points that never overshoots them (monotone cubic, as Swift Charts' .monotone). */
private fun monotonePath(xs: FloatArray, ys: FloatArray): Path {
    val n = xs.size
    val path = Path().apply { moveTo(xs[0], ys[0]) }
    if (n == 2) return path.apply { lineTo(xs[1], ys[1]) }
    val dx = FloatArray(n - 1) { xs[it + 1] - xs[it] }
    val slope = FloatArray(n - 1) { if (dx[it] == 0f) 0f else (ys[it + 1] - ys[it]) / dx[it] }
    val m = FloatArray(n)
    m[0] = slope[0]
    m[n - 1] = slope[n - 2]
    for (i in 1 until n - 1) {
        m[i] = if (slope[i - 1] * slope[i] <= 0f) {
            0f
        } else {
            val w1 = 2 * dx[i] + dx[i - 1]
            val w2 = dx[i] + 2 * dx[i - 1]
            (w1 + w2) / (w1 / slope[i - 1] + w2 / slope[i])
        }
    }
    for (i in 0 until n - 1) {
        val h = dx[i] / 3
        path.cubicTo(xs[i] + h, ys[i] + m[i] * h, xs[i + 1] - h, ys[i + 1] - m[i + 1] * h, xs[i + 1], ys[i + 1])
    }
    return path
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

/** A date and time picked in the phone's time zone, in the input's look: tap opens the date picker, then the time. */
@Composable
fun DateTimeField(value: Instant, onChange: (Instant) -> Unit) {
    var step by remember { mutableStateOf(0) } // 0 closed, 1 date, 2 time
    var date by remember { mutableStateOf(LocalDate.now()) }
    val zone = ZoneId.systemDefault()
    Row(
        Modifier.fillMaxWidth().inputChrome(step != 0).clip(inputShape).clickable { step = 1 }.padding(horizontal = 12.dp, vertical = 12.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(dateTime(value), Modifier.weight(1f), style = TextStyle(fontSize = 16.sp, fontFeatureSettings = "tnum"), color = Theme.text)
        Icon(Icons.Outlined.CalendarMonth, null, Modifier.size(20.dp), tint = Theme.muted)
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
            containerColor = Theme.card,
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
