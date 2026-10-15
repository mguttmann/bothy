// SPDX-License-Identifier: Apache-2.0
//
// The Kotlin core of the device keys (brief M5-01): Android Keystore, StrongBox, BiometricPrompt.
// JVM tests in src/test run with the in-memory KeyStoreAccess; instrumented tests in
// src/androidTest run on a device or emulator (android/run_instrumented_tests.sh).

plugins {
    id("com.android.library")
}

android {
    namespace = "devicekey.core"

    compileSdk = 36

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    defaultConfig {
        // 15 2: the Android minimum is API 26.
        minSdk = 26
        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"
    }

    testOptions {
        unitTests.all {
            it.testLogging {
                events("passed", "skipped", "failed")
                exceptionFormat = org.gradle.api.tasks.testing.logging.TestExceptionFormat.FULL
            }
        }
    }
}

kotlin {
    compilerOptions {
        jvmTarget = org.jetbrains.kotlin.gradle.dsl.JvmTarget.JVM_17
    }
}

dependencies {
    // Keep equal to ../build.gradle.kts, which compiles these sources into the plugin.
    implementation("androidx.biometric:biometric:1.1.0")

    testImplementation("junit:junit:4.13.2")
    testImplementation("org.jetbrains.kotlin:kotlin-test-junit")

    androidTestImplementation("androidx.test:runner:1.6.2")
    androidTestImplementation("androidx.test.ext:junit:1.2.1")
}
