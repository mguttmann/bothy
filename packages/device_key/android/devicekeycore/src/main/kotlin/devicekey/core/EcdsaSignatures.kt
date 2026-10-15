// SPDX-License-Identifier: Apache-2.0

package devicekey.core

import java.math.BigInteger

/**
 * Conversions between the DER form of an ECDSA signature, which `Signature` produces, and the
 * JOSE form `R || S` of RFC 7518 3.4, which DPoP and the step-up confirmation carry.
 */
object EcdsaSignatures {
    /** The length of one P-256 coordinate or signature half. */
    const val PART_LENGTH = 32

    /**
     * Converts a DER `SEQUENCE { INTEGER r, INTEGER s }` to `R || S`, 32 bytes each.
     *
     * Each integer may be shorter than 32 bytes (leading zero bytes dropped) or 33 bytes (a zero
     * byte that keeps a value with the high bit set positive); both are normalised.
     *
     * @param der the signature as `SHA256withECDSA` returns it.
     * @return 64 bytes.
     * @throws IllegalArgumentException when [der] is not such a sequence.
     */
    fun derToRaw(der: ByteArray): ByteArray {
        require(der.size >= 8 && der[0] == 0x30.toByte()) { "not a DER sequence" }
        // A P-256 signature is at most 72 bytes, so the sequence length fits the short form.
        val length = der[1].toInt()
        require(length in 0..0x7f && length == der.size - 2) { "bad sequence length" }
        val (r, next) = integer(der, 2)
        val (s, end) = integer(der, next)
        require(end == der.size) { "trailing bytes" }
        return r + s
    }

    private fun integer(der: ByteArray, offset: Int): Pair<ByteArray, Int> {
        require(offset + 2 <= der.size && der[offset] == 0x02.toByte()) { "not a DER integer" }
        val length = der[offset + 1].toInt()
        val start = offset + 2
        require(length in 1..PART_LENGTH + 1 && start + length <= der.size) { "bad integer length" }
        var value = der.copyOfRange(start, start + length)
        require(value[0] >= 0) { "negative integer" }
        if (value.size > 1 && value[0] == 0.toByte()) {
            // Only a zero that keeps the high bit of the next byte positive is allowed (DER).
            require(value[1] < 0) { "non-minimal integer" }
            value = value.copyOfRange(1, value.size)
        }
        require(value.size <= PART_LENGTH) { "integer longer than 32 bytes" }
        return ByteArray(PART_LENGTH - value.size) + value to start + length
    }

    /**
     * Converts `R || S` back to DER, for verification with `java.security.Signature`.
     *
     * @param raw 64 bytes.
     * @return the DER sequence.
     */
    fun rawToDer(raw: ByteArray): ByteArray {
        require(raw.size == 2 * PART_LENGTH) { "raw signature must be 64 bytes" }
        fun part(bytes: ByteArray): ByteArray {
            val value = BigInteger(1, bytes).toByteArray()
            return byteArrayOf(0x02, value.size.toByte()) + value
        }
        val body = part(raw.copyOfRange(0, PART_LENGTH)) + part(raw.copyOfRange(PART_LENGTH, 2 * PART_LENGTH))
        return byteArrayOf(0x30, body.size.toByte()) + body
    }

    /**
     * Encodes a coordinate of a P-256 point as 32 bytes big-endian.
     *
     * @param value the non-negative coordinate.
     * @return 32 bytes.
     * @throws IllegalArgumentException when [value] does not fit.
     */
    fun coordinate(value: BigInteger): ByteArray {
        require(value.signum() >= 0 && value.bitLength() <= 8 * PART_LENGTH) { "coordinate out of range" }
        val bytes = value.toByteArray()
        val trimmed = if (bytes.size > PART_LENGTH) bytes.copyOfRange(bytes.size - PART_LENGTH, bytes.size) else bytes
        return ByteArray(PART_LENGTH - trimmed.size) + trimmed
    }
}
