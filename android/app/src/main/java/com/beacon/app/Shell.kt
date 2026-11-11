package com.beacon.app

import android.Manifest
import android.content.pm.PackageManager
import android.net.Uri
import android.os.Build
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.padding
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Folder
import androidx.compose.material.icons.outlined.Insights
import androidx.compose.material.icons.outlined.MonitorHeart
import androidx.compose.material.icons.outlined.NotificationsActive
import androidx.compose.material.icons.outlined.Settings
import androidx.compose.material3.Icon
import androidx.compose.material3.NavigationBar
import androidx.compose.material3.NavigationBarItem
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalContext
import androidx.core.content.ContextCompat
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.navigation.NavGraph.Companion.findStartDestination
import androidx.navigation.NavHostController
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.currentBackStackEntryAsState
import androidx.navigation.compose.rememberNavController
import androidx.navigation.navArgument

enum class Tab(val route: String, val label: String, val icon: ImageVector) {
    Overview("overview", "Overview", Icons.Outlined.Insights),
    Monitors("monitors", "Monitors", Icons.Outlined.MonitorHeart),
    Alerts("alerts", "Alerts", Icons.Outlined.NotificationsActive),
    Projects("projects", "Projects", Icons.Outlined.Folder),
    Settings("settings", "Settings", Icons.Outlined.Settings),
}

/** The signed-in app: five tabs, each keeping its own back stack. Mirrors iOS MainTabView. */
@Composable
fun Shell() {
    val nav = rememberNavController()
    val snack = remember { SnackbarHostState() }
    var tab by rememberSaveable { mutableStateOf(Tab.Overview) }
    val entry by nav.currentBackStackEntryAsState()
    LaunchedEffect(entry) { Tab.entries.firstOrNull { it.route == entry?.destination?.route }?.let { tab = it } }

    fun go(to: Tab) {
        tab = to
        nav.navigate(to.route) {
            popUpTo(nav.graph.findStartDestination().id) { saveState = true }
            launchSingleTop = true
            restoreState = true
        }
    }

    // A tapped alert opens its monitor on the Monitors tab.
    val pending by App.container.pendingMonitor.collectAsStateWithLifecycle()
    LaunchedEffect(pending) {
        val id = pending ?: return@LaunchedEffect
        go(Tab.Monitors)
        nav.navigate("monitor/$id")
        App.container.pendingMonitor.value = null
    }

    AskForNotifications()

    CompositionLocalProvider(LocalSnack provides snack) {
        Scaffold(
            bottomBar = {
                NavigationBar {
                    Tab.entries.forEach { t ->
                        NavigationBarItem(
                            selected = t == tab,
                            onClick = { if (t == tab) nav.popBackStack(t.route, inclusive = false) else go(t) },
                            icon = { Icon(t.icon, null) },
                            label = { Text(t.label) },
                        )
                    }
                }
            },
            snackbarHost = { SnackbarHost(snack) },
            contentWindowInsets = WindowInsets(0),
        ) { padding ->
            Routes(nav, Modifier.padding(padding))
        }
    }
}

@Composable
private fun Routes(nav: NavHostController, modifier: Modifier) {
    val back: () -> Unit = { nav.popBackStack() }
    val openMonitor: (String) -> Unit = { nav.navigate("monitor/$it") }
    NavHost(nav, startDestination = Tab.Overview.route, modifier = modifier) {
        composable(Tab.Overview.route) { OverviewScreen(openMonitor) }
        composable(Tab.Monitors.route) { MonitorsScreen(projectId = null, title = "Monitors", onBack = null, openMonitor = openMonitor) }
        composable(Tab.Alerts.route) { AlertsScreen(openMonitor) }
        composable(Tab.Projects.route) {
            ProjectsScreen(openProject = { project -> nav.navigate("project/${project.id}?name=${Uri.encode(project.name)}") })
        }
        composable(Tab.Settings.route) {
            SettingsScreen(openChannels = { nav.navigate("channels") }, openMaintenance = { nav.navigate("maintenance") })
        }
        composable("monitor/{id}") { MonitorDetailScreen(it.arguments?.getString("id").orEmpty(), onBack = back) }
        composable("project/{id}?name={name}", arguments = listOf(navArgument("name") { defaultValue = "" })) {
            MonitorsScreen(
                projectId = it.arguments?.getString("id"),
                title = it.arguments?.getString("name").orEmpty().ifEmpty { "Project" },
                onBack = back,
                openMonitor = openMonitor,
            )
        }
        composable("channels") { ChannelsScreen(onBack = back) }
        composable("maintenance") { MaintenanceScreen(onBack = back) }
    }
}

/** Asks once for Android 13+'s notification permission, then hands this phone's push token to the server. */
@Composable
private fun AskForNotifications() {
    val context = LocalContext.current
    val ask = rememberLauncherForActivityResult(ActivityResultContracts.RequestPermission()) { App.container.push.register() }
    LaunchedEffect(Unit) {
        val needsAsking = Build.VERSION.SDK_INT >= 33 &&
            ContextCompat.checkSelfPermission(context, Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED
        if (needsAsking && App.container.push.configured) ask.launch(Manifest.permission.POST_NOTIFICATIONS)
        else App.container.push.register()
    }
}
