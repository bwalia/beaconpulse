plugins {
    id("com.android.application")
}

android {
    namespace = "com.beacon.twa"
    compileSdk = 36

    defaultConfig {
        minSdk = 26
        targetSdk = 36
        // CI passes its run number, so every upload to Play has a higher code.
        versionCode = (findProperty("versionCode") as String?)?.toInt() ?: 1
        versionName = (findProperty("versionName") as String?) ?: "1.0.0"
    }

    buildFeatures {
        resValues = true
    }

    // One flavor per brand: its id and website here, its icon and splash colour in
    // app/src/<brand>/res. The app opens https://<host>, which must serve
    // /.well-known/assetlinks.json for this package (frontend/src/brand/<brand>.ts → android).
    flavorDimensions += "brand"
    productFlavors {
        create("sysops") {
            dimension = "brand"
            applicationId = "com.sysops247.app"
            resValue("string", "app_name", "SysOps 24/7")
            site("sysops247.com")
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
}

/** Points the flavor at its website: the launch URL and the app → site half of the link check. */
fun com.android.build.api.dsl.ApplicationProductFlavor.site(host: String) {
    manifestPlaceholders["host"] = host
    resValue(
        "string", "asset_statements",
        """[{\"relation\":[\"delegate_permission/common.handle_all_urls\"],\"target\":{\"namespace\":\"web\",\"site\":\"https://$host\"}}]""",
    )
}

dependencies {
    implementation("com.google.androidbrowserhelper:androidbrowserhelper:2.7.4")
}
