// SPDX-License-Identifier: Apache-2.0

import 'package:bothy_viz/bothy_viz.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

void main() {
  test(
    'every SLO of the generated catalog has a display name in de and en',
    () async {
      for (final locale in AppLocalizations.supportedLocales) {
        final l10n = await AppLocalizations.delegate.load(locale);
        for (final entry in sloCatalog) {
          expect(
            sloDisplayName(l10n, entry.metric),
            isNotEmpty,
            reason: '${entry.l10nKey} $locale',
          );
        }
        expect(sloDisplayName(l10n, 'tool_calls'), isNull);
      }
    },
  );

  test('the catalog has the six sli metrics, sorted', () {
    final metrics = sloCatalog.map((e) => e.metric).toList();
    expect(metrics, hasLength(6));
    expect(metrics, [...metrics]..sort());
    expect(metrics, everyElement(startsWith('sli.')));
    expect(sloEntry('sli.reconnect')?.slo, 'bothy-sync/reconnect');
  });
}
