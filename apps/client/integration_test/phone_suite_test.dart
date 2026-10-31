// SPDX-License-Identifier: Apache-2.0

// The phone integration tests in one app build (brief M5-02 Design note 7): the iOS simulator
// run of `make apps-platform` builds and installs the app once per test file, so this entry
// runs every file of integration_test/phone/ in one build. Each file also runs alone.

import 'package:flutter_test/flutter_test.dart';

import 'phone/app_lock_test.dart' as app_lock;
import 'phone/background_test.dart' as background;
import 'phone/dictation_test.dart' as dictation;
import 'phone/emergency_test.dart' as emergency;
import 'phone/login_flow_test.dart' as login_flow;
import 'phone/mobile_run_graph_test.dart' as mobile_run_graph;
import 'phone/navigation_flow_test.dart' as navigation_flow;
import 'phone/offline_test.dart' as offline;
import 'phone/phone_threads_test.dart' as phone_threads;
import 'phone/viz_test.dart' as viz;
import 'phone/voice_test.dart' as voice;

void main() {
  group('login flow', login_flow.main);
  group('navigation flow', navigation_flow.main);
  group('threads and new chat', phone_threads.main);
  group('background', background.main);
  group('offline', offline.main);
  group('emergency', emergency.main);
  group('run graph', mobile_run_graph.main);
  group('live', viz.main);
  group('voice and photos', voice.main);
  group('dictation and inserts', dictation.main);
  group('app lock', app_lock.main);
}
