// SPDX-License-Identifier: Apache-2.0

package com.example.client

import androidx.test.runner.AndroidJUnit4
import java.io.ByteArrayInputStream
import java.io.File
import java.io.FileNotFoundException
import java.io.InputStream
import java.nio.file.Files
import java.security.MessageDigest
import java.util.concurrent.CountDownLatch
import java.util.concurrent.Executors
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.asCoroutineDispatcher
import kotlinx.coroutines.cancel
import kotlinx.coroutines.joinAll
import kotlinx.coroutines.runBlocking
import org.junit.After
import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith

/**
 * The share receiver of the Android runner (brief M5-06 Design note 6): the copies into the
 * app's cache, the names, the refused items and the one pending share that the app takes once.
 */
@RunWith(AndroidJUnit4::class)
class ShareReceiverTest {
    private lateinit var cache: File
    private val executor = Executors.newSingleThreadExecutor()
    private val scope = CoroutineScope(SupervisorJob() + executor.asCoroutineDispatcher())
    private val source = FakeSource()
    private val logs = mutableListOf<String>()
    private var ids = 0

    @Before
    fun setUp() {
        cache = Files.createTempDirectory("share-test").toFile()
    }

    @After
    fun tearDown() {
        scope.cancel()
        executor.shutdownNow()
        cache.deleteRecursively()
    }

    private fun receiver() =
        ShareReceiver(
            root = File(cache, "share"),
            source = source,
            scope = scope,
            newId = { "share-${++ids}" },
            log = { logs += it },
        )

    @Test
    fun aLargeFileIsCopiedInChunksAndByteEqual() = runBlocking {
        // Several buffers long and not a multiple of the buffer, so the last read is partial.
        val size = ShareReceiver.COPY_BUFFER_BYTES * 5 + 123
        val bytes = ByteArray(size) { (it * 31 + 7).toByte() }
        val stream = RecordingStream(bytes)
        source.add("content://docs.example.com/1", bytes, name = "report.pdf", type = "application/pdf", stream = stream)
        val receiver = receiver()

        receiver.receive(ShareRequest(text = null, streams = listOf(SharedStream("content://docs.example.com/1", "application/pdf"))))
        val share = receiver.takePendingShare()

        assertNotNull(share)
        val file = share!!.files.single()
        assertEquals("report.pdf", file.name)
        assertEquals("application/pdf", file.mimeType)
        assertEquals(size.toLong(), file.sizeBytes)
        assertEquals(File(cache, "share/share-1/1-report.pdf").absolutePath, file.path)
        assertArrayEquals(sha256(bytes), sha256(File(file.path).readBytes()))
        // Streamed: no single read asks for more than one buffer, and the stream is closed.
        assertTrue("max read ${stream.maxRead}", stream.maxRead <= ShareReceiver.COPY_BUFFER_BYTES)
        assertTrue(stream.closed)
    }

    @Test
    fun multipleItemsKeepTheirOrderAndTheText() = runBlocking {
        source.add("content://photos.example.com/a", byteArrayOf(1, 2, 3), name = "a.jpg", type = "image/jpeg")
        source.add("content://photos.example.com/b", byteArrayOf(4, 5), name = "b.png", type = "image/png")
        val receiver = receiver()

        receiver.receive(
            ShareRequest(
                text = "Look, Sam",
                streams =
                    listOf(
                        SharedStream("content://photos.example.com/a", "image/*"),
                        SharedStream("content://photos.example.com/b", "image/*"),
                    ),
            ),
        )
        val share = receiver.takePendingShare()!!

        assertEquals("share-1", share.id)
        assertEquals("Look, Sam", share.text)
        assertEquals(listOf("a.jpg", "b.png"), share.files.map { it.name })
        assertEquals(listOf("image/jpeg", "image/png"), share.files.map { it.mimeType })
        assertEquals(listOf(3L, 2L), share.files.map { it.sizeBytes })
        assertEquals(listOf("1-a.jpg", "2-b.png"), share.files.map { File(it.path).name })
    }

    @Test
    fun textOnlyIsAShareWithoutFiles() = runBlocking {
        val receiver = receiver()

        receiver.receive(ShareRequest(text = "https://chat.example.com/t/1", streams = emptyList()))
        val share = receiver.takePendingShare()!!

        assertEquals("https://chat.example.com/t/1", share.text)
        assertTrue(share.files.isEmpty())
    }

