// SPDX-License-Identifier: Apache-2.0

package devicekey.core

import android.os.Build
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyInfo
import android.security.keystore.KeyProperties
import android.security.keystore.StrongBoxUnavailableException
import java.security.KeyFactory
import java.security.KeyPairGenerator
import java.security.KeyStore
import java.security.PrivateKey
import java.security.Signature
import java.security.interfaces.ECPublicKey
import java.security.spec.ECGenParameterSpec

/**
 * The [KeyStoreAccess] of the Android Keystore (15 9 row Android, brief M5-01 Design note 2).
 *
 * Keys are P-256 signing keys with digest SHA-256 and never leave the Keystore. StrongBox is tried
 * first from API 28; on [StrongBoxUnavailableException] the key is created again in the trusted
 * execution environment, and where there is none the Keystore keeps a software key, which
 * [securityLevel] reports.
 *
 * @param sdkInt the device's API level; tests pass another one to reach the older branches.
 * @param preferStrongBox whether StrongBox is tried at all; tests switch it off.
 */
class AndroidKeyStoreAccess(
    private val sdkInt: Int = Build.VERSION.SDK_INT,
    private val preferStrongBox: Boolean = true,
) : KeyStoreAccess {
    private val keyStore: KeyStore by lazy { KeyStore.getInstance(PROVIDER).apply { load(null) } }

    override fun publicKey(alias: String): ECPublicKey? = keyStore.getCertificate(alias)?.publicKey as ECPublicKey?

    override fun generate(request: KeyRequest): ECPublicKey {
        // Below API 28 setIsStrongBoxBacked does not exist: an SDK check, never a caught
        // NoSuchMethodError (Design note 2).
        if (preferStrongBox && sdkInt >= Build.VERSION_CODES.P) {
            try {
                return generate(spec(request, strongBox = true))
            } catch (_: StrongBoxUnavailableException) {
                // No secure element on this device: the trusted execution environment follows.
            }
        }
        return generate(spec(request, strongBox = false))
    }

    private fun generate(spec: KeyGenParameterSpec): ECPublicKey {
        val generator = KeyPairGenerator.getInstance(KeyProperties.KEY_ALGORITHM_EC, PROVIDER)
        generator.initialize(spec)
        return generator.generateKeyPair().public as ECPublicKey
    }

    /**
     * The parameters of one key (Design note 2): `dpop` without user authentication; `step_up`
     * with authentication for every use and invalidated by new biometrics.
     */
    @Suppress("DEPRECATION") // setUserAuthenticationValidityDurationSeconds is the API 26 to 29 path.
    internal fun spec(request: KeyRequest, strongBox: Boolean): KeyGenParameterSpec {
        val builder =
            KeyGenParameterSpec.Builder(request.alias, KeyProperties.PURPOSE_SIGN)
                .setAlgorithmParameterSpec(ECGenParameterSpec("secp256r1"))
                .setDigests(KeyProperties.DIGEST_SHA256)
        if (request.purpose == KeyPurpose.STEP_UP) {
            builder.setUserAuthenticationRequired(true)
            if (sdkInt >= Build.VERSION_CODES.R) {
                builder.setUserAuthenticationParameters(
                    0,
                    KeyProperties.AUTH_BIOMETRIC_STRONG or KeyProperties.AUTH_DEVICE_CREDENTIAL,
                )
            } else {
                // -1: every use needs a fresh biometric authentication (D-M5-01-3).
                builder.setUserAuthenticationValidityDurationSeconds(-1)
            }
            builder.setInvalidatedByBiometricEnrollment(true)
        }
        request.attestationChallenge?.let { builder.setAttestationChallenge(it) }
        if (strongBox && sdkInt >= Build.VERSION_CODES.P) builder.setIsStrongBoxBacked(true)
        return builder.build()
    }

    override fun securityLevel(alias: String): SecurityLevel {
        val info = keyInfo(alias)
        if (sdkInt >= Build.VERSION_CODES.S) {
            return when (info.securityLevel) {
                KeyProperties.SECURITY_LEVEL_STRONGBOX -> SecurityLevel.STRONGBOX
                KeyProperties.SECURITY_LEVEL_TRUSTED_ENVIRONMENT -> SecurityLevel.TRUSTED_ENVIRONMENT
                else -> SecurityLevel.SOFTWARE
            }
        }
        @Suppress("DEPRECATION") // isInsideSecureHardware is the API 26 to 30 path.
        return if (info.isInsideSecureHardware) SecurityLevel.TRUSTED_ENVIRONMENT else SecurityLevel.SOFTWARE
    }

    /**
     * @param alias an existing key.
     * @return the Keystore's description of the key (user authentication, security level).
     */
    fun keyInfo(alias: String): KeyInfo {
        val key = privateKey(alias)
        return KeyFactory.getInstance(key.algorithm, PROVIDER).getKeySpec(key, KeyInfo::class.java)
    }

    override fun initSign(alias: String): Signature =
        Signature.getInstance("SHA256withECDSA").apply { initSign(privateKey(alias)) }

    private fun privateKey(alias: String): PrivateKey =
        keyStore.getKey(alias, null) as PrivateKey? ?: throw DeviceKeyException(DeviceKeyErrorCode.NOT_AVAILABLE)

    override fun certificateChain(alias: String): List<ByteArray> =
        keyStore.getCertificateChain(alias)?.map { it.encoded } ?: emptyList()

    override fun delete(alias: String) {
        if (keyStore.containsAlias(alias)) keyStore.deleteEntry(alias)
    }

    companion object {
        /** The JCA provider of the Android Keystore. */
        const val PROVIDER = "AndroidKeyStore"
    }
}
