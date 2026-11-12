@file:UseSerializers(InstantSerializer::class)

package com.beacon.app

import kotlinx.serialization.KSerializer
import kotlinx.serialization.SerializationException
import kotlinx.serialization.Serializable
import kotlinx.serialization.UseSerializers
import kotlinx.serialization.descriptors.PrimitiveKind
import kotlinx.serialization.descriptors.PrimitiveSerialDescriptor
import kotlinx.serialization.encoding.Decoder
import kotlinx.serialization.encoding.Encoder
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.booleanOrNull
import kotlinx.serialization.json.contentOrNull
import kotlinx.serialization.json.intOrNull
import java.time.Instant
import java.time.OffsetDateTime

// Wire models. Keys arrive snake_case and are mapped by the Json naming strategy in
// Api.kt, so properties stay camelCase. Mirrors ios/Sources/Networking/DTOs.swift.

@Serializable
data class AuthResponse(val accessToken: String, val refreshToken: String, val user: AuthUser)

@Serializable
data class AuthUser(val id: String, val orgId: String, val email: String, val name: String, val role: String)

/** The standard collection envelope: `{ "data": [...], "pagination": {...} }`. */
@Serializable
data class Page<T>(val data: List<T>)

@Serializable
data class Monitor(
    val id: String,
    val name: String,
    val type: String,
    val target: String = "",
    val enabled: Boolean = true,
    val intervalSeconds: Int = 60,
    val timeoutSeconds: Int = 10,
    val graceSeconds: Int? = null,
    /** Kept as raw JSON so an edit sends back every key, including ones only the web sets. */
    val settings: JsonObject = JsonObject(emptyMap()),
    val lastStatus: String = "unknown",
    val lastCheckedAt: Instant? = null,
    val projectId: String? = null,
    val pingUrl: String? = null,
    val lastPingAt: Instant? = null,
) {
    val kind: MonitorKind? get() = MonitorKind.entries.firstOrNull { it.wire == type }
}

enum class MonitorKind(val wire: String) {
    HTTP("http"), HTTPS("https"), SSL("ssl"), TCP("tcp"), ICMP("icmp"), DNS("dns"),
    HEARTBEAT("heartbeat"), GITHUB_ACTIONS("github_actions");

    /** A heartbeat is pinged by the customer's job, so it has no probe target. */
    val needsTarget get() = this != HEARTBEAT
    /** Probed by us on a schedule, so a timeout applies. */
    val isProbed get() = this != HEARTBEAT && this != GITHUB_ACTIONS
    /** Method, status codes, headers… — the same set the web form shows them for. */
    val takesRequestSettings get() = this == HTTP || this == HTTPS || this == SSL

    val targetHint
        get() = when (this) {
            HTTP, HTTPS -> "https://example.com"
            SSL -> "example.com:443"
            TCP -> "host:port"
            ICMP, DNS -> "example.com"
            HEARTBEAT -> ""
            GITHUB_ACTIONS -> "owner/repo"
        }

    companion object {
        /** A GitHub Actions monitor's ingest token is shown once, on creation, so those are made on the web. */
        val creatable = listOf(HTTP, HTTPS, SSL, TCP, ICMP, DNS, HEARTBEAT)
    }
}

@Serializable
data class MetricPoint(val t: Instant, val v: Double)

@Serializable
data class MonitorMetrics(
    val uptimePercent: Double = 0.0,
    val responseMsAvg: Double? = null,
    val responseMs: List<MetricPoint>? = null,
)

@Serializable
data class Overview(
    val uptimePercent: Double = 0.0,
    val avgResponseMs: Double = 0.0,
    val uptimeSeries: List<MetricPoint> = emptyList(),
    val responseSeries: List<MetricPoint> = emptyList(),
    val monitors: List<MonitorUptime> = emptyList(),
)

@Serializable
data class MonitorUptime(val monitorId: String, val monitorName: String, val target: String = "", val avgResponseMs: Double = 0.0)

@Serializable
data class ActiveAlert(
    val name: String,
    val severity: String = "",
    val monitorId: String,
    val monitorName: String,
    val since: Instant? = null,
    val inMaintenance: Boolean = false,
)

@Serializable
data class MonitorUsage(val monitorsUsed: Int, val monitorsLimit: Int, val minIntervalSeconds: Int)

@Serializable
data class Project(
    val id: String,
    val name: String,
    val description: String = "",
    val environment: String = "production",
)

