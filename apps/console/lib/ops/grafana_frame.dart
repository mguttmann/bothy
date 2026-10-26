// SPDX-License-Identifier: Apache-2.0

/// The iframe of the embedded Grafana dashboard (docs/spec/25-visualization.md 9.2 item 5,
/// SEC-VZ-2, brief M6-17 Design note 3). The web build creates a real `<iframe>` through
/// [HtmlElementView] and `package:web` (`grafana_frame_web.dart`); every other platform, the
/// test VM included, gets an inert box (`grafana_frame_stub.dart`) that loads nothing.
library;

import 'package:flutter/widgets.dart';

import 'grafana_frame_stub.dart'
    if (dart.library.js_interop) 'grafana_frame_web.dart'
    as impl;

/// The `sandbox` of the iframe: scripts may run (Grafana is a script application) and the frame
/// keeps its own origin (the auth proxy's cookie), but nothing else: no top navigation, no
/// popups, no forms (25 9.2 item 5). This lifts the sandbox only for a frame of the console's own
/// origin, which `bothy config check` refuses (with every port of the console's host). A frame on
/// another host of the same site cannot reach the console through `document.domain` either,
/// because the console is cross-origin isolated (SEC-HDR-3), which disables it.
const String grafanaFrameSandbox = 'allow-scripts allow-same-origin';

/// The `referrerpolicy` of the iframe: Grafana never learns the console's address.
const String grafanaFrameReferrerPolicy = 'no-referrer';

/// The embedded dashboard at [url]: an iframe with [grafanaFrameSandbox] and
/// [grafanaFrameReferrerPolicy] that fills the space it is given. The console's CSP allows
/// exactly the origin of [url] in `frame-src`; the console passes no identity and no token to
/// it (25 9.2 item 2).
class ConsoleGrafanaFrame extends StatelessWidget {
  /// Creates the frame of [url] with the accessible name [label].
  const ConsoleGrafanaFrame({
    super.key,
    required this.url,
    required this.label,
  });

  /// The dashboard, `grafana_embed_url` of `getInstance`.
  final Uri url;

  /// The accessible name of the frame (its `title`).
  final String label;

  @override
  Widget build(BuildContext context) => impl.grafanaFrame(url, label);
}
