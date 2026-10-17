// SPDX-License-Identifier: Apache-2.0

package devicekey.core

import java.security.MessageDigest
import java.security.Signature
import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Assert.fail
import org.junit.Test

/** The core against the in-memory KeyStoreAccess (brief M5-01, tests in src/test). */
class DeviceKeyCoreTest {
    private val scope = "https://chat.example.com"
    private val texts = PromptTexts(title = "Confirm", subtitle = null, negativeButton = "Cancel")

    /** A prompt host that answers with a fixed outcome and records the request. */
    private class FakePromptHost(var outcome: (Signature) -> PromptOutcome) : PromptHost {
        val requests = mutableListOf<Pair<PromptTexts, Boolean>>()

        override fun authenticate(
            signature: Signature,
            texts: PromptTexts,
            allowDeviceCredential: Boolean,
            callback: (PromptOutcome) -> Unit,
        ) {
            requests += texts to allowDeviceCredential
            callback(outcome(signature))
        }
    }

    private val store = InMemoryKeyStoreAccess()
    private val host = FakePromptHost { PromptOutcome.Authenticated(it) }

    private fun core(sdkInt: Int = 34, stepUpAvailable: Boolean = true, prefix: String = "test.device_key") =
        DeviceKeyCore(
            store = store,
            stepUpSigner = StepUpSigner(host, sdkInt),
            stepUpAvailable = { stepUpAvailable },
            prefix = prefix,
        )

    private fun hex(bytes: ByteArray) = bytes.joinToString("") { "%02x".format(it) }

    private fun sha256Hex(text: String) =
        hex(MessageDigest.getInstance("SHA-256").digest(text.toByteArray(Charsets.UTF_8)))

    private fun signBlocking(core: DeviceKeyCore, purpose: KeyPurpose, data: ByteArray, prompt: PromptTexts? = texts): Result<ByteArray> {
        var result: Result<ByteArray>? = null
        core.sign(scope, purpose, data, prompt) { result = it }
        return result ?: error("callback was not called")
    }

    private fun errorOf(result: Result<ByteArray>): DeviceKeyErrorCode =
        (result.exceptionOrNull() as DeviceKeyException).code

    @Test
    fun aliasPerScopeAndPurpose() {
        val core = core(prefix = "test.device_key")
        val hash = sha256Hex(scope)
        assertEquals("test.device_key.$hash.dpop", core.alias(scope, KeyPurpose.DPOP))
        assertEquals("test.device_key.$hash.step_up", core.alias(scope, KeyPurpose.STEP_UP))
        assertNotEquals(core.alias(scope, KeyPurpose.DPOP), core.alias("https://other.example.com", KeyPurpose.DPOP))
        assertEquals("device_key.$hash.dpop", DeviceKeyCore(store, StepUpSigner(host, 34), { true }).alias(scope, KeyPurpose.DPOP))
    }

    @Test
    fun ensureCreatesOnceAndReportsTheKey() {
        val core = core()
        assertNull(core.keyInfo(scope, KeyPurpose.DPOP))
        val first = core.ensureKey(scope, KeyPurpose.DPOP)
        val again = core.ensureKey(scope, KeyPurpose.DPOP)
        assertEquals(32, first.x.size)
        assertEquals(32, first.y.size)
        assertArrayEquals(first.x, again.x)
        assertFalse(first.hardwareBound)
        assertFalse(first.requiresUserPresence)
        val stepUp = core.ensureKey(scope, KeyPurpose.STEP_UP)
        assertTrue(stepUp.requiresUserPresence)
        assertFalse(stepUp.x.contentEquals(first.x))
        assertEquals(stepUp, core.keyInfo(scope, KeyPurpose.STEP_UP))
    }

    @Test
    fun hardwareBoundFollowsTheSecurityLevel() {
        assertTrue(SecurityLevel.STRONGBOX.hardwareBound)
        assertTrue(SecurityLevel.TRUSTED_ENVIRONMENT.hardwareBound)
        assertFalse(SecurityLevel.SOFTWARE.hardwareBound)
        store.level = SecurityLevel.TRUSTED_ENVIRONMENT
        assertTrue(core().ensureKey(scope, KeyPurpose.DPOP).hardwareBound)
    }

    @Test
    fun dpopSignsWithoutPromptAndVerifies() {
        val core = core()
        val info = core.ensureKey(scope, KeyPurpose.DPOP)
        val data = "header.payload".toByteArray()
        val raw = signBlocking(core, KeyPurpose.DPOP, data, prompt = null).getOrThrow()
        assertEquals(64, raw.size)
        assertTrue(host.requests.isEmpty())
        val verifier = Signature.getInstance("SHA256withECDSA")
        verifier.initVerify(store.publicKey(core.alias(scope, KeyPurpose.DPOP)))
        verifier.update(data)
        assertTrue(verifier.verify(EcdsaSignatures.rawToDer(raw)))
        assertEquals(32, info.x.size)
        assertEquals(64, core.signDpop(scope, data).size)
    }

