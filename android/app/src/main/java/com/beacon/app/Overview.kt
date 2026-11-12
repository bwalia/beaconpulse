package com.beacon.app

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.outlined.Build
import androidx.compose.material.icons.outlined.CheckCircle
import androidx.compose.material.icons.outlined.ErrorOutline
import androidx.compose.material.icons.outlined.WarningAmber
import androidx.compose.material3.Card
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.ListItem
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import java.util.Locale

/** The windows the API accepts (?hours=1|6|24|168|720). */
private val windows = listOf(1 to "1h", 6 to "6h", 24 to "24h", 168 to "7d", 720 to "30d")

/** The org-wide dashboard: uptime and response, trend charts, per-monitor rows. Mirrors iOS OverviewView. */
@Composable
fun OverviewScreen(openMonitor: (String) -> Unit) {
    var hours by rememberSaveable { mutableStateOf(24) }
    val loader = rememberLoader("overview-$hours") { App.container.api.get<Overview>("/api/v1/overview", "hours" to "$hours") }

    Screen("Overview") { padding ->
        Column(Modifier.padding(padding).fillMaxSize()) {
            Segmented(windows, hours, { hours = it }, Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 8.dp))
            LoadableContent(loader) { overview ->
                LazyColumn(contentPadding = PaddingValues(16.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
                    item {
                        Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                            Tile("Uptime", "%.2f%%".format(Locale.US, overview.uptimePercent), uptimeColor(overview.uptimePercent), Modifier.weight(1f))
                            Tile("Avg response", "%.0f ms".format(Locale.US, overview.avgResponseMs), MaterialTheme.colorScheme.onSurface, Modifier.weight(1f))
                        }
                    }
                    item {
                        Panel("Uptime") { LineChart(overview.uptimeSeries, Green, filled = true) { "%.0f%%".format(Locale.US, it) } }
                    }
                    item {
                        Panel("Response time") { LineChart(overview.responseSeries, MaterialTheme.colorScheme.primary, filled = false) { "%.0f ms".format(Locale.US, it) } }
                    }
                    item {
                        Panel("Monitors") {
                            if (overview.monitors.isEmpty()) {
                                Text("No monitors reporting yet.", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                            }
                            overview.monitors.forEachIndexed { i, m ->
                                if (i > 0) HorizontalDivider()
                                Row(
                                    Modifier.fillMaxWidth().clickable { openMonitor(m.monitorId) }.padding(vertical = 6.dp),
                                    verticalAlignment = Alignment.CenterVertically,
                                ) {
                                    Column(Modifier.weight(1f)) {
                                        Text(m.monitorName, style = MaterialTheme.typography.bodyLarge, fontWeight = FontWeight.Medium)
                                        Text(m.target, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant, maxLines = 1, overflow = TextOverflow.Ellipsis)
                                    }
                                    Text("%.0f ms".format(Locale.US, m.avgResponseMs), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                                }
                            }
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun Tile(label: String, value: String, color: Color, modifier: Modifier) {
    Card(modifier) {
        Column(Modifier.padding(16.dp)) {
            Text(label, style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
            Text(value, style = MaterialTheme.typography.headlineSmall, fontWeight = FontWeight.Bold, color = color)
        }
    }
}

private fun uptimeColor(percent: Double) = when {
    percent >= 99.9 -> Green
    percent >= 99 -> Amber
    else -> Color(0xFFEA580C)
}

/** The org's currently-firing alerts. Mirrors iOS AlertsView. */
@Composable
fun AlertsScreen(openMonitor: (String) -> Unit) {
    val loader = rememberLoader("alerts") { App.container.api.get<Page<ActiveAlert>>("/api/v1/alerts", "limit" to "200").data }
    Screen("Alerts") { padding ->
        Column(Modifier.padding(padding)) {
            LoadableContent(loader) { alerts ->
                if (alerts.isEmpty()) {
                    Message(Icons.Outlined.CheckCircle, "All clear", "No active alerts right now.")
                } else {
                    LazyColumn(Modifier.fillMaxSize()) {
                        items(alerts, key = { "${it.name}:${it.monitorId}" }) { alert ->
                            val critical = alert.severity.equals("critical", ignoreCase = true)
                            ListItem(
                                modifier = Modifier.clickable { openMonitor(alert.monitorId) },
                                leadingContent = {
                                    Icon(
                                        if (critical) Icons.Outlined.ErrorOutline else Icons.Outlined.WarningAmber,
                                        alert.severity,
                                        tint = if (critical) Red else Amber,
                                    )
                                },
                                headlineContent = { Text(alert.monitorName, fontWeight = FontWeight.Medium) },
                                supportingContent = {
                                    Column {
                                        Text(alert.name)
                                        alert.since?.let { Text("Firing ${relative(it)}", style = MaterialTheme.typography.bodySmall) }
                                    }
                                },
                                trailingContent = {
                                    if (alert.inMaintenance) Icon(Icons.Outlined.Build, "Under maintenance", Modifier.size(20.dp))
                                },
                            )
                            HorizontalDivider()
                        }
                        item { Spacer(Modifier.size(80.dp)) }
                    }
                }
            }
        }
    }
}
