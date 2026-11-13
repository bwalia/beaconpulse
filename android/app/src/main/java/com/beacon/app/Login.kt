package com.beacon.app

import android.app.Activity
import android.content.Context
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.systemBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Error
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalView
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.LinkAnnotation
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.TextLinkStyles
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.withLink
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.core.view.WindowCompat
import androidx.credentials.CredentialManager
import androidx.credentials.CustomCredential
import androidx.credentials.GetCredentialRequest
import androidx.credentials.exceptions.GetCredentialCancellationException
import androidx.credentials.exceptions.GetCredentialException
import androidx.credentials.exceptions.NoCredentialException
import com.google.android.libraries.identity.googleid.GetSignInWithGoogleOption
import com.google.android.libraries.identity.googleid.GoogleIdTokenCredential
import kotlinx.coroutines.launch

private val base = BuildConfig.API_BASE_URL.trimEnd('/')

/** Sign-in: email/password and Continue with Google over the brand hero; sign-up opens over it. Mirrors iOS LoginView. */
@Composable
fun LoginScreen() {
    val app = App.container
    val context = LocalContext.current
    val scope = rememberCoroutineScope()
    var email by rememberSaveable { mutableStateOf("") }
    var password by rememberSaveable { mutableStateOf("") }
    var busy by rememberSaveable { mutableStateOf(false) }
    var error by rememberSaveable { mutableStateOf<String?>(null) }
    var signingUp by rememberSaveable { mutableStateOf(false) }

    /** Runs a sign-in; on success the session flips the app to the signed-in UI. */
    fun submit(fallback: String, block: suspend () -> Unit) {
        if (busy) return
        busy = true
        error = null
        scope.launch {
            attempt { block() }.onFailure { error = it.userMessage(fallback) }
            busy = false
        }
    }

    val signIn = { if (email.isNotBlank() && password.isNotEmpty()) submit("Sign in failed. Please try again.") { app.login(email.trim(), password) } }

    LightStatusBarIcons()
    Box(Modifier.fillMaxSize().background(Theme.canvas)) {
        // The brand gradient behind the header, with faint rings echoing the mark.
        Box(
            Modifier.fillMaxWidth().height(420.dp).background(Theme.brandGradient).drawBehind {
                drawRings(listOf(220.dp, 340.dp, 460.dp), Offset(center.x + 120.dp.toPx(), center.y - 40.dp.toPx()))
            },
        )
        Column(
            Modifier.fillMaxSize().verticalScroll(rememberScrollState()).systemBarsPadding().imePadding().padding(20.dp),
            verticalArrangement = Arrangement.spacedBy(22.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            Spacer(Modifier.height(28.dp))
            Header()

            val shape = RoundedCornerShape(20.dp)
            Column(
                Modifier.widthIn(max = 440.dp).fillMaxWidth()
                    .shadow(16.dp, shape, ambientColor = Color(0x22000000), spotColor = Color(0x22000000))
                    .clip(shape).background(Theme.card).border(1.dp, Theme.border, shape).padding(22.dp),
                verticalArrangement = Arrangement.spacedBy(14.dp),
            ) {
                Input(
                    email, { email = it },
                    placeholder = "Email",
                    keyboard = KeyboardOptions(keyboardType = KeyboardType.Email, imeAction = ImeAction.Next, autoCorrectEnabled = false),
                )
                Input(
                    password, { password = it },
                    placeholder = "Password",
                    secret = true,
                    keyboard = KeyboardOptions(keyboardType = KeyboardType.Password, imeAction = ImeAction.Done),
                    actions = KeyboardActions(onDone = { signIn() }),
                )
                // The reset email links to the website anyway, so the flow lives there.
                Text(
                    "Forgot password?",
                    Modifier.align(Alignment.End).clip(RoundedCornerShape(6.dp)).clickable(role = Role.Button) { openUrl(context, "$base/forgot-password") }
                        .padding(horizontal = 4.dp, vertical = 2.dp),
                    style = TextStyle(fontSize = 13.sp, fontWeight = FontWeight.Medium),
                    color = Theme.accent,
                )
                error?.let {
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        Icon(Icons.Filled.Error, null, Modifier.size(16.dp), tint = Down)
                        Spacer(Modifier.width(6.dp))
                        Text(it, style = TextStyle(fontSize = 13.sp), color = Down)
                    }
                }
                AccentButton("Sign in", signIn, Modifier.fillMaxWidth().height(50.dp), enabled = email.isNotBlank() && password.isNotEmpty(), busy = busy)

                Row(verticalAlignment = Alignment.CenterVertically) {
                    RowDivider(Modifier.weight(1f))
                    Text("or", Modifier.padding(horizontal = 10.dp), style = MaterialTheme.typography.bodySmall, color = Theme.muted)
                    RowDivider(Modifier.weight(1f))
                }

                val buttonShape = RoundedCornerShape(12.dp)
                Row(
                    Modifier.fillMaxWidth().height(50.dp).clip(buttonShape).background(Theme.card).border(1.dp, Theme.border, buttonShape)
                        .clickable(enabled = !busy, role = Role.Button) {
                            submit("Google sign-in failed. Please try again.") {
                                val token = googleIdToken(context) ?: return@submit
                                app.signInWithGoogle(token)
                            }
                        },
                    horizontalArrangement = Arrangement.Center,
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Image(painterResource(R.drawable.ic_google), null, Modifier.size(20.dp))
                    Spacer(Modifier.width(10.dp))
                    Text("Continue with Google", style = TextStyle(fontSize = 16.sp, fontWeight = FontWeight.Medium), color = Theme.text)
                }
            }

            Row(verticalAlignment = Alignment.CenterVertically) {
                Text("New here?", style = TextStyle(fontSize = 14.sp), color = Theme.muted)
                Text(
                    "Create an account",
                    Modifier.clip(RoundedCornerShape(6.dp)).clickable(enabled = !busy, role = Role.Button) { signingUp = true }.padding(4.dp),
                    style = TextStyle(fontSize = 14.sp, fontWeight = FontWeight.SemiBold),
                    color = Theme.accent,
                )
            }
        }
    }

    if (signingUp) RegisterDialog(onDismiss = { signingUp = false })
}

