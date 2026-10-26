// SPDX-License-Identifier: Apache-2.0

// "Live" in the browser (brief M6-16 Tests; 01 5.5; 25 5.1, 6.6): the page polls getStatsLive of
// the real server under the enforced CSP and shows the frame, every view of D12 shows its table
// view, and TV mode covers the page and ends on a click. The runner fails the phase on any CSP
// violation of the page. A simulated key press is not used here: in the browser's test binding
// `sendKeyEvent` throws a null check error; the widget and desktop tests prove that a key ends
// TV mode.

import 'package:bothy_viz/bothy_viz.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:patrol/patrol.dart';

import 'support.dart';

void main() {
  patrolTest('Live polls getStatsLive, shows every view with its table and '
      'TV mode ends on a click', ($) async {
    await openConsole($, '/live');
    await waitFor($, byKey('live.tabs'));
    // "As of <time>" stands only under a frame the server answered (getStatsLive).
    await waitFor($, find.textContaining('As of '));

    for (final kind in [
      VizViewKind.liveTopology,
      VizViewKind.backend,
      VizViewKind.delivery,
      VizViewKind.executive,
      VizViewKind.timeline,
    ]) {
      await tapKey($, 'live.tab.${kind.name}');
      await tapKey($, 'viz-as-table');
      await waitFor($, find.byType(VizTable));
    }

    await tapKey($, 'live.tv');
    await waitFor($, find.byType(TvMode));
    expect(byKey('live.tabs'), findsNothing);
    await $.tester.tapAt($.tester.getCenter(find.byType(TvMode)));
    await waitGone($, find.byType(TvMode));
    await waitFor($, byKey('live.tabs'));
  });
}
