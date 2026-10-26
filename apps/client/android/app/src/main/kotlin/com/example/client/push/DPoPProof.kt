// SPDX-License-Identifier: Apache-2.0

package com.example.client.push

import devicekey.core.DeviceKeyCore
import devicekey.core.KeyPurpose
import java.net.URI
import java.security.MessageDigest
import java.security.SecureRandom
import java.util.Base64

/**
 * The public key of a proof as its JWK coordinates.
 *
 * @property x the x coordinate of the P-256 public key, 32 bytes big-endian.
 * @property y the y coordinate, 32 bytes big-endian.
 */
class ProofKey(val x: ByteArray, val y: ByteArray)

/**
 * The signer of a DPoP proof: the public key as JWK members and ES256 signatures as `R || S`.
 */
interface ProofSigner {
    /** The x coordinate of the P-256 public key, 32 bytes big-endian. */
    val x: ByteArray

    /** The y coordinate of the P-256 public key, 32 bytes big-endian. */
    val y: ByteArray

    /**
     * Signs [data] with ES256.
     *
     * @return 64 bytes, `R || S`.
     * @throws Exception when the key cannot sign.
     */
    fun sign(data: ByteArray): ByteArray
}

/**
 * The `dpop` device key of an instance through [DeviceKeyCore] (M5-01), without the user
 * (docs/spec/08-identity.md 6.1, 7.3).
 *
 * It only opens an existing key and never creates one: a missing key means the app has not
 * registered this device, and a new key would not match the session.
 */
class DeviceKeyProofSigner private constructor(
    private val core: DeviceKeyCore,
    private val scope: String,
    override val x: ByteArray,
    override val y: ByteArray,
) : ProofSigner {
    override fun sign(data: ByteArray): ByteArray = core.signDpop(scope, data)

    /** Opens the key. */
    companion object {
        /**
         * The `dpop` key of [scope] (the `instance_id`), or null when there is none or the
         * Keystore fails.
         */
        fun open(core: DeviceKeyCore, scope: String): DeviceKeyProofSigner? {
            val info = runCatching { core.keyInfo(scope, KeyPurpose.DPOP) }.getOrNull() ?: return null
            return DeviceKeyProofSigner(core, scope, info.x, info.y)
        }
    }
}

/**
 * DPoP proofs after RFC 9449 in the profile of the Dart `DpopProofFactory` and the Swift
 * `DPoPProof` of ClientKit (docs/spec/08-identity.md 7.3; brief M5-04 Design note 2,
 * docs/decisions.d/M5-04.md D-M5-04-2).
 *
 * Header members `typ` (`dpop+jwt`), `alg` (`ES256`) and `jwk` (`crv`, `kty`, `x`, `y`) in this
 * order; payload `jti`, `htm`, `htu`, `iat`, `ath` when a token is sent and `nonce` when one is
 * known, in this order; JSON without white space with the escaping of Dart's `jsonEncode`. The
 * JVM test holds the signing input equal to the contract vector of M5-04.
 *
 * ```kotlin
 * val proof = DPoPProof.make("GET", url, previewToken, nonce, signer, nowSeconds)
 * ```
 */
object DPoPProof {
    /**
     * Builds a compact JWS proof.
     *
     * @param method the HTTP method; upper-cased into `htm`.
     * @param url the request URL; normalized into `htu` ([normalizeHtu]).
     * @param accessToken adds `ath`, the base64url SHA-256 of the token; null for none.
     * @param nonce the server's last `DPoP-Nonce`, or null.
     * @param signer the device key.
     * @param nowSeconds `iat` in whole Unix seconds.
     * @param jti the `jti`; a fresh UUIDv7 by default.
     * @return `header.payload.signature` in base64url without padding.
     * @throws IllegalArgumentException for a URL that is not absolute `http` or `https`, or a
     *   signature that is not 64 bytes; or what the signer throws.
     */
    fun make(
        method: String,
        url: String,
        accessToken: String?,
        nonce: String?,
        signer: ProofSigner,
        nowSeconds: Long,
        jti: String = UuidV7.make(nowSeconds * 1000),
    ): String {
        val input = signingInput(method, url, accessToken, nonce, ProofKey(signer.x, signer.y), nowSeconds, jti)
        val signature = signer.sign(input.toByteArray(Charsets.US_ASCII))
        require(signature.size == 64) { "the signer returned no R || S signature" }
        return "$input.${base64Url(signature)}"
    }

