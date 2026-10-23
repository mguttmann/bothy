// SPDX-License-Identifier: Apache-2.0

// The incoming webhooks of the tests (brief M6-08 Tests): Repo Bridge active, Paused Bot
// suspended; "Build alerts" in Marketing/Kampagnen writes as Repo Bridge, "Old pager" in
// Marketing/Intern writes as an app that is no longer listed and was never used.

import 'package:admin_ui/admin_ui.dart';

import '../admin/support.dart';

/// Seeds the apps and incoming webhooks into [r].
void seedIncoming(FakeAdminRepository r) {
  r.seedApp(repoBridgeManifest());
  final paused = Map.of(repoBridgeManifest())
    ..['slug'] = 'paused-bot'
    ..['name'] = 'Paused Bot';
  r.seedApp(paused, state: AppInstallState.suspended);
  r.incomingByChannel['campaigns'] = [
    IncomingWebhookInfo(
      id: 'hook-a',
      appId: 'app-repo-bridge',
      channelId: 'campaigns',
      name: 'Build alerts',
      createdAt: DateTime.utc(2026, 10, 1, 7),
      lastUsedAt: DateTime.utc(2026, 10, 17, 8, 30),
    ),
  ];
  r.incomingByChannel['internal'] = [
    IncomingWebhookInfo(
      id: 'hook-b',
      appId: 'app-gone',
      channelId: 'internal',
      name: 'Old pager',
      createdAt: DateTime.utc(2026, 9, 1, 7),
    ),
  ];
}
