// SPDX-License-Identifier: Apache-2.0

/// The frame of the embedded dashboard outside the browser: an empty box with the frame's name
/// for the semantics tree. It loads nothing; only the web build embeds Grafana.
library;

import 'package:flutter/widgets.dart';

/// An inert stand-in for the iframe of [url], labelled [label].
Widget grafanaFrame(Uri url, String label) =>
    Semantics(label: label, child: const SizedBox.expand());
