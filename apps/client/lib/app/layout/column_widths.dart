// SPDX-License-Identifier: Apache-2.0

/// The widths of the desktop columns and the personal setting that stores them (docs/spec/
/// 01-produkt.md 5.1 and 11; brief M4-18 Design note 5; docs/DECISIONS.md D-M4-18-5).
library;

import 'package:design_system/design_system.dart';

import '../../settings/settings_providers.dart';

/// The columns of D1 and D2 by the names of `layout.column_widths`.
abstract final class AppColumn {
  /// Spaces and channels.
  static const String sidebar = 'sidebar';

  /// The post list of a forum.
  static const String postList = 'post_list';

  /// The history of a thread; it takes the rest of the width and has no stored width.
  static const String history = 'history';

  /// The session column.
  static const String session = 'session';

  /// The columns with a stored width, left to right.
  static const List<String> sized = [sidebar, postList, session];
}

/// The widths of the sized columns in logical pixels.
///
/// The defaults are the layout tokens of the design system; a stored width replaces its
/// default. A width is never below `DsLayout.columnMin`.
final class ColumnWidths {
  const ColumnWidths._(this._widths);

  /// The widths without a stored value.
  static const ColumnWidths defaults = ColumnWidths._({});

  /// The key of the personal setting (01 11).
  static const String settingKey = 'layout.column_widths';

  static const Map<String, double> _defaults = {
    AppColumn.sidebar: DsLayout.sidebarWidth,
    AppColumn.postList: DsLayout.listWidth,
    AppColumn.session: DsLayout.sessionWidth,
  };

  final Map<String, double> _widths;

  /// Reads the setting value: an object of column names and non-negative integers (01 11).
  /// Unknown names and values that are no non-negative number are skipped, so a value written
  /// by a newer app never breaks this one.
  factory ColumnWidths.fromSetting(Object? value) {
    if (value is! Map) return defaults;
    return ColumnWidths._({
      for (final MapEntry(:key, :value) in value.entries)
        if (_defaults.containsKey(key) && value is num && value >= 0)
          key as String: value.toDouble(),
    });
  }

  /// The width of [column] (one of [AppColumn.sized]).
  double of(String column) {
    final width = _widths[column] ?? _defaults[column] ?? DsLayout.columnMin;
    return width < DsLayout.columnMin ? DsLayout.columnMin : width;
  }

  /// A copy with [column] at [width], at least `DsLayout.columnMin`.
  ColumnWidths withWidth(String column, double width) => ColumnWidths._({
    ..._widths,
    column: width < DsLayout.columnMin ? DsLayout.columnMin : width,
  });

  /// The setting value: the widths that differ from the defaults, rounded to whole pixels.
  Map<String, int> toSetting() => {
    for (final column in AppColumn.sized)
      if (_widths.containsKey(column)) column: of(column).round(),
  };

  @override
  bool operator ==(Object other) =>
      other is ColumnWidths &&
      AppColumn.sized.every((c) => other.of(c) == of(c));

  @override
  int get hashCode => Object.hashAll(AppColumn.sized.map(of));
}

/// Writes the column widths through the settings writer: the local `user_settings` row changes
/// at once (so Settings offers "Reset" right after a drag) and the outbox carries the write
/// (one lane per setting, M4-15; 04 14; brief M4-25 Design note 1).
final class ColumnWidthsWriter {
  /// Creates the writer over [settings].
  ColumnWidthsWriter(this._settings);

  final SettingsRepository _settings;

  /// Stores [widths] as `layout.column_widths` (`putMySetting`).
  Future<void> save(ColumnWidths widths) =>
      _settings.set(ColumnWidths.settingKey, widths.toSetting());

  /// Restores the defaults: deletes the setting (`deleteMySetting`).
  Future<void> reset() => _settings.reset(ColumnWidths.settingKey);
}
