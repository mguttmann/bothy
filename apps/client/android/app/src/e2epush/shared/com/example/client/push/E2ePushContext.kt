// SPDX-License-Identifier: Apache-2.0

package com.example.client.push

import android.content.ContentProvider
import android.content.ContentValues
import android.content.Context
import android.database.Cursor
import android.net.Uri

/**
 * Keeps the application context for the E2E [PushBackend], whose `token()` takes no context
 * because the three push backends share one signature (brief M5-11 Design note 2). The system
 * creates a provider before the application's `onCreate`, so the context is there before the
 * app can ask for a token. It serves no data and is not exported.
 */
class E2ePushContext : ContentProvider() {
    override fun onCreate(): Boolean {
        current = context?.applicationContext
        return true
    }

    override fun query(
        uri: Uri,
        projection: Array<out String>?,
        selection: String?,
        selectionArgs: Array<out String>?,
        sortOrder: String?,
    ): Cursor? = null

    override fun getType(uri: Uri): String? = null

    override fun insert(uri: Uri, values: ContentValues?): Uri? = null

    override fun delete(uri: Uri, selection: String?, selectionArgs: Array<out String>?): Int = 0

    override fun update(uri: Uri, values: ContentValues?, selection: String?, selectionArgs: Array<out String>?): Int = 0

    /** The context of the process. */
    companion object {
        @Volatile private var current: Context? = null

        /** The application context, or null before the provider started. */
        fun application(): Context? = current
    }
}
