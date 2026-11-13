package com.beacon.app

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.IntrinsicSize
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Report
import androidx.compose.material.icons.filled.Verified
import androidx.compose.material.icons.filled.Warning
import androidx.compose.material.icons.outlined.Build
import androidx.compose.material.icons.outlined.Schedule
import androidx.compose.material.icons.outlined.Speed
import androidx.compose.material.icons.outlined.VerifiedUser
import androidx.compose.material.icons.outlined.WifiTethering
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import java.util.Locale

/** The windows the API accepts (?hours=1|6|24|168|720). */
private val windows = listOf(1 to "1h", 6 to "6h", 24 to "24h", 168 to "7d", 720 to "30d")

/** The org-wide dashboard: headline figures, trend charts, per-monitor rows. Mirrors iOS OverviewView. */
@Composable
fun OverviewScreen(openMonitor: (String) -> Unit) {
    var hours by rememberSaveable { mutableStateOf(24) }
    val loader = rememberLoader("overview-$hours") { App.container.api.get<Overview>("/api/v1/overview", "hours" to "$hours") }

    Screen("Overview") { padding ->
        Column(Modifier.padding(padding).fillMaxSize()) {
            Segmented(windows, hours, { hours = it }, Modifier.fillMaxWidth().padding(start = 16.dp, end = 16.dp, top = 4.dp, bottom = 8.dp))
            LoadableContent(loader) { overview ->
                val window = windows.first { it.first == hours }.second
                LazyColumn(contentPadding = PaddingValues(16.dp, 8.dp, 16.dp, 24.dp), verticalArrangement = Arrangement.spacedBy(14.dp)) {
                    item {
                        val tiles = listOf<@Composable () -> Unit>(
                            {
                                StatTile(
                                    "Uptime", "%.2f%%".format(Locale.US, overview.uptimePercent),
                                    tint = uptimeColor(overview.uptimePercent), icon = Icons.Outlined.VerifiedUser,
                                )
                            },
                            { StatTile("Avg response", "%.0f ms".format(Locale.US, overview.avgResponseMs), icon = Icons.Outlined.Speed) },
                            {
                                StatTile(
                                    "Monitors", "${overview.monitors.size}",
                                    icon = Icons.Outlined.WifiTethering, note = "Reporting in $window",
                                )
                            },
                        )
                        AdaptiveGrid(tiles, minWidth = 150.dp, stretchLastRow = true) { it() }
                    }
                    item { ChartCard("Uptime", overview.uptimeSeries, Up) { "%.0f%%".format(Locale.US, it) } }
                    item { ChartCard("Response time", overview.responseSeries, Theme.accent) { "%.0f ms".format(Locale.US, it) } }
                    item { MonitorsCard(overview.monitors, openMonitor) }
                }
            }
        }
    }
}

@Composable
private fun ChartCard(title: String, series: List<MetricPoint>, color: Color, label: (Double) -> String) {
    Card(spacing = 12.dp) {
        Text(title, style = cardTitle)
        TrendChart(series, color, label = label)
    }
}

/** A card's heading. */
val cardTitle = TextStyle(fontSize = 17.sp, fontWeight = FontWeight.SemiBold)

@Composable
private fun MonitorsCard(monitors: List<MonitorUptime>, openMonitor: (String) -> Unit) {
    Column(Modifier.fillMaxWidth().card()) {
        Row(Modifier.padding(start = 16.dp, end = 16.dp, top = 16.dp, bottom = 8.dp), verticalAlignment = Alignment.CenterVertically) {
            Text("Monitors", style = cardTitle, modifier = Modifier.weight(1f))
            Eyebrow("Avg response")
        }
        if (monitors.isEmpty()) {
            Text("No monitors reporting yet.", Modifier.padding(16.dp), style = MaterialTheme.typography.bodySmall, color = Theme.muted)
        }
        monitors.forEachIndexed { i, m ->
            if (i > 0) RowDivider(Modifier.padding(start = 16.dp))
            Row(
                Modifier.fillMaxWidth().clickable { openMonitor(m.monitorId) }.padding(start = 16.dp, end = 10.dp, top = 11.dp, bottom = 11.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(3.dp)) {
                    Text(m.monitorName, style = TextStyle(fontSize = 15.sp, fontWeight = FontWeight.SemiBold), maxLines = 1, overflow = TextOverflow.Ellipsis)
                    Text(m.target, style = monoCaption, color = Theme.muted, maxLines = 1, overflow = TextOverflow.Ellipsis)
                }
                Spacer(Modifier.width(10.dp))
                Text(
                    "%.0f ms".format(Locale.US, m.avgResponseMs),
                    Modifier.background(Theme.subtle, CircleShape).padding(horizontal = 8.dp, vertical = 4.dp),
                    style = TextStyle(fontSize = 12.sp, fontWeight = FontWeight.SemiBold, fontFeatureSettings = "tnum"),
                )
                Spacer(Modifier.width(4.dp))
                Chevron()
            }
        }
        Spacer(Modifier.height(6.dp))
    }
}

