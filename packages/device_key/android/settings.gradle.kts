// SPDX-License-Identifier: Apache-2.0
//
// The standalone build of the plugin's Android folder (brief M5-01). It exists for the core's
// tests: `./gradlew :devicekeycore:testDebugUnitTest` (JVM) and
// `./gradlew :devicekeycore:connectedDebugAndroidTest` (device or emulator). An app never reads
// this file: Flutter includes the plugin through `build.gradle.kts` alone.

pluginManagement {
    repositories {
        google()
        mavenCentral()
        gradlePluginPortal()
    }
    // The version an app's build supplies when it includes the plugin; keep equal to the
    // buildscript classpath of build.gradle.kts.
    plugins {
        id("com.android.library") version "9.1.0"
    }
}

// Dependency locking of this standalone build (SonarQube rule S8569,
// docs/decisions.d/SONAR-MAIN-GATE.md D-SONAR-MAIN-GATE-2): every configuration of both projects
// resolves the versions in their gradle.lockfile, the buildscript classpath of build.gradle.kts
// those in buildscript-gradle.lockfile. Set here and not in build.gradle.kts, because an app reads
// that file and resolves the plugin against its own dependencies. After a change of a dependency
// or of the Android Gradle plugin run in this folder:
// ./gradlew dependencies buildEnvironment :devicekeycore:dependencies --write-locks
gradle.beforeProject {
    dependencyLocking {
        lockAllConfigurations()
    }
    buildscript.configurations.configureEach {
        resolutionStrategy.activateDependencyLocking()
    }
}

rootProject.name = "device_key"

include(":devicekeycore")
