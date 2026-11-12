// SPDX-License-Identifier: Apache-2.0

package devicekey.core

import java.util.concurrent.Executor
import java.util.concurrent.Executors

/**
 * Runs the calls of the plugin channel that create, read or delete keys off the calling thread
 * (docs/decisions.d/AUD-pkg-device-key-01.md).
 *
 * The plugin is called on the platform thread, which draws the app. A StrongBox key can take
 * seconds to create and every other call talks to the Keystore, so none of them works there: each
 * runs on [executor], one at a time and in the order of the calls, as they ran on the platform
 * thread before, and answers through its callback on that executor.
 *
 * ```kotlin
 * val calls = DeviceKeyCalls(DeviceKeyCore.forContext(context) { activity })
 * calls.ensureKey("https://chat.example.com", KeyPurpose.DPOP) { result -> reply(result) }
 * ```
 *
 * Signatures do not come through here: the DPoP signature must not wait behind the creation of a
 * key, and the step-up signature starts its prompt on the main thread ([DeviceKeyCore.sign]).
 *
 * @param core the key code.
 * @param executor where the calls run; one daemon thread by default. Tests may pass their own.
 */
class DeviceKeyCalls(
    private val core: DeviceKeyCore,
    private val executor: Executor = Executors.newSingleThreadExecutor { task ->
        Thread(task, "device_key.keys").apply { isDaemon = true }
    },
) {
    /**
     * [DeviceKeyCore.ensureKey] on the executor.
     *
     * @param callback receives the key or a [DeviceKeyException], exactly once.
     */
    fun ensureKey(scope: String, purpose: KeyPurpose, callback: (Result<DeviceKeyInfo>) -> Unit) =
        run(callback) { core.ensureKey(scope, purpose) }

    /**
     * [DeviceKeyCore.keyInfo] on the executor.
     *
     * @param callback receives the key, null without one, or a [DeviceKeyException], exactly once.
     */
    fun keyInfo(scope: String, purpose: KeyPurpose, callback: (Result<DeviceKeyInfo?>) -> Unit) =
        run(callback) { core.keyInfo(scope, purpose) }

    /**
     * [DeviceKeyCore.attestation] on the executor; it creates the key it attests.
     *
     * @param callback receives the certificate chain, null without one, or a [DeviceKeyException],
     *   exactly once.
     */
    fun attestation(scope: String, purpose: KeyPurpose, challenge: ByteArray, callback: (Result<ByteArray?>) -> Unit) =
        run(callback) { core.attestation(scope, purpose, challenge) }

    /**
     * [DeviceKeyCore.deleteKeys] on the executor.
     *
     * @param callback receives the success or a [DeviceKeyException], exactly once.
     */
    fun deleteKeys(scope: String, callback: (Result<Unit>) -> Unit) = run(callback) { core.deleteKeys(scope) }

    private fun <T> run(callback: (Result<T>) -> Unit, body: () -> T) {
        executor.execute { callback(runCatching(body)) }
    }
}
