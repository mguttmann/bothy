// SPDX-License-Identifier: Apache-2.0

package com.example.client

import java.io.File
import java.io.FileOutputStream
import java.io.InputStream
import java.text.Normalizer
import java.util.UUID
import kotlin.coroutines.cancellation.CancellationException
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Deferred
import kotlinx.coroutines.async
import kotlinx.coroutines.currentCoroutineContext
import kotlinx.coroutines.ensureActive
import kotlinx.coroutines.launch

/**
 * One item of a share as the intent names it.
 *
 * @property uri the content URI of `EXTRA_STREAM` as a string.
 * @property intentType the MIME type of the intent, possibly a wildcard such as "any image"
 *   (Android sets the common type of all items of `ACTION_SEND_MULTIPLE`).
 */
data class SharedStream(val uri: String, val intentType: String?)

/**
 * What a share intent carries, read from it by [ShareIntents.requestOf].
 *
 * @property text `EXTRA_TEXT` (text or a link), or null.
 * @property streams the items of `EXTRA_STREAM` in their order.
 */
data class ShareRequest(val text: String?, val streams: List<SharedStream>)

/**
 * Reads shared items; the app's implementation is [ContentResolverSource]. Every method may
 * throw what the provider throws (for example `SecurityException` for an expired grant or
 * `FileNotFoundException`); [ShareReceiver] skips that item.
 */
interface ShareSource {
    /** Opens [uri] for reading; the caller closes the stream. */
    fun open(uri: String): InputStream

    /** The display name the provider gives (`OpenableColumns.DISPLAY_NAME`), or null. */
    fun displayName(uri: String): String?

    /** The MIME type the provider gives (`ContentResolver.getType`), or null. */
    fun type(uri: String): String?

    /**
     * Whether a provider of another app serves [uri]: false when no provider has its
     * authority or the provider is this app's own. The app reads its own providers with its own
     * rights, so a sender could otherwise name a file of a non-exported provider of the app.
     */
    fun isForeign(uri: String): Boolean
}

/**
 * The share receiver of the Android runner and the host side of [ShareAndroidHostApi]
 * (pigeons/share_android.dart; brief M5-06 Design note 6).
 *
 * [receive] copies every supported item of a share on [scope] (a background dispatcher) into
 * `<root>/<random id>/<index>-<safe name>`, streamed through a buffer of
 * [COPY_BUFFER_BYTES], so a large video never sits in memory. It keeps one pending share: a
 * newer share replaces one the app has not taken yet and deletes its copies.
 * [takePendingShare] hands the share to the app once its copies are complete and forgets it;
 * the app deletes the copies after its outbox took the files into its own storage.
 *
 * Items are skipped, never fatal: a type outside [ShareTypes.MIME_TYPES] (an
 * `ACTION_SEND_MULTIPLE` with mixed items arrives with the wildcard type of all files, so
 * each item's own type decides), a URI of a missing provider or of the app's own
 * ([ShareSource.isForeign]), a URI that is not a
 * `content://` URI, and an item the provider cannot deliver. Only `content://` is read because
 * since API 24 a `file://` URI cannot cross apps and the app holds no storage permission, so
 * the only files a `file://` URI could reach are the app's own data, which a hostile sender
 * would name to make the app upload it.
 *
 * The receiver lives as long as the process ([ShareIntents.receiver]), so a share survives a
 * recreated activity; the activity attaches the notification of its Flutter engine.
 *
 * @param root the folder of the copies, `cacheDir/share`.
 * @param source reads the shared items.
 * @param scope runs the copies; a background scope with a `SupervisorJob`.
 * @param newId the ID of a new share.
 * @param log receives one line per skipped item; it names the index and the reason, never a
 *   name, a URI or content.
 */
