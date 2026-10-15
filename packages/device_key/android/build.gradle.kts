// SPDX-License-Identifier: Apache-2.0
//
// The Android part of the Flutter plugin `device_key` (brief M5-01). Flutter includes this
// project in an app as `:device_key`. The key code lives in the module `devicekeycore`; this
// library compiles the core's sources into itself, so an app that depends on the plugin (the
// push service of M5-05, the share activity of M5-06) uses the same classes without including a
// second Gradle project (docs/DECISIONS.md D-M5-01-6).

group = "devicekey.flutter"
version = "1.0-SNAPSHOT"

buildscript {
    val kotlinVersion = "2.4.0"
    repositories {
        google()
        mavenCentral()
    }

    dependencies {
        classpath("com.android.tools.build:gradle:9.1.0")
        classpath("org.jetbrains.kotlin:kotlin-gradle-plugin:$kotlinVersion")
    }
}

allprojects {
    repositories {
        google()
        mavenCentral()
    }
}

plugins {
    id("com.android.library")
}

android {
    namespace = "devicekey.flutter"

    compileSdk = 36

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    sourceSets {
        getByName("main") {
            java.srcDirs("src/main/kotlin", "devicekeycore/src/main/kotlin")
        }
    }

    defaultConfig {
        // 15 2: the Android minimum is API 26.
        minSdk = 26
    }
}

kotlin {
    compilerOptions {
        jvmTarget = org.jetbrains.kotlin.gradle.dsl.JvmTarget.JVM_17
    }
}

dependencies {
    // The generated Pigeon code runs the asynchronous `sign` in a coroutine.
    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-android:1.10.2")
    // Keep equal to devicekeycore/build.gradle.kts: these are the core's dependencies.
    implementation("androidx.biometric:biometric:1.1.0")
}
