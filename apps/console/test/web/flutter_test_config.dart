// SPDX-License-Identifier: Apache-2.0

import 'dart:async';

/// The browser tests need no golden setup; this file replaces the one of `test/`, which reads
/// fonts through `dart:io` and cannot compile for the browser.
Future<void> testExecutable(FutureOr<void> Function() testMain) async =>
    testMain();
