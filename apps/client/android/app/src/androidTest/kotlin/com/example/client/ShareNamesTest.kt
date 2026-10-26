// SPDX-License-Identifier: Apache-2.0

package com.example.client

import androidx.test.runner.AndroidJUnit4
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith

/**
 * The names and types of shared items (docs/spec/12-files.md 6.2; brief M5-06 Design note 6):
 * the display name is cleaned like the server cleans it, and the name of the copy in the cache
 * always fits the file system.
 */
@RunWith(AndroidJUnit4::class)
class ShareNamesTest {
    @Test
    fun theDisplayNameKeepsOnlyItsLastSegment() {
        assertEquals("passwd", ShareReceiver.safeName("../../etc/passwd", "text/plain"))
        assertEquals("b.txt", ShareReceiver.safeName("a\\b.txt", "text/plain"))
    }

    @Test
    fun controlBidiAndZeroWidthCharactersAreRemoved() {
        // U+202E would show "photo_gpj.exe" as "photo_exe.jpg".
        assertEquals("photo_gpj.exe", ShareReceiver.safeName("photo_‮gpj.exe", "image/jpeg"))
        assertEquals("ab.pdf", ShareReceiver.safeName("a\u0000​b\u0007.pdf", "application/pdf"))
        assertEquals("notes.txt", ShareReceiver.safeName("  notes.txt \n", "text/plain"))
    }

    @Test
    fun theNameIsNormalizedToNfc() {
        // "e" followed by the combining acute accent becomes one character.
        assertEquals("café.txt", ShareReceiver.safeName("café.txt", "text/plain"))
    }

    @Test
    fun aMissingOrEmptyNameIsDerivedFromTheType() {
        assertEquals("file.jpg", ShareReceiver.safeName(null, "image/jpeg"))
        assertEquals("file.pdf", ShareReceiver.safeName("", "application/pdf"))
        assertEquals("file.png", ShareReceiver.safeName("..", "image/png"))
        assertEquals("file.txt", ShareReceiver.safeName("/", "text/plain"))
        assertEquals("file", ShareReceiver.safeName(" . ", "application/x-unknown"))
    }

    @Test
    fun theCopyNameFitsTheFileSystemAndKeepsTheExtension() {
        val long = "ä".repeat(200) + ".pdf"
        val name = ShareReceiver.copyName(12, long)

        assertTrue("${name.toByteArray().size} bytes", name.toByteArray(Charsets.UTF_8).size <= ShareReceiver.NAME_MAX_BYTES)
        assertTrue(name.startsWith("12-ä"))
        assertTrue(name.endsWith(".pdf"))
        assertEquals("3-photo.jpg", ShareReceiver.copyName(3, "photo.jpg"))
    }

    @Test
    fun theTypeIsTheFirstSupportedOne() {
        assertEquals("image/png", ShareReceiver.resolveType("image/png", "image/*"))
        assertEquals("image/jpeg", ShareReceiver.resolveType(null, "image/jpeg"))
        assertEquals("text/plain", ShareReceiver.resolveType("TEXT/PLAIN; charset=utf-8", null))
        // The provider's guess is not in the list; the sender's concrete type is.
        assertEquals("application/pdf", ShareReceiver.resolveType("application/octet-stream", "application/pdf"))
        assertNull(ShareReceiver.resolveType(null, "*/*"))
        assertNull(ShareReceiver.resolveType("application/x-msdownload", "*/*"))
    }
}
