// SPDX-License-Identifier: Apache-2.0

/// Mirror tables of the instance (15 6.1: `instance` of 03 `instance_settings` with branding and
/// evaluated switches, `commands` for suggestions offline).
library;

import 'package:drift/drift.dart';

/// The one row of the API `InstanceInfo`: branding (03 `instance_settings`), evaluated
/// switches and the limits the server delivers.
@DataClassName('InstanceRow')
class InstanceTable extends Table {
  @override
  String get tableName => 'instance';

  /// API `InstanceInfo.instance_id`.
  TextColumn get instanceId => text()();

  /// 03 `instance_settings` key `branding.product_name`.
  TextColumn get productName => text()();

  /// 03 `instance_settings` key `branding.accent_color`.
  TextColumn get accentColor => text().nullable()();

  /// 03 `instance_settings` key `branding.logo_file_id`.
  TextColumn get logoFileId => text().nullable()();

  /// API `Branding.version` (03 `instance_settings.version`).
  IntColumn get brandingVersion => integer()();

  /// 03 `instance_settings` key `locale.default`.
  TextColumn get localeDefault => text()();

  /// API `InstanceInfo.spec_version`.
  TextColumn get specVersion => text()();

  /// API `InstanceInfo.features` (evaluated switches) as a JSON object.
  TextColumn get features => text()();

  /// API `InstanceLimits` as a JSON object.
  TextColumn get limits => text()();

  @override
  Set<Column<Object>> get primaryKey => {instanceId};
}

/// 03 `commands` without `created_at` and `removed_at` (a removed command is deleted).
@DataClassName('CommandRow')
class Commands extends Table {
  @override
  String get tableName => 'commands';

  /// 03 `commands.id`.
  TextColumn get id => text()();

  /// 03 `commands.owner_principal_id`.
  TextColumn get ownerPrincipalId => text()();

  /// 03 `commands.name`.
  TextColumn get name => text()();

  /// 03 `commands.description`.
  TextColumn get description => text()();

  /// 03 `commands.scope_kind`.
  TextColumn get scopeKind => text()();

  /// API `Command.scope_id` (03 `scope_space_id` or `scope_channel_id`).
  TextColumn get scopeId => text().nullable()();

  /// 03 `commands.options` as JSON text.
  TextColumn get options => text()();

  /// 03 `commands.required_level`.
  TextColumn get requiredLevel => text()();

  /// 03 `commands.control`.
  BoolColumn get control => boolean()();

  /// 03 `commands.version`.
  IntColumn get version => integer()();

  @override
  Set<Column<Object>> get primaryKey => {id};
}
