// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'access_effect.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$AccessEffect extends AccessEffect {
  @override
  final PrincipalRef principal;
  @override
  final TargetKind targetKind;
  @override
  final String? targetId;
  @override
  final LevelOrNone beforeLevel;
  @override
  final LevelOrNone afterLevel;
  @override
  final BuiltList<Capability> capabilitiesAdded;

  factory _$AccessEffect([void Function(AccessEffectBuilder)? updates]) =>
      (AccessEffectBuilder()..update(updates))._build();

  _$AccessEffect._({
    required this.principal,
    required this.targetKind,
    this.targetId,
    required this.beforeLevel,
    required this.afterLevel,
    required this.capabilitiesAdded,
  }) : super._();
  @override
  AccessEffect rebuild(void Function(AccessEffectBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  AccessEffectBuilder toBuilder() => AccessEffectBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AccessEffect &&
        principal == other.principal &&
        targetKind == other.targetKind &&
        targetId == other.targetId &&
        beforeLevel == other.beforeLevel &&
        afterLevel == other.afterLevel &&
        capabilitiesAdded == other.capabilitiesAdded;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, principal.hashCode);
    _$hash = $jc(_$hash, targetKind.hashCode);
    _$hash = $jc(_$hash, targetId.hashCode);
    _$hash = $jc(_$hash, beforeLevel.hashCode);
    _$hash = $jc(_$hash, afterLevel.hashCode);
    _$hash = $jc(_$hash, capabilitiesAdded.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AccessEffect')
          ..add('principal', principal)
          ..add('targetKind', targetKind)
          ..add('targetId', targetId)
          ..add('beforeLevel', beforeLevel)
          ..add('afterLevel', afterLevel)
          ..add('capabilitiesAdded', capabilitiesAdded))
        .toString();
  }
}

class AccessEffectBuilder
    implements Builder<AccessEffect, AccessEffectBuilder> {
  _$AccessEffect? _$v;

  PrincipalRefBuilder? _principal;
  PrincipalRefBuilder get principal =>
      _$this._principal ??= PrincipalRefBuilder();
  set principal(PrincipalRefBuilder? principal) =>
      _$this._principal = principal;

  TargetKind? _targetKind;
  TargetKind? get targetKind => _$this._targetKind;
  set targetKind(TargetKind? targetKind) => _$this._targetKind = targetKind;

  String? _targetId;
  String? get targetId => _$this._targetId;
  set targetId(String? targetId) => _$this._targetId = targetId;

  LevelOrNone? _beforeLevel;
  LevelOrNone? get beforeLevel => _$this._beforeLevel;
  set beforeLevel(LevelOrNone? beforeLevel) =>
      _$this._beforeLevel = beforeLevel;

  LevelOrNone? _afterLevel;
  LevelOrNone? get afterLevel => _$this._afterLevel;
  set afterLevel(LevelOrNone? afterLevel) => _$this._afterLevel = afterLevel;

  ListBuilder<Capability>? _capabilitiesAdded;
  ListBuilder<Capability> get capabilitiesAdded =>
      _$this._capabilitiesAdded ??= ListBuilder<Capability>();
  set capabilitiesAdded(ListBuilder<Capability>? capabilitiesAdded) =>
      _$this._capabilitiesAdded = capabilitiesAdded;

  AccessEffectBuilder() {
    AccessEffect._defaults(this);
  }

  AccessEffectBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _principal = $v.principal.toBuilder();
      _targetKind = $v.targetKind;
      _targetId = $v.targetId;
      _beforeLevel = $v.beforeLevel;
      _afterLevel = $v.afterLevel;
      _capabilitiesAdded = $v.capabilitiesAdded.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AccessEffect other) {
    _$v = other as _$AccessEffect;
  }

  @override
  void update(void Function(AccessEffectBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AccessEffect build() => _build();

  _$AccessEffect _build() {
    _$AccessEffect _$result;
    try {
      _$result =
          _$v ??
          _$AccessEffect._(
            principal: principal.build(),
            targetKind: BuiltValueNullFieldError.checkNotNull(
              targetKind,
              r'AccessEffect',
              'targetKind',
            ),
            targetId: targetId,
            beforeLevel: BuiltValueNullFieldError.checkNotNull(
              beforeLevel,
              r'AccessEffect',
              'beforeLevel',
            ),
            afterLevel: BuiltValueNullFieldError.checkNotNull(
              afterLevel,
              r'AccessEffect',
              'afterLevel',
            ),
            capabilitiesAdded: capabilitiesAdded.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'principal';
        principal.build();

        _$failedField = 'capabilitiesAdded';
        capabilitiesAdded.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'AccessEffect',
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
