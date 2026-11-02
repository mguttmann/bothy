// SPDX-License-Identifier: Apache-2.0

// The open dialog warns before the step-up when the person's own session user on the host is the
// owner account (ADR 0073, D-OA-11): the session runs with administrator rights. A separate
// session user shows no such warning.

import 'package:api_client/api_client.dart';
import 'package:built_collection/built_collection.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:terminal_ui/terminal_ui.dart';

import 'support/fakes.dart';
import 'support/widgets.dart';

Host _host({required bool ownerAccount}) => Host(
  (b) => b
    ..id = hostIdText
    ..name = 'Home server'
    ..state = HostState.active
    ..llmCredentials = LlmCredentials.none
    ..declaredServerHost = true
    ..keyGeneration = 1
    ..profiles = ListBuilder<HostProfile>([
      HostProfile(
        (p) => p
          ..name = 'shell'
          ..displayName = 'Shell'
          ..kind = HostProfileKindEnum.shell,
      ),
    ])
    ..sessionUsers = ListBuilder<HostSessionUser>([
      HostSessionUser(
        (s) => s
          ..userId = alexId
          ..osUser = 'admin'
          ..createdAt = DateTime.utc(2026, 11)
          ..ownerAccount = ownerAccount ? true : null,
      ),
    ])
    ..connected = true
    ..createdAt = DateTime.utc(2026, 11)
    ..version = 1,
);

void main() {
  for (final ownerAccount in [true, false]) {
    testWidgets('owner account $ownerAccount: warning in the open dialog', (
      tester,
    ) async {
      await pumpApp(
        tester,
        OpenTerminalDialog(
          host: _host(ownerAccount: ownerAccount),
          initial: const OpenChoice(profile: 'shell'),
          selfUserId: alexId,
        ),
      );
      final warning = find.textContaining(
        'your own account with administrator rights',
      );
      expect(warning, ownerAccount ? findsOneWidget : findsNothing);
      if (ownerAccount) expect(find.textContaining('admin'), findsWidgets);
    });
  }
}
