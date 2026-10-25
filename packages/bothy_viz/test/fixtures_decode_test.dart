// SPDX-License-Identifier: Apache-2.0

import 'package:api_client/api_client.dart' as api;
import 'package:built_value/serializer.dart';
import 'package:flutter_test/flutter_test.dart';

import 'support/fixtures.dart';

/// The serializer of the schema that a fixture names before its first dot (fixtures/README.md).
final Map<String, Serializer<Object>> serializers = {
  'StatsLiveFrame': api.StatsLiveFrame.serializer,
  'StatsTimeline': api.StatsTimeline.serializer,
  'StatsSloReport': api.StatsSloReport.serializer,
  'StatsBackendReport': api.StatsBackendReport.serializer,
  'StatsSeries': api.StatsSeries.serializer,
  'CostSeries': api.CostSeries.serializer,
  'Run': api.Run.serializer,
};

void main() {
  test('there are fixtures', () => expect(fixtureNames(), isNotEmpty));

  for (final name in fixtureNames()) {
    test('$name decodes with the generated serializers', () async {
      final schema = name.split('.').first;
      final serializer = serializers[schema];
      expect(serializer, isNotNull, reason: 'no serializer for $schema');
      final value = await fixtures.decode(name, serializer!);
      expect(value, isNotNull);
    });
  }

  test('a fixture that breaks its schema fails to decode', () async {
    expect(
      () => fixtures.decode(
        'StatsSloReport.normal.json',
        api.StatsLiveFrame.serializer,
      ),
      throwsA(anything),
    );
  });
}
