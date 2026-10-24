// SPDX-License-Identifier: Apache-2.0

/// The personal settings of the signed-in person, read from `user_settings` in the local
/// database (docs/spec/01-product.md 11; 15 4 "settings, theme, density, motion").
library;

import 'package:design_system/design_system.dart';
import 'package:flutter/widgets.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../layout/column_widths.dart';
import 'ports.dart';
import 'session.dart';

part 'settings.g.dart';

/// Every stored personal setting by key; empty before the login.
@Riverpod(keepAlive: true)
Stream<Map<String, Object?>> userSettings(Ref ref) async* {
  final db = ref.watch(localDatabaseProvider);
  if (db == null) {
    yield const {};
    return;
  }
  yield* db.personalDao.watchSettings().map(
    (settings) => {for (final s in settings) s.key: s.value},
  );
}

/// The column widths of `layout.column_widths`, or the defaults.
@Riverpod(keepAlive: true)
ColumnWidths columnWidths(Ref ref) {
  final settings = ref.watch(userSettingsProvider).value ?? const {};
  return ColumnWidths.fromSetting(settings[ColumnWidths.settingKey]);
}

/// The design context from `appearance.*`, the system brightness and the accent of the
/// instance (01 11, 01 12).
@riverpod
DsContext appearance(
  Ref ref,
  Brightness platformBrightness,
  bool platformReduceMotion,
) {
  final settings = ref.watch(userSettingsProvider).value ?? const {};
  final accent = ref.watch(currentInstanceProvider)?.accentColor;
  String? text(String key) =>
      settings[key] is String ? settings[key]! as String : null;
  return resolveDsContext(
    appearanceTheme: text('appearance.theme'),
    appearanceDensity: text('appearance.density'),
    appearanceReduceMotion: text('appearance.reduce_motion'),
    platformBrightness: platformBrightness,
    platformReduceMotion: platformReduceMotion,
    // 01 11: the density is fixed to `touch` on the phone (brief M5-02 fixed requirement 12).
    isTouchPlatform: ref.watch(appPortsProvider).formFactor.isPhone,
    accentKey: accent,
  );
}
