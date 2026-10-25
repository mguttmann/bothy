// SPDX-License-Identifier: Apache-2.0

package com.example.client

import android.app.Activity
import android.content.Intent
import android.os.Bundle

/**
 * Receives every link from outside the app (the URL scheme of 15 11, the OIDC redirect, the
 * App Links of the public domain) and hands it to [MainActivity] (brief M5-03 Design note 2;
 * docs/decisions.d/M5-03.md D-M5-03-3).
 *
 * The flags bring the existing [MainActivity] to the front with the link (`onNewIntent`) and
 * clear what lies above it in its task, such as the Custom Tab of a login, so the app never
 * runs a second Flutter engine for a link. Without a running app the link starts it.
 */
class LinkActivity : Activity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        val link = intent?.data
        if (link != null) {
            startActivity(
                Intent(this, MainActivity::class.java)
                    .setAction(Intent.ACTION_VIEW)
                    .setData(link)
                    .addFlags(
                        Intent.FLAG_ACTIVITY_NEW_TASK or
                            Intent.FLAG_ACTIVITY_CLEAR_TOP or
                            Intent.FLAG_ACTIVITY_SINGLE_TOP,
                    ),
            )
        }
        finish()
    }
}
