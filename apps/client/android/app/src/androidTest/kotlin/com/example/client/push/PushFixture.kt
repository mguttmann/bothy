// SPDX-License-Identifier: Apache-2.0

package com.example.client.push

import android.Manifest
import android.app.Notification
import android.app.NotificationManager
import android.content.Context
import android.os.Build
import androidx.test.platform.app.InstrumentationRegistry
import androidx.work.testing.TestListenableWorkerBuilder
import com.example.client.AndroidNotificationChannel
import com.example.client.NotificationChannelRegistrar
import com.example.client.Patience
import devicekey.core.DeviceKeyCore
import devicekey.core.KeyPurpose
import devicekey.core.forContext
import kotlinx.coroutines.runBlocking
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import okhttp3.mockwebserver.RecordedRequest
import okhttp3.tls.HandshakeCertificates
import okhttp3.tls.HeldCertificate
import org.junit.Assert.assertTrue
import java.net.InetAddress
import java.util.UUID
import java.util.concurrent.atomic.AtomicLong
import javax.net.ssl.HttpsURLConnection

/**
 * The world of the instrumented push tests (brief M5-05 Tests): a TLS fake server on the
 * device's loopback with a CA made for this run that only these tests trust, the real `dpop`
 * key of a fresh instance through `DeviceKeyCore`, an encrypted store of its own, a recording
 * scheduler, a clock the test turns and the four channels of M5-03. [close] removes all of it.
 */
class PushFixture : AutoCloseable {
    val context: Context = InstrumentationRegistry.getInstrumentation().targetContext
    val instance: String = UUID.randomUUID().toString()
    // Starts at the device's time, because the system drops a notification whose `when` lies
    // weeks back; from there only the test moves it.
    val clock = AtomicLong(System.currentTimeMillis())
    val scheduled = mutableListOf<PreviewRequest>()
    private val handed = mutableListOf<Pair<String, Notification>>()
    val manager: NotificationManager = context.getSystemService(NotificationManager::class.java)

    private val root = HeldCertificate.Builder().certificateAuthority(0).commonName("push test CA").build()
    private val leaf =
        HeldCertificate.Builder()
            .signedBy(root)
            .addSubjectAlternativeName("localhost")
            .build()
    private val serverCertificates = HandshakeCertificates.Builder().heldCertificate(leaf, root.certificate).build()
    private val clientCertificates = HandshakeCertificates.Builder().addTrustedCertificate(root.certificate).build()
    private val keys = DeviceKeyCore.forContext(context) { null }
    private val preferences = "push_store_test_${UUID.randomUUID()}"
    val server = MockWebServer().apply { useHttps(serverCertificates.sslSocketFactory(), false) }
    val store = PushStore(SharedPreferencesValues(context.getSharedPreferences(preferences, Context.MODE_PRIVATE)), KeystoreCipher(ALIAS))
    private val requests = mutableListOf<RecordedRequest>()

    /** Trusts only this run's CA, with the platform's host name check. */
    val connector =
        HttpsConnector { url ->
            (url.openConnection() as HttpsURLConnection).apply { sslSocketFactory = clientCertificates.sslSocketFactory() }
        }

    val runtime =
        PushRuntime(
            context = context,
            store = store,
            scheduler = { scheduled += it },
            signer = { scope -> DeviceKeyProofSigner.open(keys, scope) },
            connector = connector,
            clock = { clock.get() },
            posted = { tag, notification -> synchronized(handed) { handed += tag to notification } },
        )

    init {
        if (Build.VERSION.SDK_INT >= 33) {
            InstrumentationRegistry.getInstrumentation().uiAutomation
                .grantRuntimePermission(context.packageName, Manifest.permission.POST_NOTIFICATIONS)
        }
        NotificationChannelRegistrar(context).register(
            listOf(
                AndroidNotificationChannel("mentions", "Mentions", "Mentions", true, listOf("mention", "dm")),
                AndroidNotificationChannel("questions", "Questions", "Questions", true, listOf("input_needed", "security", "admin")),
                AndroidNotificationChannel("messages", "All messages", "All messages", false, listOf("message")),
                AndroidNotificationChannel("runs", "Runs", "Runs", false, listOf("run_done", "run_failed")),
            ),
        )
        keys.ensureKey(instance, KeyPurpose.DPOP)
        server.start(InetAddress.getByName("localhost"), 0)
        PushRuntime.install(runtime)
        clearNotifications()
    }

