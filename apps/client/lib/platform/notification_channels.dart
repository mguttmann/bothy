// SPDX-License-Identifier: Apache-2.0

/// The notification channels of the Android app (docs/spec/01-product.md 10.3; docs/spec/
/// 13-push-notifications.md 2; docs/spec/15-client-architecture.md 10.2; brief M5-03 Design
/// note 4; docs/decisions.d/M5-03.md D-M5-03-4).
///
/// The app hands the four channels to the runner at every start, so they are visible and
/// changeable in the system settings before the first push arrives; the push service of M5-05
/// posts into them by the kind of the notification ([PushChannel.ofKind]).
library;

import 'package:l10n/l10n.dart';

import '../src/generated/android_platform_api.g.dart';

export '../src/generated/android_platform_api.g.dart'
    show AndroidNotificationChannel, AndroidPlatformApi;

/// One notification channel of 01 10.3: a stable ID, its importance and the notification kinds
/// of 13 2 that go into it.
///
/// The importance is only the start value: Android keeps what the person sets in the system
/// settings, and a quiet notification (`quiet = 1`, 13 3.4) goes into the same channel with
/// `setSilent(true)` (M5-05), so the channel settings stay the person's only control.
enum PushChannel {
  /// Mentions and direct messages: someone addresses the person.
  mentions('mentions', high: true, kinds: ['mention', 'dm']),

  /// Questions of runs, new devices waiting for approval and administration reminders: each
  /// asks the person to act.
  questions(
    'questions',
    high: true,
    kinds: ['input_needed', 'security', 'admin'],
  ),

  /// Every new message at the level `all`.
  messages('messages', high: false, kinds: ['message']),

  /// The end of a run the person started.
  runs('runs', high: false, kinds: ['run_done', 'run_failed']);

  const PushChannel(this.id, {required this.high, required this.kinds});

  /// The channel ID in the system; never changes, because Android keeps the person's
  /// settings per ID.
  final String id;

  /// Whether the channel starts with `IMPORTANCE_HIGH` (sound and heads-up) instead of
  /// `IMPORTANCE_DEFAULT`.
  final bool high;

  /// The notification kinds (wire names of 13 2) posted into this channel.
  final List<String> kinds;

  /// The channel of the notification kind [kind]; a kind this build does not know (a newer
  /// server) goes to [messages], the channel with the least urgency that still shows it.
  ///
  /// Example: `PushChannel.ofKind('dm')` is [mentions].
  static PushChannel ofKind(String kind) => values.firstWhere(
    (channel) => channel.kinds.contains(kind),
    orElse: () => messages,
  );

  /// The name the system settings show, in the language of [l].
  String name(AppLocalizations l) => switch (this) {
    mentions => l.pushChannelMentions,
    questions => l.pushChannelQuestions,
    messages => l.pushChannelMessages,
    runs => l.pushChannelRuns,
  };

  /// The description the system settings show, in the language of [l].
  String description(AppLocalizations l) => switch (this) {
    mentions => l.pushChannelMentionsDescription,
    questions => l.pushChannelQuestionsDescription,
    messages => l.pushChannelMessagesDescription,
    runs => l.pushChannelRunsDescription,
  };

  /// This channel as the runner receives it, with the texts of [l].
  AndroidNotificationChannel toAndroid(AppLocalizations l) =>
      AndroidNotificationChannel(
        id: id,
        name: name(l),
        description: description(l),
        high: high,
        kinds: kinds,
      );
}

/// Hands every [PushChannel] with the texts of [l] (the device language) to the runner over
/// [api]; the runner creates the missing channels and updates the names of the existing ones.
///
/// Throws the `PlatformException` of the runner.
Future<void> registerNotificationChannels(
  AndroidPlatformApi api,
  AppLocalizations l,
) => api.registerNotificationChannels([
  for (final channel in PushChannel.values) channel.toAndroid(l),
]);