    @Test
    fun unsupportedTypesAndForeignSchemesAreSkipped() = runBlocking {
        source.add("content://files.example.com/tool", byteArrayOf(9), name = "tool.exe", type = "application/x-msdownload")
        source.add("content://files.example.com/raw", byteArrayOf(9), name = "raw", type = null)
        source.add("file:///data/data/com.example.chat/shared_prefs/secrets.xml", byteArrayOf(9), name = "secrets.xml", type = "text/plain")
        source.add("content://files.example.com/ok", byteArrayOf(7, 7), name = "notes.txt", type = "text/plain")
        val receiver = receiver()

        receiver.receive(
            ShareRequest(
                text = null,
                streams =
                    listOf(
                        SharedStream("content://files.example.com/tool", "*/*"),
                        // No type from the resolver and only a wildcard from the intent.
                        SharedStream("content://files.example.com/raw", "*/*"),
                        SharedStream("file:///data/data/com.example.chat/shared_prefs/secrets.xml", "text/plain"),
                        SharedStream("content://files.example.com/ok", "*/*"),
                    ),
            ),
        )
        val share = receiver.takePendingShare()!!

        assertEquals(listOf("notes.txt"), share.files.map { it.name })
        assertEquals("1-notes.txt", File(share.files.single().path).name)
        assertFalse("the file URI was never opened", source.opened.any { it.startsWith("file:") })
        assertEquals(3, logs.size)
        assertTrue(logs.none { "secrets" in it || "tool" in it })
    }

    @Test
    fun itemsOfTheAppsOwnOrOfAMissingProviderAreNeverOpened() = runBlocking {
        // A provider of the app itself (or no provider at all) serves these URIs.
        source.add("content://com.example.chat.files/databases/app.db", byteArrayOf(9), name = "app.db.pdf", type = "application/pdf", foreign = false)
        source.add("content://missing.example.com/1", byteArrayOf(9), name = "gone.pdf", type = "application/pdf", foreign = false)
        source.add("content://docs.example.com/2", byteArrayOf(7), name = "notes.pdf", type = "application/pdf")
        val receiver = receiver()

        receiver.receive(
            ShareRequest(
                text = null,
                streams =
                    listOf(
                        SharedStream("content://com.example.chat.files/databases/app.db", "application/pdf"),
                        SharedStream("content://missing.example.com/1", "application/pdf"),
                        SharedStream("content://docs.example.com/2", "application/pdf"),
                    ),
            ),
        )
        val share = receiver.takePendingShare()!!

        assertEquals(listOf("notes.pdf"), share.files.map { it.name })
        assertEquals(listOf("content://docs.example.com/2"), source.opened)
        assertEquals(2, logs.size)
    }

    @Test
    fun aFailingItemIsSkippedAndLeavesNoPartialCopy() = runBlocking {
        source.add("content://files.example.com/gone", null, name = "gone.pdf", type = "application/pdf")
        source.add("content://files.example.com/broken", ByteArray(10), name = "broken.pdf", type = "application/pdf", stream = FailingStream())
        val receiver = receiver()

        receiver.receive(
            ShareRequest(
                text = null,
                streams =
                    listOf(
                        SharedStream("content://files.example.com/gone", "application/pdf"),
                        SharedStream("content://files.example.com/broken", "application/pdf"),
                    ),
            ),
        )

        // Nothing usable remains, so there is no share, and its folder is gone.
        assertNull(receiver.takePendingShare())
        assertFalse(File(cache, "share/share-1").exists())
    }

    @Test
    fun theTypeFallsBackToAConcreteIntentType() = runBlocking {
        source.add("content://files.example.com/clip", byteArrayOf(1), name = "clip.mp4", type = null)
        val receiver = receiver()

        receiver.receive(ShareRequest(text = null, streams = listOf(SharedStream("content://files.example.com/clip", "video/mp4; codecs=avc1"))))

        assertEquals("video/mp4", receiver.takePendingShare()!!.files.single().mimeType)
    }

    @Test
    fun thePendingShareIsTakenOnce() = runBlocking {
        val receiver = receiver()
        assertNull(receiver.takePendingShare())

        receiver.receive(ShareRequest(text = "once", streams = emptyList()))

        assertEquals("once", receiver.takePendingShare()?.text)
        assertNull(receiver.takePendingShare())
    }

    @Test
    fun aNewerShareReplacesTheUntakenOneAndDeletesItsCopies() = runBlocking {
        // The first copy is under way when the second share arrives.
        val stream = GatedStream()
        source.add("content://photos.example.com/a", ByteArray(10), name = "a.jpg", type = "image/jpeg", stream = stream)
        val receiver = receiver()

        receiver.receive(ShareRequest(text = null, streams = listOf(SharedStream("content://photos.example.com/a", "image/jpeg"))))
        stream.started.await()
        assertTrue(File(cache, "share/share-1/1-a.jpg").exists())
        receiver.receive(ShareRequest(text = "second", streams = emptyList()))
        stream.release.countDown()
        val share = receiver.takePendingShare()!!
        awaitBackgroundWork()

        assertEquals("share-2", share.id)
        assertFalse(File(cache, "share/share-1").exists())
    }

