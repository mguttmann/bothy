// SPDX-License-Identifier: Apache-2.0

import ClientKitShare

/// The Share Extension of the iOS app (brief M5-06; ADR 0056 item 2; docs/spec/
/// 15-client-architecture.md 12): Swift and SwiftUI without the Flutter engine. All logic is
/// `ClientKitShareViewController` of ClientKit, which shows the share sheet, copies the shared
/// items into the App Group and sends them through the extension's own child session, or
/// leaves them in the share inbox for the app.
final class ShareViewController: ClientKitShareViewController {}