    /**
     * The JWS signing input `BASE64URL(header) || '.' || BASE64URL(payload)`.
     *
     * @throws IllegalArgumentException for a URL that is not absolute `http` or `https`.
     */
    fun signingInput(
        method: String,
        url: String,
        accessToken: String?,
        nonce: String?,
        key: ProofKey,
        nowSeconds: Long,
        jti: String,
    ): String {
        val jwk =
            "{${member("crv", "P-256")},${member("kty", "EC")},${member("x", base64Url(key.x))},${member("y", base64Url(key.y))}}"
        val header = "{${member("typ", "dpop+jwt")},${member("alg", "ES256")},${quote("jwk")}:$jwk}"
        val payload = buildList {
            add(member("jti", jti))
            add(member("htm", method.uppercase()))
            add(member("htu", normalizeHtu(url)))
            add("${quote("iat")}:$nowSeconds")
            if (accessToken != null) add(member("ath", accessTokenHash(accessToken)))
            if (nonce != null) add(member("nonce", nonce))
        }.joinToString(",", "{", "}")
        return "${base64Url(header.toByteArray(Charsets.UTF_8))}.${base64Url(payload.toByteArray(Charsets.UTF_8))}"
    }

    /**
     * The `htu` form of [url] (RFC 9449 4.2; 08 7.3), as the Dart `normalizeHtu`: scheme and
     * host in lower case, the default port removed (443 for `https`, 80 for `http`), query and
     * fragment dropped, the path kept as sent (`/` when empty), an IPv6 host in brackets.
     *
     * @throws IllegalArgumentException for another scheme or a URL without a host.
     */
    fun normalizeHtu(url: String): String {
        val uri = runCatching { URI(url) }.getOrElse { throw IllegalArgumentException("not a URL") }
        val scheme = uri.scheme?.lowercase()
        val rawHost = uri.host
        require((scheme == "https" || scheme == "http") && !rawHost.isNullOrEmpty()) {
            "must be an absolute http or https URL"
        }
        val bare = rawHost.lowercase().removePrefix("[").removeSuffix("]")
        val authority = if (bare.contains(':')) "[$bare]" else bare
        val defaultPort = if (scheme == "https") 443 else 80
        val port = if (uri.port == -1 || uri.port == defaultPort) "" else ":${uri.port}"
        val path = uri.rawPath.takeUnless { it.isNullOrEmpty() } ?: "/"
        return "$scheme://$authority$port$path"
    }

    /** The `ath` value of RFC 9449 4.2: base64url of the SHA-256 of the token's UTF-8 bytes. */
    fun accessTokenHash(accessToken: String): String =
        base64Url(MessageDigest.getInstance("SHA-256").digest(accessToken.toByteArray(Charsets.UTF_8)))

    /** Base64url without padding (RFC 7515 2). */
    fun base64Url(data: ByteArray): String = Base64.getUrlEncoder().withoutPadding().encodeToString(data)

    private fun member(name: String, value: String) = "${quote(name)}:${quote(value)}"

    /**
     * A JSON string as Dart's `jsonEncode` writes it: `"` and `\` escaped, control characters as
     * `\b`, `\t`, `\n`, `\f`, `\r` or `\u00xx` (lower-case hex), everything else as is.
     */
    internal fun quote(text: String): String =
        buildString {
            append('"')
            for (c in text) {
                when {
                    c == '"' -> append("\\\"")
                    c == '\\' -> append("\\\\")
                    c == '\b' -> append("\\b")
                    c == '\t' -> append("\\t")
                    c == '\n' -> append("\\n")
                    c == '\u000C' -> append("\\f")
                    c == '\r' -> append("\\r")
                    c.code < 0x20 -> append("\\u%04x".format(c.code))
                    else -> append(c)
                }
            }
            append('"')
        }
}

/** UUIDv7 strings (RFC 9562 5.7) for the `jti` of a proof. */
object UuidV7 {
    private val random = SecureRandom()

    /**
     * A new UUIDv7 in the canonical lower-case form: 48 bits of [nowMillis], the version and
     * variant bits, the rest random.
     */
    fun make(nowMillis: Long): String {
        val bytes = ByteArray(16)
        random.nextBytes(bytes)
        for (i in 0 until 6) bytes[i] = (nowMillis ushr (8 * (5 - i))).toByte()
        bytes[6] = ((bytes[6].toInt() and 0x0f) or 0x70).toByte()
        bytes[8] = ((bytes[8].toInt() and 0x3f) or 0x80).toByte()
        val hex = bytes.joinToString("") { "%02x".format(it) }
        return listOf(hex.substring(0, 8), hex.substring(8, 12), hex.substring(12, 16), hex.substring(16, 20), hex.substring(20))
            .joinToString("-")
    }
}
