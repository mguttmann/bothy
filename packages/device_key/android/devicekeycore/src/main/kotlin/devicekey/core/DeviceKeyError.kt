// SPDX-License-Identifier: Apache-2.0

package devicekey.core

import android.os.Build
import android.security.KeyStoreException
import android.security.keystore.KeyPermanentlyInvalidatedException
import android.security.keystore.UserNotAuthenticatedException
import androidx.biometric.BiometricPrompt

/**
 * Why a device key operation failed. The codes are the error codes of the Pigeon channel and the
 * same on every platform (docs/DECISIONS.md D-M4-10-1).
 *
 * @property wire the code on the channel.
 * @property message the fixed text that travels with the code; it never names a key or a server.
 */
enum class DeviceKeyErrorCode(val wire: String, val message: String) {
    /** The user cancelled the prompt or pressed its negative button. */
    USER_CANCELLED("user_cancelled", "the user cancelled the confirmation"),

    /** The key can no longer be used, for example after new biometrics were enrolled. */
    KEY_INVALIDATED("key_invalidated", "the device key is no longer valid"),

    /** There is no key, or the device cannot create or use one for this purpose right now. */
    NOT_AVAILABLE("not_available", "the device key is not available"),

    /** Any other failure. */
    INTERNAL("internal", "the device key operation failed"),
}

/**
 * A failed device key operation with a stable [code] and its fixed text.
 *
 * It never carries the original exception: Keystore messages can name aliases and parameters, and
 * a logger would print a cause. [classify] is the only way platform exceptions become this type.
 *
 * @property code why the operation failed.
 */
class DeviceKeyException(val code: DeviceKeyErrorCode) : Exception(code.message) {
    companion object {
        /**
         * Maps any throwable of a Keystore or prompt operation to a [DeviceKeyException].
         *
         * The cause chain is searched: a [DeviceKeyException] keeps its code,
         * [KeyPermanentlyInvalidatedException] becomes [DeviceKeyErrorCode.KEY_INVALIDATED],
         * [UserNotAuthenticatedException] and, from API 33, a [KeyStoreException] with
         * `ERROR_USER_AUTHENTICATION_REQUIRED` (a step-up signature without the prompt; the
         * Keystore reports it at `sign`) [DeviceKeyErrorCode.NOT_AVAILABLE], everything else
         * [DeviceKeyErrorCode.INTERNAL]. Below API 33 the Keystore's error class is hidden, so that
         * case is `internal` there.
         *
         * @param error the failure.
         * @return the exception to report; [error] itself when it already is one.
         */
        fun classify(error: Throwable): DeviceKeyException {
            if (error is DeviceKeyException) return error
            var current: Throwable? = error
            val seen = mutableSetOf<Throwable>()
            while (current != null && seen.add(current)) {
                when (current) {
                    is DeviceKeyException -> return DeviceKeyException(current.code)
                    is KeyPermanentlyInvalidatedException -> return DeviceKeyException(DeviceKeyErrorCode.KEY_INVALIDATED)
                    is UserNotAuthenticatedException -> return DeviceKeyException(DeviceKeyErrorCode.NOT_AVAILABLE)
                }
                if (userNotAuthenticated(current)) return DeviceKeyException(DeviceKeyErrorCode.NOT_AVAILABLE)
                current = current.cause
            }
            return DeviceKeyException(DeviceKeyErrorCode.INTERNAL)
        }

        private fun userNotAuthenticated(error: Throwable): Boolean =
            Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU &&
                error is KeyStoreException &&
                error.numericErrorCode == KeyStoreException.ERROR_USER_AUTHENTICATION_REQUIRED
    }
}

/** Maps the error codes of `BiometricPrompt` to [DeviceKeyErrorCode]s (brief M5-01, Design note 4). */
object PromptErrors {
    /**
     * @param errorCode the code `onAuthenticationError` received.
     * @return [DeviceKeyErrorCode.USER_CANCELLED] for a cancel or the negative button,
     *   [DeviceKeyErrorCode.NOT_AVAILABLE] where no authenticator can be used (no hardware, nothing
     *   enrolled, locked out), [DeviceKeyErrorCode.INTERNAL] otherwise.
     */
    fun map(errorCode: Int): DeviceKeyErrorCode =
        when (errorCode) {
            BiometricPrompt.ERROR_USER_CANCELED,
            BiometricPrompt.ERROR_NEGATIVE_BUTTON,
            BiometricPrompt.ERROR_CANCELED,
            -> DeviceKeyErrorCode.USER_CANCELLED

            BiometricPrompt.ERROR_NO_BIOMETRICS,
            BiometricPrompt.ERROR_HW_NOT_PRESENT,
            BiometricPrompt.ERROR_HW_UNAVAILABLE,
            BiometricPrompt.ERROR_NO_DEVICE_CREDENTIAL,
            BiometricPrompt.ERROR_LOCKOUT,
            BiometricPrompt.ERROR_LOCKOUT_PERMANENT,
            -> DeviceKeyErrorCode.NOT_AVAILABLE

            else -> DeviceKeyErrorCode.INTERNAL
        }
}
