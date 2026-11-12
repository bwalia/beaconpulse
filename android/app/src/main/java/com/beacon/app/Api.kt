package com.beacon.app

import android.content.Context
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import android.util.Base64
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.withContext
import kotlinx.serialization.ExperimentalSerializationApi
import kotlinx.serialization.Serializable
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonNamingStrategy
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.contentOrNull
import kotlinx.serialization.json.encodeToJsonElement
import okhttp3.Authenticator
import okhttp3.HttpUrl.Companion.toHttpUrl
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import java.io.IOException
import java.net.SocketTimeoutException
import java.security.KeyStore
import java.util.concurrent.TimeUnit
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec

/** The API speaks snake_case; null fields are left out so an edit never clears what it didn't touch. */
@OptIn(ExperimentalSerializationApi::class)
val json = Json {
    namingStrategy = JsonNamingStrategy.SnakeCase
    ignoreUnknownKeys = true
    explicitNulls = false
    coerceInputValues = true
}

/** Every API failure, carrying a message written for the person using the app. */
class ApiError(message: String, val status: Int = 0) : Exception(message)

fun Throwable.userMessage(fallback: String) = (this as? ApiError)?.message ?: fallback

/** Runs [block], turning failures into a Result without swallowing coroutine cancellation. */
suspend fun <T> attempt(block: suspend () -> T): Result<T> = try {
    Result.success(block())
} catch (e: CancellationException) {
    throw e
} catch (e: Exception) {
    Result.failure(e)
}

inline fun <reified T> decode(text: String): T = try {
    json.decodeFromString(text)
} catch (e: IllegalArgumentException) { // SerializationException included
    throw ApiError("We couldn’t read the server’s response.")
}

private val JSON_TYPE = "application/json".toMediaType()

/**
 * The single HTTP entry point: JSON over OkHttp with typed errors. The authenticated
 * instance carries the bearer and one transparent refresh-and-retry on 401 (see
 * [authenticatedClient]); the auth endpoints use a plain one. Paths are full,
 * e.g. "/api/v1/monitors". Mirrors ios/Sources/Networking/APIClient.swift.
 */
class Api(val baseUrl: String, private val http: OkHttpClient) {

    suspend fun call(method: String, path: String, query: Map<String, String> = emptyMap(), body: JsonElement? = null): String =
        withContext(Dispatchers.IO) {
            val url = (baseUrl.trimEnd('/') + path).toHttpUrl().newBuilder()
                .apply { query.toSortedMap().forEach { (k, v) -> addQueryParameter(k, v) } }
                .build()
            val payload = when {
                body != null -> json.encodeToString(body).toRequestBody(JSON_TYPE)
                method == "POST" || method == "PUT" || method == "PATCH" -> ByteArray(0).toRequestBody(JSON_TYPE)
                else -> null
            }
            val request = Request.Builder().url(url).method(method, payload).header("Accept", "application/json").build()
            val response = try {
                http.newCall(request).execute()
            } catch (e: SocketTimeoutException) {
                throw ApiError("The request timed out. Please try again.")
            } catch (e: IOException) {
                throw ApiError("Couldn’t reach the server. Check your connection and try again.")
            }
            response.use {
                val text = it.body?.string().orEmpty()
                if (!it.isSuccessful) throw httpError(it.code, text)
                text
            }
        }

    suspend inline fun <reified T> get(path: String, vararg query: Pair<String, String>): T = decode(call("GET", path, query.toMap()))
    suspend inline fun <reified B> send(method: String, path: String, body: B): String = call(method, path, body = json.encodeToJsonElement(body))
    suspend fun send(method: String, path: String): String = call(method, path)

    private fun httpError(status: Int, text: String): ApiError {
        // The server's own message when it sent one ({"error":{"message":…}}), else a plain default.
        val message = runCatching {
            ((json.parseToJsonElement(text) as JsonObject)["error"] as JsonObject)["message"].let { (it as JsonPrimitive).contentOrNull }
        }.getOrNull()?.takeIf { it.isNotBlank() && status < 500 }
        return ApiError(
            message ?: when (status) {
                401 -> "Your session has expired. Please sign in again."
                403 -> "You don’t have permission to do that."
                404 -> "That could not be found."
                429 -> "Too many requests. Please wait a moment and try again."
                else -> "Something went wrong on the server. Please try again."
            },
            status,
        )
    }
}

fun baseHttpClient(): OkHttpClient = OkHttpClient.Builder()
    .connectTimeout(15, TimeUnit.SECONDS)
    .readTimeout(30, TimeUnit.SECONDS)
    .writeTimeout(30, TimeUnit.SECONDS)
    .build()

