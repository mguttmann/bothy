// SPDX-License-Identifier: Apache-2.0

package devicekey.core

import java.security.GeneralSecurityException
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertSame
import org.junit.Test

/** Errors leave the core with a stable code and a fixed text only (brief M5-01, Design note 4). */
class DeviceKeyErrorTest {
    // Stands for key bytes and an alias in a platform message.
    private val secret = "sentinelkeymaterial " + "q8Ez".repeat(8) + " alias test.device_key.sentinel"

    @Test
    fun errorMappingCarriesNoKeyMaterial() {
        val causes =
            listOf(
                RuntimeException(secret),
                GeneralSecurityException(secret, IllegalStateException(secret)),
                IllegalArgumentException(secret),
            )
        for (cause in causes) {
            val mapped = DeviceKeyException.classify(cause)
            assertEquals(DeviceKeyErrorCode.INTERNAL, mapped.code)
            for (text in listOf(mapped.message, mapped.toString(), mapped.code.message)) {
                assertFalse(text.orEmpty().contains("sentinel"))
                assertFalse(text.orEmpty().contains("q8Ez"))
            }
            // The cause stays out, so no logger prints its message.
            assertNull(mapped.cause)
        }
    }

    @Test
    fun codesAreTheWireCodesOfTheChannel() {
        assertEquals(
            listOf("user_cancelled", "key_invalidated", "not_available", "internal"),
            DeviceKeyErrorCode.entries.map { it.wire },
        )
    }

    @Test
    fun aDeviceKeyExceptionKeepsItsCode() {
        val original = DeviceKeyException(DeviceKeyErrorCode.USER_CANCELLED)
        assertSame(original, DeviceKeyException.classify(original))
        val nested = RuntimeException(GeneralSecurityException(DeviceKeyException(DeviceKeyErrorCode.KEY_INVALIDATED)))
        assertEquals(DeviceKeyErrorCode.KEY_INVALIDATED, DeviceKeyException.classify(nested).code)
    }

    @Test
    fun keystoreExceptionsAreClassifiedByType() {
        assertEquals(
            DeviceKeyErrorCode.KEY_INVALIDATED,
            DeviceKeyException.classify(RuntimeException(android.security.keystore.KeyPermanentlyInvalidatedException(secret))).code,
        )
        assertEquals(
            DeviceKeyErrorCode.NOT_AVAILABLE,
            DeviceKeyException.classify(java.security.SignatureException(android.security.keystore.UserNotAuthenticatedException(secret))).code,
        )
    }

    @Test
    fun promptErrorCodesMapToStableCodes() {
        val p = androidx.biometric.BiometricPrompt::class.java
        fun code(name: String) = p.getField(name).getInt(null)
        for (name in listOf("ERROR_USER_CANCELED", "ERROR_NEGATIVE_BUTTON", "ERROR_CANCELED")) {
            assertEquals(name, DeviceKeyErrorCode.USER_CANCELLED, PromptErrors.map(code(name)))
        }
        for (name in listOf("ERROR_NO_BIOMETRICS", "ERROR_HW_NOT_PRESENT", "ERROR_HW_UNAVAILABLE", "ERROR_NO_DEVICE_CREDENTIAL", "ERROR_LOCKOUT", "ERROR_LOCKOUT_PERMANENT")) {
            assertEquals(name, DeviceKeyErrorCode.NOT_AVAILABLE, PromptErrors.map(code(name)))
        }
        for (name in listOf("ERROR_TIMEOUT", "ERROR_VENDOR", "ERROR_UNABLE_TO_PROCESS", "ERROR_NO_SPACE")) {
            assertEquals(name, DeviceKeyErrorCode.INTERNAL, PromptErrors.map(code(name)))
        }
    }
}
