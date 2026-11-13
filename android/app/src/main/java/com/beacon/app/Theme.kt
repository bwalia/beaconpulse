package com.beacon.app

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.KeyboardArrowRight
import androidx.compose.material.icons.outlined.Dns
import androidx.compose.material.icons.outlined.Language
import androidx.compose.material.icons.outlined.Lan
import androidx.compose.material.icons.outlined.MonitorHeart
import androidx.compose.material.icons.outlined.SettingsSuggest
import androidx.compose.material.icons.outlined.VerifiedUser
import androidx.compose.material.icons.outlined.WifiTethering
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.Immutable
import androidx.compose.runtime.ReadOnlyComposable
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.drawscope.DrawScope
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.lerp
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

// The app's visual system, matching the web dashboard and ios/Sources/Shared/Theme.swift:
// slate surfaces, white cards with a hairline border and a soft shadow, the brand accent,
// bold tabular figures and small uppercase labels. Light and dark.

/** The colours of one appearance. The accent is the flavor's `R.color.brand`. */
@Immutable
class Palette(
    val accent: Color,
    val accentDeep: Color,
    val canvas: Color,
    val card: Color,
    val border: Color,
    val subtle: Color,
    val muted: Color,
    val text: Color,
    val dark: Boolean,
)

val LocalPalette = staticCompositionLocalOf<Palette> { error("No AppTheme") }

/** Slate surfaces around the brand accent: slate-50/950 canvas, white/slate-900 cards. */
fun palette(accent: Color, dark: Boolean) = Palette(
    accent = accent,
    accentDeep = deepen(accent),
    canvas = if (dark) Color(0xFF020617) else Color(0xFFF8FAFC),
    card = if (dark) Color(0xFF0F172A) else Color.White,
    border = if (dark) Color(0xFF1E293B) else Color(0xFFE2E8F0),
    subtle = if (dark) Color(0xFF1E293B) else Color(0xFFF1F5F9),
    muted = if (dark) Color(0xFF94A3B8) else Color(0xFF64748B),
    text = if (dark) Color(0xFFF1F5F9) else Color(0xFF0F172A),
    dark = dark,
)

/** The deep end of the brand gradient: the accent's RGB × 0.62, as iOS derives it. */
fun deepen(color: Color) = Color(color.red * 0.62f, color.green * 0.62f, color.blue * 0.62f, color.alpha)

object Theme {
    val accent: Color @Composable @ReadOnlyComposable get() = LocalPalette.current.accent
    val accentDeep: Color @Composable @ReadOnlyComposable get() = LocalPalette.current.accentDeep
    val canvas: Color @Composable @ReadOnlyComposable get() = LocalPalette.current.canvas
    val card: Color @Composable @ReadOnlyComposable get() = LocalPalette.current.card
    val border: Color @Composable @ReadOnlyComposable get() = LocalPalette.current.border
    val subtle: Color @Composable @ReadOnlyComposable get() = LocalPalette.current.subtle
    val muted: Color @Composable @ReadOnlyComposable get() = LocalPalette.current.muted
    val text: Color @Composable @ReadOnlyComposable get() = LocalPalette.current.text

    /** The accent gradient of the hero, the profile card and the brand tile. */
    val brandGradient: Brush @Composable @ReadOnlyComposable get() = Brush.linearGradient(listOf(accent, accentDeep))

    /** Headline figures: bold, tabular, tight. */
    val figure = TextStyle(fontWeight = FontWeight.Bold, fontSize = 26.sp, lineHeight = 32.sp, letterSpacing = (-0.4).sp, fontFeatureSettings = "tnum")
}

// Status colours, the same in light and dark (emerald, red, amber, slate).
val Up = Color(0xFF10B981)
val Down = Color(0xFFEF4444)
val Degraded = Color(0xFFF59E0B)
val Paused = Color(0xFF94A3B8)
val Blue = Color(0xFF3B82F6)
val Violet = Color(0xFF8B5CF6)

fun statusColor(status: String) = when (status) {
    "up" -> Up
    "down" -> Down
    "degraded" -> Degraded
    else -> Paused
}

fun uptimeColor(percent: Double) = when {
    percent >= 99.9 -> Up
    percent >= 99 -> Degraded
    else -> Down
}

/** A paused monitor reads as paused whatever its last check said. */
val Monitor.status get() = if (enabled) lastStatus else "paused"

/** The icon for a monitor type, shared by lists and forms. */
fun monitorTypeIcon(type: String): ImageVector = when (type.lowercase()) {
    "http", "https" -> Icons.Outlined.Language
    "ssl", "tls" -> Icons.Outlined.VerifiedUser
    "dns" -> Icons.Outlined.Dns
    "tcp" -> Icons.Outlined.Lan
    "heartbeat" -> Icons.Outlined.MonitorHeart
    "github_actions" -> Icons.Outlined.SettingsSuggest
    else -> Icons.Outlined.WifiTethering
}

