package com.beacon.app

import android.content.Intent
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.getValue
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.lerp
import androidx.compose.ui.res.colorResource
import androidx.lifecycle.compose.collectAsStateWithLifecycle

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        openTappedAlert(intent)
        setContent {
            AppTheme {
                val user by App.container.session.user.collectAsStateWithLifecycle()
                if (user == null) LoginScreen() else Shell()
            }
        }
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        openTappedAlert(intent)
    }

    /** A tapped alert (ours or the system's) carries the monitor id; the signed-in UI opens it. */
    private fun openTappedAlert(intent: Intent?) {
        intent?.getStringExtra(Push.EXTRA_MONITOR)?.let { App.container.pendingMonitor.value = it }
    }
}

/**
 * Material 3 dressed in the app's palette (see Theme.kt): the flavor's accent on slate
 * surfaces, light and dark. Screens read the palette through [Theme].
 */
@Composable
fun AppTheme(content: @Composable () -> Unit) {
    val dark = isSystemInDarkTheme()
    val p = palette(colorResource(R.color.brand), dark)
    val tint = lerp(p.card, p.accent, if (dark) 0.22f else 0.12f)
    val base = if (dark) darkColorScheme() else lightColorScheme()
    val scheme = base.copy(
        primary = p.accent,
        onPrimary = Color.White,
        primaryContainer = tint,
        onPrimaryContainer = if (dark) lerp(p.accent, Color.White, 0.55f) else p.accentDeep,
        secondary = p.accent,
        onSecondary = Color.White,
        secondaryContainer = tint,
        onSecondaryContainer = if (dark) lerp(p.accent, Color.White, 0.55f) else p.accentDeep,
        tertiary = p.accentDeep,
        background = p.canvas,
        onBackground = p.text,
        surface = p.card,
        onSurface = p.text,
        surfaceVariant = p.subtle,
        onSurfaceVariant = p.muted,
        surfaceTint = Color.Transparent,
        surfaceBright = p.card,
        surfaceDim = p.canvas,
        surfaceContainerLowest = p.card,
        surfaceContainerLow = p.card,
        surfaceContainer = p.card,
        surfaceContainerHigh = p.card,
        surfaceContainerHighest = p.subtle,
        outline = if (dark) Color(0xFF334155) else Color(0xFFCBD5E1),
        outlineVariant = p.border,
        error = Down,
        onError = Color.White,
    )
    CompositionLocalProvider(LocalPalette provides p) {
        MaterialTheme(colorScheme = scheme, content = content)
    }
}
