// SPDX-License-Identifier: Apache-2.0

package com.example.client.push

import android.content.Context
import com.google.firebase.messaging.FirebaseMessagingService
import com.google.firebase.messaging.RemoteMessage

/**
 * The FCM service of the app (docs/spec/15-client-architecture.md 10.2; ADR 0056 item 3; brief
 * M5-05 Design note 1): Kotlin only, no Flutter engine. A data message goes to [PushReceiver],
 * which posts the placeholder at once and schedules the preview fetch; a new token (`onNewToken`,
 * `onRegistered`) goes to the app through [PushTokens].
 */
class PushMessagingService : FirebaseMessagingService() {
    override fun onMessageReceived(message: RemoteMessage) {
        handle(applicationContext, message)
    }

    @Deprecated("firebase-messaging 26 reports new tokens through onRegistered as well")
    override fun onNewToken(token: String) {
        PushTokens.publish(token)
    }

    override fun onRegistered(token: String) {
        PushTokens.publish(token)
    }

    /** The handling without the service object, which the instrumented tests call. */
    companion object {
        /** Handles [message] as `onMessageReceived` does. */
        fun handle(context: Context, message: RemoteMessage): PushReceiver.Handled =
            PushReceiver(PushRuntime.of(context)).onMessage(message.data)
    }
}