// ---------------- surfaces ----------------

/** The web dashboard's card: white (slate-900 in dark), a hairline border, a soft shadow. */
@Composable
fun Modifier.card(radius: Dp = 14.dp): Modifier {
    val shape = RoundedCornerShape(radius)
    val dark = LocalPalette.current.dark
    return this
        .shadow(if (dark) 0.dp else 1.5.dp, shape, clip = false, ambientColor = Color(0x0D000000), spotColor = Color(0x14000000))
        .clip(shape)
        .background(Theme.card)
        .border(1.dp, Theme.border, shape)
}

/** A padded card holding a column of content, optionally tappable as a whole. */
@Composable
fun Card(
    modifier: Modifier = Modifier,
    padding: Dp = 16.dp,
    spacing: Dp = 10.dp,
    onClick: (() -> Unit)? = null,
    content: @Composable ColumnScope.() -> Unit,
) {
    Column(
        modifier.fillMaxWidth().card().then(if (onClick != null) Modifier.clickable(onClick = onClick) else Modifier).padding(padding),
        verticalArrangement = Arrangement.spacedBy(spacing),
        content = content,
    )
}

/** A hairline between rows inside a card. */
@Composable
fun RowDivider(modifier: Modifier = Modifier) {
    Box(modifier.fillMaxWidth().height(1.dp).background(Theme.border))
}

/** Lays [items] out in equal columns at least [minWidth] wide, like SwiftUI's adaptive grid. */
@Composable
fun <T> AdaptiveGrid(
    items: List<T>,
    minWidth: Dp,
    modifier: Modifier = Modifier,
    spacing: Dp = 12.dp,
    stretchLastRow: Boolean = false,
    cell: @Composable (T) -> Unit,
) {
    BoxWithConstraints(modifier.fillMaxWidth()) {
        val columns = ((maxWidth + spacing) / (minWidth + spacing)).toInt().coerceIn(1, items.size.coerceAtLeast(1))
        Column(verticalArrangement = Arrangement.spacedBy(spacing)) {
            items.chunked(columns).forEach { row ->
                Row(horizontalArrangement = Arrangement.spacedBy(spacing)) {
                    row.forEach { Box(Modifier.weight(1f)) { cell(it) } }
                    if (!stretchLastRow) repeat(columns - row.size) { Spacer(Modifier.weight(1f)) }
                }
            }
        }
    }
}

// ---------------- labels & badges ----------------

/** A small uppercase label, as the web uses above figures and sections. */
@Composable
fun Eyebrow(text: String, modifier: Modifier = Modifier, color: Color = Theme.muted) {
    Text(
        text.uppercase(),
        modifier,
        color = color,
        style = TextStyle(fontSize = 11.sp, fontWeight = FontWeight.SemiBold, letterSpacing = 0.8.sp),
        maxLines = 1,
    )
}

/** A headline figure in a card: label, big tabular value, optional icon and note. */
@Composable
fun StatTile(label: String, value: String, modifier: Modifier = Modifier, tint: Color = Theme.text, icon: ImageVector? = null, note: String? = null) {
    Column(modifier.fillMaxWidth().card().padding(16.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Eyebrow(label, Modifier.weight(1f))
            if (icon != null) IconTile(icon, Theme.accent, size = 26.dp, iconSize = 15.dp, radius = 7.dp, filled = false)
        }
        Text(value, style = Theme.figure, color = tint, maxLines = 1, overflow = TextOverflow.Ellipsis)
        if (note != null) Text(note, style = MaterialTheme.typography.bodySmall, color = Theme.muted)
    }
}

/** An icon on a rounded tile: tinted (soft background) or [filled] (gradient, white glyph). */
@Composable
fun IconTile(icon: ImageVector, tint: Color, size: Dp = 38.dp, iconSize: Dp = 19.dp, radius: Dp = 10.dp, filled: Boolean = false) {
    val shape = RoundedCornerShape(radius)
    val background = if (filled) {
        Modifier.background(Brush.verticalGradient(listOf(lerp(tint, Color.White, 0.14f), tint)), shape)
    } else {
        Modifier.background(tint.copy(alpha = 0.13f), shape)
    }
    Box(Modifier.size(size).then(background), contentAlignment = Alignment.Center) {
        Icon(icon, null, Modifier.size(iconSize), tint = if (filled) Color.White else tint)
    }
}

