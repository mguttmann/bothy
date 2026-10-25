// SPDX-License-Identifier: Apache-2.0

package com.example.client.push

import android.app.Notification
import android.util.Log
import androidx.test.runner.AndroidJUnit4
import androidx.work.OutOfQuotaPolicy
import com.example.client.R
import okhttp3.mockwebserver.MockWebServer
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import java.math.BigInteger
import java.security.AlgorithmParameters
import java.security.KeyFactory
import java.security.Signature
import java.security.spec.ECGenParameterSpec
import java.security.spec.ECParameterSpec
import java.security.spec.ECPoint
import java.security.spec.ECPublicKeySpec
import java.util.Base64

/**
 * The push service of brief M5-05 on a device: a constructed FCM `data` map goes through the
 * same receiver the service calls, the preview worker runs with the WorkManager test driver,
 * and the preview comes from a TLS fake server on the device's loopback (no Flutter engine
 * starts; no other network call is made).
 */
@RunWith(AndroidJUnit4::class)
class PushServiceTest {
    private val fixture = PushFixture()
    private val receiver = PushReceiver(fixture.runtime)
    private val texts = PushTexts.of(fixture.context)
    private val thread = "0192f1d2-7c4e-7a10-9a3b-2f6c1e0d4a11"

    @After
    fun tearDown() = fixture.close()

    private fun Notification.title() = extras.getCharSequence(Notification.EXTRA_TITLE).toString()

    private fun Notification.text() = extras.getCharSequence(Notification.EXTRA_TEXT).toString()

    @Test
    fun placeholderPostedImmediately() {
        fixture.signIn()
        val started = System.nanoTime()
        assertEquals(PushReceiver.Handled.SCHEDULED, receiver.onMessage(fixture.data("mention")))
        val shown = fixture.shown(thread)
        Log.i(TAG, "onMessage to placeholder shown: ${(System.nanoTime() - started) / 1_000_000} ms")
        assertNotNull("the placeholder is posted", shown)
        assertEquals("mentions", shown!!.channelId)
        assertEquals(thread, shown.group)
        assertEquals(texts.title, shown.title())
        assertEquals(fixture.context.getString(R.string.push_body_mention), shown.text())
        assertEquals(3, shown.number)
        // No network in onMessageReceived: the fetch is the worker's.
        assertTrue(fixture.received().isEmpty())
        assertEquals(1, fixture.scheduled.size)
    }

    @Test
    fun textReplacesPlaceholderWhenReachable() {
        fixture.signIn()
        receiver.onMessage(fixture.data("mention"))
        fixture.answer(200, PushFixture.PREVIEW)
        val started = System.nanoTime()
        fixture.runWorker(fixture.scheduled.single())
        val shown = fixture.shown(thread) { it?.title() == "tickets · Invoice export fails" }!!
        Log.i(TAG, "worker to text shown: ${(System.nanoTime() - started) / 1_000_000} ms")
        assertEquals("Alex: Sam can you check the export?", shown.text())
        assertEquals(0, fixture.store.takeCounts().placeholders)
    }

    @Test
    fun placeholderStaysWithoutNetwork() {
        fixture.signIn()
        receiver.onMessage(fixture.data("dm"))
        fixture.server.shutdown()
        assertEquals(PreviewOutcome.PLACEHOLDER, PreviewJob(fixture.runtime).run(fixture.scheduled.single()))
        val shown = fixture.shown(thread)!!
        assertEquals(texts.title, shown.title())
        assertEquals(texts.bodyOf("dm"), shown.text())
        assertEquals(1, fixture.store.takeCounts().placeholders)
    }

    @Test
    fun notAvailableOn404() {
        fixture.signIn()
        receiver.onMessage(fixture.data("message"))
        fixture.answer(404, "{\"code\":\"not_found\"}")
        fixture.runWorker(fixture.scheduled.single())
        assertEquals(texts.notAvailable, fixture.shown(thread) { it?.text() == texts.notAvailable }!!.text())
    }

    @Test
    fun previewModeNoneFetchesNothing() {
        fixture.signIn(previewMode = "none")
        assertEquals(PushReceiver.Handled.POSTED, receiver.onMessage(fixture.data("mention")))
        assertTrue(fixture.scheduled.isEmpty())
        // A fetch scheduled before the mode changed fetches nothing either.
        fixture.signIn(previewMode = "full")
        receiver.onMessage(fixture.data("mention"))
        fixture.signIn(previewMode = "none")
        assertEquals(PreviewOutcome.PLACEHOLDER, PreviewJob(fixture.runtime).run(fixture.scheduled.single()))
        assertTrue(fixture.received().isEmpty())
        assertEquals(texts.title, fixture.shown(thread)!!.title())
    }

