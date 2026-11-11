package com.beacon.app

import android.Manifest
import android.app.Application
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Build
import androidx.core.app.NotificationCompat
import androidx.core.app.NotificationManagerCompat
import androidx.core.content.ContextCompat
import com.google.firebase.FirebaseApp
import com.google.firebase.FirebaseOptions
import com.google.firebase.messaging.FirebaseMessaging
import com.google.firebase.messaging.FirebaseMessagingService
import com.google.firebase.messaging.RemoteMessage
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.launch

class App : Application() {
    override fun onCreate() {
        super.onCreate()
        container = Container(this)
        container.push.start()
    }

    companion object {
        /** The composition root, built once at launch (the iOS AppContainer). */
        lateinit var container: Container
            private set
    }
}

class Container(app: Application) {
    private val vault = Vault(app)
    private val http = baseHttpClient()
    private val authApi = Api(BuildConfig.API_BASE_URL, http)

    val session = Session(authApi, vault::load, vault::save)
    /** Everything but the auth endpoints: bearer + one transparent refresh on 401. */
    val api = Api(BuildConfig.API_BASE_URL, authenticatedClient(http, session))
    val push = Push(app, api, session)

    /** A tapped alert's monitor, waiting for the signed-in UI to open it. */
    val pendingMonitor = MutableStateFlow<String?>(null)

    suspend fun login(email: String, password: String) = session.adopt(decode(authApi.send("POST", "/api/v1/auth/login", LoginBody(email, password))))
    suspend fun register(body: RegisterBody) = session.adopt(decode(authApi.send("POST", "/api/v1/auth/register", body)))
    suspend fun signInWithGoogle(idToken: String) = session.adopt(decode(authApi.send("POST", "/api/v1/auth/google", GoogleBody(idToken))))

    suspend fun signOut() {
        push.unregister()
        session.signOut()
    }
}

/**
 * The device side of alert push: Firebase setup from the flavor's ids, the alerts
 * channel, handing this phone's token to /api/v1/devices, and dropping it on sign-out.
 * The fan-out lives in the backend's fcm notifier. Without Firebase ids for the brand,
 * push is simply off. Mirrors ios/Sources/Push/PushManager.swift.
 */
class Push(private val app: Application, private val api: Api, private val session: Session) {
    val configured = BuildConfig.FIREBASE_APP_ID.isNotEmpty() && BuildConfig.FIREBASE_API_KEY.isNotEmpty() &&
        BuildConfig.FIREBASE_PROJECT_ID.isNotEmpty() && BuildConfig.FIREBASE_SENDER_ID.isNotEmpty()
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    @Volatile private var registered: String? = null

    fun start() {
        val channel = NotificationChannel(CHANNEL, "Monitor alerts", NotificationManager.IMPORTANCE_HIGH)
            .apply { description = "When a monitor goes down or recovers" }
        app.getSystemService(NotificationManager::class.java).createNotificationChannel(channel)
        if (!configured || FirebaseApp.getApps(app).isNotEmpty()) return
        FirebaseApp.initializeApp(
            app,
            FirebaseOptions.Builder()
                .setApplicationId(BuildConfig.FIREBASE_APP_ID)
                .setApiKey(BuildConfig.FIREBASE_API_KEY)
                .setProjectId(BuildConfig.FIREBASE_PROJECT_ID)
                .setGcmSenderId(BuildConfig.FIREBASE_SENDER_ID)
                .build(),
        )
    }

    /** Sends this phone's token to the server (call once signed in). */
    fun register() {
        if (configured) FirebaseMessaging.getInstance().token.addOnSuccessListener(::onToken)
    }

    fun onToken(token: String) {
        if (!configured || session.user.value == null || token == registered) return
        scope.launch {
            attempt { api.send("POST", "/api/v1/devices", DeviceBody(token, "android")) }.onSuccess { registered = token }
        }
    }

    /** Stops this phone getting the org's alerts (on sign-out). */
    suspend fun unregister() {
        val token = registered ?: return
        attempt { api.send("DELETE", "/api/v1/devices", DeviceBody(token)) }
        registered = null
    }

    val notificationsAllowed get() = NotificationManagerCompat.from(app).areNotificationsEnabled()

    /** Shows an alert that arrived while the app was open (the system shows the rest). */
    fun show(title: String, body: String, monitorId: String?) {
        if (Build.VERSION.SDK_INT >= 33 &&
            ContextCompat.checkSelfPermission(app, Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED
        ) return
        val open = Intent(app, MainActivity::class.java)
            .putExtra(EXTRA_MONITOR, monitorId)
            .addFlags(Intent.FLAG_ACTIVITY_SINGLE_TOP or Intent.FLAG_ACTIVITY_CLEAR_TOP)
        val tap = PendingIntent.getActivity(
            app, (monitorId ?: title).hashCode(), open,
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
        )
        val notification = NotificationCompat.Builder(app, CHANNEL)
            .setSmallIcon(R.drawable.ic_notification)
            .setColor(ContextCompat.getColor(app, R.color.brand))
            .setContentTitle(title)
            .setContentText(body)
            .setStyle(NotificationCompat.BigTextStyle().bigText(body))
            .setPriority(NotificationCompat.PRIORITY_HIGH)
            .setAutoCancel(true)
            .setContentIntent(tap)
            .build()
        NotificationManagerCompat.from(app).notify((monitorId ?: title).hashCode(), notification)
    }

    companion object {
        const val CHANNEL = "alerts"
        /** The data key the backend puts the monitor id under (same as iOS). */
        const val EXTRA_MONITOR = "monitor_id"
    }
}

class PushService : FirebaseMessagingService() {
    override fun onNewToken(token: String) = App.container.push.onToken(token)

    override fun onMessageReceived(message: RemoteMessage) {
        val title = message.notification?.title ?: message.data["title"] ?: return
        val body = message.notification?.body ?: message.data["body"].orEmpty()
        App.container.push.show(title, body, message.data[Push.EXTRA_MONITOR])
    }
}
