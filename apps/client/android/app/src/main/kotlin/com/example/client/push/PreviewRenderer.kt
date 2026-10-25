// SPDX-License-Identifier: Apache-2.0

package com.example.client.push

import android.content.Context
import com.example.client.R

/**
 * The localized texts of a push notification, from `strings.xml` of `res/values` and `res/values-<lang>`, which
 * `tools/catalog/apple_strings.py` writes from the ARB files of `packages/l10n` (brief M5-05
 * Design note 5; D-M5-04-7).
 *
 * @property title the placeholder title (`push_title`).
 * @property bodies the placeholder body per kind (`push_body_<kind>`).
 * @property notAvailable "Message not available".
 * @property titleFormat `<channel> · <thread title>` with the positional `%1$s` and `%2$s`.
 * @property textFormat `<sender>: <excerpt>` with the positional `%1$s` and `%2$s`.
 */
data class PushTexts(
    val title: String,
    val bodies: Map<String, String>,
    val notAvailable: String,
    val titleFormat: String,
    val textFormat: String,
) {
    /** The placeholder body of [kind]; a kind this build does not know gets the one of `message`. */
    fun bodyOf(kind: String): String = bodies[kind] ?: bodies.getValue("message")

    /** Reading the resources. */
    companion object {
        /** The texts of [context] in the device language. */
        fun of(context: Context): PushTexts {
            val r = context.resources
            return PushTexts(
                title = r.getString(R.string.push_title),
                bodies =
                    mapOf(
                        "message" to r.getString(R.string.push_body_message),
                        "mention" to r.getString(R.string.push_body_mention),
                        "dm" to r.getString(R.string.push_body_dm),
                        "run_done" to r.getString(R.string.push_body_run_done),
                        "run_failed" to r.getString(R.string.push_body_run_failed),
                        "input_needed" to r.getString(R.string.push_body_input_needed),
                        "security" to r.getString(R.string.push_body_security),
                        "admin" to r.getString(R.string.push_body_admin),
                    ),
                notAvailable = r.getString(R.string.push_message_not_available),
                titleFormat = r.getString(R.string.push_preview_title),
                textFormat = r.getString(R.string.push_preview_text),
            )
        }
    }
}

/**
 * Title and body of a notification with a preview, by the rules of the Notification Service
 * Extension (brief M5-04 Design note 4; D-M5-04-4) and the desktop toast (01 10.2).
 */
object PreviewRenderer {
    /**
     * What the notification shows for [preview].
     *
     * The title is `<channel> · <thread title>`, only the channel when the thread has no title,
     * and the placeholder title when no channel name came (a DM, or `none`). The body is
     * `<sender>: <excerpt>`; without an excerpt (`names`) the excerpt is the placeholder body,
     * without a sender the body is the placeholder alone.
     *
     * @return title and body.
     */
    fun render(preview: PushPreview, kind: String, texts: PushTexts): Pair<String, String> {
        val placeholderBody = texts.bodyOf(kind)
        val channel = present(preview.channelName)
        val thread = present(preview.threadTitle)
        val title =
            when {
                channel != null && thread != null -> format(texts.titleFormat, channel, thread)
                channel != null -> channel
                else -> texts.title
            }
        val sender = present(preview.senderDisplayName)
        val body = if (sender != null) format(texts.textFormat, sender, present(preview.excerpt) ?: placeholderBody) else placeholderBody
        return title to body
    }

    private fun present(value: String?): String? = value?.takeIf { it.isNotBlank() }

    /**
     * Formats with `%1$s`, `%2$s` and `%%` in one pass, so an argument is never read as a format;
     * a format without both arguments (a broken strings file) falls back to the plain join.
     */
    internal fun format(format: String, first: String, second: String): String {
        if (!format.contains("%1\$s") || !format.contains("%2\$s")) return "$first $second"
        val out = StringBuilder()
        var i = 0
        while (i < format.length) {
            when {
                format.startsWith("%1\$s", i) -> out.append(first).also { i += 4 }
                format.startsWith("%2\$s", i) -> out.append(second).also { i += 4 }
                format.startsWith("%%", i) -> out.append('%').also { i += 2 }
                else -> out.append(format[i]).also { i += 1 }
            }
        }
        return out.toString()
    }
}