/** The mark on a frosted tile, the brand name and the prompt, in white over the hero. */
@Composable
private fun Header() {
    Column(horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(12.dp)) {
        val tile = RoundedCornerShape(18.dp)
        Box(
            Modifier.size(68.dp).background(Color.White.copy(alpha = 0.18f), tile).border(1.dp, Color.White.copy(alpha = 0.35f), tile),
            contentAlignment = Alignment.Center,
        ) { BrandMark(Modifier.size(44.dp), strokeUnits = 1.65f) }
        Text(stringResource(R.string.app_name), style = TextStyle(fontSize = 34.sp, fontWeight = FontWeight.Bold, letterSpacing = (-0.5).sp), color = Color.White)
        Text("Sign in to your monitors", style = TextStyle(fontSize = 16.sp), color = Color.White.copy(alpha = 0.85f))
    }
}

/** White status-bar icons while the hero is under them; back to the theme's on leaving. */
@Composable
private fun LightStatusBarIcons() {
    val view = LocalView.current
    DisposableEffect(view) {
        val window = (view.context as? Activity)?.window
        val bars = window?.let { WindowCompat.getInsetsController(it, view) }
        val before = bars?.isAppearanceLightStatusBars
        bars?.isAppearanceLightStatusBars = false
        onDispose { if (before != null) bars.isAppearanceLightStatusBars = before }
    }
}

/**
 * The Google account chooser, returning an ID token for /api/v1/auth/google, or null
 * when the person backs out. Needs an Android OAuth client for this package and
 * signing key in the brand's Google Cloud project.
 */
private suspend fun googleIdToken(context: Context): String? {
    val option = GetSignInWithGoogleOption.Builder(BuildConfig.GOOGLE_SERVER_CLIENT_ID).build()
    val request = GetCredentialRequest.Builder().addCredentialOption(option).build()
    val credential = try {
        CredentialManager.create(context).getCredential(context, request).credential
    } catch (e: GetCredentialCancellationException) {
        return null
    } catch (e: NoCredentialException) {
        throw ApiError("Add a Google account to this phone, or sign in with your email.")
    } catch (e: GetCredentialException) {
        throw ApiError("Google sign-in isn’t available right now. Sign in with your email instead.")
    }
    if (credential is CustomCredential && credential.type == GoogleIdTokenCredential.TYPE_GOOGLE_ID_TOKEN_CREDENTIAL) {
        return GoogleIdTokenCredential.createFrom(credential.data).idToken
    }
    throw ApiError("Google sign-in returned an unexpected response.")
}

/** Email/password sign-up: creates an organisation and its owner, then signs in. */
@Composable
private fun RegisterDialog(onDismiss: () -> Unit) {
    val scope = rememberCoroutineScope()
    var org by rememberSaveable { mutableStateOf("") }
    var name by rememberSaveable { mutableStateOf("") }
    var email by rememberSaveable { mutableStateOf("") }
    var password by rememberSaveable { mutableStateOf("") }
    var saving by rememberSaveable { mutableStateOf(false) }
    var error by rememberSaveable { mutableStateOf<String?>(null) }

    FormDialog(
        title = "Create account",
        onDismiss = onDismiss,
        canSave = org.isNotBlank() && name.isNotBlank() && email.isNotBlank() && password.length >= 8,
        saving = saving,
        error = error,
        saveLabel = "Create",
        onSave = {
            saving = true
            error = null
            scope.launch {
                // Success swaps the whole app to the signed-in UI, which closes this too.
                attempt { App.container.register(RegisterBody(org.trim(), name.trim(), email.trim(), password)) }
                    .onFailure { error = it.userMessage("Couldn’t create your account.") }
                saving = false
            }
        },
    ) {
        Column(Modifier.fillMaxWidth().padding(top = 8.dp), horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(12.dp)) {
            BrandTile(48.dp)
            Text(
                "Create your ${stringResource(R.string.app_name)} account",
                style = TextStyle(fontSize = 20.sp, fontWeight = FontWeight.SemiBold),
                textAlign = TextAlign.Center,
            )
        }
        FormCard {
            Field("Organisation name") { Input(org, { org = it }, placeholder = "e.g. Acme Ltd") }
            Field("Your name") { Input(name, { name = it }, placeholder = "Jane Doe") }
            Field("Email") {
                Input(email, { email = it }, placeholder = "you@example.com", keyboard = KeyboardOptions(keyboardType = KeyboardType.Email, autoCorrectEnabled = false))
            }
            Field("Password", hint = "At least 8 characters.") {
                Input(password, { password = it }, secret = true, keyboard = KeyboardOptions(keyboardType = KeyboardType.Password))
            }
        }
        val linkStyle = TextLinkStyles(SpanStyle(color = Theme.accent, fontWeight = FontWeight.Medium))
        Text(
            buildAnnotatedString {
                append("By creating an account you agree to the ")
                withLink(LinkAnnotation.Url("$base/terms", linkStyle)) { append("Terms of Service") }
                append(" and ")
                withLink(LinkAnnotation.Url("$base/privacy", linkStyle)) { append("Privacy Policy") }
                append(".")
            },
            Modifier.fillMaxWidth().padding(horizontal = 8.dp),
            style = MaterialTheme.typography.bodySmall.copy(color = Theme.muted, textAlign = TextAlign.Center),
        )
    }
}
