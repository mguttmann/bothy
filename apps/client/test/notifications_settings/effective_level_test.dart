// SPDX-License-Identifier: Apache-2.0

// The effective level for display (brief M4-33 Design note 1; 13 3.1): every step of the order
// with and without `muted_until`, DM channels, the personal and the instance default, and the
// label of the source.

import 'package:client/notifications_settings/effective_level.dart';
import 'package:domain/domain.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:l10n/l10n.dart';

final DateTime now = DateTime.utc(2026, 10, 20, 12);

NotificationPreference pref(
  NotificationTargetKind kind,
  NotificationLevel level, {
  DateTime? mutedUntil,
}) => NotificationPreference(
  targetKind: kind,
  targetId: '0192f4a0-7b1c-7fd5-831e-d4370e087874',
  level: level,
  mutedUntil: mutedUntil,
  version: const ResourceVersion(1),
);

NotificationPreference thread(NotificationLevel l, {DateTime? until}) =>
    pref(NotificationTargetKind.thread, l, mutedUntil: until);
NotificationPreference channel(NotificationLevel l, {DateTime? until}) =>
    pref(NotificationTargetKind.channel, l, mutedUntil: until);
NotificationPreference space(NotificationLevel l, {DateTime? until}) =>
    pref(NotificationTargetKind.space, l, mutedUntil: until);

EffectiveLevel eff({
  NotificationPreference? threadPref,
  NotificationPreference? channelPref,
  NotificationPreference? spacePref,
  NotificationLevel? channelDefault,
  bool isDm = false,
  NotificationLevel? personalDefault,
  NotificationLevel instanceDefault = NotificationLevel.mentions,
}) => effectiveLevel(
  threadPref: threadPref,
  channelPref: channelPref,
  spacePref: spacePref,
  channelDefault: channelDefault,
  isDm: isDm,
  personalDefault: personalDefault,
  instanceDefault: instanceDefault,
  now: now,
);

void main() {
  const all = NotificationLevel.all;
  const mentions = NotificationLevel.mentions;
  const mute = NotificationLevel.mute;

  group('the order of 13 3.1', () {
    test('1. the own row of the thread wins over everything', () {
      final e = eff(
        threadPref: thread(all),
        channelPref: channel(mute),
        spacePref: space(mute),
        channelDefault: mute,
        isDm: true,
        personalDefault: mute,
      );
      expect(e.level, all);
      expect(e.source, LevelSource.thread);
    });

    test('2. the own row of the channel', () {
      final e = eff(
        channelPref: channel(mute),
        spacePref: space(all),
        channelDefault: all,
        personalDefault: all,
      );
      expect((e.level, e.source), (mute, LevelSource.channel));
    });

    test('3. the own row of the space', () {
      final e = eff(spacePref: space(all), channelDefault: mute);
      expect((e.level, e.source), (all, LevelSource.space));
    });

    test('4. the default of the channel', () {
      final e = eff(channelDefault: mute, isDm: true, personalDefault: all);
      expect((e.level, e.source), (mute, LevelSource.channelDefault));
    });

    test('5. a DM channel without a default is all', () {
      final e = eff(isDm: true, personalDefault: mute);
      expect((e.level, e.source), (all, LevelSource.dm));
    });

    test('6. the personal default', () {
      final e = eff(personalDefault: mute);
      expect((e.level, e.source), (mute, LevelSource.personalDefault));
    });

    test('7. the instance default', () {
      expect(
        (eff().level, eff().source),
        (mentions, LevelSource.instanceDefault),
      );
      final e = eff(instanceDefault: all);
      expect((e.level, e.source), (all, LevelSource.instanceDefault));
    });

    test('a row of a newer server with an unknown level counts as absent', () {
      final e = eff(
        threadPref: thread(NotificationLevel.unknown),
        channelPref: channel(mute),
      );
      expect((e.level, e.source), (mute, LevelSource.channel));
    });
  });

  group('muted_until of the selected row', () {
    final later = now.add(const Duration(hours: 1));
    final earlier = now.subtract(const Duration(minutes: 1));

    test('in the future gives mute with the end', () {
      final e = eff(threadPref: thread(all, until: later));
      expect(e.level, mute);
      expect(e.source, LevelSource.thread);
      expect(e.mutedUntil, later);
      expect(e.storedLevel, all);
    });

    test('passed has no effect', () {
      final e = eff(channelPref: channel(all, until: earlier));
      expect(e.level, all);
      expect(e.mutedUntil, isNull);
    });

    test('exactly now has no effect (it must lie in the future)', () {
      expect(eff(spacePref: space(all, until: now)).level, all);
    });

    test('only the selected row counts', () {
      final e = eff(
        threadPref: thread(mentions),
        channelPref: channel(all, until: later),
      );
      expect((e.level, e.source), (mentions, LevelSource.thread));
      expect(e.mutedUntil, isNull);
    });

    test('a passed muted_until keeps a stored mute', () {
      expect(eff(channelPref: channel(mute, until: earlier)).level, mute);
    });
  });

  group('inherited level', () {
    test('is the level without the own row of the target', () {
      final e = inheritedLevel(
        NotificationTargetKind.channel,
        spacePref: space(all),
        channelDefault: mute,
        isDm: false,
        personalDefault: null,
        instanceDefault: mentions,
        now: now,
      );
      expect((e.level, e.source), (all, LevelSource.space));
    });

    test('a space inherits only the personal and the instance default', () {
      final e = inheritedLevel(
        NotificationTargetKind.space,
        channelDefault: mute,
        isDm: true,
        personalDefault: null,
        instanceDefault: mentions,
        now: now,
      );
      expect((e.level, e.source), (mentions, LevelSource.instanceDefault));
    });

    test('a thread inherits from its channel', () {
      final e = inheritedLevel(
        NotificationTargetKind.thread,
        channelPref: channel(mute),
        isDm: false,
        personalDefault: all,
        instanceDefault: mentions,
        now: now,
      );
      expect((e.level, e.source), (mute, LevelSource.channel));
    });
  });

  group('labels', () {
    final en = lookupAppLocalizations(const Locale('en'));
    final de = lookupAppLocalizations(const Locale('de'));

    test('level names', () {
      expect(levelLabel(en, all), 'All');
      expect(levelLabel(en, mentions), 'Mentions');
      expect(levelLabel(en, mute), 'Mute');
      expect(levelLabel(de, mute), isNot(levelLabel(en, mute)));
    });

    test('"inherits: All" names the inherited level', () {
      expect(inheritsLabel(en, eff(personalDefault: all)), 'inherits: All');
    });

    test('the source of each step has its own text', () {
      final texts = {
        for (final s in LevelSource.values) levelSourceLabel(en, s),
      };
      expect(texts, hasLength(LevelSource.values.length));
    });
  });
}
