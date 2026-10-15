// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'access_changed_payload.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$AccessChangedPayload extends AccessChangedPayload {
  @override
  final int permissionVersion;
  @override
  final BuiltList<AccessTarget> added;
  @override
  final BuiltList<AccessTarget> removed;
  @override
  final BuiltList<AccessTarget> changed;

  factory _$AccessChangedPayload([
    void Function(AccessChangedPayloadBuilder)? updates,
  ]) => (AccessChangedPayloadBuilder()..update(updates))._build();

  _$AccessChangedPayload._({
    required this.permissionVersion,
    required this.added,
    required this.removed,
    required this.changed,
  }) : super._();
  @override
  AccessChangedPayload rebuild(
    void Function(AccessChangedPayloadBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  AccessChangedPayloadBuilder toBuilder() =>
      AccessChangedPayloadBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AccessChangedPayload &&
        permissionVersion == other.permissionVersion &&
        added == other.added &&
        removed == other.removed &&
        changed == other.changed;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, permissionVersion.hashCode);
    _$hash = $jc(_$hash, added.hashCode);
    _$hash = $jc(_$hash, removed.hashCode);
    _$hash = $jc(_$hash, changed.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AccessChangedPayload')
          ..add('permissionVersion', permissionVersion)
          ..add('added', added)
          ..add('removed', removed)
          ..add('changed', changed))
        .toString();
  }
}

class AccessChangedPayloadBuilder
    implements Builder<AccessChangedPayload, AccessChangedPayloadBuilder> {
  _$AccessChangedPayload? _$v;

  int? _permissionVersion;
  int? get permissionVersion => _$this._permissionVersion;
  set permissionVersion(int? permissionVersion) =>
      _$this._permissionVersion = permissionVersion;

  ListBuilder<AccessTarget>? _added;
  ListBuilder<AccessTarget> get added =>
      _$this._added ??= ListBuilder<AccessTarget>();
  set added(ListBuilder<AccessTarget>? added) => _$this._added = added;

  ListBuilder<AccessTarget>? _removed;
  ListBuilder<AccessTarget> get removed =>
      _$this._removed ??= ListBuilder<AccessTarget>();
  set removed(ListBuilder<AccessTarget>? removed) => _$this._removed = removed;

  ListBuilder<AccessTarget>? _changed;
  ListBuilder<AccessTarget> get changed =>
      _$this._changed ??= ListBuilder<AccessTarget>();
  set changed(ListBuilder<AccessTarget>? changed) => _$this._changed = changed;

  AccessChangedPayloadBuilder() {
    AccessChangedPayload._defaults(this);
  }

  AccessChangedPayloadBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _permissionVersion = $v.permissionVersion;
      _added = $v.added.toBuilder();
      _removed = $v.removed.toBuilder();
      _changed = $v.changed.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AccessChangedPayload other) {
    _$v = other as _$AccessChangedPayload;
  }

  @override
  void update(void Function(AccessChangedPayloadBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AccessChangedPayload build() => _build();

  _$AccessChangedPayload _build() {
    _$AccessChangedPayload _$result;
    try {
      _$result =
          _$v ??
          _$AccessChangedPayload._(
            permissionVersion: BuiltValueNullFieldError.checkNotNull(
              permissionVersion,
              r'AccessChangedPayload',
              'permissionVersion',
            ),
            added: added.build(),
            removed: removed.build(),
            changed: changed.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'added';
        added.build();
        _$failedField = 'removed';
        removed.build();
        _$failedField = 'changed';
        changed.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'AccessChangedPayload',
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