/** Text on a tinted capsule. */
@Composable
fun Pill(text: String, color: Color, uppercase: Boolean = false, icon: ImageVector? = null) {
    Row(
        Modifier.background(color.copy(alpha = 0.13f), CircleShape).padding(horizontal = if (uppercase) 8.dp else 9.dp, vertical = if (uppercase) 3.dp else 4.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(4.dp),
    ) {
        if (icon != null) Icon(icon, null, Modifier.size(13.dp), tint = color)
        Text(
            if (uppercase) text.uppercase() else text,
            color = color,
            style = TextStyle(fontSize = if (uppercase) 10.sp else 12.sp, fontWeight = FontWeight.SemiBold, letterSpacing = if (uppercase) 0.5.sp else 0.sp),
            maxLines = 1,
        )
    }
}

/** A monitor's state as a tinted pill: dot + word. */
@Composable
fun StatusBadge(status: String) {
    val color = statusColor(status)
    Row(
        Modifier.background(color.copy(alpha = 0.13f), CircleShape).padding(horizontal = 9.dp, vertical = 4.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(5.dp),
    ) {
        Box(Modifier.size(7.dp).background(color, CircleShape))
        Text(status.replaceFirstChar { it.uppercase() }, color = color, style = TextStyle(fontSize = 12.sp, fontWeight = FontWeight.SemiBold), maxLines = 1)
    }
}

/** A monitor type (HTTPS, DNS…) as a small monospaced tag. */
@Composable
fun TypeBadge(text: String) {
    Text(
        text.uppercase(),
        Modifier.background(Theme.subtle, RoundedCornerShape(5.dp)).padding(horizontal = 6.dp, vertical = 2.dp),
        color = Theme.muted,
        style = TextStyle(fontSize = 10.sp, fontWeight = FontWeight.SemiBold, fontFamily = FontFamily.Monospace),
        maxLines = 1,
    )
}

/** The quiet chevron at the end of a tappable row. */
@Composable
fun Chevron() {
    Icon(Icons.AutoMirrored.Filled.KeyboardArrowRight, null, Modifier.size(18.dp), tint = Theme.muted.copy(alpha = 0.7f))
}

// ---------------- brand ----------------

/** The ticks of the brand mark in its 24-unit box, as on the web (a ring with eight ticks). */
private val markTicks = listOf(
    floatArrayOf(12f, 4.5f, 12f, 6.5f), floatArrayOf(12f, 17.5f, 12f, 19.5f),
    floatArrayOf(4.5f, 12f, 6.5f, 12f), floatArrayOf(17.5f, 12f, 19.5f, 12f),
    floatArrayOf(6.7f, 6.7f, 8.1f, 8.1f), floatArrayOf(15.9f, 15.9f, 17.3f, 17.3f),
    floatArrayOf(17.3f, 6.7f, 15.9f, 8.1f), floatArrayOf(8.1f, 15.9f, 6.7f, 17.3f),
)

/** Draws the brand mark centred in this scope, stroked [strokeUnits] wide (the web uses 1.75). */
fun DrawScope.drawBrandMark(color: Color, strokeUnits: Float = 1.75f) {
    val s = size.minDimension / 24f
    val origin = Offset(center.x - 12f * s, center.y - 12f * s)
    val width = strokeUnits * s
    drawCircle(color, radius = 2.5f * s, center = center, style = Stroke(width))
    markTicks.forEach { (x1, y1, x2, y2) ->
        drawLine(color, origin + Offset(x1 * s, y1 * s), origin + Offset(x2 * s, y2 * s), width, StrokeCap.Round)
    }
}

/** The brand mark on its own. */
@Composable
fun BrandMark(modifier: Modifier, color: Color = Color.White, strokeUnits: Float = 1.75f) {
    Canvas(modifier) { drawBrandMark(color, strokeUnits) }
}

/** The brand mark in its rounded accent tile, as in the web header. */
@Composable
fun BrandTile(size: Dp = 36.dp) {
    Box(Modifier.size(size).background(Theme.brandGradient, RoundedCornerShape(size * 0.26f)), contentAlignment = Alignment.Center) {
        BrandMark(Modifier.size(size * 0.76f), strokeUnits = 1.6f)
    }
}

/** Faint concentric rings, echoing the mark, over a brand gradient. */
fun DrawScope.drawRings(diameters: List<Dp>, centre: Offset, alpha: Float = 0.08f) {
    diameters.forEach { drawCircle(Color.White.copy(alpha = alpha), radius = it.toPx() / 2, center = centre, style = Stroke(1.dp.toPx())) }
}

/** Spacer for the end of a scrolling list, clear of a floating button. */
@Composable
fun FabClearance() = Spacer(Modifier.height(88.dp).width(1.dp))
