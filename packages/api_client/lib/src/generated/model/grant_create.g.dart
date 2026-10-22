// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'grant_create.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$GrantCreate extends GrantCreate {
  @override
  final GrantSubjectKind subjectKind;
  @override
  final String subjectId;
  @override
  final TargetKind targetKind;
  @override
  final String? targetId;
  @override
  final Level? level;
  @override
  final BuiltList<Capability>? capabilities;
  @override
  final DateTime? expiresAt;
  @override
  final String reason;
  @override
  final ResolveI1? resolveI1;

  factory _$GrantCreate([void Function(GrantCreateBuilder)? updates]) =>
      (GrantCreateBuilder()..update(updates))._build();

  _$GrantCreate._({
    required this.subjectKind,
    required this.subjectId,
    required this.targetKind,
    this.targetId,
    this.level,
    this.capabilities,
    this.expiresAt,
    required this.reason,
    this.resolveI1,
  }) : super._();
  @override
  GrantCreate rebuild(void Function(GrantCreateBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  GrantCreateBuilder toBuilder() => GrantCreateBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is GrantCreate &&
        subjectKind == other.subjectKind &&
        subjectId == other.subjectId &&
        targetKind == other.targetKind &&
        targetId == other.targetId &&
        level == other.level &&
        capabilities == other.capabilities &&
        expiresAt == other.expiresAt &&
        reason == other.reason &&
        resolveI1 == other.resolveI1;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, subjectKind.hashCode);
    _$hash = $jc(_$hash, subjectId.hashCode);
    _$hash = $jc(_$hash, targetKind.hashCode);
    _$hash = $jc(_$hash, targetId.hashCode);
    _$hash = $jc(_$hash, level.hashCode);
    _$hash = $jc(_$hash, capabilities.hashCode);
    _$hash = $jc(_$hash, expiresAt.hashCode);
    _$hash = $jc(_$hash, reason.hashCode);
    _$hash = $jc(_$hash, resolveI1.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'GrantCreate')
          ..add('subjectKind', subjectKind)
          ..add('subjectId', subjectId)
          ..add('targetKind', targetKind)
          ..add('targetId', targetId)
          ..add('level', level)
          ..add('capabilities', capabilities)
          ..add('expiresAt', expiresAt)
          ..add('reason', reason)
          ..add('resolveI1', resolveI1))
        .toString();
  }
}

class GrantCreateBuilder implements Builder<GrantCreate, GrantCreateBuilder> {
  _$GrantCreate? _$v;

  GrantSubjectKind? _subjectKind;
  GrantSubjectKind? get subjectKind => _$this._subjectKind;
  set subjectKind(GrantSubjectKind? subjectKind) =>
      _$this._subjectKind = subjectKind;

  String? _subjectId;
  String? get subjectId => _$this._subjectId;
  set subjectId(String? subjectId) => _$this._subjectId = subjectId;

  TargetKind? _targetKind;
  TargetKind? get targetKind => _$this._targetKind;
  set targetKind(TargetKind? targetKind) => _$this._targetKind = targetKind;

  String? _targetId;
  String? get targetId => _$this._targetId;
  set targetId(String? targetId) => _$this._targetId = targetId;

  Level? _level;
  Level? get level => _$this._level;
  set level(Level? level) => _$this._level = level;

  ListBuilder<Capability>? _capabilities;
  ListBuilder<Capability> get capabilities =>
      _$this._capabilities ??= ListBuilder<Capability>();
  set capabilities(ListBuilder<Capability>? capabilities) =>
      _$this._capabilities = capabilities;

  DateTime? _expiresAt;
  DateTime? get expiresAt => _$this._expiresAt;
  set expiresAt(DateTime? expiresAt) => _$this._expiresAt = expiresAt;

  String? _reason;
  String? get reason => _$this._reason;
  set reason(String? reason) => _$this._reason = reason;

  ResolveI1Builder? _resolveI1;
  ResolveI1Builder get resolveI1 => _$this._resolveI1 ??= ResolveI1Builder();
  set resolveI1(ResolveI1Builder? resolveI1) => _$this._resolveI1 = resolveI1;

  GrantCreateBuilder() {
    GrantCreate._defaults(this);
  }

  GrantCreateBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _subjectKind = $v.subjectKind;
      _subjectId = $v.subjectId;
      _targetKind = $v.targetKind;
      _targetId = $v.targetId;
      _level = $v.level;
      _capabilities = $v.capabilities?.toBuilder();
      _expiresAt = $v.expiresAt;
      _reason = $v.reason;
      _resolveI1 = $v.resolveI1?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(GrantCreate other) {
    _$v = other as _$GrantCreate;
  }

  @override
  void update(void Function(GrantCreateBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  GrantCreate build() => _build();

  _$GrantCreate _build() {
    _$GrantCreate _$result;
    try {
      _$result =
          _$v ??
          _$GrantCreate._(
            subjectKind: BuiltValueNullFieldError.checkNotNull(
              subjectKind,
              r'GrantCreate',
              'subjectKind',
            ),
            subjectId: BuiltValueNullFieldError.checkNotNull(
              subjectId,
              r'GrantCreate',
              'subjectId',
            ),
            targetKind: BuiltValueNullFieldError.checkNotNull(
              targetKind,
              r'GrantCreate',
              'targetKind',
            ),
            targetId: targetId,
            level: level,
            capabilities: _capabilities?.build(),
            expiresAt: expiresAt,
            reason: BuiltValueNullFieldError.checkNotNull(
              reason,
              r'GrantCreate',
              'reason',
            ),
            resolveI1: _resolveI1?.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'capabilities';
        _capabilities?.build();

        _$failedField = 'resolveI1';
        _resolveI1?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'GrantCreate',
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