    /** The server address as discovery gives it. */
    val serverUrl: String get() = server.url("/").toString().removeSuffix("/")

    /** Signs in: the registration and the settings the app hands over. */
    fun signIn(previewMode: String = "full", timeoutMillis: Long = 20_000, serverUrl: String = this.serverUrl) {
        store.saveRegistration(instance, PushRegistration(serverUrl, "reg-1", TOKEN))
        store.writeSettings(instance, previewMode, timeoutMillis)
    }

    /** The FCM `data` of 13 6.2 for [thread]. */
    fun data(kind: String = "mention", thread: String = "0192f1d2-7c4e-7a10-9a3b-2f6c1e0d4a11", nid: String = UUID.randomUUID().toString(), quiet: Boolean = false) =
        mapOf(
            "v" to "1",
            "nid" to nid,
            "k" to kind,
            "i" to instance,
            "t" to thread,
            "m" to "0192f1d3-0a44-7f00-b1c2-d3e4f5a6b7c8",
            "badge" to "3",
            "quiet" to if (quiet) "1" else "0",
        )

    /** Answers the next request with [status] and [body]. */
    fun answer(status: Int, body: String = "", vararg headers: Pair<String, String>) {
        val response = MockResponse().setResponseCode(status).setBody(body)
        for ((name, value) in headers) response.addHeader(name, value)
        server.enqueue(response)
    }

    /** Runs the [PreviewWorker] of [request] with the WorkManager test driver, to its end. */
    fun runWorker(request: PreviewRequest) {
        val worker = TestListenableWorkerBuilder<PreviewWorker>(context).setInputData(request.toData()).build()
        runBlocking { worker.doWork() }
    }

    /**
     * The shown push notification of [tag] once [until] holds for it (by default: it is shown).
     * `notify` and `cancel` reach the system asynchronously, so the test waits for the state it
     * expects, bounded only by [com.example.client.Patience].
     */
    fun shown(tag: String, until: (Notification?) -> Boolean = { it != null }): Notification? {
        var found: Notification? = null
        Patience.until("notification $tag") {
            found = manager.activeNotifications.firstOrNull { it.tag == tag && it.id == NotificationPoster.ID }?.notification
            until(found)
        }
        return found
    }

    /**
     * The last notification of [tag] the poster handed to the system. An update can be shed by
     * the system's enqueue rate limit when tests post quickly (D-M5-05-2), so the tests read
     * replacements here and the drawer only for first posts and cancels.
     */
    fun handed(tag: String): Notification? = synchronized(handed) { handed.lastOrNull { it.first == tag }?.second }

    /** Every request the server received so far. */
    fun received(): List<RecordedRequest> {
        while (true) requests += server.takeRequest(0, java.util.concurrent.TimeUnit.MILLISECONDS) ?: break
        return requests.toList()
    }

    override fun close() {
        // The push service never calls anything but the preview endpoint (08 7.4: no refresh,
        // no login, no session call outside the app).
        for (request in received()) {
            assertTrue(request.path.orEmpty(), request.path.orEmpty().startsWith("/api/v1/push/preview?"))
        }
        PushRuntime.install(null)
        clearNotifications()
        server.shutdown()
        keys.deleteKeys(instance)
        context.deleteSharedPreferences(preferences)
        KeystoreCipher(ALIAS).deleteKey()
    }

    private fun clearNotifications() {
        for (shown in manager.activeNotifications) if (shown.id == NotificationPoster.ID) manager.cancel(shown.tag, shown.id)
        Patience.until("no push notification is shown") { manager.activeNotifications.none { it.id == NotificationPoster.ID } }
    }

    companion object {
        /** The preview token of the tests (the example of the API documentation). */
        const val TOKEN = "bothy_pvw_EXAMPLEpushpreviewtoken00000000000000000000_4Z1L0A"
        private const val ALIAS = "push_store_test"

        /** A `full` preview of the API documentation's example. */
        const val PREVIEW =
            "{\"kind\":\"mention\",\"thread_id\":\"0192f1d2-7c4e-7a10-9a3b-2f6c1e0d4a11\"," +
                "\"message_id\":\"0192f1d3-0a44-7f00-b1c2-d3e4f5a6b7c8\",\"badge\":3,\"space_name\":\"Support\"," +
                "\"channel_name\":\"tickets\",\"thread_title\":\"Invoice export fails\",\"sender_display_name\":\"Alex\"," +
                "\"excerpt\":\"Sam can you check the export?\"}"
    }
}
