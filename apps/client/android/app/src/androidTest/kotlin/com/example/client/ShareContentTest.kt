// SPDX-License-Identifier: Apache-2.0

package com.example.client

import android.content.ContentValues
import android.content.Intent
import android.net.Uri
import android.os.Build
import android.os.Bundle
import android.os.Environment
import android.os.Parcelable
import android.provider.MediaStore
import androidx.test.platform.app.InstrumentationRegistry
import androidx.test.runner.AndroidJUnit4
import java.io.File
import java.util.UUID
import java.util.concurrent.Executors
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.asCoroutineDispatcher
import kotlinx.coroutines.cancel
import kotlinx.coroutines.runBlocking
import org.junit.After
import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Assume.assumeTrue
import org.junit.Test
import org.junit.runner.RunWith

/**
 * The Android side of a share on the device (brief M5-06 Design note 6): reading the intent
 * extras, and [ContentResolverSource] with the real `ContentResolver` and `PackageManager`.
 */
@RunWith(AndroidJUnit4::class)
class ShareContentTest {
    private val context = InstrumentationRegistry.getInstrumentation().targetContext
    private val source = ContentResolverSource(context)
    private val executor = Executors.newSingleThreadExecutor()
    private val scope = CoroutineScope(SupervisorJob() + executor.asCoroutineDispatcher())
    private val root = File(context.cacheDir, "share-content-test-${UUID.randomUUID()}")

    @After
    fun tearDown() {
        scope.cancel()
        executor.shutdownNow()
        root.deleteRecursively()
    }

    @Test
    fun aSingleShareCarriesItsStreamAndText() {
        val uri = Uri.parse("content://docs.example.com/1")
        val intent =
            Intent(Intent.ACTION_SEND)
                .setType("application/pdf")
                .putExtra(Intent.EXTRA_STREAM, uri)
                .putExtra(Intent.EXTRA_TEXT, "For Robin")

        val request = ShareIntents.requestOf(intent)!!

        assertEquals("For Robin", request.text)
        assertEquals(listOf(SharedStream("content://docs.example.com/1", "application/pdf")), request.streams)
    }

    @Test
    fun aMultipleShareCarriesEveryStreamAndJoinsTheTexts() {
        val intent =
            Intent(Intent.ACTION_SEND_MULTIPLE)
                .setType("image/*")
                .putParcelableArrayListExtra(
                    Intent.EXTRA_STREAM,
                    arrayListOf<Parcelable>(Uri.parse("content://photos.example.com/a"), Uri.parse("content://photos.example.com/b")),
                ).putCharSequenceArrayListExtra(Intent.EXTRA_TEXT, arrayListOf<CharSequence>("one", "two"))

        val request = ShareIntents.requestOf(intent)!!

        assertEquals("one\ntwo", request.text)
        assertEquals(listOf("content://photos.example.com/a", "content://photos.example.com/b"), request.streams.map { it.uri })
        assertTrue(request.streams.all { it.intentType == "image/*" })
    }

    @Test
    fun extrasOfAWrongTypeAreIgnored() {
        // The activity is exported: a sender's wrong types must not end the process.
        val single = Intent(Intent.ACTION_SEND).setType("image/png").putExtra(Intent.EXTRA_STREAM, Bundle())
        val multiple =
            Intent(Intent.ACTION_SEND_MULTIPLE)
                .setType("image/png")
                .putParcelableArrayListExtra(Intent.EXTRA_STREAM, arrayListOf<Parcelable>(Bundle(), Uri.parse("content://photos.example.com/a")))

        assertTrue(ShareIntents.requestOf(single)!!.streams.isEmpty())
        assertEquals(listOf("content://photos.example.com/a"), ShareIntents.requestOf(multiple)!!.streams.map { it.uri })
        assertNull(ShareIntents.requestOf(Intent(Intent.ACTION_VIEW, Uri.parse("https://chat.example.com/t/1"))))
    }

    @Test
    fun onlyProvidersOfOtherAppsAreForeign() {
        // androidx.startup's provider is one of the app's own (the merged manifest declares it).
        assertFalse(source.isForeign("content://${context.packageName}.androidx-startup/x"))
        assertFalse(source.isForeign("content://missing.example.com/x"))
        assertFalse(source.isForeign("content:///no-authority"))
        assertTrue(source.isForeign("content://${MediaStore.AUTHORITY}/external/images/media/1"))
    }

    @Test
    fun aPhotoOfTheMediaStoreIsCopiedByteEqual() = runBlocking {
        // Without a storage permission an app writes its own media items only from API 29.
        assumeTrue(Build.VERSION.SDK_INT >= 29)
        val resolver = context.contentResolver
        val bytes = ByteArray(ShareReceiver.COPY_BUFFER_BYTES * 3 + 17) { (it * 13 + 5).toByte() }
        val values =
            ContentValues().apply {
                put(MediaStore.MediaColumns.DISPLAY_NAME, "share-test-${UUID.randomUUID()}.jpg")
                put(MediaStore.MediaColumns.MIME_TYPE, "image/jpeg")
                put(MediaStore.MediaColumns.RELATIVE_PATH, Environment.DIRECTORY_PICTURES)
            }
        val uri = resolver.insert(MediaStore.Images.Media.getContentUri(MediaStore.VOLUME_EXTERNAL_PRIMARY), values)!!
        try {
            resolver.openOutputStream(uri)!!.use { it.write(bytes) }
            val name = source.displayName(uri.toString())!!
            val receiver = ShareReceiver(root = root, source = source, scope = scope)

            receiver.receive(ShareRequest(text = null, streams = listOf(SharedStream(uri.toString(), "image/*"))))
            val file = receiver.takePendingShare()!!.files.single()

            assertEquals(name, file.name)
            assertEquals("image/jpeg", file.mimeType)
            assertEquals(bytes.size.toLong(), file.sizeBytes)
            assertArrayEquals(bytes, File(file.path).readBytes())
        } finally {
            resolver.delete(uri, null, null)
        }
    }
}
