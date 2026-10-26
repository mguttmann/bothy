// SPDX-License-Identifier: Apache-2.0

package com.example.client

import android.app.Activity
import android.content.Intent
import android.os.Bundle

/**
 * Receives `ACTION_SEND` and `ACTION_SEND_MULTIPLE` of other apps for the concrete types of
 * [ShareTypes] and hands the share to [MainActivity] (brief M5-06 Design note 6; the manifest's
 * share filters).
 *
 * A forwarding activity, like [LinkActivity], instead of filters on [MainActivity]: a share
 * starts its receiver in the sending app's task, where a single-top [MainActivity] would be a
 * second instance with a second Flutter engine. The flags bring the running [MainActivity] to
 * the front with the share (`onNewIntent`) or start the app with it, so there is always one
 * engine, and a share works when the app is not running. The forwarded intent keeps the clip
 * data and passes the read grants on (`FLAG_GRANT_READ_URI_PERMISSION`), because the grants of
 * this activity end when it finishes, before the copies in the background are done; those of
 * [MainActivity] last while its task lives.
 */
class ShareActivity : Activity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        if (savedInstanceState == null && ShareIntents.requestOf(intent) != null) {
            startActivity(
                Intent(intent)
                    .setClass(this, MainActivity::class.java)
                    .setFlags(
                        Intent.FLAG_ACTIVITY_NEW_TASK or
                            Intent.FLAG_ACTIVITY_CLEAR_TOP or
                            Intent.FLAG_ACTIVITY_SINGLE_TOP or
                            Intent.FLAG_GRANT_READ_URI_PERMISSION,
                    ),
            )
        }
        finish()
    }
}
