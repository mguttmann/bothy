// SPDX-License-Identifier: Apache-2.0

package com.example.client.push

import android.app.Service
import android.content.Intent
import android.os.IBinder

/**
 * The stand-in of the FCM service in a build without a Firebase configuration: the manifest
 * declares the service disabled (`pushEnabled` is false, D-M5-05-5), so the system never starts
 * it; it exists only so the declaration names a class.
 */
class PushMessagingService : Service() {
    override fun onBind(intent: Intent?): IBinder? = null
}
