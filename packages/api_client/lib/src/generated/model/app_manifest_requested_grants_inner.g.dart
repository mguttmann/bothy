// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'app_manifest_requested_grants_inner.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$AppManifestRequestedGrantsInner
    extends AppManifestRequestedGrantsInner {
  @override
  final TargetKind targetKind;
  @override
  final Level level;
  @override
  final BuiltList<Capability>? capabilities;
  @override
  final String purpose;

  factory _$AppManifestRequestedGrantsInner([
    void Function(AppManifestRequestedGrantsInnerBuilder)? updates,
  ]) => (AppManifestRequestedGrantsInnerBuilder()..update(updates))._build();

  _$AppManifestRequestedGrantsInner._({
    required this.targetKind,
    required this.level,
    this.capabilities,
    required this.purpose,
  }) : super._();
  @override
  AppManifestRequestedGrantsInner rebuild(
    void Function(AppManifestRequestedGrantsInnerBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  AppManifestRequestedGrantsInnerBuilder toBuilder() =>
      AppManifestRequestedGrantsInnerBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AppManifestRequestedGrantsInner &&
        targetKind == other.targetKind &&
        level == other.level &&
        capabilities == other.capabilities &&
        purpose == other.purpose;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, targetKind.hashCode);
    _$hash = $jc(_$hash, level.hashCode);
    _$hash = $jc(_$hash, capabilities.hashCode);
    _$hash = $jc(_$hash, purpose.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AppManifestRequestedGrantsInner')
          ..add('targetKind', targetKind)
          ..add('level', level)
          ..add('capabilities', capabilities)
          ..add('purpose', purpose))
        .toString();
  }
}

class AppManifestRequestedGrantsInnerBuilder
    implements
        Builder<
          AppManifestRequestedGrantsInner,
          AppManifestRequestedGrantsInnerBuilder
        > {
  _$AppManifestRequestedGrantsInner? _$v;

  TargetKind? _targetKind;
  TargetKind? get targetKind => _$this._targetKind;
  set targetKind(TargetKind? targetKind) => _$this._targetKind = targetKind;

  Level? _level;
  Level? get level => _$this._level;
  set level(Level? level) => _$this._level = level;

  ListBuilder<Capability>? _capabilities;
  ListBuilder<Capability> get capabilities =>
      _$this._capabilities ??= ListBuilder<Capability>();
  set capabilities(ListBuilder<Capability>? capabilities) =>
      _$this._capabilities = capabilities;

  String? _purpose;
  String? get purpose => _$this._purpose;
  set purpose(String? purpose) => _$this._purpose = purpose;

  AppManifestRequestedGrantsInnerBuilder() {
    AppManifestRequestedGrantsInner._defaults(this);
  }

  AppManifestRequestedGrantsInnerBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _targetKind = $v.targetKind;
      _level = $v.level;
      _capabilities = $v.capabilities?.toBuilder();
      _purpose = $v.purpose;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AppManifestRequestedGrantsInner other) {
    _$v = other as _$AppManifestRequestedGrantsInner;
  }

  @override
  void update(void Function(AppManifestRequestedGrantsInnerBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AppManifestRequestedGrantsInner build() => _build();

  _$AppManifestRequestedGrantsInner _build() {
    _$AppManifestRequestedGrantsInner _$result;
    try {
      _$result =
          _$v ??
          _$AppManifestRequestedGrantsInner._(
            targetKind: BuiltValueNullFieldError.checkNotNull(
              targetKind,
              r'AppManifestRequestedGrantsInner',
              'targetKind',
            ),
            level: BuiltValueNullFieldError.checkNotNull(
              level,
              r'AppManifestRequestedGrantsInner',
              'level',
            ),
            capabilities: _capabilities?.build(),
            purpose: BuiltValueNullFieldError.checkNotNull(
              purpose,
              r'AppManifestRequestedGrantsInner',
              'purpose',
            ),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'capabilities';
        _capabilities?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'AppManifestRequestedGrantsInner',
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
