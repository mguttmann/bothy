// SPDX-License-Identifier: Apache-2.0

package devicekey.core

import java.math.BigInteger
import java.security.KeyPairGenerator
import java.security.Signature
import java.security.spec.ECGenParameterSpec
import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertThrows
import org.junit.Test

/** The DER to `R || S` conversion of RFC 7518 3.4 (brief M5-01, Design note 4). */
class EcdsaSignaturesTest {
    private fun integer(value: ByteArray): ByteArray = byteArrayOf(0x02, value.size.toByte()) + value

    private fun der(r: ByteArray, s: ByteArray): ByteArray {
        val body = integer(r) + integer(s)
        return byteArrayOf(0x30, body.size.toByte()) + body
    }

    private fun bytes(size: Int, first: Int, fill: Int = 0x11): ByteArray =
        ByteArray(size) { if (it == 0) first.toByte() else fill.toByte() }

    @Test
    fun derToRawConversion() {
        // 32 bytes: the high bit is clear, so DER needs no leading zero.
        val r32 = bytes(32, 0x7f)
        // 33 bytes: the high bit of the value is set, so DER adds a zero byte.
        val s33 = byteArrayOf(0) + bytes(32, 0x80)
        val raw = EcdsaSignatures.derToRaw(der(r32, s33))
        assertEquals(64, raw.size)
        assertArrayEquals(r32, raw.copyOfRange(0, 32))
        assertArrayEquals(s33.copyOfRange(1, 33), raw.copyOfRange(32, 64))

        // 31 bytes: a value with a leading zero byte is left-padded back to 32 bytes.
        val r31 = bytes(31, 0x05)
        val raw31 = EcdsaSignatures.derToRaw(der(r31, r32))
        assertArrayEquals(byteArrayOf(0) + r31, raw31.copyOfRange(0, 32))
        assertArrayEquals(r32, raw31.copyOfRange(32, 64))

        // Very short values (two leading zero bytes and more) are padded the same way.
        val tiny = byteArrayOf(0x01)
        val rawTiny = EcdsaSignatures.derToRaw(der(tiny, tiny))
        assertArrayEquals(ByteArray(31) + tiny, rawTiny.copyOfRange(0, 32))
        assertArrayEquals(ByteArray(31) + tiny, rawTiny.copyOfRange(32, 64))
    }

    @Test
    fun derToRawRejectsMalformedInput() {
        val r = bytes(32, 0x10)
        val valid = der(r, r)
        val malformed =
            listOf(
                ByteArray(0),
                valid.copyOf(valid.size - 1),
                valid + byteArrayOf(0),
                byteArrayOf(0x31) + valid.copyOfRange(1, valid.size),
                der(bytes(34, 0x01), r),
                der(byteArrayOf(0) + bytes(32, 0x10), r),
                der(bytes(32, 0x80), r),
            )
        for (input in malformed) {
            assertThrows(IllegalArgumentException::class.java) { EcdsaSignatures.derToRaw(input) }
        }
    }

    @Test
    fun rawSignatureOfTheJdkVerifiesAfterConversionBack() {
        val generator = KeyPairGenerator.getInstance("EC")
        generator.initialize(ECGenParameterSpec("secp256r1"))
        val pair = generator.generateKeyPair()
        val data = "signing input".toByteArray()
        repeat(32) {
            val signer = Signature.getInstance("SHA256withECDSA")
            signer.initSign(pair.private)
            signer.update(data)
            val raw = EcdsaSignatures.derToRaw(signer.sign())
            assertEquals(64, raw.size)
            val verifier = Signature.getInstance("SHA256withECDSA")
            verifier.initVerify(pair.public)
            verifier.update(data)
            assertEquals(true, verifier.verify(EcdsaSignatures.rawToDer(raw)))
        }
    }

    @Test
    fun coordinateIsThirtyTwoBytesBigEndian() {
        assertArrayEquals(ByteArray(31) + byteArrayOf(1), EcdsaSignatures.coordinate(BigInteger.ONE))
        val high = BigInteger(1, bytes(32, 0xff))
        assertArrayEquals(bytes(32, 0xff), EcdsaSignatures.coordinate(high))
        assertThrows(IllegalArgumentException::class.java) {
            EcdsaSignatures.coordinate(BigInteger.ONE.shiftLeft(256))
        }
    }
}