    @Test
    fun theAppIsToldOnlyAfterItAskedOnce() = runBlocking {
        val receiver = receiver()
        var told = 0
        val first = { told++; Unit }
        receiver.attach(first)

        // A share before the app asked: the app takes it when it starts.
        receiver.receive(ShareRequest(text = "cold", streams = emptyList()))
        assertEquals(0, told)
        assertEquals("cold", receiver.takePendingShare()?.text)

        receiver.receive(ShareRequest(text = "warm", streams = emptyList()))
        assertEquals(1, told)

        // A new engine has not asked yet; the late detach of the old one keeps the new one.
        val second = { told += 10; Unit }
        receiver.attach(second)
        receiver.receive(ShareRequest(text = "again", streams = emptyList()))
        assertEquals(1, told)
        receiver.takePendingShare()
        receiver.detach(first)
        receiver.receive(ShareRequest(text = "still attached", streams = emptyList()))
        assertEquals(11, told)
        receiver.takePendingShare()

        // A detached engine is never told.
        receiver.detach(second)
        receiver.receive(ShareRequest(text = "detached", streams = emptyList()))
        assertEquals(11, told)
    }

    @Test
    fun anEmptyShareIsIgnored() = runBlocking {
        val receiver = receiver()

        receiver.receive(ShareRequest(text = "  ", streams = emptyList()))

        assertNull(receiver.takePendingShare())
    }

    /** Waits for the copies and clean-ups the receiver started, without a clock. */
    private suspend fun awaitBackgroundWork() {
        scope.coroutineContext[Job]!!.children.toList().joinAll()
    }

    private fun sha256(bytes: ByteArray): ByteArray = MessageDigest.getInstance("SHA-256").digest(bytes)

    /** Serves the bytes of a few URIs like a content resolver; null bytes mean "not found". */
    private class FakeSource : ShareSource {
        private class Entry(
            val bytes: ByteArray?,
            val name: String?,
            val type: String?,
            val stream: InputStream?,
            val foreign: Boolean,
        )

        private val entries = mutableMapOf<String, Entry>()
        val opened = mutableListOf<String>()

        fun add(
            uri: String,
            bytes: ByteArray?,
            name: String?,
            type: String?,
            stream: InputStream? = null,
            foreign: Boolean = true,
        ) {
            entries[uri] = Entry(bytes, name, type, stream, foreign)
        }

        override fun open(uri: String): InputStream {
            opened += uri
            val entry = entries.getValue(uri)
            val bytes = entry.bytes ?: throw FileNotFoundException("no such item")
            return entry.stream ?: ByteArrayInputStream(bytes)
        }

        override fun displayName(uri: String): String? = entries.getValue(uri).name

        override fun type(uri: String): String? = entries.getValue(uri).type

        override fun isForeign(uri: String): Boolean = entries.getValue(uri).foreign
    }

    /** Records the largest read request and whether it was closed. */
    private class RecordingStream(bytes: ByteArray) : InputStream() {
        private val inner = ByteArrayInputStream(bytes)
        var maxRead = 0
        var closed = false

        override fun read(): Int = inner.read()

        override fun read(b: ByteArray, off: Int, len: Int): Int {
            maxRead = maxOf(maxRead, len)
            return inner.read(b, off, len)
        }

        override fun close() {
            closed = true
        }
    }

    /** Delivers one byte, blocks its second read until [release], then ends. */
    private class GatedStream : InputStream() {
        val started = CountDownLatch(1)
        val release = CountDownLatch(1)
        private var reads = 0

        override fun read(): Int = throw java.io.IOException("unused")

        override fun read(b: ByteArray, off: Int, len: Int): Int {
            if (reads++ == 0) {
                b[off] = 1
                return 1
            }
            if (reads > 2) return -1
            started.countDown()
            release.await()
            b[off] = 2
            return 1
        }
    }

    /** Fails after the first bytes, as a provider that goes away during the copy. */
    private class FailingStream : InputStream() {
        private var reads = 0

        override fun read(): Int = throw java.io.IOException("gone")

        override fun read(b: ByteArray, off: Int, len: Int): Int {
            if (reads++ > 0) throw java.io.IOException("gone")
            b[off] = 1
            return 1
        }
    }
}
