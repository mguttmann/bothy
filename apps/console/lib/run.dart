// SPDX-License-Identifier: Apache-2.0

import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter_web_plugins/url_strategy.dart';

import 'app.dart';
import 'platform/browser.dart';
import 'routes.dart';

/// Kept for the life of the page, so the semantics stay on.
SemanticsHandle? _semantics;

/// Starts the console in [environment]: semantics on before the first frame (ADR 0028
/// "Confirmation"; the console does not wait for a screen reader to ask), path URLs below
/// [consoleBasePath] instead of `#` fragments (the OIDC callbacks are paths), then the app.
void runConsole(ConsoleEnvironment environment) {
  WidgetsFlutterBinding.ensureInitialized();
  _semantics ??= SemanticsBinding.instance.ensureSemantics();
  setUrlStrategy(PathUrlStrategy(ConsolePlatformLocation()));
  runApp(ConsoleApp(environment: environment));
}

/// The location of the browser tab with [consoleBasePath] as its base. `index.html` has no
/// `<base href>` element, because the CSP of the console has `base-uri 'none'` and would block it
/// (docs/spec/19-sicherheit.md 5, docs/DECISIONS.md D-M6-03-2); the path URL strategy would
/// otherwise read the document URL as its base and fail on every deep link.
class ConsolePlatformLocation extends BrowserPlatformLocation {
  /// Creates the location.
  ConsolePlatformLocation();

  /// Always `/console/`, the path below which the server serves the console.
  @override
  String getBaseHref() => '$consoleBasePath/';
}

/// Releases the semantics handle of [runConsole]; only tests call it, because the binding of a
/// widget test requires every handle to be disposed at its end.
@visibleForTesting
void releaseConsoleSemantics() {
  _semantics?.dispose();
  _semantics = null;
}
