// SPDX-License-Identifier: Apache-2.0

// Accessibility of M11 Running now and M12 Run graph on the phone, portrait (brief M5-10
// Design note 2; 01 5.3 M11, M12, 8): every card and row with agent, state and time in words,
// Stop where allowed, "As list".

import '../../goldens/phone/run_graph_phone_golden_test.dart' as run_graph;
import 'support.dart';

void main() {
  for (final name in ['m11_running_portrait', 'm12_run_graph_portrait']) {
    phoneScreenTests(
      name.startsWith('m11') ? 'M11 Running now' : 'M12 Run graph',
      build: run_graph.scenes[name]!,
    );
  }
}
