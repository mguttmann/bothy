// SPDX-License-Identifier: Apache-2.0

package com.example.client

import android.content.pm.ApplicationInfo
import androidx.test.runner.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.xmlpull.v1.XmlPullParser

/**
 * The installed app allows no backup, and its rules exclude every domain from the cloud backup,
 * the device transfer (API 31 and later) and the full backup (API 30 and earlier), so neither
 * the database nor the secrets leave the device (15 6.3; brief M5-03 Design note 1).
 */
@RunWith(AndroidJUnit4::class)
class BackupExclusionTest {
    private val context = InstrumentationRegistry.getInstrumentation().targetContext

    @Test
    fun theMergedManifestAllowsNoBackup() {
        assertEquals(0, context.applicationInfo.flags and ApplicationInfo.FLAG_ALLOW_BACKUP)
    }

    @Test
    fun theDataExtractionRulesExcludeEveryDomain() {
        val sections = excludes(R.xml.data_extraction_rules)
        assertEquals(setOf("cloud-backup", "device-transfer"), sections.keys)
        for ((section, domains) in sections) assertEquals(section, DOMAINS, domains)
    }

    @Test
    fun theFullBackupRulesExcludeEveryDomain() {
        assertEquals(mapOf("full-backup-content" to DOMAINS), excludes(R.xml.backup_rules))
    }

    /** The domains excluded with the path "." per section of the rules file [resource]. */
    private fun excludes(resource: Int): Map<String, Set<String>> {
        val result = mutableMapOf<String, MutableSet<String>>()
        val parser = context.resources.getXml(resource)
        var section = ""
        while (parser.next() != XmlPullParser.END_DOCUMENT) {
            if (parser.eventType != XmlPullParser.START_TAG) continue
            when (parser.name) {
                "exclude" -> {
                    assertEquals(".", parser.getAttributeValue(null, "path"))
                    result.getOrPut(section) { mutableSetOf() }
                        .add(parser.getAttributeValue(null, "domain"))
                }
                "include" -> throw AssertionError("the rules include ${parser.getAttributeValue(null, "domain")}")
                "data-extraction-rules" -> Unit
                else -> section = parser.name
            }
        }
        assertTrue(result.isNotEmpty())
        return result
    }

    private companion object {
        val DOMAINS =
            setOf(
                "root",
                "file",
                "database",
                "sharedpref",
                "external",
                "device_root",
                "device_file",
                "device_database",
                "device_sharedpref",
            )
    }
}
