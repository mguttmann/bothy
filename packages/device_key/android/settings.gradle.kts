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

rootProject.name = "device_key"

include(":devicekeycore")
