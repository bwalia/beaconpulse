// AGP 9 compiles Kotlin itself; these two add Compose and JSON. Kotlin 2.3.20 is the
// version AGP 9.0.1 is known to build with here (same pair as ../fishers).
plugins {
    id("com.android.application") version "9.0.1" apply false
    id("org.jetbrains.kotlin.plugin.compose") version "2.3.20" apply false
    id("org.jetbrains.kotlin.plugin.serialization") version "2.3.20" apply false
}
