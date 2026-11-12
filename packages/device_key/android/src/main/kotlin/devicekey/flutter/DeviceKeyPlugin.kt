// SPDX-License-Identifier: Apache-2.0

package devicekey.flutter

import androidx.fragment.app.FragmentActivity
import devicekey.core.DeviceKeyCalls
import devicekey.core.DeviceKeyCore
import devicekey.core.DeviceKeyErrorCode
import devicekey.core.DeviceKeyException
import devicekey.core.DeviceKeyInfo
import devicekey.core.KeyPurpose
import devicekey.core.PromptTexts
import devicekey.core.forContext
import io.flutter.embedding.engine.plugins.FlutterPlugin
import io.flutter.embedding.engine.plugins.activity.ActivityAware
import io.flutter.embedding.engine.plugins.activity.ActivityPluginBinding
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.suspendCancellableCoroutine
import kotlinx.coroutines.withContext

/**
 * The Flutter plugin `device_key` for Android: connects the Pigeon channel to [DeviceKeyCore]
 * (brief M5-01).
 *
 * The step-up prompt needs the app's activity to be a `FragmentActivity` (in Flutter:
 * `FlutterFragmentActivity`); with another activity, or none, the step-up key reports
 * `not_available`. Every error leaves as a [FlutterError] with a stable code and a fixed text.
 */
class DeviceKeyPlugin : FlutterPlugin, ActivityAware {
    private var activity: FragmentActivity? = null
    private var api: HostApi? = null

    /** Registers the host API on the engine's messenger. */
    override fun onAttachedToEngine(binding: FlutterPlugin.FlutterPluginBinding) {
        val host = HostApi(DeviceKeyCore.forContext(binding.applicationContext) { activity })
        api = host
        DeviceKeyHostApi.setUp(binding.binaryMessenger, host)
    }

    /** Removes the host API. */
    override fun onDetachedFromEngine(binding: FlutterPlugin.FlutterPluginBinding) {
        DeviceKeyHostApi.setUp(binding.binaryMessenger, null)
        api = null
    }

    /** Keeps the activity for the step-up prompt. */
    override fun onAttachedToActivity(binding: ActivityPluginBinding) {
        activity = binding.activity as? FragmentActivity
    }

    /** Drops the activity while it is recreated. */
    override fun onDetachedFromActivityForConfigChanges() {
        activity = null
    }

    /** Keeps the recreated activity. */
    override fun onReattachedToActivityForConfigChanges(binding: ActivityPluginBinding) {
        onAttachedToActivity(binding)
    }

    /** Drops the activity. */
    override fun onDetachedFromActivity() {
        activity = null
    }
}

/**
 * The host side of the Pigeon interface over [core].
 *
 * Pigeon calls every method on the platform thread; none works there. Creating, reading and
 * deleting keys run on the thread of [DeviceKeyCalls], the DPoP signature on a worker, and the
 * step-up signature only starts its prompt here.
 *
 * @param core the key code shared with the push service and the share activity.
 */
internal class HostApi(private val core: DeviceKeyCore) : DeviceKeyHostApi {
    private val calls = DeviceKeyCalls(core)

    override suspend fun ensureKey(scope: String, purpose: KeyPurposeMessage): DeviceKeyInfoMessage =
        bridged { message(answered { calls.ensureKey(scope, purpose.core, it) }) }

    override suspend fun keyInfo(scope: String, purpose: KeyPurposeMessage): DeviceKeyInfoMessage? =
        bridged { answered { calls.keyInfo(scope, purpose.core, it) }?.let(::message) }

    override suspend fun sign(
        scope: String,
        purpose: KeyPurposeMessage,
        data: ByteArray,
        prompt: PromptTextsMessage?,
    ): ByteArray =
        bridged {
            when (purpose) {
                // A StrongBox signature can take a while; keep it off the platform thread.
                KeyPurposeMessage.DPOP -> withContext(Dispatchers.Default) { core.signDpop(scope, data) }
                // The prompt starts on the main thread, where Pigeon calls this method.
                KeyPurposeMessage.STEP_UP ->
                    answered { core.sign(scope, KeyPurpose.STEP_UP, data, prompt?.let(::texts), it) }
            }
        }

    override suspend fun attestation(scope: String, purpose: KeyPurposeMessage, challenge: ByteArray): ByteArray? =
        bridged { answered { calls.attestation(scope, purpose.core, challenge, it) } }

    override suspend fun deleteKeys(scope: String) {
        bridged { answered { calls.deleteKeys(scope, it) } }
    }

    /** Waits for the one answer [start] hands to its callback, without blocking the thread. */
    private suspend fun <T> answered(start: ((Result<T>) -> Unit) -> Unit): T =
        suspendCancellableCoroutine { continuation -> start { continuation.resumeWith(it) } }

    private fun message(info: DeviceKeyInfo) =
        DeviceKeyInfoMessage(
            x = info.x,
            y = info.y,
            hardwareBound = info.hardwareBound,
            requiresUserPresence = info.requiresUserPresence,
        )

    private fun texts(message: PromptTextsMessage) =
        PromptTexts(title = message.title, subtitle = message.subtitle, negativeButton = message.negativeButton)

    /** Runs [body] and turns every failure into a [FlutterError] with the fixed code and text only. */
    private inline fun <T> bridged(body: () -> T): T =
        try {
            body()
        } catch (e: Exception) {
            val code: DeviceKeyErrorCode = DeviceKeyException.classify(e).code
            throw FlutterError(code.wire, code.message, null)
        }
}

/** The purpose in the core's terms. */
private val KeyPurposeMessage.core: KeyPurpose
    get() =
        when (this) {
            KeyPurposeMessage.DPOP -> KeyPurpose.DPOP
            KeyPurposeMessage.STEP_UP -> KeyPurpose.STEP_UP
        }
