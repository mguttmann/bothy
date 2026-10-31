// SPDX-License-Identifier: Apache-2.0

package com.example.client

import android.app.Activity
import io.flutter.plugin.common.BinaryMessenger

/**
 * The text input probe of the phone integration tests exists only in debug builds
 * (src/debug/kotlin); in a profile or release build it registers nothing.
 */
object TextInputProbe {
    /** Does nothing: no channel outside debug builds. */
    @Suppress("UNUSED_PARAMETER")
    fun attach(messenger: BinaryMessenger, activity: Activity) = Unit
}
