// SPDX-License-Identifier: Apache-2.0

package com.example.client

import android.content.Intent
import android.content.pm.PackageManager
import android.content.pm.ResolveInfo
import android.os.Build
import androidx.test.platform.app.InstrumentationRegistry
import androidx.test.runner.AndroidJUnit4
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith

/**
 * The installed app takes shares exactly for [ShareTypes.MIME_TYPES], at [ShareActivity]
 * (docs/spec/15-client-architecture.md 12; brief M5-06 Design note 6 and fixed requirement 17).
 */
@RunWith(AndroidJUnit4::class)
class ShareManifestTest {
    private val context = InstrumentationRegistry.getInstrumentation().targetContext

    @Test
    fun everySupportedTypeResolvesToTheShareActivity() {
        for (type in ShareTypes.MIME_TYPES) {
            assertEquals(type, listOf(ShareActivity::class.java.name), ownActivities(Intent.ACTION_SEND, type))
        }
        for (type in listOf("image/jpeg", "application/pdf")) {
            assertEquals(type, listOf(ShareActivity::class.java.name), ownActivities(Intent.ACTION_SEND_MULTIPLE, type))
        }
    }

    @Test
    fun otherConcreteTypesDoNotResolveToTheApp() {
        // A type outside the list of each top-level type: none resolves, so no filter of the
        // merged manifest takes every type or every type of a group.
        val unknown = listOf("application", "audio", "image", "text", "video").map { "$it/x-example" }
        for (type in listOf("application/octet-stream") + unknown) {
            assertEquals(type, emptyList<String>(), ownActivities(Intent.ACTION_SEND, type))
        }
    }

    @Test
    fun wildcardIntentTypesStillResolveSoEachItemsTypeDecides() {
        // Android matches the wildcard type of an intent against the concrete types of a filter
        // (an ACTION_SEND_MULTIPLE of a JPEG and a PNG arrives as "image/" plus a star), so these
        // reach the app; ShareReceiver then takes only the items of a supported type.
        for (type in listOf("*/*", "image/*")) {
            assertEquals(type, listOf(ShareActivity::class.java.name), ownActivities(Intent.ACTION_SEND, type))
        }
        assertEquals(emptyList<String>(), ownActivities(Intent.ACTION_SEND, "chemical/*"))
    }

    @Test
    fun theSupportedTypesAreConcreteAndUnique() {
        assertEquals(ShareTypes.MIME_TYPES.size, ShareTypes.MIME_TYPES.toSet().size)
        for (type in ShareTypes.MIME_TYPES) {
            assertTrue(type, Regex("[a-z]+/[a-z0-9.+-]+").matches(type))
        }
        for (required in listOf("text/plain", "image/jpeg", "image/png", "image/heic", "image/heif", "image/webp", "image/gif", "application/pdf")) {
            assertTrue(required, required in ShareTypes.MIME_TYPES)
        }
    }

    /** The activities of this app that a share intent with [action] and [type] resolves to. */
    private fun ownActivities(action: String, type: String): List<String> {
        val intent = Intent(action).setType(type)
        val found: List<ResolveInfo> =
            if (Build.VERSION.SDK_INT >= 33) {
                context.packageManager.queryIntentActivities(
                    intent,
                    PackageManager.ResolveInfoFlags.of(PackageManager.MATCH_DEFAULT_ONLY.toLong()),
                )
            } else {
                @Suppress("DEPRECATION")
                context.packageManager.queryIntentActivities(intent, PackageManager.MATCH_DEFAULT_ONLY)
            }
        return found.map { it.activityInfo }.filter { it.packageName == context.packageName }.map { it.name }
    }
}
