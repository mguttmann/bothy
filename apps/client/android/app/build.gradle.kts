// SPDX-License-Identifier: Apache-2.0
//
// The Android app (brief M5-03 Design note 1; docs/decisions.d/M5-03.md D-M5-03-1, D-M5-03-2).
// The application ID, the label, the URL scheme and the App Links host come from
// ../branding.properties, which tool/branding_defines.dart writes from the build branding
// (neutral example.com values in the checked-in file).

import java.util.Properties

plugins {
    id("com.android.application")
    // The Flutter Gradle Plugin must be applied after the Android and Kotlin Gradle plugins.
    id("dev.flutter.flutter-gradle-plugin")
}

// Dependency locking (docs/decisions.d/M5-03.md D-M5-03-8): the resolved versions of the app
// stay in ../gradle.lockfile, next to the settings of the build; after a change of a dependency
// or of the Flutter version run `./gradlew :app:dependencies --write-locks` in android/.
dependencyLocking {
    lockAllConfigurations()
    lockFile = rootProject.file("gradle.lockfile")
    // The engine artifacts follow the pinned Flutter version, and a build resolves only those of
    // its target ABIs, which a lock of all ABIs would refuse.
    ignoredDependencies.add("io.flutter:*")
}

val branding =
    Properties().apply {
        rootProject.file("branding.properties").inputStream().use { load(it) }
    }

fun brandingValue(key: String): String =
    requireNotNull(branding.getProperty(key)) { "branding.properties lacks $key" }

val appLinksHost = brandingValue("appLinksHost")

android {
    // The Kotlin package of the runner; the application ID below is the build's own.
    namespace = "com.example.client"
    compileSdk = flutter.compileSdkVersion
    ndkVersion = flutter.ndkVersion

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
        // flutter_local_notifications uses java.time on API 26.
        isCoreLibraryDesugaringEnabled = true
    }

    defaultConfig {
        applicationId = brandingValue("applicationId")
        // 15 2: the Android minimum is API 26.
        minSdk = 26
        targetSdk = flutter.targetSdkVersion
        versionCode = flutter.versionCode
        versionName = flutter.versionName
        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"
        manifestPlaceholders +=
            mapOf(
                "appName" to brandingValue("appName"),
                "urlScheme" to brandingValue("urlScheme"),
                // The OIDC redirect `<application ID>:/oidc/callback` (08 2.2, RFC 8252 7.1).
                "redirectScheme" to brandingValue("applicationId"),
                // Without a host the App Links alias stays disabled and nothing is verified;
                // the placeholder host is a reserved name that never resolves (RFC 2606).
                "appLinksEnabled" to appLinksHost.isNotEmpty().toString(),
                "appLinksHost" to appLinksHost.ifEmpty { "applinks.invalid" },
            )
    }

    buildTypes {
        release {
            // Signing comes with the distribution of M7; until then a release build is signed
            // with the debug key so `flutter build apk --release` works on a developer machine.
            signingConfig = signingConfigs.getByName("debug")
            // R8 shrinks and obfuscates the release build (D-M5-03-8).
            isMinifyEnabled = true
            isShrinkResources = true
            proguardFiles(
                getDefaultProguardFile("proguard-android-optimize.txt"),
                "proguard-rules.pro",
            )
        }
    }
}

kotlin {
    compilerOptions {
        jvmTarget = org.jetbrains.kotlin.gradle.dsl.JvmTarget.JVM_17
    }
}

flutter {
    source = "../.."
}

dependencies {
    coreLibraryDesugaring("com.android.tools:desugar_jdk_libs:2.1.5")
    // Custom Tabs for the OIDC login (15 8).
    implementation("androidx.browser:browser:1.9.0")
    // Passkeys through the Credential Manager's platform provider (API 34 and later, D-M5-03-3).
    implementation("androidx.credentials:credentials:1.5.0")
    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-android:1.10.2")

    // The version the integration_test plugin brings into the app (Gradle's consistent
    // resolution holds the test APK to it); its runner has everything the tests use.
    androidTestImplementation("androidx.test:runner:1.3.0")
}
