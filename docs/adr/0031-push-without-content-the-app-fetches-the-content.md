---
status: accepted
date: 2026-10-10
decision-makers: Planning lead, Team G platform services
---

# Push without content, the app fetches the content itself

## Context and Problem Statement

Mobile apps on iOS and Android receive messages in the background only through Apple Push
Notification service (APNs) and Firebase Cloud Messaging (FCM). A permanent connection of their
own in the background is not possible on iOS. Push providers are third parties. How does the
device show a readable notification without Apple or Google seeing message content and without a
push bypassing permissions that changed after sending?

## Decision Drivers

- No message text, no name at Apple or Google.
- Permissions at display time, not only at send time (B7, basic sync rule).
- Works within the limits of the platforms (payload, runtime of the extension).
- As little key management as possible in addition to the device binding.
- Behavior when the device cannot reach its own server at the moment.

## Considered Options

- Push only with IDs and a neutral placeholder; the app (iOS: Notification Service Extension)
  fetches the content through the API
- Push with plaintext content
- Push with end-to-end encrypted content for the device
- No push, only the app's own connection

## Decision Outcome

Chosen option: "Push only with IDs and a neutral placeholder", because it reveals no content
and checks the permissions again on fetch. The payload contains kind, notification ID, instance,
thread and message ID; title and text are keys in the language files of the app. The
iOS extension or the Android app fetches a preview with its own, device-bound
preview token. Details: [13-push-notifications.md](../spec/13-push-notifications.md).

### Consequences

- Good, because Apple and Google see only times, tokens and random IDs.
- Good, because a revocation after sending takes effect: the fetch answers `404`, and the device
  shows a neutral text.
- Good, because the payload is small and has a fixed structure and stays far below the Apple
  limit.
- Good, because the fetch also serves as a receipt confirmation for the delivery measurement.
- Bad, because the content appears only when the device reaches the server. Otherwise the
  placeholder remains. Instances in private networks notice this when the device's VPN is off.
- Bad, because a separate preview token and a separate endpoint are needed.
- Bad, because without Apple's filtering entitlement a notification about a deleted message
  cannot be suppressed but only shown neutrally.

### Confirmation

- A test serializes APNs and FCM payloads and checks them against an allow-list of
  keys and value types (only IDs, numbers, fixed keys).
- A test revokes permissions between event and fetch and expects `404` from the
  preview endpoint.
- The confirmation rate per platform is a signal in the dashboard (fetches per push).

## Pros and Cons of the Options

### Push only with IDs and a neutral placeholder

- Good, because no content lies with the provider.
- Good, because `authorize` runs with current permissions on fetch.
- Neutral, because the extension on iOS runs only for a visible alert with `mutable-content`
  and has about 30 seconds [Q1, verified]; that is enough for one API fetch.
- Bad, because the content needs the server to be reachable.

### Push with plaintext content

- Good, because simplest and without the server needing to be reachable.
- Bad, because FCM is only transport-encrypted, not end-to-end [Q2, verified]; content and
  names would lie with the provider.
- Bad, because permissions apply only at send time.

### Push with end-to-end encrypted content

- Good, because the content appears even when the server is not reachable.
- Bad, because another key per device for encryption becomes necessary (also using the
  DPoP key for it would mean one key for two purposes).
- Bad, because permissions apply only at send time, and revoked content still appears on
  the device.
- Bad, because long texts must be truncated at the size limit of the payload.
- Can be added later if the confirmation rate shows that placeholders remain too
  often.

### No push, only the app's own connection

- Good, because no third party is involved.
- Bad, because iOS suspends apps shortly after they move to the background and does not support
  WebSocket tasks in background sessions [Q3, verified]; mobile notifications would never arrive.
- For Windows this is the path of the first milestones (app in the notification area), because
  WNS needs package identity and an Entra registration.

## More Information

- Sources (retrieved 2026-10-10): Q1
  https://developer.apple.com/documentation/usernotifications/modifying-content-in-newly-delivered-notifications
  [verified]; Q2 https://firebase.google.com/docs/cloud-messaging/customize-messages/set-message-type
  [verified]; Q3 https://developer.apple.com/forums/thread/716118 [verified].
- Baseline ruling R-Push; [13-push-notifications.md](../spec/13-push-notifications.md).