/** URLs and hosts under a name. */
val monoCaption = TextStyle(fontSize = 12.sp, fontFamily = FontFamily.Monospace)

/** The org's currently-firing alerts. Mirrors iOS AlertsView. */
@Composable
fun AlertsScreen(openMonitor: (String) -> Unit) {
    val loader = rememberLoader("alerts") { App.container.api.get<Page<ActiveAlert>>("/api/v1/alerts", "limit" to "200").data }
    Screen("Alerts") { padding ->
        Column(Modifier.padding(padding)) {
            LoadableContent(loader) { alerts ->
                if (alerts.isEmpty()) {
                    Message(
                        Icons.Filled.Verified, "All clear",
                        "No active alerts right now. You’ll get a notification the moment something goes wrong.",
                        tint = Up,
                    )
                } else {
                    LazyColumn(Modifier.fillMaxSize(), contentPadding = PaddingValues(16.dp, 4.dp, 16.dp, 24.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
                        items(alerts, key = { "${it.name}:${it.monitorId}" }) { alert -> AlertRow(alert) { openMonitor(alert.monitorId) } }
                    }
                }
            }
        }
    }
}

fun severityColor(severity: String) = when (severity.lowercase()) {
    "critical" -> Down
    "warning" -> Degraded
    else -> Color(0xFFEAB308) // yellow-500
}

/** An alert as a card with a severity bar down its left edge. */
@Composable
private fun AlertRow(alert: ActiveAlert, onClick: () -> Unit) {
    val color = severityColor(alert.severity)
    Row(
        Modifier.fillMaxWidth().card().clickable(onClick = onClick).height(IntrinsicSize.Min).padding(start = 14.dp, end = 8.dp, top = 14.dp, bottom = 14.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Box(Modifier.width(4.dp).fillMaxHeight().background(color, RoundedCornerShape(2.dp)))
        Spacer(Modifier.width(12.dp))
        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(4.dp)) {
            Text(alert.monitorName, style = TextStyle(fontSize = 17.sp, fontWeight = FontWeight.SemiBold), maxLines = 1, overflow = TextOverflow.Ellipsis)
            Text(alert.name, style = MaterialTheme.typography.bodyMedium, color = Theme.muted)
            alert.since?.let { MetaLine(Icons.Outlined.Schedule, "Firing ${relative(it)}") }
        }
        Spacer(Modifier.width(8.dp))
        Column(horizontalAlignment = Alignment.End, verticalArrangement = Arrangement.spacedBy(6.dp)) {
            val critical = alert.severity.equals("critical", ignoreCase = true)
            Pill(alert.severity.replaceFirstChar { it.uppercase() }.ifEmpty { "Alert" }, color, icon = if (critical) Icons.Filled.Report else Icons.Filled.Warning)
            if (alert.inMaintenance) MetaLine(Icons.Outlined.Build, "Maintenance", bold = true)
        }
        Chevron()
    }
}

/** A small icon and muted caption: "Firing 5 min ago". */
@Composable
fun MetaLine(icon: ImageVector, text: String, bold: Boolean = false) {
    Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(4.dp)) {
        Icon(icon, null, Modifier.size(13.dp), tint = Theme.muted)
        Text(text, style = TextStyle(fontSize = 12.sp, fontWeight = if (bold) FontWeight.SemiBold else FontWeight.Normal), color = Theme.muted)
    }
}
