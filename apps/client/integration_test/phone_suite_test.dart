// SPDX-License-Identifier: Apache-2.0

// The phone integration tests in one app build (brief M5-02 Design note 7): the iOS simulator
// run of `make apps-platform` builds and installs the app once per test file, so this entry
// runs every file of integration_test/phone/ in one build. Each file also runs alone.

import 'package:flutter_test/flutter_test.dart';

import 'phone/background_test.dart' as background;
import 'phone/emergency_test.dart' as emergency;
import 'phone/login_flow_test.dart' as login_flow;
import 'phone/navigation_flow_test.dart' as navigation_flow;
import 'phone/offline_test.dart' as offline;

void main() {
  group('login flow', login_flow.main);
  group('navigation flow', navigation_flow.main);
  group('background', background.main);
  group('offline', offline.main);
  group('emergency', emergency.main);
}
