// SPDX-License-Identifier: Apache-2.0

package devicekey.core

import android.content.pm.PackageManager
import android.os.Build
import android.security.keystore.KeyProperties
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import java.io.ByteArrayInputStream
import java.security.KeyStore
import java.security.Signature
import java.security.cert.CertificateFactory
import java.security.cert.X509Certificate
import java.util.UUID
import org.junit.After
import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Assert.fail
import org.junit.Assume.assumeFalse
import org.junit.Assume.assumeTrue
import org.junit.Test
import org.junit.runner.RunWith

/**
 * The core against the real Android Keystore of a device or emulator (brief M5-01, tests in
 * src/androidTest). Every key has a test-only alias prefix and is deleted after each test, also
 * after a failure.
 */
@RunWith(AndroidJUnit4::class)
class DeviceKeyCoreInstrumentedTest {
    private val context = InstrumentationRegistry.getInstrumentation().targetContext
    private val prefix = "test.device_key.${UUID.randomUUID()}"
    private val scope = "https://chat.example.com"
    private val otherScope = "https://other.example.com"
    private val texts = PromptTexts(title = "Confirm", subtitle = null, negativeButton = "Cancel")

    /** A prompt host that hands the signature back without any authentication. */
    private class UnauthenticatedHost : PromptHost {
        var calls = 0

        override fun authenticate(
            signature: Signature,
            texts: PromptTexts,
            allowDeviceCredential: Boolean,
            callback: (PromptOutcome) -> Unit,
        ) {
            calls++
            callback(PromptOutcome.Authenticated(signature))
        }
    }

    /** Reports every key as a software key: the fallback where neither StrongBox nor a TEE exists. */
    private class SoftwareOnly(private val inner: KeyStoreAccess) : KeyStoreAccess by inner {
        override fun securityLevel(alias: String) = SecurityLevel.SOFTWARE
    }

    private val host = UnauthenticatedHost()
    private val store = AndroidKeyStoreAccess()
    private val stepUpAvailable = stepUpAvailability(context)

    private fun core(access: KeyStoreAccess = store) =
        DeviceKeyCore(access, StepUpSigner(host, Build.VERSION.SDK_INT), stepUpAvailable, prefix)

    @After
    fun deleteTestKeys() {
        val keyStore = KeyStore.getInstance(AndroidKeyStoreAccess.PROVIDER).apply { load(null) }
        keyStore.aliases().toList().filter { it.startsWith("$prefix.") }.forEach { keyStore.deleteEntry(it) }
    }

    private fun signBlocking(core: DeviceKeyCore, purpose: KeyPurpose, data: ByteArray): Result<ByteArray> {
        var result: Result<ByteArray>? = null
        core.sign(scope, purpose, data, texts) { result = it }
        return result ?: error("callback was not called")
    }

    private fun verify(core: DeviceKeyCore, purpose: KeyPurpose, data: ByteArray, raw: ByteArray): Boolean {
        val verifier = Signature.getInstance("SHA256withECDSA")
        verifier.initVerify(store.publicKey(core.alias(scope, purpose)))
        verifier.update(data)
        return verifier.verify(EcdsaSignatures.rawToDer(raw))
    }

    private fun assumeStepUp() = assumeTrue("no authenticator for the step-up key on this device", stepUpAvailable())

    @Test
    fun createsTwoDistinctKeysPerScope() {
        assumeStepUp()
        val core = core()
        val dpop = core.ensureKey(scope, KeyPurpose.DPOP)
        val stepUp = core.ensureKey(scope, KeyPurpose.STEP_UP)
        assertFalse(dpop.x.contentEquals(stepUp.x))
        assertFalse(dpop.requiresUserPresence)
        assertTrue(stepUp.requiresUserPresence)
        assertEquals(dpop, core.ensureKey(scope, KeyPurpose.DPOP))
        assertEquals(stepUp, core.keyInfo(scope, KeyPurpose.STEP_UP))
    }

    @Test
    fun scopesAreSeparated() {
        val core = core()
        val first = core.ensureKey(scope, KeyPurpose.DPOP)
        assertNull(core.keyInfo(otherScope, KeyPurpose.DPOP))
        val second = core.ensureKey(otherScope, KeyPurpose.DPOP)
        assertFalse(first.x.contentEquals(second.x))
    }

    @Test
    fun signatureIsRawAndVerifies() {
        val core = core()
        core.ensureKey(scope, KeyPurpose.DPOP)
        val data = "header.payload".toByteArray()
        repeat(16) {
            val raw = signBlocking(core, KeyPurpose.DPOP, data).getOrThrow()
            assertEquals(64, raw.size)
            assertTrue(verify(core, KeyPurpose.DPOP, data, raw))
        }
        assertTrue(verify(core, KeyPurpose.DPOP, data, core.signDpop(scope, data)))
    }

    @Test
    fun dpopSignsWithoutPrompt() {
        val core = core()
        core.ensureKey(scope, KeyPurpose.DPOP)
        val keyInfo = store.keyInfo(core.alias(scope, KeyPurpose.DPOP))
        assertFalse(keyInfo.isUserAuthenticationRequired)
        assertTrue(signBlocking(core, KeyPurpose.DPOP, byteArrayOf(1, 2, 3)).isSuccess)
        assertEquals(0, host.calls)
    }

