// SPDX-License-Identifier: Apache-2.0

// The iframe of the embedded Grafana dashboard in a real browser (brief M6-17 Design note 3, 25
// 9.2 item 5): run with `flutter test --platform chrome test/web` from apps/console.
@TestOn('browser')
library;

import 'package:console/ops/grafana_frame.dart';
import 'package:console/ops/grafana_frame_web.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:web/web.dart' as web;

void main() {
  test(
    'the iframe is sandboxed, sends no referrer and loads exactly the URL',
    () {
      final frame =
          web.document.createElement('iframe') as web.HTMLIFrameElement;
      final url = Uri.parse(
        'https://grafana.example.com/d/ops/operations?orgId=1',
      );
      configureGrafanaFrame(frame, url, 'Grafana dashboard');
      expect(frame.getAttribute('sandbox'), grafanaFrameSandbox);
      expect(frame.sandbox.value, 'allow-scripts allow-same-origin');
      expect(frame.getAttribute('referrerpolicy'), 'no-referrer');
      expect(frame.referrerPolicy, 'no-referrer');
      expect(frame.src, url.toString());
      expect(frame.title, 'Grafana dashboard');
    },
  );
}
