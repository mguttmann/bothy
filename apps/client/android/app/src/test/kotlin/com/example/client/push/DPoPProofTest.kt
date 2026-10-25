// SPDX-License-Identifier: Apache-2.0

package com.example.client.push

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.File
import java.math.BigInteger
import java.security.AlgorithmParameters
import java.security.KeyFactory
import java.security.KeyPairGenerator
import java.security.Signature
import java.security.interfaces.ECPublicKey
import java.security.spec.ECGenParameterSpec
import java.security.spec.ECParameterSpec
import java.security.spec.ECPoint
import java.security.spec.ECPublicKeySpec
import java.util.Base64

/**
 * The Kotlin proof equals the Dart `DpopProofFactory` and the Swift `DPoPProof` on the contract
 * vector of M5-04 (D-M5-04-2), and its signature verifies with the public key.
 */
class DPoPProofTest {
    private val vector: Map<String, String> = readVector()

    @Test
    fun signingInputEqualsTheContractVector() {
        val input =
            DPoPProof.signingInput(
                method = vector.getValue("method"),
                url = vector.getValue("url"),
                accessToken = vector.getValue("access_token"),
                nonce = vector.getValue("nonce"),
                x = decode(vector.getValue("x")),
                y = decode(vector.getValue("y")),
                nowSeconds = vector.getValue("iat").toLong(),
                jti = vector.getValue("jti"),
            )
        assertEquals(vector.getValue("signing_input"), input)
    }

    @Test
    fun theProofVerifiesWithThePublicKey() {
        val pair = KeyPairGenerator.getInstance("EC").apply { initialize(ECGenParameterSpec("secp256r1")) }.generateKeyPair()
        val public = pair.public as ECPublicKey
        val signer =
            object : ProofSigner {
                override val x = coordinate(public.w.affineX)
                override val y = coordinate(public.w.affineY)

                override fun sign(data: ByteArray): ByteArray {
                    val der = Signature.getInstance("SHA256withECDSA").apply { initSign(pair.private) ; update(data) }.sign()
                    return derToRaw(der)
                }
            }
        val proof = DPoPProof.make("get", "https://chat.example.com/api/v1/push/preview?nid=1", "bothy_pvw_EXAMPLE", null, signer, 1_760_000_000, "jti-1")
        val parts = proof.split('.')
        assertEquals(3, parts.size)
        val payload = String(Base64.getUrlDecoder().decode(parts[1]))
        assertEquals(
            "{\"jti\":\"jti-1\",\"htm\":\"GET\",\"htu\":\"https://chat.example.com/api/v1/push/preview\",\"iat\":1760000000," +
                "\"ath\":\"${DPoPProof.accessTokenHash("bothy_pvw_EXAMPLE")}\"}",
            payload,
        )
        val key = rebuild(signer.x, signer.y)
        val verifier = Signature.getInstance("SHA256withECDSA").apply { initVerify(key); update("${parts[0]}.${parts[1]}".toByteArray()) }
        assertTrue(verifier.verify(rawToDer(Base64.getUrlDecoder().decode(parts[2]))))
    }

    @Test
    fun htuIsNormalizedLikeDart() {
        assertEquals("https://chat.example.com/", DPoPProof.normalizeHtu("HTTPS://Chat.Example.com:443"))
        assertEquals("http://chat.example.com:8443/a%20b", DPoPProof.normalizeHtu("http://chat.example.com:8443/a%20b?q=1#f"))
        assertEquals("https://[::1]:8443/x", DPoPProof.normalizeHtu("https://[::1]:8443/x"))
    }

    @Test(expected = IllegalArgumentException::class)
    fun anotherSchemeIsRefused() {
        DPoPProof.normalizeHtu("ftp://chat.example.com/")
    }

    @Test
    fun stringsAreEscapedLikeJsonEncode() {
        assertEquals("\"a\\\"b\\\\c\\n\\u0001/é\"", DPoPProof.quote("a\"b\\c\n\u0001/é"))
    }

    @Test
    fun uuidV7CarriesTheTimeAndTheVersion() {
        val id = UuidV7.make(0x0199c82cc000L)
        assertTrue(id, Regex("0199c82c-c000-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}").matches(id))
    }

    private companion object {
        // The vector of M5-04, shared with the Dart and the Swift tests.
        fun readVector(): Map<String, String> {
            val file = File("../../ios/ClientKit/Tests/ClientKitTests/Fixtures/dpop_vector.json")
            val pairs = Regex("\"([a-z_]+)\":\\s*(\"((?:[^\"\\\\]|\\\\.)*)\"|(\\d+))").findAll(file.readText())
            return pairs.associate { it.groupValues[1] to (it.groupValues[3].ifEmpty { it.groupValues[4] }) }
        }

        fun decode(text: String): ByteArray = Base64.getUrlDecoder().decode(text)

        fun coordinate(value: BigInteger): ByteArray {
            val bytes = value.toByteArray().dropWhile { it == 0.toByte() }.toByteArray()
            return ByteArray(32 - bytes.size) + bytes
        }

        fun rebuild(x: ByteArray, y: ByteArray): ECPublicKey {
            val parameters = AlgorithmParameters.getInstance("EC").apply { init(ECGenParameterSpec("secp256r1")) }
            val spec = parameters.getParameterSpec(ECParameterSpec::class.java)
            val point = ECPoint(BigInteger(1, x), BigInteger(1, y))
            return KeyFactory.getInstance("EC").generatePublic(ECPublicKeySpec(point, spec)) as ECPublicKey
        }

        // DER SEQUENCE { INTEGER r, INTEGER s } to R || S and back; the device key core does the
        // same with its strict parser (D-M5-01-4).
        fun derToRaw(der: ByteArray): ByteArray {
            var i = 2
            if (der[1].toInt() and 0x80 != 0) i += der[1].toInt() and 0x7f
            val rLen = der[i + 1].toInt()
            val r = der.copyOfRange(i + 2, i + 2 + rLen)
            i += 2 + rLen
            val sLen = der[i + 1].toInt()
            val s = der.copyOfRange(i + 2, i + 2 + sLen)
            return coordinate(BigInteger(1, r)) + coordinate(BigInteger(1, s))
        }

        fun rawToDer(raw: ByteArray): ByteArray {
            fun int(bytes: ByteArray): ByteArray {
                val v = BigInteger(1, bytes).toByteArray()
                return byteArrayOf(0x02, v.size.toByte()) + v
            }
            val body = int(raw.copyOfRange(0, 32)) + int(raw.copyOfRange(32, 64))
            return byteArrayOf(0x30, body.size.toByte()) + body
        }
    }
}
