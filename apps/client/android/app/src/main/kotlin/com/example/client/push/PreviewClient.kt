// SPDX-License-Identifier: Apache-2.0

package com.example.client.push

import org.json.JSONObject
import java.io.IOException
import java.net.URI
import java.net.URL
import java.net.URLEncoder
import javax.net.ssl.HttpsURLConnection

/**
 * The answer of `getPushPreview` (schema `PushPreview`, docs/spec/13-push-notifications.md 6.4);
 * which members are present depends on `notify.preview_mode`.
 *
 * [toString] names no value, so a log line never carries content.
 */
class PushPreview(
    /** The kind, echoed from the request. */
    val kind: String,
    /** The badge number at the time of the fetch. */
    val badge: Int,
    /** The channel name (`full`, `names`). */
    val channelName: String?,
    /** The thread title (`full`, `names`). */
    val threadTitle: String?,
    /** The author's display name (`full`, `names`). */
    val senderDisplayName: String?,
    /** The excerpt (`full`). */
    val excerpt: String?,
) {
    override fun toString(): String = "PushPreview(<content>)"

    /** Decoding. */
    companion object {
        /** The kinds of `NotificationKind`. */
        val KINDS = setOf("message", "mention", "dm", "run_done", "run_failed", "input_needed", "security", "admin")

        /**
         * Decodes a response body: a missing or mistyped `kind` or `badge`, an unknown kind or
         * a mistyped text member fails; unknown members are ignored.
         *
         * @return the preview, or null when the body does not match.
         */
        fun decode(body: String): PushPreview? {
            val json = runCatching { JSONObject(body) }.getOrNull() ?: return null
            val kind = json.opt("kind") as? String ?: return null
            if (kind !in KINDS) return null
            val badge = (json.opt("badge") as? Number)?.takeIf { it.toDouble() == it.toLong().toDouble() }?.toInt() ?: return null
            var valid = true
            fun text(name: String): String? {
                if (!json.has(name) || json.isNull(name)) return null
                return json.opt(name) as? String ?: run {
                    valid = false
                    null
                }
            }
            val preview = PushPreview(kind, badge, text("channel_name"), text("thread_title"), text("sender_display_name"), text("excerpt"))
            return preview.takeIf { valid }
        }
    }
}

/** The outcome of one preview fetch. */
sealed interface PreviewResult {
    /** The server answered with [preview]. */
    data class Content(val preview: PushPreview) : PreviewResult

    /** `404`: no right, deleted or unknown message; the notification says "Message not available". */
    data object NotAvailable : PreviewResult

    /** Any other status, a redirect, a network error, a timeout or a malformed body. */
    data object Failed : PreviewResult
}

/**
 * Opens the connection of one request; the platform's `HttpsURLConnection` on the device, one
 * that trusts the per-run test CA in the instrumented tests.
 */
fun interface HttpsConnector {
    /** Opens [url], which is always `https`. */
    fun open(url: URL): HttpsURLConnection
}

/** The platform's connection with its certificate and host name validation. */
val PlatformConnector = HttpsConnector { url -> url.openConnection() as HttpsURLConnection }

/**
 * `GET /api/v1/push/preview` with the preview token and a DPoP proof of the `dpop` device key
 * (docs/spec/13-push-notifications.md 6.4; 15 10.2; brief M5-05 Design note 2).
 *
 * Its only credential is the preview token of the push registration: never a refresh token,
 * never a login or session endpoint (08 7.4). Only an `https` server address is used, redirects
 * are not followed (one counts as a failure), and nothing is cached and no cookie is sent, so
 * the token and the proof never travel in clear text or to another URL. On `401` with
 * `use_dpop_nonce` and a new `DPoP-Nonce` it repeats once; every nonce it sees is kept.
 *
 * @param registration the server address and the preview token.
 * @param signer the `dpop` device key.
 * @param store keeps the last nonce per server.
 * @param connector opens the connection.
 * @param nowSeconds the clock of `iat`.
 */
