// SPDX-License-Identifier: Apache-2.0

/// The typed editors of the instance settings: one entry per key of the configuration reference
/// section 6 with its kind, default and allowed values (brief M6-09 Design note 1; Ruling
/// D-M6-09-1). `InstanceSetting` of the API carries only key, value, version and who changed
/// it, so the kind of each key lives here; `test/instance/setting_kinds_test.dart` holds this
/// table equal to the table of the configuration reference, as `AppConfig` is held equal to its
/// document (D-W-115).
library;

/// How a setting is entered.
enum SettingKind {
  /// Free text (`string`).
  text,

  /// One accent key of the palette (`key of the category colors`, ADR 0044).
  paletteKey,

  /// A UUID (`uuid`).
  uuid,

  /// A language tag of the product (`BCP 47`, 01 9).
  locale,
}

/// Which operation writes a setting (DECISIONS M1-46/1).
enum SettingRoute {
  /// `putInstanceSetting`: edited in the settings list.
  settings,

  /// `updateBranding` and the logo operations: edited on the branding screen only.
  branding,
}

/// One key of the configuration reference section 6. Defaults are the server's: a key without a
/// row is listed with its default and version 0 (DECISIONS M1-46/3).
final class SettingSchema {
  /// Creates the entry.
  const SettingSchema({
    required this.key,
    required this.kind,
    required this.docType,
    required this.route,
  });

  /// The key, for example `locale.default`.
  final String key;

  /// The editor of the key.
  final SettingKind kind;

  /// The text of the column "Type" of the configuration reference, for example `BCP 47`.
  final String docType;

  /// The operation that writes the key.
  final SettingRoute route;
}

/// The accent keys of the palette in the order of ADR 0044 and 01 12; the server holds the same
/// list (`kernel/designkeys`). No other value can be chosen (R-PR-08).
const List<String> accentPaletteKeys = [
  'spruce',
  'loch',
  'heather',
  'rowan',
  'amber',
  'gorse',
  'moss',
];

/// The keys of the configuration reference section 6, in its order.
const List<SettingSchema> instanceSettingSchemas = [
  SettingSchema(
    key: 'branding.product_name',
    kind: SettingKind.text,
    docType: 'string',
    route: SettingRoute.branding,
  ),
  SettingSchema(
    key: 'branding.accent_color',
    kind: SettingKind.paletteKey,
    docType: 'key of the category colors',
    route: SettingRoute.branding,
  ),
  SettingSchema(
    key: 'branding.logo_file_id',
    kind: SettingKind.uuid,
    docType: 'uuid',
    route: SettingRoute.branding,
  ),
  SettingSchema(
    key: 'locale.default',
    kind: SettingKind.locale,
    docType: 'BCP 47',
    route: SettingRoute.settings,
  ),
];

/// The schema of [key], or null for a key this build does not know (shown read-only, with the
/// note to use the API).
SettingSchema? settingSchemaOf(String key) {
  for (final s in instanceSettingSchemas) {
    if (s.key == key) return s;
  }
  return null;
}
