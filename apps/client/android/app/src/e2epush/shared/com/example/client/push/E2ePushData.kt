// SPDX-License-Identifier: Apache-2.0

package com.example.client.push

import java.nio.ByteBuffer
import java.nio.charset.CharacterCodingException
import java.util.Base64

/**
 * The `data_b64` extra of the E2E push broadcast (brief M5-11 Design note 2): standard base64
 * of UTF-8 JSON, one object whose values are all strings, as the `data` of an FCM message (13
 * 6.2) and as the runner's relay writes it.
 *
 * The app has no JSON library that the JVM tests can run (`org.json` is a stub there), so a
 * strict reader of exactly this shape decodes it.
 */
object E2ePushData {
    /**
     * Decodes [dataB64].
     *
     * @param dataB64 the extra, or null when the broadcast has none.
     * @return the map in the order of the JSON, or null when [dataB64] is null, not base64, not
     *   UTF-8, or not a single JSON object of string values with distinct keys.
     */
    fun decode(dataB64: String?): Map<String, String>? {
        if (dataB64 == null) return null
        val bytes =
            try {
                Base64.getDecoder().decode(dataB64)
            } catch (e: IllegalArgumentException) {
                return null
            }
        // A strict decoder: invalid UTF-8 is an error, not a replacement character.
        val text =
            try {
                Charsets.UTF_8.newDecoder().decode(ByteBuffer.wrap(bytes)).toString()
            } catch (e: CharacterCodingException) {
                return null
            }
        return StringObjectReader(text).read()
    }
}

/** Reads one JSON object of string values (RFC 8259) from [text]; any other JSON is refused. */
private class StringObjectReader(private val text: String) {
    private var at = 0

    /** The object, or null when [text] is anything else or has more after the object. */
    fun read(): Map<String, String>? {
        val members = LinkedHashMap<String, String>()
        skipSpace()
        if (!take('{')) return null
        skipSpace()
        if (!take('}')) {
            do {
                skipSpace()
                val key = string() ?: return null
                skipSpace()
                if (!take(':')) return null
                skipSpace()
                val value = string() ?: return null
                // A repeated key has no single meaning; FCM data never repeats one.
                if (members.put(key, value) != null) return null
                skipSpace()
            } while (take(','))
            if (!take('}')) return null
        }
        skipSpace()
        return members.takeIf { at == text.length }
    }

    private fun string(): String? {
        if (!take('"')) return null
        val out = StringBuilder()
        while (at < text.length) {
            val c = text[at++]
            when {
                c == '"' -> return out.toString()
                c == '\\' -> out.append(escape() ?: return null)
                // Control characters must be escaped inside a JSON string.
                c < ' ' -> return null
                else -> out.append(c)
            }
        }
        return null
    }

    private fun escape(): Char? {
        if (at >= text.length) return null
        return when (text[at++]) {
            '"' -> '"'
            '\\' -> '\\'
            '/' -> '/'
            'b' -> '\b'
            'f' -> '\u000c'
            'n' -> '\n'
            'r' -> '\r'
            't' -> '\t'
            'u' -> unicode()
            else -> null
        }
    }

    // Four ASCII hex digits; a surrogate pair arrives as two escapes, each one UTF-16 unit.
    private fun unicode(): Char? {
        if (at + 4 > text.length) return null
        var code = 0
        repeat(4) {
            val digit =
                when (val c = text[at++]) {
                    in '0'..'9' -> c - '0'
                    in 'a'..'f' -> c - 'a' + 10
                    in 'A'..'F' -> c - 'A' + 10
                    else -> return null
                }
            code = code * 16 + digit
        }
        return code.toChar()
    }

    private fun take(c: Char): Boolean {
        if (at < text.length && text[at] == c) {
            at++
            return true
        }
        return false
    }

    private fun skipSpace() {
        while (at < text.length && text[at] in SPACE) at++
    }

    private companion object {
        /** The white space of RFC 8259. */
        const val SPACE = " \t\n\r"
    }
}
