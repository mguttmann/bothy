// SPDX-License-Identifier: Apache-2.0

package devicekey.core

import android.content.Context
import android.os.Build
import androidx.biometric.BiometricManager
import androidx.biometric.BiometricManager.Authenticators.BIOMETRIC_STRONG
import androidx.biometric.BiometricManager.Authenticators.DEVICE_CREDENTIAL
import androidx.biometric.BiometricPrompt
import androidx.core.content.ContextCompat
import androidx.fragment.app.FragmentActivity
import java.security.Signature

/**
 * The [PromptHost] of an app: `BiometricPrompt` with a `CryptoObject` around the signature.
 *
 * `BiometricPrompt` needs a [FragmentActivity] in the foreground (a Flutter app uses
 * `FlutterFragmentActivity`); without one the step-up reports `not_available`.
 *
 * @param activity returns the current activity, or null.
 */
class BiometricPromptHost(private val activity: () -> FragmentActivity?) : PromptHost {
    override fun authenticate(
        signature: Signature,
        texts: PromptTexts,
        allowDeviceCredential: Boolean,
        callback: (PromptOutcome) -> Unit,
    ) {
        val host = activity()
        if (host == null) {
            callback(PromptOutcome.Failed(DeviceKeyErrorCode.NOT_AVAILABLE))
            return
        }
        val prompt =
            BiometricPrompt(
                host,
                ContextCompat.getMainExecutor(host),
                object : BiometricPrompt.AuthenticationCallback() {
                    override fun onAuthenticationSucceeded(result: BiometricPrompt.AuthenticationResult) {
                        val unlocked = result.cryptoObject?.signature
                        callback(
                            if (unlocked == null) {
                                PromptOutcome.Failed(DeviceKeyErrorCode.INTERNAL)
                            } else {
                                PromptOutcome.Authenticated(unlocked)
                            },
                        )
                    }

                    override fun onAuthenticationError(errorCode: Int, errString: CharSequence) {
                        callback(PromptOutcome.Failed(PromptErrors.map(errorCode)))
                    }
                    // onAuthenticationFailed (a finger not recognised) keeps the prompt open.
                },
            )
        val info =
            BiometricPrompt.PromptInfo.Builder()
                .setTitle(texts.title)
                .apply { texts.subtitle?.let { setSubtitle(it) } }
                .apply {
                    if (allowDeviceCredential) {
                        // A prompt that accepts the device credential must not have a negative button.
                        setAllowedAuthenticators(BIOMETRIC_STRONG or DEVICE_CREDENTIAL)
                    } else {
                        setAllowedAuthenticators(BIOMETRIC_STRONG)
                        setNegativeButtonText(texts.negativeButton)
                    }
                }
                .setConfirmationRequired(true)
                .build()
        prompt.authenticate(info, BiometricPrompt.CryptoObject(signature))
    }
}

/**
 * Whether the step-up key can be created on this device now (Design note 3): from API 30 strong
 * biometrics or a device credential, before it enrolled strong biometrics.
 *
 * @param context any context of the app.
 * @param sdkInt the device's API level.
 * @return a check for [DeviceKeyCore]'s `stepUpAvailable`.
 */
fun stepUpAvailability(context: Context, sdkInt: Int = Build.VERSION.SDK_INT): () -> Boolean {
    val manager = BiometricManager.from(context.applicationContext)
    val authenticators = if (sdkInt >= StepUpSigner.DEVICE_CREDENTIAL_SDK) BIOMETRIC_STRONG or DEVICE_CREDENTIAL else BIOMETRIC_STRONG
    return { manager.canAuthenticate(authenticators) == BiometricManager.BIOMETRIC_SUCCESS }
}

/**
 * Creates the core of an app with the Android Keystore, `BiometricPrompt` and the step-up check.
 *
 * @param context any context of the app.
 * @param activity returns the current [FragmentActivity] for the step-up prompt, or null in the
 *   background (the push service signs with the DPoP key only).
 * @return the core.
 */
fun DeviceKeyCore.Companion.forContext(context: Context, activity: () -> FragmentActivity?): DeviceKeyCore =
    DeviceKeyCore(
        store = AndroidKeyStoreAccess(),
        stepUpSigner = StepUpSigner(BiometricPromptHost(activity), Build.VERSION.SDK_INT),
        stepUpAvailable = stepUpAvailability(context),
    )