class PreviewClient(
    private val registration: PushRegistration,
    private val signer: ProofSigner,
    private val store: PushStore,
    private val connector: HttpsConnector,
    private val nowSeconds: () -> Long,
) {
    /**
     * Fetches the preview of one push within [timeoutMillis] per request
     * (`client.push.fetch_timeout`).
     *
     * @param nid `nid` of the payload.
     * @param messageId `m` of the payload.
     * @param kind `k` of the payload.
     * @return the outcome; never throws.
     */
    fun fetch(nid: String, messageId: String, kind: String, timeoutMillis: Int): PreviewResult {
        val url = previewUrl(registration.serverUrl, nid, messageId, kind) ?: return PreviewResult.Failed
        val origin = DPoPProof.normalizeHtu(url).removeSuffix(URI(url).rawPath)
        var nonce = store.nonce(origin)
        repeat(2) { attempt ->
            val answer = runCatching { exchange(url, nonce, timeoutMillis) }.getOrNull() ?: return PreviewResult.Failed
            answer.nonce?.let { store.rememberNonce(origin, it) }
            when {
                answer.status == 200 -> return PushPreview.decode(answer.body)?.let { PreviewResult.Content(it) } ?: PreviewResult.Failed
                answer.status == 404 -> return PreviewResult.NotAvailable
                answer.status == 401 && attempt == 0 && answer.asksForNonce() -> {
                    if (answer.nonce == null || answer.nonce == nonce) return PreviewResult.Failed
                    nonce = answer.nonce
                }
                else -> return PreviewResult.Failed
            }
        }
        return PreviewResult.Failed
    }

    private class Answer(val status: Int, val body: String, val nonce: String?, val challenge: String?) {
        fun asksForNonce(): Boolean =
            challenge?.contains("use_dpop_nonce") == true ||
                runCatching { JSONObject(body) }.getOrNull()?.let {
                    it.optString("code") == "use_dpop_nonce" || it.optString("error") == "use_dpop_nonce"
                } == true
    }

    private fun exchange(url: String, nonce: String?, timeoutMillis: Int): Answer {
        val proof = DPoPProof.make("GET", url, registration.previewToken, nonce, signer, nowSeconds())
        val connection = connector.open(URL(url))
        try {
            connection.instanceFollowRedirects = false
            connection.useCaches = false
            connection.connectTimeout = timeoutMillis
            connection.readTimeout = timeoutMillis
            connection.requestMethod = "GET"
            connection.setRequestProperty("Authorization", "DPoP ${registration.previewToken}")
            connection.setRequestProperty("DPoP", proof)
            connection.setRequestProperty("Accept", "application/json")
            val status = connection.responseCode
            val stream = if (status in 200..299) connection.inputStream else connection.errorStream
            val body = stream?.use { input -> readLimited(input) }.orEmpty()
            return Answer(
                status,
                body,
                connection.getHeaderField("DPoP-Nonce")?.takeIf { it.isNotEmpty() },
                connection.getHeaderField("WWW-Authenticate"),
            )
        } finally {
            connection.disconnect()
        }
    }

    private fun readLimited(input: java.io.InputStream): String {
        val bytes = java.io.ByteArrayOutputStream()
        val buffer = ByteArray(BUFFER)
        while (true) {
            val n = input.read(buffer)
            if (n < 0) break
            bytes.write(buffer, 0, n)
            if (bytes.size() > MAX_BODY) throw IOException("preview body too large")
        }
        return bytes.toString(Charsets.UTF_8.name())
    }

    /** URL building. */
    companion object {
        private const val BUFFER = 8192

        // A preview is a few hundred bytes; a larger answer is not one (no configuration key,
        // the server's own limit is notify.preview_max_chars on the excerpt).
        private const val MAX_BODY = 256 * 1024

        /**
         * The preview URL of [server] with the query of 13 6.4.
         *
         * @return the URL, or null for an address that is not `https` with a host.
         */
        fun previewUrl(server: String, nid: String, messageId: String, kind: String): String? {
            val uri = runCatching { URI(server) }.getOrNull() ?: return null
            // No user info: an address with one is not a server address of discovery.
            if (uri.scheme?.lowercase() != "https" || uri.host.isNullOrEmpty() || uri.rawUserInfo != null) return null
            val base = (uri.rawPath ?: "").removeSuffix("/")
            fun q(value: String) = URLEncoder.encode(value, "UTF-8")
            return "https://${uri.rawAuthority}$base/api/v1/push/preview?nid=${q(nid)}&message_id=${q(messageId)}&kind=${q(kind)}"
        }
    }
}
