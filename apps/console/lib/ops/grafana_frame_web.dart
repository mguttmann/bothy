// SPDX-License-Identifier: Apache-2.0

/// The frame of the embedded dashboard in the browser: a real `<iframe>` through
/// [HtmlElementView] (brief M6-17 Design note 3). `package:web` is allowed in the console only;
/// the apps never embed web content (25 9.1, SEC-VZ-1).
library;

import 'package:flutter/widgets.dart';
import 'package:web/web.dart' as web;

import 'grafana_frame.dart';

/// The iframe of [url], labelled [label], configured by [configureGrafanaFrame].
Widget grafanaFrame(Uri url, String label) => HtmlElementView.fromTagName(
  key: ValueKey(url),
  tagName: 'iframe',
  onElementCreated: (element) =>
      configureGrafanaFrame(element as web.HTMLIFrameElement, url, label),
);

/// Sets the attributes of the dashboard's iframe [frame]: the sandbox and the referrer policy
/// first, so the very first navigation is already sandboxed and sends no referrer, then the
/// title [label], the size and last the source [url]. Setting `src` of an iframe is no Trusted
/// Types sink, so the console's `require-trusted-types-for 'script'` does not stand in the way.
void configureGrafanaFrame(web.HTMLIFrameElement frame, Uri url, String label) {
  frame
    ..setAttribute('sandbox', grafanaFrameSandbox)
    ..setAttribute('referrerpolicy', grafanaFrameReferrerPolicy)
    ..title = label;
  frame.style
    ..border = '0'
    ..width = '100%'
    ..height = '100%';
  frame.src = url.toString();
}
