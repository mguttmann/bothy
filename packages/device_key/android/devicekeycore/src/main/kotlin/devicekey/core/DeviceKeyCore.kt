// SPDX-License-Identifier: Apache-2.0

package devicekey.core

import java.security.MessageDigest

/**
 * Creates, uses and deletes the two device keys per server (08 6.1, D-L10): `dpop` signs without
 * the user, `step_up` asks for every signature.
 *
 * The Flutter plugin, the push service (M5-05) and the share activity (M5-06) all go through this
 * class, so the keys have one implementation (15 9). Every method reports failures as
 * [DeviceKeyException] with a stable code and no key material.
 *
 * ```kotlin
 * val core = DeviceKeyCore.forContext(context) { activity as? FragmentActivity }
 * val signature = core.signDpop("https://chat.example.com", signingInput)
 * ```
 *
 * @param store the key storage.
 * @param stepUpSigner runs the prompt for the step-up key.
 * @param stepUpAvailable whether the device has an authenticator the step-up key accepts right
 *   now; without it a missing step-up key cannot be created (`not_available`, D-M5-01-3).
 * @param prefix the first segment of every alias; tests use their own.
 */
class DeviceKeyCore(
    private val store: KeyStoreAccess,
    private val stepUpSigner: StepUpSigner,
    private val stepUpAvailable: () -> Boolean,
    private val prefix: String = DEFAULT_PREFIX,
) {
    /**
     * The Keystore alias `<prefix>.<sha256(scope)>.<purpose>` (lowercase hex), so a server address
     * never appears in the Keystore.
     *
     * @param scope separates the keys per server, for example its origin.
     * @param purpose the key's purpose.
     * @return the alias.
     */
    fun alias(scope: String, purpose: KeyPurpose): String {
        val digest = MessageDigest.getInstance("SHA-256").digest(scope.toByteArray(Charsets.UTF_8))
        return "$prefix.${digest.joinToString("") { "%02x".format(it) }}.${purpose.wire}"
    }

    /**
     * Returns the key of [purpose] for [scope], creating it if it is missing.
     *
     * @throws DeviceKeyException `not_available` for a missing step-up key on a device without an
     *   authenticator the key accepts.
     */
    fun ensureKey(scope: String, purpose: KeyPurpose): DeviceKeyInfo =
        guarded {
            info(alias(scope, purpose), purpose) ?: create(KeyRequest(alias(scope, purpose), purpose, null))
        }

    /**
     * Returns the key of [purpose] for [scope], or null; never creates one.
     *
     * @throws DeviceKeyException on a Keystore failure.
     */
    fun keyInfo(scope: String, purpose: KeyPurpose): DeviceKeyInfo? = guarded { info(alias(scope, purpose), purpose) }

    /**
     * Signs [data] with the DPoP key of [scope], synchronously and without the user.
     *
     * @return `R || S`, 64 bytes.
     * @throws DeviceKeyException `not_available` when the key is missing.
     */
    fun signDpop(scope: String, data: ByteArray): ByteArray =
        guarded {
            val signature = initSign(alias(scope, KeyPurpose.DPOP))
            signature.update(data)
            EcdsaSignatures.derToRaw(signature.sign())
        }

    /**
     * Signs [data] with the key of [purpose] for [scope]. The step-up key shows the prompt first,
     * so this call must start on the main thread; the DPoP key answers at once.
     *
     * @param prompt the prompt texts; required for the step-up key (`internal` without them).
     * @param callback receives `R || S` or a [DeviceKeyException], exactly once.
     */
    fun sign(
        scope: String,
        purpose: KeyPurpose,
        data: ByteArray,
        prompt: PromptTexts?,
        callback: (Result<ByteArray>) -> Unit,
    ) {
        if (purpose == KeyPurpose.DPOP) {
            callback(runCatching { signDpop(scope, data) })
            return
        }
        val signature =
            try {
                val texts = prompt ?: throw DeviceKeyException(DeviceKeyErrorCode.INTERNAL)
                initSign(alias(scope, purpose)) to texts
            } catch (e: Exception) {
                callback(Result.failure(DeviceKeyException.classify(e)))
                return
            }
        stepUpSigner.sign(signature.first, data, signature.second, callback)
    }

    /**
     * Returns the attestation of the key of [purpose] for [scope] over [challenge]: the DER
     * certificates of the key, leaf first, concatenated (D-M5-01-2).
     *
     * Android binds the challenge when it creates the key, so there is an attestation only when
     * this call creates the key; for an existing key, and where the device does not attest keys
     * (a chain of one self-signed certificate), the result is null. The app therefore asks for the
     * attestation before [ensureKey] when it registers.
     *
     * @throws DeviceKeyException as [ensureKey].
     */
    fun attestation(scope: String, purpose: KeyPurpose, challenge: ByteArray): ByteArray? =
        guarded {
            val alias = alias(scope, purpose)
            if (store.publicKey(alias) != null) return@guarded null
            create(KeyRequest(alias, purpose, challenge.copyOf()))
            val chain = store.certificateChain(alias)
            if (chain.size < 2) null else chain.reduce { acc, cert -> acc + cert }
        }

    /**
     * Deletes both keys of [scope]; missing keys are not an error. There is no key change (08 5.4):
     * the app registers as a new device afterwards.
     */
    fun deleteKeys(scope: String) {
        guarded { KeyPurpose.entries.forEach { store.delete(alias(scope, it)) } }
    }

    private fun create(request: KeyRequest): DeviceKeyInfo {
        if (request.purpose == KeyPurpose.STEP_UP && !stepUpAvailable()) {
            throw DeviceKeyException(DeviceKeyErrorCode.NOT_AVAILABLE)
        }
        store.generate(request)
        return info(request.alias, request.purpose) ?: throw DeviceKeyException(DeviceKeyErrorCode.INTERNAL)
    }

    private fun info(alias: String, purpose: KeyPurpose): DeviceKeyInfo? {
        val key = store.publicKey(alias) ?: return null
        return DeviceKeyInfo(
            x = EcdsaSignatures.coordinate(key.w.affineX),
            y = EcdsaSignatures.coordinate(key.w.affineY),
            hardwareBound = store.securityLevel(alias).hardwareBound,
            requiresUserPresence = purpose == KeyPurpose.STEP_UP,
        )
    }

    private fun initSign(alias: String) =
        if (store.publicKey(alias) == null) throw DeviceKeyException(DeviceKeyErrorCode.NOT_AVAILABLE) else store.initSign(alias)

    private inline fun <T> guarded(body: () -> T): T =
        try {
            body()
        } catch (e: Exception) {
            throw DeviceKeyException.classify(e)
        }

    companion object {
        /** The alias prefix of the app's keys. */
        const val DEFAULT_PREFIX = "device_key"
    }
}
