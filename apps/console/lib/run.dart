// SPDX-License-Identifier: Apache-2.0

import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter_web_plugins/url_strategy.dart';

import 'app.dart';
import 'platform/browser.dart';

/// Kept for the life of the page, so the semantics stay on.
SemanticsHandle? _semantics;

/// Starts the console in [environment]: semantics on before the first frame (ADR 0028
/// "Confirmation"; the console does not wait for a screen reader to ask), path URLs below the
/// `<base href>` instead of `#` fragments (the OIDC callbacks are paths), then the app.
void runConsole(ConsoleEnvironment environment) {
  WidgetsFlutterBinding.ensureInitialized();
  _semantics ??= SemanticsBinding.instance.ensureSemantics();
  usePathUrlStrategy();
  runApp(ConsoleApp(environment: environment));
}

/// Releases the semantics handle of [runConsole]; only tests call it, because the binding of a
/// widget test requires every handle to be disposed at its end.
@visibleForTesting
void releaseConsoleSemantics() {
  _semantics?.dispose();
  _semantics = null;
}
