// SPDX-License-Identifier: Apache-2.0
//
// The Android app (brief M5-03 Design note 1; docs/decisions.d/M5-03.md D-M5-03-1, D-M5-03-2).
// The application ID, the label, the URL scheme and the App Links host come from
// ../branding.properties, which tool/branding_defines.dart writes from the build branding
// (neutral example.com values in the checked-in file).
//
// Push (brief M5-05; docs/decisions.d/M5-05.md D-M5-05-5, ADR 0072): only a build whose
// configuration provides app/google-services.json (the Firebase project of the instance, never
// tracked) gets Firebase Cloud Messaging: the Google services plugin, the Firebase messaging
// library, the service of src/firebase (enabled in the manifest) and its own lock file. Without it the app builds from
// src/nofirebase, links no Firebase or Play services code and has no push.

import java.util.Properties

plugins {
    id("com.android.application")
    // The Flutter Gradle Plugin must be applied after the Android and Kotlin Gradle plugins.
    id("dev.flutter.flutter-gradle-plugin")
}

val firebaseConfigured = file("google-services.json").isFile

if (firebaseConfigured) {
    apply(plugin = "com.google.gms.google-services")
}

// Dependency locking (docs/decisions.d/M5-03.md D-M5-03-8): the resolved versions of the app
// stay in ../gradle.lockfile, next to the settings of the build; after a change of a dependency
// or of the Flutter version run `./gradlew :app:dependencies --write-locks` in android/.
//
// A build with Firebase resolves more libraries, so it keeps its own lock file
// (gradle-firebase.lockfile, D-M5-05-5); write it with a google-services.json in place.
dependencyLocking {
    lockAllConfigurations()
    lockFile = rootProject.file(if (firebaseConfigured) "gradle-firebase.lockfile" else "gradle.lockfile")
    // The engine artifacts follow the pinned Flutter version, and a build resolves only those of
    // its target ABIs, which a lock of all ABIs would refuse.
    ignoredDependencies.add("io.flutter:*")
    // An empty artifact of the Kotlin standard library since Kotlin 1.9.20, aligned to
    // kotlin-stdlib, which stays locked. In the build with Firebase the instrumented test run
    // brings it into the app's runtime classpath through the test APK's consistent resolution,
    // which `--write-locks` does not see, so its lock entry could never match both
    // (D-M5-05-5).
    ignoredDependencies.add("org.jetbrains.kotlin:kotlin-stdlib-common")
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
        // The runner of patrol (dev dependency, brief M5-06): it runs the device tests of
        // integration_test/share through MainActivityTest and the plain instrumented tests of
        // androidTest like AndroidJUnitRunner, which it extends.
        testInstrumentationRunner = "pl.leancode.patrol.PatrolJUnitRunner"
        // `patrol test` builds the app with its bundle of Dart tests (-Ptarget=.../test_bundle.dart);
        // that run executes only the bridge, whose app has no MainActivity of its own to test.
        if ((findProperty("target") as String?)?.endsWith("test_bundle.dart") == true) {
            testInstrumentationRunnerArguments["class"] = "com.example.client.MainActivityTest"
        }
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
                // Without a Firebase configuration the push service stays disabled (D-M5-05-5).
                "pushEnabled" to firebaseConfigured.toString(),
            )
    }

    sourceSets {
        getByName("main") {
            // The push backend: the Firebase service and token source, or the stand-in without
            // push; both define the same `PushBackend`, so the rest of the app is the same.
            java.srcDir(if (firebaseConfigured) "src/firebase/kotlin" else "src/nofirebase/kotlin")
        }
        getByName("androidTest") {
            if (firebaseConfigured) java.srcDir("src/androidTestFirebase/kotlin")
        }
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
    // The expedited preview fetch of a push (15 10.2; brief M5-05 Design note 2).
    implementation("androidx.work:work-runtime:2.12.0")
    if (firebaseConfigured) {
        // FirebaseMessagingService and the FCM token (13 5.2; ADR 0072).
        implementation("com.google.firebase:firebase-messaging:26.0.1")
    }

    testImplementation("junit:junit:4.12")

    // The version the integration_test plugin brings into the app (Gradle's consistent
    // resolution holds the test APK to it); its runner has everything the tests use.
    androidTestImplementation("androidx.test:runner:1.3.0")
    // The WorkManager test driver and a TLS fake server with a per-run test CA (brief M5-05 Tests).
    androidTestImplementation("androidx.work:work-testing:2.12.0")
    // OkHttp 4: the 5.x Android artifacts need compile SDK 37.
    androidTestImplementation("com.squareup.okhttp3:mockwebserver:4.12.0")
    androidTestImplementation("com.squareup.okhttp3:okhttp-tls:4.12.0")
    // The Accessibility Test Framework of the accessibility checks (brief M5-10): only its
    // hierarchy checks, which AccessibilityChecksTest runs on the tree TalkBack reads. Its
    // Espresso integration and the test libraries it names stay out, so the test APK keeps the
    // versions above.
    androidTestImplementation(
        "com.google.android.apps.common.testing.accessibility.framework:accessibility-test-framework:4.1.1",
    ) {
        exclude(group = "androidx.test")
        exclude(group = "androidx.test.espresso")
        exclude(group = "androidx.test.services")
        exclude(group = "com.google.android.material")
    }
    // Its API returns Guava collections with checker annotations, which it names for the runtime
    // only; the versions the app already resolves.
    androidTestImplementation("com.google.guava:guava:33.3.1-android")
    androidTestImplementation("org.checkerframework:checker-qual:3.43.0")

    constraints {
        // patrol (the runner of the device tests, brief M5-06) brings OkHttp 2 with Okio 1 into
        // the debug app, and consistent resolution holds the test APK to it, where okhttp-tls of
        // the push tests needs Okio 3. Okio 3 keeps the binary API of Okio 1 for Java callers.
        debugImplementation("com.squareup.okio:okio:3.6.0")
        // The Accessibility Test Framework names versions with known advisories
        // (jsoup 1.15.1, protobuf-javalite 3.19.1); the test APK takes the fixed ones.
        androidTestImplementation("org.jsoup:jsoup:1.23.2")
        androidTestImplementation("com.google.protobuf:protobuf-javalite:3.25.9")
    }
}
