// SPDX-License-Identifier: Apache-2.0

package com.example.client

import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context
import io.flutter.plugin.common.BinaryMessenger
import io.flutter.plugin.common.MethodChannel

/**
 * The clipboard of the block copies over the method channel `bothy/clipboard` (docs/spec/
 * 01-product.md 6.12 item 4, brief M4-48; `PlatformRichClipboard` in
 * lib/app/rich_clipboard.dart): `write` puts one clip with the plain text and the HTML form of
 * a quote on the primary clipboard, so a rich editor pastes the formatting and every other app
 * the text.
 */
class RichClipboardChannel(messenger: BinaryMessenger, context: Context) {
    private val clipboard = context.getSystemService(ClipboardManager::class.java)

    init {
        MethodChannel(messenger, NAME).setMethodCallHandler { call, result ->
            if (call.method != "write") {
                result.notImplemented()
                return@setMethodCallHandler
            }
            val text = call.argument<String>("text")
            val html = call.argument<String>("html")
            if (text == null || html == null || clipboard == null) {
                result.error("clipboard", "text and html expected", null)
                return@setMethodCallHandler
            }
            clipboard.setPrimaryClip(ClipData.newHtmlText(LABEL, text, html))
            result.success(null)
        }
    }

    private companion object {
        const val NAME = "bothy/clipboard"
        const val LABEL = "text"
    }
}