    @Test
    fun quietPostsSilently() {
        fixture.signIn(previewMode = "none")
        receiver.onMessage(fixture.data("message", thread = "quiet-thread", quiet = true))
        receiver.onMessage(fixture.data("message", thread = "loud-thread"))
        assertEquals(Notification.GROUP_ALERT_SUMMARY, fixture.shown("quiet-thread")!!.groupAlertBehavior)
        assertEquals(Notification.GROUP_ALERT_ALL, fixture.shown("loud-thread")!!.groupAlertBehavior)
    }

    @Test
    fun syncClearsThreads() {
        fixture.signIn(previewMode = "none")
        for (t in listOf("t-1", "t-2", "t-3")) {
            receiver.onMessage(fixture.data("message", thread = t))
            assertNotNull(fixture.shown(t))
        }
        val sync = mapOf("v" to "1", "nid" to "sync-1", "k" to "sync", "i" to fixture.instance, "badge" to "1", "cleared" to "t-1,t-3")
        assertEquals(PushReceiver.Handled.SYNCED, receiver.onMessage(sync))
        assertNull(fixture.shown("t-1") { it == null })
        assertNull(fixture.shown("t-3") { it == null })
        assertEquals(1, fixture.shown("t-2") { it?.number == 1 }!!.number)
        assertTrue("a sync never fetches", fixture.scheduled.isEmpty())
    }

    @Test
    fun previewRequestCarriesNidAndMessageId() {
        fixture.signIn()
        val data = fixture.data("run_done")
        receiver.onMessage(data)
        fixture.answer(200, PushFixture.PREVIEW.replace("\"mention\"", "\"run_done\""))
        fixture.runWorker(fixture.scheduled.single())
        val request = fixture.received().single()
        val url = fixture.server.url(request.path!!)
        assertEquals(data["nid"], url.queryParameter("nid"))
        assertEquals(data["m"], url.queryParameter("message_id"))
        assertEquals("run_done", url.queryParameter("kind"))
        assertEquals("DPoP ${PushFixture.TOKEN}", request.getHeader("Authorization"))
        assertNull("no cookie", request.getHeader("Cookie"))
        // The proof is signed by the dpop device key of the instance and bound to the token.
        val proof = request.getHeader("DPoP")!!.split('.')
        val header = String(Base64.getUrlDecoder().decode(proof[0]))
        val payload = String(Base64.getUrlDecoder().decode(proof[1]))
        assertTrue(payload, payload.contains("\"ath\":\"${DPoPProof.accessTokenHash(PushFixture.TOKEN)}\""))
        assertTrue(payload, payload.contains("\"htu\":\"${fixture.serverUrl}/api/v1/push/preview\""))
        val x = Regex("\"x\":\"([^\"]+)\"").find(header)!!.groupValues[1]
        val y = Regex("\"y\":\"([^\"]+)\"").find(header)!!.groupValues[1]
        assertTrue("the proof verifies", verify("${proof[0]}.${proof[1]}", proof[2], x, y))
    }

    @Test
    fun aNonceIsRetriedOnce() {
        fixture.signIn()
        receiver.onMessage(fixture.data("mention"))
        fixture.answer(401, "{\"code\":\"use_dpop_nonce\"}", "DPoP-Nonce" to "nonce-2", "WWW-Authenticate" to "DPoP error=\"use_dpop_nonce\"")
        fixture.answer(200, PushFixture.PREVIEW)
        fixture.runWorker(fixture.scheduled.single())
        val requests = fixture.received()
        assertEquals(2, requests.size)
        val second = String(Base64.getUrlDecoder().decode(requests[1].getHeader("DPoP")!!.split('.')[1]))
        assertTrue(second, second.contains("\"nonce\":\"nonce-2\""))
        assertEquals("Alex: Sam can you check the export?", fixture.shown(thread) { it?.text() == "Alex: Sam can you check the export?" }!!.text())
    }

