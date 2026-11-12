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

/** Material 3 in the brand's colour, light and dark. */
@Composable
fun AppTheme(content: @Composable () -> Unit) {
    val brand = colorResource(R.color.brand)
    val scheme = if (isSystemInDarkTheme()) {
        darkColorScheme(
            primary = lerp(brand, Color.White, 0.35f),
            onPrimary = Color(0xFF0B1B3A),
            primaryContainer = lerp(brand, Color(0xFF0F172A), 0.55f),
            onPrimaryContainer = lerp(brand, Color.White, 0.8f),
            background = Color(0xFF0F172A),
            surface = Color(0xFF0F172A),
            surfaceContainerLowest = Color(0xFF0B1222),
            surfaceContainerLow = Color(0xFF131C30),
            surfaceContainer = Color(0xFF172036),
            surfaceContainerHigh = Color(0xFF1B253C),
            surfaceContainerHighest = Color(0xFF1E293B),
        )
    } else {
        lightColorScheme(
            primary = brand,
            onPrimary = Color.White,
            primaryContainer = lerp(brand, Color.White, 0.86f),
            onPrimaryContainer = lerp(brand, Color.Black, 0.45f),
            background = Color.White,
            surface = Color.White,
            surfaceContainerLowest = Color.White,
            surfaceContainerLow = Color(0xFFF8FAFC),
            surfaceContainer = Color(0xFFF1F5F9),
            surfaceContainerHigh = Color(0xFFEDF1F6),
            surfaceContainerHighest = Color(0xFFF1F5F9),
        )
    }
    MaterialTheme(colorScheme = scheme, content = content)
}
