// SPDX-License-Identifier: Apache-2.0

package com.example.client

import io.flutter.plugin.common.BinaryMessenger
import io.flutter.plugin.common.MethodChannel

/**
 * The method channel `bothy/deep_links` of the runners (15 11; `MethodChannelDeepLinks` in
 * lib/app/deep_links.dart): `takePending` returns the links that arrived before the app asked,
 * and every later link goes to the app with `open`.
 */
class DeepLinkChannel(messenger: BinaryMessenger) {
    private val channel = MethodChannel(messenger, NAME)
    private val pending = mutableListOf<String>()
    private var appListens = false

    init {
        channel.setMethodCallHandler { call, result ->
            if (call.method == "takePending") {
                appListens = true
                result.success(pending.toList())
                pending.clear()
            } else {
                result.notImplemented()
            }
        }
    }

    /** Hands [link] to the app, or keeps it until the app takes the pending links. */
    fun open(link: String) {
        if (appListens) channel.invokeMethod("open", link) else pending.add(link)
    }

    private companion object {
        const val NAME = "bothy/deep_links"
    }
}
