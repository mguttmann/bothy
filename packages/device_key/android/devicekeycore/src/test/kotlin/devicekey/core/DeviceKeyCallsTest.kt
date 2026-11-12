// SPDX-License-Identifier: Apache-2.0

package devicekey.core

import java.security.interfaces.ECPublicKey
import java.util.Collections
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotSame
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * The calls of the plugin channel that create, read or delete keys (docs/decisions.d/
 * AUD-pkg-device-key-01.md): none of them works on the thread that calls, which in the app is the
 * platform thread.
 */
class DeviceKeyCallsTest {
    private val scope = "https://chat.example.com"

    /** A store whose key generation takes a while, as StrongBox does. */
    private class SlowStore(private val inner: InMemoryKeyStoreAccess = InMemoryKeyStoreAccess()) :
        KeyStoreAccess by inner {
        val started = CountDownLatch(1)
        val release = CountDownLatch(1)
        val generated: MutableList<String> = Collections.synchronizedList(mutableListOf<String>())

        override fun generate(request: KeyRequest): ECPublicKey {
            started.countDown()
            // Bounded, so a version that creates the key on the calling thread fails instead of hanging.
            release.await(3, TimeUnit.SECONDS)
            generated += request.alias
            return inner.generate(request)
        }
    }

    private val store = SlowStore()
    private val core =
        DeviceKeyCore(
            store = store,
            stepUpSigner = StepUpSigner(
                object : PromptHost {
                    override fun authenticate(
                        signature: java.security.Signature,
                        texts: PromptTexts,
                        allowDeviceCredential: Boolean,
                        callback: (PromptOutcome) -> Unit,
                    ) = callback(PromptOutcome.Authenticated(signature))
                },
                34,
            ),
            stepUpAvailable = { true },
            prefix = "test.device_key",
        )
    private val calls = DeviceKeyCalls(core)

    @Test
    fun creatingAKeyDoesNotHoldTheCallingThread() {
        val caller = Thread.currentThread()
        var answeredOn: Thread? = null
        var result: Result<DeviceKeyInfo>? = null
        val answered = CountDownLatch(1)

        calls.ensureKey(scope, KeyPurpose.DPOP) {
            answeredOn = Thread.currentThread()
            result = it
            answered.countDown()
        }

        // The call came back while the key is still being created.
        assertEquals("the call waited for the key on the calling thread", 1L, answered.count)
        assertTrue(store.started.await(5, TimeUnit.SECONDS))
        store.release.countDown()
        assertTrue(answered.await(5, TimeUnit.SECONDS))
        assertNotSame(caller, answeredOn)
        assertEquals(32, result!!.getOrThrow().x.size)
    }

    @Test
    fun callsRunOneAtATimeInTheOrderOfTheCalls() {
        store.release.countDown()
        val order: MutableList<String> = Collections.synchronizedList(mutableListOf<String>())
        val done = CountDownLatch(4)
        calls.ensureKey(scope, KeyPurpose.DPOP) { order += "ensure dpop ${it.isSuccess}"; done.countDown() }
        calls.keyInfo(scope, KeyPurpose.DPOP) { order += "info ${it.getOrNull() != null}"; done.countDown() }
        calls.deleteKeys(scope) { order += "delete ${it.isSuccess}"; done.countDown() }
        calls.keyInfo(scope, KeyPurpose.DPOP) { order += "info ${it.getOrNull() != null}"; done.countDown() }
        assertTrue(done.await(5, TimeUnit.SECONDS))
        assertEquals(listOf("ensure dpop true", "info true", "delete true", "info false"), order)
    }

    @Test
    fun attestationAndFailuresArriveThroughTheCallback() {
        store.release.countDown()
        val done = CountDownLatch(2)
        var attestation: Result<ByteArray?>? = null
        var failure: Result<DeviceKeyInfo>? = null
        calls.attestation(scope, KeyPurpose.DPOP, ByteArray(16)) { attestation = it; done.countDown() }
        val unavailable = DeviceKeyCalls(
            DeviceKeyCore(store, StepUpSigner(NoPrompt, 34), stepUpAvailable = { false }, prefix = "test.device_key"),
        )
        unavailable.ensureKey(scope, KeyPurpose.STEP_UP) { failure = it; done.countDown() }
        assertTrue(done.await(5, TimeUnit.SECONDS))
        // The in-memory store attests nothing: the key is created, the chain is empty.
        assertNull(attestation!!.getOrThrow())
        assertEquals(1, store.generated.size)
        assertEquals(
            DeviceKeyErrorCode.NOT_AVAILABLE,
            (failure!!.exceptionOrNull() as DeviceKeyException).code,
        )
    }

    private object NoPrompt : PromptHost {
        override fun authenticate(
            signature: java.security.Signature,
            texts: PromptTexts,
            allowDeviceCredential: Boolean,
            callback: (PromptOutcome) -> Unit,
        ) = callback(PromptOutcome.Failed(DeviceKeyErrorCode.NOT_AVAILABLE))
    }
}
