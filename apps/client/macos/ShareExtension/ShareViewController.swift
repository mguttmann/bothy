// SPDX-License-Identifier: Apache-2.0

import ClientKitShare

/// The Share Extension of the macOS app (brief M5-06; ADR 0056 item 2; docs/spec/
/// 15-client-architecture.md 12): the same code as on iOS, without the Flutter engine; all logic
/// is `ClientKitShareViewController` of ClientKit.
final class ShareViewController: ClientKitShareViewController {}
