// SPDX-License-Identifier: Apache-2.0

package com.example.client

import android.app.Activity
import android.content.ClipData
import android.content.ClipboardManager
import android.view.View
import android.view.ViewGroup
import android.view.inputmethod.EditorInfo
import android.view.inputmethod.ExtractedTextRequest
import android.view.inputmethod.InputConnection
import io.flutter.embedding.android.FlutterView
import io.flutter.plugin.common.BinaryMessenger
import io.flutter.plugin.common.MethodChannel

/**
 * The text input probe of the phone integration tests (DICTATION-MOBILE), in debug builds only;
 * every other build type has the no-op of src/probeoff.
 *
 * The method channel `bothy/text_input_probe` drives the focused Flutter text field through an
 * [InputConnection] of the [FlutterView], as an input method does: `commitText` (a whole
 * transcript), `compose` (voice typing: `setComposingText` for each partial result, then
 * `commitText` of the final one or `finishComposingText`) and `paste` (the text on the primary
 * clipboard, then `performContextMenuAction(android.R.id.paste)`, an input method's clipboard).
 * `text` changes nothing. Every call answers the text of the field as the connection holds it
 * afterwards; without a text input client it fails with `no_input`.
 *
 * Example (Dart): `MethodChannel('bothy/text_input_probe').invokeMethod('commitText', {'text': 'Hi'})`.
 */
object TextInputProbe {
    private const val NAME = "bothy/text_input_probe"

    /** Connects the channel to [messenger]; [activity] holds the Flutter view. */
    fun attach(messenger: BinaryMessenger, activity: Activity) {
        MethodChannel(messenger, NAME).setMethodCallHandler { call, result ->
            val view = flutterView(activity.window.decorView)
            // A new connection, as the input method manager creates on each start of input; it
            // edits the same editing state as the input method's own.
            val connection = view?.onCreateInputConnection(EditorInfo())
            if (connection == null) {
                result.error("no_input", "no text input client is active", null)
                return@setMethodCallHandler
            }
            when (call.method) {
                "commitText" -> connection.commitText(call.argument<String>("text") ?: "", 1)
                "compose" -> {
                    for (partial in call.argument<List<String>>("partials").orEmpty()) {
                        connection.setComposingText(partial, 1)
                    }
                    val commit = call.argument<String>("commit")
                    if (commit != null) {
                        connection.commitText(commit, 1)
                    } else {
                        connection.finishComposingText()
                    }
                }
                "paste" -> {
                    val clipboard = activity.getSystemService(ClipboardManager::class.java)
                    clipboard.setPrimaryClip(
                        ClipData.newPlainText("text", call.argument<String>("text") ?: ""),
                    )
                    connection.performContextMenuAction(android.R.id.paste)
                }
                "text" -> Unit
                else -> {
                    result.notImplemented()
                    return@setMethodCallHandler
                }
            }
            result.success(connection.getExtractedText(ExtractedTextRequest(), 0)?.text?.toString() ?: "")
        }
    }

    private fun flutterView(view: View): FlutterView? {
        if (view is FlutterView) return view
        if (view is ViewGroup) {
            for (i in 0 until view.childCount) {
                flutterView(view.getChildAt(i))?.let { return it }
            }
        }
        return null
    }
}
