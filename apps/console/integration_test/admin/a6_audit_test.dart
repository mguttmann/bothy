// SPDX-License-Identifier: Apache-2.0

// A6 in the browser (brief M6-04 Design note 4): filter the audit log by action, check the hash
// chain and export CSV as the owner; the runner checks the downloaded file.

import 'package:flutter_test/flutter_test.dart';
import 'package:patrol/patrol.dart';

import 'support.dart';

void main() {
  patrolTest('A6 filter by action, check the chain and export CSV', ($) async {
    await openConsole($, '/audit');
    await waitFor($, byKey('audit.list'));
    await typeKey($, 'audit.filter.action', 'access.view_as');
    await $.tester.testTextInput.receiveAction(TextInputAction.done);
    await tapKey($, 'audit.filter.apply');
    await waitFor($, find.textContaining('access.view_as'));

    await tapKey($, 'audit.check');
    await waitFor($, find.textContaining('Chain checked up to'));

    await typeKey($, 'audit.filter.from', dayFromToday(-1));
    await typeKey($, 'audit.filter.to', dayFromToday(1));
    await $.tester.testTextInput.receiveAction(TextInputAction.done);
    await tapKey($, 'audit.exportCsv');
    await waitFor($, find.text('Export saved.'));
    expectStepUp();
  });
}
