// SPDX-License-Identifier: Apache-2.0

import 'package:flutter/widgets.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'app/app.dart';
import 'app/bootstrap.dart';
import 'app/providers/ports.dart';

/// Starts the desktop app on macOS and Windows with the ports of the platform
/// ([productionPorts]); the integration tests start it from
/// `integration_test/support/test_app.dart` instead.
void main() {
  WidgetsFlutterBinding.ensureInitialized();
  runApp(
    ProviderScope(
      overrides: [appPortsProvider.overrideWithValue(productionPorts())],
      child: const ClientApp(),
    ),
  );
}