class ShareReceiver(
    private val root: File,
    private val source: ShareSource,
    private val scope: CoroutineScope,
    private val newId: () -> String = { UUID.randomUUID().toString() },
    private val log: (String) -> Unit = {},
) : ShareAndroidHostApi {
    private class Pending(val folder: File, val copies: Deferred<SharedContentRecord?>)

    private val lock = Any()
    private var pending: Pending? = null
    private var notify: (() -> Unit)? = null
    private var appAsked = false

    /**
     * Starts the copies of [request] and keeps it as the pending share; tells the attached
     * engine when its app already asked once, otherwise the app takes the share when it
     * starts. A request without text and without items is ignored.
     */
    fun receive(request: ShareRequest) {
        val text = request.text?.takeIf { it.isNotBlank() }
        if (text == null && request.streams.isEmpty()) return
        val id = newId()
        val folder = File(root, id)
        val copies = scope.async { copyAll(id, folder, text, request.streams) }
        val tell: (() -> Unit)?
        val replaced: Pending?
        synchronized(lock) {
            replaced = pending
            pending = Pending(folder, copies)
            tell = notify.takeIf { appAsked }
        }
        replaced?.let { old ->
            // Cancelled at once, so its copy stops at the next buffer; deleted once it stopped.
            old.copies.cancel()
            scope.launch {
                old.copies.join()
                old.folder.deleteRecursively()
            }
        }
        tell?.invoke()
    }

    /**
     * The pending share once its copies are complete, or null; taking it removes it, so a
     * second call returns null until the next share. Marks the attached engine's app as
     * listening, so later shares notify it.
     */
    override suspend fun takePendingShare(): SharedContentRecord? {
        val taken =
            synchronized(lock) {
                appAsked = true
                pending.also { pending = null }
            }
        return taken?.copies?.await()
    }

    /**
     * Attaches the engine that [notify] tells about a new share (on the thread of [receive],
     * the main thread). Its app has not asked yet, so it is told only after its first
     * [takePendingShare].
     */
    fun attach(notify: () -> Unit) {
        synchronized(lock) {
            this.notify = notify
            appAsked = false
        }
    }

    /**
     * Detaches [notify] of [attach]; shares wait for the next engine to ask. Does nothing when
     * a newer engine has attached since, because a finished activity can be destroyed after
     * the next one started.
     */
    fun detach(notify: () -> Unit) {
        synchronized(lock) {
            if (this.notify !== notify) return
            this.notify = null
            appAsked = false
        }
    }

    private suspend fun copyAll(
        id: String,
        folder: File,
        text: String?,
        streams: List<SharedStream>,
    ): SharedContentRecord? {
        val files = mutableListOf<SharedFileRecord>()
        streams.forEachIndexed { position, stream ->
            val item = position + 1
            if (!stream.uri.startsWith("content://", ignoreCase = true)) {
                log("share item $item skipped: not a content URI")
                return@forEachIndexed
            }
            try {
                if (!source.isForeign(stream.uri)) {
                    log("share item $item skipped: not a provider of another app")
                    return@forEachIndexed
                }
                val type = resolveType(source.type(stream.uri), stream.intentType)
                if (type == null) {
                    log("share item $item skipped: type not supported")
                    return@forEachIndexed
                }
                val name = safeName(source.displayName(stream.uri), type)
                folder.mkdirs()
                val target = File(folder, copyName(files.size + 1, name))
                try {
                    val size = source.open(stream.uri).use { copyStream(it, target) }
                    files += SharedFileRecord(path = target.absolutePath, name = name, mimeType = type, sizeBytes = size)
                } catch (e: Exception) {
                    // A partial copy goes; one that cannot is named, the cache drops it later.
                    if (target.exists() && !target.delete()) log("share item $item: partial copy kept")
                    throw e
                }
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                log("share item $item skipped: ${e.javaClass.simpleName}")
            }
        }
        if (text == null && files.isEmpty()) {
            folder.deleteRecursively()
            return null
        }
        return SharedContentRecord(id = id, text = text, files = files)
    }

    /** Copies [input] into [target] one buffer at a time; stops when the share is replaced. */
    private suspend fun copyStream(input: InputStream, target: File): Long {
        val buffer = ByteArray(COPY_BUFFER_BYTES)
        var total = 0L
        FileOutputStream(target).use { out ->
            while (true) {
                currentCoroutineContext().ensureActive()
                val read = input.read(buffer)
                if (read < 0) break
                out.write(buffer, 0, read)
                total += read
            }
        }
        return total
    }

    /** The cleaning rules and sizes, shared with the tests. */
    companion object {
        /**
         * The buffer of one copy step: large enough for few system calls, small enough that a
         * share of any size needs no more memory than this.
         */
        const val COPY_BUFFER_BYTES = 64 * 1024

        /** The longest file name of the Linux file systems (`NAME_MAX`), in UTF-8 bytes. */
        const val NAME_MAX_BYTES = 255

        /**
         * The display name cleaned like the server cleans it (docs/spec/12-files.md 6.2):
         * Unicode NFC, only the last path segment, without control, format (bidi and zero
         * width) characters and outer whitespace; an empty, `.` or `..` name becomes
         * `file.<extension of [mimeType]>`. The server shortens it to `file.name_max_bytes`.
         *
         * @param displayName the provider's name, or null.
         * @param mimeType the item's supported type.
         * @return a name without `/` or `\` that is never empty.
         */
        fun safeName(displayName: String?, mimeType: String): String {
            val normalized = Normalizer.normalize(displayName.orEmpty(), Normalizer.Form.NFC)
            val segment = normalized.substringAfterLast('/').substringAfterLast('\\')
            val cleaned =
                segment
                    .filterNot { it.category == CharCategory.CONTROL || it.category == CharCategory.FORMAT }
                    .trim()
            if (cleaned.isNotEmpty() && cleaned != "." && cleaned != "..") return cleaned
            return ShareTypes.EXTENSIONS[mimeType]?.let { "file.$it" } ?: "file"
        }

        /**
         * The file name of copy number [index]: `<index>-<name>`, with the part before the
         * extension shortened so the whole fits [NAME_MAX_BYTES].
         */
        fun copyName(index: Int, name: String): String {
            val full = "$index-$name"
            if (full.toByteArray(Charsets.UTF_8).size <= NAME_MAX_BYTES) return full
            val dot = full.lastIndexOf('.')
            val extension = if (dot > 0 && full.length - dot <= MAX_EXTENSION_CHARS) full.substring(dot) else ""
            var stem = full.substring(0, full.length - extension.length)
            val budget = NAME_MAX_BYTES - extension.toByteArray(Charsets.UTF_8).size
            while (stem.toByteArray(Charsets.UTF_8).size > budget) {
                // Drop whole code points, so no surrogate pair is cut in half.
                stem = stem.substring(0, stem.offsetByCodePoints(stem.length, -1))
            }
            return stem + extension
        }

        /**
         * The first of [resolverType] and [intentType] that is supported, without parameters
         * and in lower case, or null when neither is (a wildcard never is).
         */
        fun resolveType(resolverType: String?, intentType: String?): String? =
            listOf(resolverType, intentType)
                .mapNotNull { it?.substringBefore(';')?.trim()?.lowercase() }
                .firstOrNull { it in ShareTypes.EXTENSIONS }

        /** An extension longer than this is part of the name (`report.final-version`). */
        private const val MAX_EXTENSION_CHARS = 16
    }
}
