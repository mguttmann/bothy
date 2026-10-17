// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'grant.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$Grant extends Grant {
  @override
  final String id;
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
  final BuiltList<Capability> capabilities;
  @override
  final DateTime? expiresAt;
  @override
  final String reason;
  @override
  final String grantedBy;
  @override
  final DateTime grantedAt;
  @override
  final DateTime? revokedAt;
  @override
  final String? revokedBy;
  @override
  final String? revokeReason;

  factory _$Grant([void Function(GrantBuilder)? updates]) =>
      (GrantBuilder()..update(updates))._build();

  _$Grant._({
    required this.id,
    required this.subjectKind,
    required this.subjectId,
    required this.targetKind,
    this.targetId,
    this.level,
    required this.capabilities,
    this.expiresAt,
    required this.reason,
    required this.grantedBy,
    required this.grantedAt,
    this.revokedAt,
    this.revokedBy,
    this.revokeReason,
  }) : super._();
  @override
  Grant rebuild(void Function(GrantBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  GrantBuilder toBuilder() => GrantBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is Grant &&
        id == other.id &&
        subjectKind == other.subjectKind &&
        subjectId == other.subjectId &&
        targetKind == other.targetKind &&
        targetId == other.targetId &&
        level == other.level &&
        capabilities == other.capabilities &&
        expiresAt == other.expiresAt &&
        reason == other.reason &&
        grantedBy == other.grantedBy &&
        grantedAt == other.grantedAt &&
        revokedAt == other.revokedAt &&
        revokedBy == other.revokedBy &&
        revokeReason == other.revokeReason;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, id.hashCode);
    _$hash = $jc(_$hash, subjectKind.hashCode);
    _$hash = $jc(_$hash, subjectId.hashCode);
    _$hash = $jc(_$hash, targetKind.hashCode);
    _$hash = $jc(_$hash, targetId.hashCode);
    _$hash = $jc(_$hash, level.hashCode);
    _$hash = $jc(_$hash, capabilities.hashCode);
    _$hash = $jc(_$hash, expiresAt.hashCode);
    _$hash = $jc(_$hash, reason.hashCode);
    _$hash = $jc(_$hash, grantedBy.hashCode);
    _$hash = $jc(_$hash, grantedAt.hashCode);
    _$hash = $jc(_$hash, revokedAt.hashCode);
    _$hash = $jc(_$hash, revokedBy.hashCode);
    _$hash = $jc(_$hash, revokeReason.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'Grant')
          ..add('id', id)
          ..add('subjectKind', subjectKind)
          ..add('subjectId', subjectId)
          ..add('targetKind', targetKind)
          ..add('targetId', targetId)
          ..add('level', level)
          ..add('capabilities', capabilities)
          ..add('expiresAt', expiresAt)
          ..add('reason', reason)
          ..add('grantedBy', grantedBy)
          ..add('grantedAt', grantedAt)
          ..add('revokedAt', revokedAt)
          ..add('revokedBy', revokedBy)
          ..add('revokeReason', revokeReason))
        .toString();
  }
}

class GrantBuilder implements Builder<Grant, GrantBuilder> {
  _$Grant? _$v;

  String? _id;
  String? get id => _$this._id;
  set id(String? id) => _$this._id = id;

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

  String? _grantedBy;
  String? get grantedBy => _$this._grantedBy;
  set grantedBy(String? grantedBy) => _$this._grantedBy = grantedBy;

  DateTime? _grantedAt;
  DateTime? get grantedAt => _$this._grantedAt;
  set grantedAt(DateTime? grantedAt) => _$this._grantedAt = grantedAt;

  DateTime? _revokedAt;
  DateTime? get revokedAt => _$this._revokedAt;
  set revokedAt(DateTime? revokedAt) => _$this._revokedAt = revokedAt;

  String? _revokedBy;
  String? get revokedBy => _$this._revokedBy;
  set revokedBy(String? revokedBy) => _$this._revokedBy = revokedBy;

  String? _revokeReason;
  String? get revokeReason => _$this._revokeReason;
  set revokeReason(String? revokeReason) => _$this._revokeReason = revokeReason;

  GrantBuilder() {
    Grant._defaults(this);
  }

  GrantBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _id = $v.id;
      _subjectKind = $v.subjectKind;
      _subjectId = $v.subjectId;
      _targetKind = $v.targetKind;
      _targetId = $v.targetId;
      _level = $v.level;
      _capabilities = $v.capabilities.toBuilder();
      _expiresAt = $v.expiresAt;
      _reason = $v.reason;
      _grantedBy = $v.grantedBy;
      _grantedAt = $v.grantedAt;
      _revokedAt = $v.revokedAt;
      _revokedBy = $v.revokedBy;
      _revokeReason = $v.revokeReason;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(Grant other) {
    _$v = other as _$Grant;
  }

  @override
  void update(void Function(GrantBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  Grant build() => _build();

  _$Grant _build() {
    _$Grant _$result;
    try {
      _$result =
          _$v ??
          _$Grant._(
            id: BuiltValueNullFieldError.checkNotNull(id, r'Grant', 'id'),
            subjectKind: BuiltValueNullFieldError.checkNotNull(
              subjectKind,
              r'Grant',
              'subjectKind',
            ),
            subjectId: BuiltValueNullFieldError.checkNotNull(
              subjectId,
              r'Grant',
              'subjectId',
            ),
            targetKind: BuiltValueNullFieldError.checkNotNull(
              targetKind,
              r'Grant',
              'targetKind',
            ),
            targetId: targetId,
            level: level,
            capabilities: capabilities.build(),
            expiresAt: expiresAt,
            reason: BuiltValueNullFieldError.checkNotNull(
              reason,
              r'Grant',
              'reason',
            ),
            grantedBy: BuiltValueNullFieldError.checkNotNull(
              grantedBy,
              r'Grant',
              'grantedBy',
            ),
            grantedAt: BuiltValueNullFieldError.checkNotNull(
              grantedAt,
              r'Grant',
              'grantedAt',
            ),
            revokedAt: revokedAt,
            revokedBy: revokedBy,
            revokeReason: revokeReason,
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'capabilities';
        capabilities.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(r'Grant', _$failedField, e.toString());
      }
      rethrow;
    }
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
