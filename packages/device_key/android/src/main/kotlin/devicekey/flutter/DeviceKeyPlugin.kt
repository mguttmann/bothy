// SPDX-License-Identifier: Apache-2.0

package devicekey.flutter

import androidx.fragment.app.FragmentActivity
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
 * @param core the key code shared with the push service and the share activity.
 */
internal class HostApi(private val core: DeviceKeyCore) : DeviceKeyHostApi {
    override fun ensureKey(scope: String, purpose: KeyPurposeMessage): DeviceKeyInfoMessage =
        bridged { message(core.ensureKey(scope, purpose.core)) }

    override fun keyInfo(scope: String, purpose: KeyPurposeMessage): DeviceKeyInfoMessage? =
        bridged { core.keyInfo(scope, purpose.core)?.let(::message) }

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
                    suspendCancellableCoroutine { continuation ->
                        core.sign(scope, KeyPurpose.STEP_UP, data, prompt?.let(::texts)) { continuation.resumeWith(it) }
                    }
            }
        }

    override fun attestation(scope: String, purpose: KeyPurposeMessage, challenge: ByteArray): ByteArray? =
        bridged { core.attestation(scope, purpose.core, challenge) }

    override fun deleteKeys(scope: String) {
        bridged { core.deleteKeys(scope) }
    }

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
