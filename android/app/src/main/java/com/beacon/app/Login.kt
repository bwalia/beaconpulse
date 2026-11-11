package com.beacon.app

import android.content.Context
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
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
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.scale
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.colorResource
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.LinkAnnotation
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.withLink
import androidx.compose.ui.unit.dp
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

/** Sign-in: email/password and Continue with Google; sign-up opens over it. Mirrors iOS LoginView. */
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

    Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
        Column(
            Modifier.widthIn(max = 440.dp).fillMaxSize().verticalScroll(rememberScrollState())
                .systemBarsPadding().imePadding().padding(24.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp, Alignment.CenterVertically),
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            Box(Modifier.size(76.dp).clip(RoundedCornerShape(20.dp)).background(colorResource(R.color.brand))) {
                // The launcher art keeps a safe margin for icon masks; scaled up, the pulse fills the tile.
                Image(painterResource(R.drawable.ic_launcher_foreground), null, Modifier.size(76.dp).scale(1.5f))
            }
            Text(stringResource(R.string.app_name), style = MaterialTheme.typography.headlineMedium, fontWeight = FontWeight.Bold)
            Text("Sign in to your monitors", color = MaterialTheme.colorScheme.onSurfaceVariant)
            Spacer(Modifier.height(8.dp))

            Field(
                "Email", email, { email = it },
                keyboard = KeyboardOptions(keyboardType = KeyboardType.Email, imeAction = ImeAction.Next, autoCorrectEnabled = false),
            )
            androidx.compose.material3.OutlinedTextField(
                value = password,
                onValueChange = { password = it },
                label = { Text("Password") },
                singleLine = true,
                modifier = Modifier.fillMaxWidth(),
                visualTransformation = androidx.compose.ui.text.input.PasswordVisualTransformation(),
                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Password, imeAction = ImeAction.Done),
                keyboardActions = KeyboardActions(onDone = { signIn() }),
            )
            // The reset email links to the website anyway, so the flow lives there.
            TextButton(onClick = { openUrl(context, "$base/forgot-password") }, modifier = Modifier.align(Alignment.End)) {
                Text("Forgot password?")
            }
            error?.let { Text(it, color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodyMedium, modifier = Modifier.fillMaxWidth()) }

            Button(
                onClick = signIn,
                enabled = !busy && email.isNotBlank() && password.isNotEmpty(),
                modifier = Modifier.fillMaxWidth().height(52.dp),
            ) {
                if (busy) CircularProgressIndicator(Modifier.size(22.dp), strokeWidth = 2.dp) else Text("Sign in")
            }

            Row(verticalAlignment = Alignment.CenterVertically, modifier = Modifier.padding(vertical = 4.dp)) {
                HorizontalDivider(Modifier.weight(1f))
                Text("or", Modifier.padding(horizontal = 12.dp), color = MaterialTheme.colorScheme.onSurfaceVariant)
                HorizontalDivider(Modifier.weight(1f))
            }

            OutlinedButton(
                onClick = {
                    submit("Google sign-in failed. Please try again.") {
                        val token = googleIdToken(context) ?: return@submit
                        app.signInWithGoogle(token)
                    }
                },
                enabled = !busy,
                modifier = Modifier.fillMaxWidth().height(52.dp),
            ) {
                Image(painterResource(R.drawable.ic_google), null, Modifier.size(20.dp))
                Spacer(Modifier.size(10.dp))
                Text("Continue with Google")
            }

            TextButton(onClick = { signingUp = true }, enabled = !busy) { Text("New here? Create an account") }
        }
    }

    if (signingUp) RegisterDialog(onDismiss = { signingUp = false })
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
        Text(
            "Create your ${stringResource(R.string.app_name)} account",
            style = MaterialTheme.typography.titleMedium,
            textAlign = TextAlign.Center,
            modifier = Modifier.fillMaxWidth(),
        )
        Field("Organisation name", org, { org = it })
        Field("Your name", name, { name = it })
        Field("Email", email, { email = it }, keyboard = KeyboardOptions(keyboardType = KeyboardType.Email, autoCorrectEnabled = false))
        SecretField("Password (at least 8 characters)", password, { password = it }, KeyboardOptions(keyboardType = KeyboardType.Password))
        Text(
            buildAnnotatedString {
                append("By creating an account you agree to the ")
                withLink(LinkAnnotation.Url("$base/terms")) { append("Terms of Service") }
                append(" and ")
                withLink(LinkAnnotation.Url("$base/privacy")) { append("Privacy Policy") }
                append(".")
            },
            style = MaterialTheme.typography.bodySmall.copy(color = MaterialTheme.colorScheme.onSurfaceVariant),
        )
    }
}