@Serializable
data class Channel(
    val id: String,
    val name: String,
    val type: String,
    val enabled: Boolean = true,
    val config: Map<String, String> = emptyMap(),
) {
    /** Push channels are created and kept up to date by the server; the app only toggles them. */
    val isManaged get() = type in setOf("apns", "fcm", "webpush")
}

@Serializable
data class MaintenanceWindow(
    val id: String,
    val title: String,
    val description: String = "",
    val startsAt: Instant,
    val endsAt: Instant,
    val scope: String,
    val scopeIds: List<String> = emptyList(),
    val active: Boolean = false,
)

// ---- request bodies (null fields are left out of the JSON) ----

@Serializable data class LoginBody(val email: String, val password: String)
@Serializable data class RegisterBody(val orgName: String, val name: String, val email: String, val password: String)
@Serializable data class GoogleBody(val idToken: String)
@Serializable data class RefreshBody(val refreshToken: String)
@Serializable data class DeviceBody(val token: String, val platform: String? = null)
@Serializable data class DeleteAccountBody(val confirmEmail: String)

@Serializable
data class CreateMonitorBody(
    val projectId: String, val name: String, val type: String, val target: String, val enabled: Boolean,
    val intervalSeconds: Int, val timeoutSeconds: Int, val graceSeconds: Int?, val settings: JsonObject,
)

/** Type is immutable server-side; `settings` is the whole round-tripped object. */
@Serializable
data class UpdateMonitorBody(
    val name: String, val target: String?, val enabled: Boolean,
    val intervalSeconds: Int, val timeoutSeconds: Int, val settings: JsonObject,
)

@Serializable data class ProjectBody(val name: String, val description: String, val environment: String)

@Serializable
data class ChannelBody(
    val name: String? = null, val type: String? = null, val enabled: Boolean? = null,
    val config: Map<String, String>? = null,
    /** Null on edit keeps the stored credential. */
    val secret: String? = null,
)

@Serializable
data class WindowBody(
    val title: String, val description: String, val startsAt: Instant, val endsAt: Instant,
    val scope: String, val scopeIds: List<String>,
)

// ---- monitor settings helpers (keys are the server's snake_case) ----

fun JsonObject.str(key: String) = (this[key] as? JsonPrimitive)?.contentOrNull
fun JsonObject.int(key: String) = (this[key] as? JsonPrimitive)?.intOrNull
fun JsonObject.bool(key: String) = (this[key] as? JsonPrimitive)?.booleanOrNull

/** This object with [edits] applied: a null value removes the key, as the web form omits unset fields. */
fun JsonObject.with(edits: Map<String, JsonElement?>) =
    JsonObject(toMutableMap().apply { edits.forEach { (k, v) -> if (v == null) remove(k) else put(k, v) } })

/** "Name: value" lines, sorted so the order is stable between edits. Saved values come back as ********. */
fun headerLines(settings: JsonObject) =
    (settings["headers"] as? JsonObject).orEmpty().entries.sortedBy { it.key }
        .joinToString("\n") { "${it.key}: ${(it.value as? JsonPrimitive)?.contentOrNull.orEmpty()}" }

/** Parses "Name: value" lines (the value may contain colons); nothing left means no headers. */
fun parseHeaders(text: String): JsonObject? {
    val out = text.lines().mapNotNull { line ->
        val colon = line.indexOf(':')
        val name = if (colon > 0) line.substring(0, colon).trim() else ""
        if (name.isEmpty()) null else name to JsonPrimitive(line.substring(colon + 1).trim())
    }.toMap()
    return if (out.isEmpty()) null else JsonObject(out)
}

fun statusCodesText(settings: JsonObject) =
    (settings["valid_status_codes"] as? JsonArray).orEmpty().mapNotNull { (it as? JsonPrimitive)?.intOrNull }.joinToString(", ")

/** Parses "200, 204, 301"; nothing left means the server default (any 2xx/3xx). */
fun parseStatusCodes(text: String): JsonArray? {
    val codes = text.split(',').mapNotNull { it.trim().toIntOrNull() }
    return if (codes.isEmpty()) null else JsonArray(codes.map(::JsonPrimitive))
}

/** RFC 3339 timestamps, with or without fractional seconds. */
object InstantSerializer : KSerializer<Instant> {
    override val descriptor = PrimitiveSerialDescriptor("Instant", PrimitiveKind.STRING)
    override fun serialize(encoder: Encoder, value: Instant) = encoder.encodeString(value.toString())
    override fun deserialize(decoder: Decoder): Instant = decoder.decodeString().let {
        try {
            OffsetDateTime.parse(it).toInstant()
        } catch (e: Exception) {
            throw SerializationException("bad timestamp $it")
        }
    }
}
