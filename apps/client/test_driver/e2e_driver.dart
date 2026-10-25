// SPDX-License-Identifier: Apache-2.0

/// The driver of `flutter drive` for the desktop E2E suite on a device (brief M4-34 Design note
/// 2): `flutter test` runs a test outside `integration_test/` on flutter-tester whatever device is
/// named, so tool/e2e_suite.sh starts the suite on macOS or Windows through this driver; the exit
/// code is the suite's.
library;

import 'package:integration_test/integration_test_driver.dart';

/// Runs the suite on the device and reports its result.
Future<void> main() => integrationDriver();
