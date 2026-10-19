// SPDX-License-Identifier: Apache-2.0

/// The driver of `flutter drive` for the integration tests of the harness: it writes the
/// reported data (the measured p95) to `build/integration_response_data.json`.
library;

import 'package:integration_test/integration_test_driver.dart';

/// Runs the integration test and saves its report.
Future<void> main() => integrationDriver();
