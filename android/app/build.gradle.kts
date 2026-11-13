plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.plugin.compose")
    id("org.jetbrains.kotlin.plugin.serialization")
}

android {
    namespace = "com.beacon.app"
    compileSdk = 36

    defaultConfig {
        minSdk = 26
        targetSdk = 36
        // CI passes its run number, so every upload to Play has a higher code.
        versionCode = (findProperty("versionCode") as String?)?.toInt() ?: 1
        versionName = (findProperty("versionName") as String?) ?: "1.0.0"
    }

    buildFeatures {
        compose = true
        buildConfig = true
        resValues = true
    }

    // One flavor per brand, the Android twin of ios/Brands/<Brand>/brand.xcconfig: its
    // id, name, server and sign-in/push ids here, its icon in app/src/<brand>/res.
    flavorDimensions += "brand"
    productFlavors {
        create("sysops") {
            dimension = "brand"
            applicationId = "com.sysops247.app"
            brand(
                name = "SysOps 24/7",
                apiBaseUrl = "https://sysops247.com",
                accent = "#059669",
                // Google Cloud project sysops-24-7 (also its Firebase project). The *web*
                // OAuth client there: Android's Google sign-in returns an ID token issued to
                // it, so it must be in the API's BEACON_GOOGLE_CLIENT_ID. Its Android clients
                // (package + SHA-1 of the upload and Play signing keys) live in the same project.
                googleServerClientId = "295233398456-mi4a9ssp0ap38t9m9r210kvov0ftench.apps.googleusercontent.com",
                // From Firebase console → Project settings → the Android app. Blank = no push.
                firebaseAppId = "1:295233398456:android:32771b797e85fe02a1764d",
                firebaseApiKey = "AIzaSyC6YemBywL5w-74PFQUW-RKs7ofeRo37kU",
                firebaseProjectId = "sysops-24-7",
                firebaseSenderId = "295233398456",
            )
        }
    }

    // The Play upload key, from CI secrets. Without it the release bundle is unsigned.
    System.getenv("ANDROID_KEYSTORE_FILE")?.let { path ->
        val upload = signingConfigs.create("upload") {
            storeFile = file(path)
            storePassword = System.getenv("ANDROID_KEYSTORE_PASSWORD")
            keyAlias = System.getenv("ANDROID_KEY_ALIAS") ?: "upload"
            keyPassword = System.getenv("ANDROID_KEYSTORE_PASSWORD")
        }
        buildTypes.getByName("release").signingConfig = upload
    }

    buildTypes {
        release {
            isMinifyEnabled = true
            isShrinkResources = true
            proguardFiles(getDefaultProguardFile("proguard-android-optimize.txt"), "proguard-rules.pro")
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
    kotlin {
        compilerOptions { jvmTarget.set(org.jetbrains.kotlin.gradle.dsl.JvmTarget.JVM_17) }
    }
    packaging {
        resources.excludes += "/META-INF/{AL2.0,LGPL2.1}"
    }
}

fun com.android.build.api.dsl.ApplicationProductFlavor.brand(
    name: String, apiBaseUrl: String, accent: String, googleServerClientId: String,
    firebaseAppId: String, firebaseApiKey: String, firebaseProjectId: String, firebaseSenderId: String,
) {
    resValue("string", "app_name", name)
    resValue("color", "brand", accent)
    fun field(key: String, value: String) = buildConfigField("String", key, "\"$value\"")
    field("API_BASE_URL", apiBaseUrl)
    field("GOOGLE_SERVER_CLIENT_ID", googleServerClientId)
    field("FIREBASE_APP_ID", firebaseAppId)
    field("FIREBASE_API_KEY", firebaseApiKey)
    field("FIREBASE_PROJECT_ID", firebaseProjectId)
    field("FIREBASE_SENDER_ID", firebaseSenderId)
}

dependencies {
    implementation("androidx.core:core-ktx:1.15.0")
    implementation("androidx.activity:activity-compose:1.9.3")
    implementation("androidx.lifecycle:lifecycle-viewmodel-compose:2.8.7")
    implementation("androidx.lifecycle:lifecycle-runtime-compose:2.8.7")
    implementation("androidx.navigation:navigation-compose:2.8.5")
    implementation(platform("androidx.compose:compose-bom:2024.12.01"))
    implementation("androidx.compose.ui:ui")
    implementation("androidx.compose.material3:material3")
    implementation("androidx.compose.material:material-icons-extended")
    implementation("androidx.compose.ui:ui-tooling-preview")
    debugImplementation("androidx.compose.ui:ui-tooling")

    implementation("com.squareup.okhttp3:okhttp:4.12.0")
    implementation("org.jetbrains.kotlinx:kotlinx-serialization-json:1.7.3")
    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-android:1.9.0")

    // Continue with Google: Credential Manager shows the account chooser and hands back
    // a Google ID token for the same /api/v1/auth/google the website and iOS use.
    implementation("androidx.credentials:credentials:1.5.0")
    implementation("androidx.credentials:credentials-play-services-auth:1.5.0")
    implementation("com.google.android.libraries.identity.googleid:googleid:1.1.1")

    // Alert push. Configured in code from the flavor's ids (no google-services.json).
    implementation("com.google.firebase:firebase-messaging:24.1.0")

    testImplementation("junit:junit:4.13.2")
    testImplementation("com.squareup.okhttp3:mockwebserver:4.12.0")
    testImplementation("org.jetbrains.kotlinx:kotlinx-coroutines-test:1.9.0")
}
