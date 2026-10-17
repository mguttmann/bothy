// SPDX-License-Identifier: Apache-2.0

package devicekey.core

import java.security.Signature

/** The result of a step-up prompt. */
sealed interface PromptOutcome {
    /**
     * The user authenticated.
     *
     * @property signature the signature object the prompt unlocked (`CryptoObject.signature`).
     */
    class Authenticated(val signature: Signature) : PromptOutcome

    /**
     * The prompt ended without authentication.
     *
     * @property code the mapped error ([PromptErrors.map]).
     */
    class Failed(val code: DeviceKeyErrorCode) : PromptOutcome
}

/**
 * Shows the system prompt that authorises one signature. [BiometricPromptHost] is the real one;
 * tests inject their own.
 */
interface PromptHost {
    /**
     * Asks the user to authenticate [signature]; must be called on the main thread.
     *
     * @param signature initialised with the step-up key, wrapped in a `CryptoObject`.
     * @param texts the prompt's texts.
     * @param allowDeviceCredential whether the device PIN, pattern or password is accepted besides
     *   strong biometrics (API 30 and later, Design note 3).
     * @param callback receives the outcome exactly once.
     */
    fun authenticate(
        signature: Signature,
        texts: PromptTexts,
        allowDeviceCredential: Boolean,
        callback: (PromptOutcome) -> Unit,
    )
}

/**
 * Signs with the step-up key after the user authenticated the very operation (per-operation
 * authentication, 08 6.1).
 *
 * @param host shows the prompt.
 * @param sdkInt the device's API level: from 30 the prompt also accepts the device credential,
 *   before it strong biometrics only, because a per-operation key cannot be unlocked with the
 *   device credential on API 26 to 29 (docs/DECISIONS.md D-M5-01-3).
 */
class StepUpSigner(private val host: PromptHost, private val sdkInt: Int) {
    /** Whether the prompt accepts the device credential on this device. */
    val allowsDeviceCredential: Boolean get() = sdkInt >= DEVICE_CREDENTIAL_SDK

    /**
     * Shows the prompt for [signature] and signs [data] once the user authenticated.
     *
     * @param signature initialised with the step-up key.
     * @param data the signing input.
     * @param texts the prompt's texts.
     * @param callback receives `R || S` or a [DeviceKeyException], exactly once.
     */
    fun sign(
        signature: Signature,
        data: ByteArray,
        texts: PromptTexts,
        callback: (Result<ByteArray>) -> Unit,
    ) {
        host.authenticate(signature, texts, allowsDeviceCredential) { outcome ->
            val result =
                when (outcome) {
                    is PromptOutcome.Failed -> Result.failure(DeviceKeyException(outcome.code))
                    is PromptOutcome.Authenticated ->
                        runCatching {
                            outcome.signature.update(data)
                            EcdsaSignatures.derToRaw(outcome.signature.sign())
                        }.recoverCatching { throw DeviceKeyException.classify(it) }
                }
            callback(result)
        }
    }

    companion object {
        /** API 30 (Android 11): `setUserAuthenticationParameters` and device credential with crypto. */
        const val DEVICE_CREDENTIAL_SDK = 30
    }
}
