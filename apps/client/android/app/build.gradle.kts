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
//
// Device E2E (brief M5-11 Design notes 2 and 6): two Gradle properties change the debug build
// only (see "E2E switches" below); release and profile builds never contain anything of them.

import com.android.build.api.artifact.ScopedArtifact
import com.android.build.api.artifact.SingleArtifact
import com.android.build.api.variant.ScopedArtifacts
import java.util.Properties
import java.util.zip.ZipFile

plugins {
    id("com.android.application")
    // The Flutter Gradle Plugin must be applied after the Android and Kotlin Gradle plugins.
    id("dev.flutter.flutter-gradle-plugin")
}

val firebaseConfigured = file("google-services.json").isFile

if (firebaseConfigured) {
    apply(plugin = "com.google.gms.google-services")
}

// E2E switches (brief M5-11; set by tool/mobile_e2e_suite.sh as ORG_GRADLE_PROJECT_<name>).
// Only the debug variant changes, through its own sources, manifests and generated resources;
// without the properties the debug build is the same as before. checkReleaseE2eAbsent and
// checkProfileE2eAbsent (below) hold every other build free of them.
//
// bothyE2ePush=true: the push backend of src/e2epush (a fake FCM token per install, so the app
// asks for the permission and registers provider fcm) and the receiver through which the
// runner hands a push captured by the fake FCM to the push service. It replaces the backend, so
// it cannot be combined with a Firebase configuration.
val e2ePush =
    when (val value = findProperty("bothyE2ePush")?.toString().orEmpty()) {
        "", "false" -> false
        "true" -> true
        else -> throw GradleException("bothyE2ePush must be true or false, not \"$value\"")
    }
if (e2ePush && firebaseConfigured) {
    throw GradleException(
        "bothyE2ePush=true replaces the push backend of the debug build with the E2E one and " +
            "cannot be combined with app/google-services.json; remove one of them",
    )
}