    @Test
    fun stalePreviewNeverReplacesNewer() {
        fixture.signIn()
        receiver.onMessage(fixture.data("mention", nid = "nid-old"))
        receiver.onMessage(fixture.data("mention", nid = "nid-new"))
        val (older, newer) = fixture.scheduled
        fixture.answer(200, PushFixture.PREVIEW.replace("Sam can you check the export?", "the newer one"))
        assertEquals(PreviewOutcome.TEXT, PreviewJob(fixture.runtime).run(newer))
        assertNotNull(fixture.shown(thread) { it?.text() == "Alex: the newer one" })
        fixture.answer(200, PushFixture.PREVIEW)
        assertEquals(PreviewOutcome.STALE, PreviewJob(fixture.runtime).run(older))
        assertEquals("Alex: the newer one", fixture.shown(thread)!!.text())
    }

    @Test
    fun lateRunKeepsPlaceholder() {
        fixture.signIn(timeoutMillis = 1_000)
        receiver.onMessage(fixture.data("mention"))
        // The run starts after client.push.fetch_timeout (a non-expedited run that waited).
        fixture.clock.addAndGet(1_001)
        assertEquals(PreviewOutcome.LATE, PreviewJob(fixture.runtime).run(fixture.scheduled.single()))
        assertTrue(fixture.received().isEmpty())
        assertEquals(texts.title, fixture.shown(thread)!!.title())
        assertEquals(1, fixture.store.takeCounts().placeholders)
    }

    @Test
    fun plainHttpOrRedirectSendsNoToken() {
        val plain = MockWebServer()
        val target = MockWebServer()
        try {
            plain.start()
            fixture.signIn(serverUrl = plain.url("/").toString().removeSuffix("/"))
            receiver.onMessage(fixture.data("mention", thread = "t-plain"))
            assertEquals(PreviewOutcome.PLACEHOLDER, PreviewJob(fixture.runtime).run(fixture.scheduled.removeAt(0)))
            assertEquals("an http address gets no request", 0, plain.requestCount)

            target.start()
            fixture.signIn()
            fixture.answer(302, "", "Location" to target.url("/api/v1/push/preview").toString())
            receiver.onMessage(fixture.data("mention", thread = "t-redirect"))
            assertEquals(PreviewOutcome.PLACEHOLDER, PreviewJob(fixture.runtime).run(fixture.scheduled.removeAt(0)))
            assertEquals("the redirect is not followed", 0, target.requestCount)
            assertEquals(texts.title, fixture.shown("t-redirect")!!.title())
        } finally {
            plain.shutdown()
            target.shutdown()
        }
    }

    @Test
    fun theWorkIsExpeditedAndUniquePerNid() {
        fixture.signIn()
        receiver.onMessage(fixture.data("mention", nid = "nid-1"))
        val request = WorkManagerScheduler.workRequest(fixture.scheduled.single())
        assertTrue(request.workSpec.expedited)
        assertEquals(OutOfQuotaPolicy.RUN_AS_NON_EXPEDITED_WORK_REQUEST, request.workSpec.outOfQuotaPolicy)
        assertEquals("push-preview-nid-1", WorkManagerScheduler.uniqueName("nid-1"))
        assertFalse("the input carries no token", request.workSpec.input.keyValueMap.values.any { it.toString().contains("bothy_pvw") })
    }

    private fun verify(input: String, signature: String, x: String, y: String): Boolean {
        val parameters = AlgorithmParameters.getInstance("EC").apply { init(ECGenParameterSpec("secp256r1")) }
        val spec = parameters.getParameterSpec(ECParameterSpec::class.java)
        val point = ECPoint(BigInteger(1, Base64.getUrlDecoder().decode(x)), BigInteger(1, Base64.getUrlDecoder().decode(y)))
        val key = KeyFactory.getInstance("EC").generatePublic(ECPublicKeySpec(point, spec))
        val raw = Base64.getUrlDecoder().decode(signature)
        fun der(part: ByteArray): ByteArray {
            val v = BigInteger(1, part).toByteArray()
            return byteArrayOf(0x02, v.size.toByte()) + v
        }
        val body = der(raw.copyOfRange(0, 32)) + der(raw.copyOfRange(32, 64))
        return Signature.getInstance("SHA256withECDSA").run {
            initVerify(key)
            update(input.toByteArray())
            verify(byteArrayOf(0x30, body.size.toByte()) + body)
        }
    }

    private companion object {
        const val TAG = "PushServiceTest"
    }
}
