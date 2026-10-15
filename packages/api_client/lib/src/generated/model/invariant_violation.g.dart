// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'invariant_violation.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$InvariantViolation extends InvariantViolation {
  @override
  final String code;
  @override
  final String? agentId;
  @override
  final String? hostId;
  @override
  final String? credentialHolderId;
  @override
  final BuiltList<String> principalIds;
  @override
  final bool? organizationReady;
  @override
  final String? hint;

  factory _$InvariantViolation([
    void Function(InvariantViolationBuilder)? updates,
  ]) => (InvariantViolationBuilder()..update(updates))._build();

  _$InvariantViolation._({
    required this.code,
    this.agentId,
    this.hostId,
    this.credentialHolderId,
    required this.principalIds,
    this.organizationReady,
    this.hint,
  }) : super._();
  @override
  InvariantViolation rebuild(
    void Function(InvariantViolationBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  InvariantViolationBuilder toBuilder() =>
      InvariantViolationBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is InvariantViolation &&
        code == other.code &&
        agentId == other.agentId &&
        hostId == other.hostId &&
        credentialHolderId == other.credentialHolderId &&
        principalIds == other.principalIds &&
        organizationReady == other.organizationReady &&
        hint == other.hint;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, code.hashCode);
    _$hash = $jc(_$hash, agentId.hashCode);
    _$hash = $jc(_$hash, hostId.hashCode);
    _$hash = $jc(_$hash, credentialHolderId.hashCode);
    _$hash = $jc(_$hash, principalIds.hashCode);
    _$hash = $jc(_$hash, organizationReady.hashCode);
    _$hash = $jc(_$hash, hint.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'InvariantViolation')
          ..add('code', code)
          ..add('agentId', agentId)
          ..add('hostId', hostId)
          ..add('credentialHolderId', credentialHolderId)
          ..add('principalIds', principalIds)
          ..add('organizationReady', organizationReady)
          ..add('hint', hint))
        .toString();
  }
}

class InvariantViolationBuilder
    implements Builder<InvariantViolation, InvariantViolationBuilder> {
  _$InvariantViolation? _$v;

  String? _code;
  String? get code => _$this._code;
  set code(String? code) => _$this._code = code;

  String? _agentId;
  String? get agentId => _$this._agentId;
  set agentId(String? agentId) => _$this._agentId = agentId;

  String? _hostId;
  String? get hostId => _$this._hostId;
  set hostId(String? hostId) => _$this._hostId = hostId;

  String? _credentialHolderId;
  String? get credentialHolderId => _$this._credentialHolderId;
  set credentialHolderId(String? credentialHolderId) =>
      _$this._credentialHolderId = credentialHolderId;

  ListBuilder<String>? _principalIds;
  ListBuilder<String> get principalIds =>
      _$this._principalIds ??= ListBuilder<String>();
  set principalIds(ListBuilder<String>? principalIds) =>
      _$this._principalIds = principalIds;

  bool? _organizationReady;
  bool? get organizationReady => _$this._organizationReady;
  set organizationReady(bool? organizationReady) =>
      _$this._organizationReady = organizationReady;

  String? _hint;
  String? get hint => _$this._hint;
  set hint(String? hint) => _$this._hint = hint;

  InvariantViolationBuilder() {
    InvariantViolation._defaults(this);
  }

  InvariantViolationBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _code = $v.code;
      _agentId = $v.agentId;
      _hostId = $v.hostId;
      _credentialHolderId = $v.credentialHolderId;
      _principalIds = $v.principalIds.toBuilder();
      _organizationReady = $v.organizationReady;
      _hint = $v.hint;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(InvariantViolation other) {
    _$v = other as _$InvariantViolation;
  }

  @override
  void update(void Function(InvariantViolationBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  InvariantViolation build() => _build();

  _$InvariantViolation _build() {
    _$InvariantViolation _$result;
    try {
      _$result =
          _$v ??
          _$InvariantViolation._(
            code: BuiltValueNullFieldError.checkNotNull(
              code,
              r'InvariantViolation',
              'code',
            ),
            agentId: agentId,
            hostId: hostId,
            credentialHolderId: credentialHolderId,
            principalIds: principalIds.build(),
            organizationReady: organizationReady,
            hint: hint,
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'principalIds';
        principalIds.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'InvariantViolation',
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