// bothyE2eCa=<absolute path of a PEM file>: the debug build trusts this CA, the test CA of the
// run's TLS front, through a network security config whose debug-overrides add it (generated
// under build/), so the native preview fetch reaches https://localhost:<port> of the stack.
val e2eCa: File? =
    findProperty("bothyE2eCa")?.toString()?.takeIf { it.isNotBlank() }?.let { path ->
        File(path).takeIf { it.isAbsolute && it.isFile }
            ?: throw GradleException("bothyE2eCa must be the absolute path of a PEM file, not \"$path\"")
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
        // The push backend: the Firebase service and token source, or the stand-in without
        // push (its token source in src/nofirebase/kotlin, its disabled service, which the
        // manifest names, in src/nofirebase/service); both define the same `PushBackend` and
        // `PushMessagingService`, so the rest of the app is the same.
        val backend = if (firebaseConfigured) "src/firebase/kotlin" else "src/nofirebase/kotlin"
        if (e2ePush) {
            // Never with Firebase: every build keeps the disabled stand-in service. The debug
            // build gets the E2E backend and its receiver instead of the stand-in token source,
            // which the unit tests (debug only) then find in the app; every other build type
            // keeps the stand-in.
            getByName("main") { java.srcDir("src/nofirebase/service") }
            getByName("debug") { java.srcDirs("src/e2epush/kotlin", "src/e2epush/shared") }
            buildTypes.names.filter { it != "debug" }.forEach { buildType ->
                getByName(buildType) { java.srcDir(backend) }
            }
        } else {
            getByName("main") {
                java.srcDir(backend)
                if (!firebaseConfigured) java.srcDir("src/nofirebase/service")
            }
            // The JVM tests of the E2E receiver and token store compile those classes
            // themselves; the app has none of them.
            getByName("test") { java.srcDir("src/e2epush/shared") }
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

androidComponents {
    // The E2E switches reach only the debug variant: a source set has one manifest (src/debug
    // has its own), so their manifests and the generated resources join through the variant.
    onVariants(selector().withBuildType("debug")) { variant ->
        if (e2ePush) variant.sources.manifests.addStaticManifestFile(file("src/e2epush/AndroidManifest.xml").path)
        if (e2eCa != null) {
            val resources =
                tasks.register<E2eCaResources>("generate${variant.name.replaceFirstChar(Char::uppercase)}E2eCaResources") {
                    ca.set(e2eCa)
                }
            variant.sources.res?.addGeneratedSourceDirectory(resources, E2eCaResources::outputDirectory)
            variant.sources.manifests.addStaticManifestFile(file("src/e2eca/AndroidManifest.xml").path)
        }
    }
    // Every other build checks its merged manifest and its own classes before it is assembled,
    // so a release or profile artifact can never carry the E2E code or the CA trust.
    onVariants(selector().all()) { variant ->
        if (variant.buildType == "debug") return@onVariants
        val variantName = variant.name.replaceFirstChar(Char::uppercase)
        val check =
            tasks.register<E2eAbsentCheck>("check${variantName}E2eAbsent") {
                mergedManifest.set(variant.artifacts.get(SingleArtifact.MERGED_MANIFEST))
            }
        variant.artifacts
            .forScope(ScopedArtifacts.Scope.PROJECT)
            .use(check)
            .toGet(ScopedArtifact.CLASSES, E2eAbsentCheck::classJars, E2eAbsentCheck::classDirectories)
        tasks.named("check") { dependsOn(check) }
        tasks.configureEach {
            if (name == "assemble$variantName" || name == "bundle$variantName") dependsOn(check)
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

    constraints {
        // patrol (the runner of the device tests, brief M5-06) brings OkHttp 2 with Okio 1 into
        // the debug app, and consistent resolution holds the test APK to it, where okhttp-tls of
        // the push tests needs Okio 3. Okio 3 keeps the binary API of Okio 1 for Java callers.
        debugImplementation("com.squareup.okio:okio:3.6.0")
    }
}

/**
 * Writes the resources of `bothyE2eCa` for the debug variant (brief M5-11 Design note 6): the
 * PEM as `raw/bothy_e2e_ca.pem` and `xml/bothy_e2e_network_security_config.xml`, which keeps the
 * platform default of the app (system CAs only, no clear text) and adds the CA in
 * `debug-overrides`, which Android applies only to a debuggable app.
 *
 * @throws GradleException when the file is not a PEM certificate.
 */
abstract class E2eCaResources : DefaultTask() {
    /** The PEM file of the run's test CA. */
    @get:InputFile
    @get:PathSensitive(PathSensitivity.NONE)
    abstract val ca: RegularFileProperty

    /** The generated resource directory; the variant API places it under build/. */
    @get:OutputDirectory
    abstract val outputDirectory: DirectoryProperty

    /** Writes both resources, replacing those of an earlier CA. */
    @TaskAction
    fun write() {
        val pem = ca.get().asFile.readText()
        if (!pem.contains("-----BEGIN CERTIFICATE-----")) {
            throw GradleException("bothyE2eCa is not a PEM certificate: ${ca.get().asFile}")
        }
        val root = outputDirectory.get().asFile
        root.deleteRecursively()
        File(root, "raw").mkdirs()
        File(root, "raw/bothy_e2e_ca.pem").writeText(pem)
        File(root, "xml").mkdirs()
        File(root, "xml/bothy_e2e_network_security_config.xml").writeText(
            """
            |<?xml version="1.0" encoding="utf-8"?>
            |<!-- Generated by app/build.gradle.kts from bothyE2eCa (brief M5-11); debug builds only. -->
            |<network-security-config>
            |    <base-config cleartextTrafficPermitted="false">
            |        <trust-anchors>
            |            <certificates src="system" />
            |        </trust-anchors>
            |    </base-config>
            |    <debug-overrides>
            |        <trust-anchors>
            |            <certificates src="@raw/bothy_e2e_ca" />
            |        </trust-anchors>
            |    </debug-overrides>
            |</network-security-config>
            |
            """.trimMargin(),
        )
    }
}

/**
 * Fails when a non-debug variant carries anything of the E2E switches (brief M5-11): the
 * receiver, provider or action of `bothyE2ePush` or the network security config of
 * `bothyE2eCa` in its merged manifest, or an `E2ePush*` class among its own classes.
 *
 * @throws GradleException naming what it found.
 */
abstract class E2eAbsentCheck : DefaultTask() {
    /** The merged manifest of the variant. */
    @get:InputFile
    @get:PathSensitive(PathSensitivity.NONE)
    abstract val mergedManifest: RegularFileProperty

    /** The class jars of the app module (not its libraries). */
    @get:InputFiles
    @get:PathSensitive(PathSensitivity.RELATIVE)
    abstract val classJars: ListProperty<RegularFile>

    /** The class directories of the app module. */
    @get:InputFiles
    @get:PathSensitive(PathSensitivity.RELATIVE)
    abstract val classDirectories: ListProperty<Directory>

    /** Reads the manifest and the class names. */
    @TaskAction
    fun verify() {
        val found = mutableListOf<String>()
        val manifest = mergedManifest.get().asFile.readText()
        listOf("E2ePush", "E2E_PUSH", "bothy_e2e").filter(manifest::contains).forEach { found += "manifest: $it" }
        val e2eClass = Regex("""(^|/)E2ePush[^/]*\.class$""")
        for (directory in classDirectories.get().map { it.asFile }) {
            directory.walk().filter { it.isFile && e2eClass.containsMatchIn(it.invariantSeparatorsPath) }
                .forEach { found += "class: ${it.relativeTo(directory).invariantSeparatorsPath}" }
        }
        for (jar in classJars.get().map { it.asFile }) {
            ZipFile(jar).use { zip ->
                zip.entries().asSequence().map { it.name }.filter(e2eClass::containsMatchIn).forEach { found += "class: $it" }
            }
        }
        if (found.isNotEmpty()) {
            throw GradleException("$name: this build carries the debug-only E2E code: ${found.joinToString()}")
        }
    }
}