/** The client for everything but the auth endpoints: bearer on every call, one refresh-and-retry on 401. */
fun authenticatedClient(base: OkHttpClient, session: Session): OkHttpClient = base.newBuilder()
    .addInterceptor { chain ->
        val token = session.accessToken
        val request = chain.request()
        chain.proceed(if (token == null) request else request.newBuilder().header("Authorization", "Bearer $token").build())
    }
    .authenticator(Authenticator { _, response ->
        if (response.priorResponse != null) { // already retried with a fresh token: the session is gone
            session.clear()
            return@Authenticator null
        }
        val fresh = session.refresh(response.request.header("Authorization")?.removePrefix("Bearer "))
            ?: return@Authenticator null
        response.request.newBuilder().header("Authorization", "Bearer $fresh").build()
    })
    .build()

/**
 * The signed-in user and their tokens: the single source of truth for auth state.
 * Persisted encrypted through [load]/[save] (a [Vault] in the app). [user] drives the
 * UI: null shows sign-in. Mirrors ios/Sources/Auth/SessionStore.swift.
 */
class Session(private val authApi: Api, private val load: () -> String?, private val save: (String?) -> Unit) {
    @Serializable
    private data class Stored(val accessToken: String, val refreshToken: String, val user: AuthUser)

    private var stored: Stored? = load()?.let { runCatching { json.decodeFromString<Stored>(it) }.getOrNull() }
    private val _user = MutableStateFlow(stored?.user)
    val user: StateFlow<AuthUser?> = _user
    val accessToken: String? get() = stored?.accessToken

    @Synchronized
    fun adopt(response: AuthResponse) {
        val next = Stored(response.accessToken, response.refreshToken, response.user)
        stored = next
        save(json.encodeToString(next))
        _user.value = response.user
    }

    @Synchronized
    fun clear() {
        stored = null
        save(null)
        _user.value = null
    }

    /**
     * Swaps the refresh token for a new pair after a 401 on [failedToken], and returns
     * the new access token, or null when it can't. Synchronized, and a no-op when
     * another call already refreshed, so a burst of 401s costs one round trip. Only a
     * server refusal signs out: being offline is not a reason to lose the session.
     */
    @Synchronized
    fun refresh(failedToken: String?): String? {
        val current = stored ?: return null
        if (current.accessToken != failedToken) return current.accessToken
        return try {
            val response = runBlocking {
                decode<AuthResponse>(authApi.send("POST", "/api/v1/auth/refresh", RefreshBody(current.refreshToken)))
            }
            adopt(response)
            response.accessToken
        } catch (e: ApiError) {
            if (e.status in 400..499) clear()
            null
        }
    }

    /** Best-effort revoke on the server, then wipe locally regardless. */
    suspend fun signOut() {
        stored?.refreshToken?.let { token -> attempt { authApi.send("POST", "/api/v1/auth/logout", RefreshBody(token)) } }
        clear()
    }
}

/**
 * The session at rest: AES-GCM with a key that lives in the Android Keystore and never
 * leaves the device, so the tokens are unreadable outside this install.
 */
class Vault(context: Context) {
    private val prefs = context.getSharedPreferences("session", Context.MODE_PRIVATE)

    private fun key(): SecretKey {
        val store = KeyStore.getInstance("AndroidKeyStore").apply { load(null) }
        (store.getEntry(ALIAS, null) as? KeyStore.SecretKeyEntry)?.let { return it.secretKey }
        return KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, "AndroidKeyStore").apply {
            init(
                KeyGenParameterSpec.Builder(ALIAS, KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT)
                    .setBlockModes(KeyProperties.BLOCK_MODE_GCM)
                    .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
                    .build(),
            )
        }.generateKey()
    }

    fun save(value: String?) {
        if (value == null) {
            prefs.edit().remove(ENTRY).apply()
            return
        }
        val cipher = Cipher.getInstance(TRANSFORM).apply { init(Cipher.ENCRYPT_MODE, key()) }
        val sealed = cipher.iv + cipher.doFinal(value.toByteArray())
        prefs.edit().putString(ENTRY, Base64.encodeToString(sealed, Base64.NO_WRAP)).apply()
    }

    /** The saved session, or null when there is none or it can't be decrypted (signed out). */
    fun load(): String? = runCatching {
        val sealed = Base64.decode(prefs.getString(ENTRY, null) ?: return null, Base64.NO_WRAP)
        val cipher = Cipher.getInstance(TRANSFORM)
        cipher.init(Cipher.DECRYPT_MODE, key(), GCMParameterSpec(128, sealed, 0, 12))
        String(cipher.doFinal(sealed, 12, sealed.size - 12))
    }.getOrNull()

    private companion object {
        const val ALIAS = "session"
        const val ENTRY = "v1"
        const val TRANSFORM = "AES/GCM/NoPadding"
    }
}
