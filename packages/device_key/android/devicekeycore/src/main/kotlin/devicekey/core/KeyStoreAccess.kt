// SPDX-License-Identifier: Apache-2.0

package devicekey.core

import java.security.KeyPair
import java.security.KeyPairGenerator
import java.security.Signature
import java.security.interfaces.ECPublicKey
import java.security.spec.ECGenParameterSpec

/**
 * What to create: one P-256 signing key.
 *
 * @property alias the Keystore alias (`<prefix>.<sha256(scope)>.<purpose>`).
 * @property purpose decides the user authentication rule (08 6.1).
 * @property attestationChallenge the caller's challenge for key attestation, or null.
 */
class KeyRequest(val alias: String, val purpose: KeyPurpose, val attestationChallenge: ByteArray?)

/**
 * The key storage the core works on: the Android Keystore in the app ([AndroidKeyStoreAccess]),
 * a software map in JVM tests ([InMemoryKeyStoreAccess]).
 *
 * Every method may throw; the core turns errors into [DeviceKeyException]s.
 */
interface KeyStoreAccess {
    /**
     * @param alias the key's alias.
     * @return the public key, or null when there is no key.
     */
    fun publicKey(alias: String): ECPublicKey?

    /**
     * Creates the key [request] describes, replacing nothing: the caller checked it is missing.
     *
     * @return the new public key.
     */
    fun generate(request: KeyRequest): ECPublicKey

    /**
     * @param alias an existing key.
     * @return where its private key lives.
     */
    fun securityLevel(alias: String): SecurityLevel

    /**
     * @param alias an existing key.
     * @return a `SHA256withECDSA` signature object initialised with the private key; for the
     *   step-up key it still needs the user's authentication before `sign`.
     */
    fun initSign(alias: String): Signature

    /**
     * @param alias an existing key.
     * @return the DER certificates of the key, leaf first; empty when there are none.
     */
    fun certificateChain(alias: String): List<ByteArray>

    /** Deletes the key; a missing key is not an error. */
    fun delete(alias: String)
}

/**
 * A [KeyStoreAccess] that keeps software keys in memory, for tests on the JVM and in the app's
 * tests. Its keys are never hardware-bound unless a test sets [level].
 */
class InMemoryKeyStoreAccess : KeyStoreAccess {
    private val keys = mutableMapOf<String, KeyPair>()

    /** The level every key reports. */
    var level: SecurityLevel = SecurityLevel.SOFTWARE

    /** The certificate chain every key reports. */
    var chain: List<ByteArray> = emptyList()

    /** The attestation challenge of the last [generate] call. */
    var lastChallenge: ByteArray? = null
        private set

    override fun publicKey(alias: String): ECPublicKey? = keys[alias]?.public as ECPublicKey?

    override fun generate(request: KeyRequest): ECPublicKey {
        val generator = KeyPairGenerator.getInstance("EC")
        generator.initialize(ECGenParameterSpec("secp256r1"))
        val pair = generator.generateKeyPair()
        keys[request.alias] = pair
        lastChallenge = request.attestationChallenge
        return pair.public as ECPublicKey
    }

    override fun securityLevel(alias: String): SecurityLevel = level

    override fun initSign(alias: String): Signature {
        val pair = keys[alias] ?: throw DeviceKeyException(DeviceKeyErrorCode.NOT_AVAILABLE)
        return Signature.getInstance("SHA256withECDSA").apply { initSign(pair.private) }
    }

    override fun certificateChain(alias: String): List<ByteArray> = chain

    override fun delete(alias: String) {
        keys.remove(alias)
    }
}
