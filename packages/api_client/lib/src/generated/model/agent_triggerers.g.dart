// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'agent_triggerers.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$AgentTriggerers extends AgentTriggerers {
  @override
  final BuiltList<String> principalIds;
  @override
  final int permissionVersion;

  factory _$AgentTriggerers([void Function(AgentTriggerersBuilder)? updates]) =>
      (AgentTriggerersBuilder()..update(updates))._build();

  _$AgentTriggerers._({
    required this.principalIds,
    required this.permissionVersion,
  }) : super._();
  @override
  AgentTriggerers rebuild(void Function(AgentTriggerersBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  AgentTriggerersBuilder toBuilder() => AgentTriggerersBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AgentTriggerers &&
        principalIds == other.principalIds &&
        permissionVersion == other.permissionVersion;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, principalIds.hashCode);
    _$hash = $jc(_$hash, permissionVersion.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AgentTriggerers')
          ..add('principalIds', principalIds)
          ..add('permissionVersion', permissionVersion))
        .toString();
  }
}

class AgentTriggerersBuilder
    implements Builder<AgentTriggerers, AgentTriggerersBuilder> {
  _$AgentTriggerers? _$v;

  ListBuilder<String>? _principalIds;
  ListBuilder<String> get principalIds =>
      _$this._principalIds ??= ListBuilder<String>();
  set principalIds(ListBuilder<String>? principalIds) =>
      _$this._principalIds = principalIds;

  int? _permissionVersion;
  int? get permissionVersion => _$this._permissionVersion;
  set permissionVersion(int? permissionVersion) =>
      _$this._permissionVersion = permissionVersion;

  AgentTriggerersBuilder() {
    AgentTriggerers._defaults(this);
  }

  AgentTriggerersBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _principalIds = $v.principalIds.toBuilder();
      _permissionVersion = $v.permissionVersion;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AgentTriggerers other) {
    _$v = other as _$AgentTriggerers;
  }

  @override
  void update(void Function(AgentTriggerersBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AgentTriggerers build() => _build();

  _$AgentTriggerers _build() {
    _$AgentTriggerers _$result;
    try {
      _$result =
          _$v ??
          _$AgentTriggerers._(
            principalIds: principalIds.build(),
            permissionVersion: BuiltValueNullFieldError.checkNotNull(
              permissionVersion,
              r'AgentTriggerers',
              'permissionVersion',
            ),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'principalIds';
        principalIds.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'AgentTriggerers',
          _$failedField,
          e.toString(),
        );
      }
      rethrow;
    }
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
