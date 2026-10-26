// SPDX-License-Identifier: Apache-2.0

package com.example.client

import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.net.Uri
import android.os.Build
import android.os.Parcelable
import android.provider.OpenableColumns
import android.util.Log
import java.io.File
import java.io.FileNotFoundException
import java.io.InputStream
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob

/**
 * The Android side of sharing into the app (brief M5-06 Design note 6): reads a share intent
 * into a [ShareRequest] and holds the one [ShareReceiver] of the process.
 */
object ShareIntents {
    private const val TAG = "ShareReceiver"

    @Volatile private var shared: ShareReceiver? = null

    /**
     * The receiver of the process, created on first use: copies into `cacheDir/share` on the
     * IO dispatcher. One per process, so a share survives a recreated activity and every
     * engine reads the same pending share.
     */
    fun receiver(context: Context): ShareReceiver =
        shared ?: synchronized(this) {
            shared ?: ShareReceiver(
                root = File(context.cacheDir, "share"),
                source = ContentResolverSource(context.applicationContext),
                scope = CoroutineScope(SupervisorJob() + Dispatchers.IO),
                log = { Log.w(TAG, it) },
            ).also { shared = it }
        }

    /**
     * The share of [intent]: `EXTRA_TEXT` and the `EXTRA_STREAM` items (a single URI for
     * `ACTION_SEND`, a list for `ACTION_SEND_MULTIPLE`; the items of the clip data when a
     * sender set only those), or null when [intent] is no share.
     */
    fun requestOf(intent: Intent?): ShareRequest? {
        val action = intent?.action
        if (action != Intent.ACTION_SEND && action != Intent.ACTION_SEND_MULTIPLE) return null
        val streams = if (action == Intent.ACTION_SEND) listOfNotNull(stream(intent)) else streamList(intent)
        val uris = streams.ifEmpty { clipUris(intent) }
        val text =
            if (action == Intent.ACTION_SEND_MULTIPLE) {
                intent.getCharSequenceArrayListExtra(Intent.EXTRA_TEXT)?.joinToString("\n")
                    ?: intent.getCharSequenceExtra(Intent.EXTRA_TEXT)
            } else {
                intent.getCharSequenceExtra(Intent.EXTRA_TEXT)
            }
        return ShareRequest(text?.toString(), uris.map { SharedStream(it.toString(), intent.type) })
    }

    // The typed getters exist from API 33, but are reliable only from API 34 (as IntentCompat).
    // Below, the extra is read as any Parcelable and checked, because the activity is exported
    // and a cast of a sender's wrong type would end the process.
    @Suppress("DEPRECATION")
    private fun stream(intent: Intent): Uri? =
        if (Build.VERSION.SDK_INT >= 34) {
            intent.getParcelableExtra(Intent.EXTRA_STREAM, Uri::class.java)
        } else {
            intent.getParcelableExtra<Parcelable>(Intent.EXTRA_STREAM) as? Uri
        }

    @Suppress("DEPRECATION")
    private fun streamList(intent: Intent): List<Uri> =
        if (Build.VERSION.SDK_INT >= 34) {
            // The typed getter does not check the items of a list that never left the process.
            intent.getParcelableArrayListExtra(Intent.EXTRA_STREAM, Uri::class.java).orEmpty().filterIsInstance<Uri>()
        } else {
            intent.getParcelableArrayListExtra<Parcelable>(Intent.EXTRA_STREAM).orEmpty().filterIsInstance<Uri>()
        }

    private fun clipUris(intent: Intent): List<Uri> {
        val clip = intent.clipData ?: return emptyList()
        return (0 until clip.itemCount).mapNotNull { clip.getItemAt(it).uri }
    }
}

/**
 * Reads shared content URIs through the [Context]'s `ContentResolver`; the grants of the
 * share intent make them readable.
 */
class ContentResolverSource(private val context: Context) : ShareSource {
    override fun open(uri: String): InputStream =
        context.contentResolver.openInputStream(Uri.parse(uri))
            ?: throw FileNotFoundException("the provider gave no stream")

    override fun displayName(uri: String): String? =
        context.contentResolver
            .query(Uri.parse(uri), arrayOf(OpenableColumns.DISPLAY_NAME), null, null, null)
            ?.use { if (it.moveToFirst() && !it.isNull(0)) it.getString(0) else null }

    override fun type(uri: String): String? = context.contentResolver.getType(Uri.parse(uri))

    override fun isForeign(uri: String): Boolean {
        val authority = Uri.parse(uri).authority ?: return false
        val provider =
            if (Build.VERSION.SDK_INT >= 33) {
                context.packageManager.resolveContentProvider(authority, PackageManager.ComponentInfoFlags.of(0))
            } else {
                @Suppress("DEPRECATION")
                context.packageManager.resolveContentProvider(authority, 0)
            }
        return provider != null && provider.packageName != context.packageName
    }
}