    @Test
    fun stepUpAsksThePromptHostWithThePlatformRule() {
        core(sdkInt = 30).ensureKey(scope, KeyPurpose.STEP_UP)
        assertEquals(64, signBlocking(core(sdkInt = 30), KeyPurpose.STEP_UP, byteArrayOf(1)).getOrThrow().size)
        assertEquals(64, signBlocking(core(sdkInt = 29), KeyPurpose.STEP_UP, byteArrayOf(1)).getOrThrow().size)
        // API 30 and later: biometrics or the device credential; before: biometrics only.
        assertEquals(listOf(texts to true, texts to false), host.requests)
    }

    @Test
    fun stepUpWithoutPromptTextsIsInternal() {
        val core = core()
        core.ensureKey(scope, KeyPurpose.STEP_UP)
        assertEquals(DeviceKeyErrorCode.INTERNAL, errorOf(signBlocking(core, KeyPurpose.STEP_UP, byteArrayOf(1), prompt = null)))
        assertTrue(host.requests.isEmpty())
    }

    @Test
    fun promptErrorsMapToStableCodes() {
        val core = core()
        core.ensureKey(scope, KeyPurpose.STEP_UP)
        host.outcome = { PromptOutcome.Failed(DeviceKeyErrorCode.USER_CANCELLED) }
        assertEquals(DeviceKeyErrorCode.USER_CANCELLED, errorOf(signBlocking(core, KeyPurpose.STEP_UP, byteArrayOf(1))))
        host.outcome = { PromptOutcome.Failed(DeviceKeyErrorCode.NOT_AVAILABLE) }
        assertEquals(DeviceKeyErrorCode.NOT_AVAILABLE, errorOf(signBlocking(core, KeyPurpose.STEP_UP, byteArrayOf(1))))
    }

    @Test
    fun stepUpNotAvailableWithoutAnAuthenticator() {
        val core = core(stepUpAvailable = false)
        try {
            core.ensureKey(scope, KeyPurpose.STEP_UP)
            fail("expected not_available")
        } catch (e: DeviceKeyException) {
            assertEquals(DeviceKeyErrorCode.NOT_AVAILABLE, e.code)
        }
        // The DPoP key needs no authenticator.
        core.ensureKey(scope, KeyPurpose.DPOP)
    }

    @Test
    fun missingKeyIsNotAvailable() {
        val core = core()
        assertEquals(DeviceKeyErrorCode.NOT_AVAILABLE, errorOf(signBlocking(core, KeyPurpose.DPOP, byteArrayOf(1))))
        assertEquals(DeviceKeyErrorCode.NOT_AVAILABLE, errorOf(signBlocking(core, KeyPurpose.STEP_UP, byteArrayOf(1))))
    }

    @Test
    fun deleteRemovesBothKeysOfTheScopeOnly() {
        val core = core()
        core.ensureKey(scope, KeyPurpose.DPOP)
        core.ensureKey(scope, KeyPurpose.STEP_UP)
        core.ensureKey("https://other.example.com", KeyPurpose.DPOP)
        core.deleteKeys(scope)
        assertNull(core.keyInfo(scope, KeyPurpose.DPOP))
        assertNull(core.keyInfo(scope, KeyPurpose.STEP_UP))
        assertTrue(core.keyInfo("https://other.example.com", KeyPurpose.DPOP) != null)
        core.deleteKeys(scope)
    }

    @Test
    fun attestationOnlyWhenTheCallCreatesTheKey() {
        val core = core()
        store.chain = listOf(byteArrayOf(0x30, 0x00), byteArrayOf(0x30, 0x01, 0x05))
        val challenge = byteArrayOf(9, 9, 9)
        val created = core.attestation(scope, KeyPurpose.DPOP, challenge)
        assertArrayEquals(byteArrayOf(0x30, 0x00, 0x30, 0x01, 0x05), created)
        assertArrayEquals(challenge, store.lastChallenge)
        // The key exists now: Android binds the challenge at generation, so there is none.
        assertNull(core.attestation(scope, KeyPurpose.DPOP, challenge))
        // A chain without attestation (one self-signed certificate) is no attestation either.
        store.chain = listOf(byteArrayOf(0x30, 0x00))
        assertNull(core.attestation(scope, KeyPurpose.STEP_UP, challenge))
        assertTrue(core.keyInfo(scope, KeyPurpose.STEP_UP) != null)
    }
}
