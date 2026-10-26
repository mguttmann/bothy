// SPDX-License-Identifier: Apache-2.0

// The InstanceInfo body of the console tests is parsed by the generated client
// (packages/api_client), so a member the contract makes required fails here, at one place,
// instead of as a missing widget in a page test (M5-08: `app_lock` broke the live and Grafana
// page tests of parallel tickets).

import 'package:api_client/api_client.dart';
import 'package:flutter_test/flutter_test.dart';

import 'fakes.dart';

void main() {
  test('the InstanceInfo body of the tests is a valid InstanceInfo of the contract', () {
    final info = standardSerializers.deserializeWith(
      InstanceInfo.serializer,
      instanceInfo(
        localeDefault: 'de',
        extra: {
          'telemetry_dashboard_url': 'https://grafana.example.com/d/bothy-live',
          'grafana_embed_url': 'https://grafana.example.com/d/ops',
        },
      ),
    )!;
    expect(info.localeDefault, 'de');
    expect(info.appLock.required_, isFalse);
    expect(info.appLock.afterMs, 300000);
    expect(
      info.telemetryDashboardUrl,
      'https://grafana.example.com/d/bothy-live',
    );
    expect(info.grafanaEmbedUrl, 'https://grafana.example.com/d/ops');
  });
}