    @Test
    fun stepUpRequiresAuthentication() {
        assumeStepUp()
        val core = core()
        core.ensureKey(scope, KeyPurpose.STEP_UP)
        val keyInfo = store.keyInfo(core.alias(scope, KeyPurpose.STEP_UP))
        assertTrue(keyInfo.isUserAuthenticationRequired)
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
            assertEquals(0, keyInfo.userAuthenticationValidityDurationSeconds)
            assertEquals(
                KeyProperties.AUTH_BIOMETRIC_STRONG or KeyProperties.AUTH_DEVICE_CREDENTIAL,
                keyInfo.userAuthenticationType,
            )
        } else {
            @Suppress("DEPRECATION")
            assertEquals(-1, keyInfo.userAuthenticationValidityDurationSeconds)
        }
        // The prompt host returns the signature without an authentication: the Keystore refuses.
        val result = signBlocking(core, KeyPurpose.STEP_UP, byteArrayOf(1))
        assertEquals(1, host.calls)
        assertEquals(DeviceKeyErrorCode.NOT_AVAILABLE, (result.exceptionOrNull() as DeviceKeyException).code)
    }

    @Test
    fun stepUpNotAvailableWithoutAnAuthenticator() {
        assumeFalse("this device has an authenticator for the step-up key", stepUpAvailable())
        try {
            core().ensureKey(scope, KeyPurpose.STEP_UP)
            fail("expected not_available")
        } catch (e: DeviceKeyException) {
            assertEquals(DeviceKeyErrorCode.NOT_AVAILABLE, e.code)
        }
    }

    @Test
    fun stepUpInvalidatedByEnrollmentFlag() {
        // The flag of the parameters; the Keystore reports it in KeyInfo only for a key bound to
        // enrolled biometrics, and the real invalidation needs a person enrolling a finger.
        val request = KeyRequest("$prefix.flag.step_up", KeyPurpose.STEP_UP, null)
        assertTrue(store.spec(request, strongBox = false).isInvalidatedByBiometricEnrollment)
        assertTrue(store.spec(request, strongBox = false).isUserAuthenticationRequired)
        val dpop = store.spec(KeyRequest("$prefix.flag.dpop", KeyPurpose.DPOP, null), strongBox = false)
        assertFalse(dpop.isUserAuthenticationRequired)
    }

    @Test
    fun strongBoxWhenAvailable() {
        assumeTrue(
            "strongbox not available on this device",
            context.packageManager.hasSystemFeature(PackageManager.FEATURE_STRONGBOX_KEYSTORE),
        )
        val core = core()
        val info = core.ensureKey(scope, KeyPurpose.DPOP)
        assertTrue(info.hardwareBound)
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) {
            assertEquals(SecurityLevel.STRONGBOX, store.securityLevel(core.alias(scope, KeyPurpose.DPOP)))
        }
    }

    @Test
    fun fallbackLabelledNotHardwareBound() {
        val access = SoftwareOnly(AndroidKeyStoreAccess(preferStrongBox = false))
        val core = core(access)
        val info = core.ensureKey(scope, KeyPurpose.DPOP)
        assertFalse(info.hardwareBound)
        val data = byteArrayOf(4, 5, 6)
        assertTrue(verify(core, KeyPurpose.DPOP, data, core.signDpop(scope, data)))
    }

    @Test
    fun securityLevelMatchesTheKeystore() {
        val core = core()
        val info = core.ensureKey(scope, KeyPurpose.DPOP)
        @Suppress("DEPRECATION")
        assertEquals(store.keyInfo(core.alias(scope, KeyPurpose.DPOP)).isInsideSecureHardware, info.hardwareBound)
    }

    @Test
    fun attestationChainReturned() {
        val core = core()
        val challenge = "challenge-${UUID.randomUUID()}".toByteArray()
        val attestation = core.attestation(scope, KeyPurpose.DPOP, challenge)
        assertNotNull(core.keyInfo(scope, KeyPurpose.DPOP))
        assumeTrue("key attestation not supported on this device", attestation != null)
        val certificates =
            CertificateFactory.getInstance("X.509")
                .generateCertificates(ByteArrayInputStream(attestation))
                .map { it as X509Certificate }
        assertTrue(certificates.size >= 2)
        val leaf = certificates.first()
        assertArrayEquals(store.publicKey(core.alias(scope, KeyPurpose.DPOP))!!.encoded, leaf.publicKey.encoded)
        // The key description extension of Android key attestation carries the challenge.
        val extension = leaf.getExtensionValue("1.3.6.1.4.1.11129.2.1.17")
        assertNotNull(extension)
        assertTrue(String(extension!!, Charsets.ISO_8859_1).contains(String(challenge, Charsets.ISO_8859_1)))
        // An existing key has no fresh attestation.
        assertNull(core.attestation(scope, KeyPurpose.DPOP, challenge))
    }

    @Test
    fun deleteRemovesBothKeys() {
        val core = core()
        core.ensureKey(scope, KeyPurpose.DPOP)
        if (stepUpAvailable()) core.ensureKey(scope, KeyPurpose.STEP_UP)
        core.ensureKey(otherScope, KeyPurpose.DPOP)
        core.deleteKeys(scope)
        assertNull(core.keyInfo(scope, KeyPurpose.DPOP))
        assertNull(core.keyInfo(scope, KeyPurpose.STEP_UP))
        assertNotNull(core.keyInfo(otherScope, KeyPurpose.DPOP))
        val result = signBlocking(core, KeyPurpose.DPOP, byteArrayOf(1))
        assertEquals(DeviceKeyErrorCode.NOT_AVAILABLE, (result.exceptionOrNull() as DeviceKeyException).code)
    }
}
