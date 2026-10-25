// SPDX-License-Identifier: Apache-2.0

package com.example.client

import android.app.Activity
import android.os.Build
import androidx.credentials.CreatePublicKeyCredentialRequest
import androidx.credentials.CreatePublicKeyCredentialResponse
import androidx.credentials.CredentialManager
import androidx.credentials.GetCredentialRequest
import androidx.credentials.GetPublicKeyCredentialOption
import androidx.credentials.PublicKeyCredential
import androidx.credentials.exceptions.CreateCredentialCancellationException
import androidx.credentials.exceptions.CreateCredentialException
import androidx.credentials.exceptions.GetCredentialCancellationException
import androidx.credentials.exceptions.GetCredentialException

/**
 * Passkeys through the Credential Manager (15 8; 08 18; brief M5-03 Design note 2;
 * docs/decisions.d/M5-03.md D-M5-03-3).
 *
 * The app ships no Play services library, so passkeys need the platform provider of API 34
 * and later ([supported]). The Dart side asks only for the relying party of the build's App
 * Links host; the instance lists `android:apk-key-hash:<hash>` in `auth.webauthn_origins`.
 *
 * @param activity returns the activity the system sheet opens over, or null while there is none.
 */
class CredentialManagerPasskeys(private val activity: () -> Activity?) {
    /** Whether the platform provider serves passkeys on this device. */
    fun supported(): Boolean = Build.VERSION.SDK_INT >= MIN_SDK

    /**
     * Gets an assertion for the `PublicKeyCredentialRequestOptions` JSON [requestJson];
     * returns the `PublicKeyCredential` JSON. Throws a [FlutterError] `cancelled`,
     * `unavailable` or `failed`.
     */
    suspend fun assertion(requestJson: String): String {
        val host = ready()
        try {
            val result =
                CredentialManager.create(host).getCredential(
                    host,
                    GetCredentialRequest(listOf(GetPublicKeyCredentialOption(requestJson))),
                )
            val credential = result.credential as? PublicKeyCredential ?: throw FlutterError("failed")
            return credential.authenticationResponseJson
        } catch (_: GetCredentialCancellationException) {
            throw FlutterError("cancelled")
        } catch (_: GetCredentialException) {
            throw FlutterError("failed")
        }
    }

    /**
     * Registers a passkey for the `PublicKeyCredentialCreationOptions` JSON [creationJson];
     * returns the `PublicKeyCredential` JSON. Throws a [FlutterError] `cancelled`,
     * `unavailable` or `failed`.
     */
    suspend fun registration(creationJson: String): String {
        val host = ready()
        try {
            val response =
                CredentialManager.create(host).createCredential(
                    host,
                    CreatePublicKeyCredentialRequest(creationJson),
                )
            val created = response as? CreatePublicKeyCredentialResponse ?: throw FlutterError("failed")
            return created.registrationResponseJson
        } catch (_: CreateCredentialCancellationException) {
            throw FlutterError("cancelled")
        } catch (_: CreateCredentialException) {
            throw FlutterError("failed")
        }
    }

    private fun ready(): Activity {
        if (!supported()) throw FlutterError("unavailable")
        return activity() ?: throw FlutterError("unavailable")
    }

    private companion object {
        const val MIN_SDK = 34
    }
}
