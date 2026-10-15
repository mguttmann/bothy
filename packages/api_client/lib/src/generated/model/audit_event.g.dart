// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'audit_event.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$AuditEvent extends AuditEvent {
  @override
  final int seq;
  @override
  final String id;
  @override
  final DateTime occurredAt;
  @override
  final String? actorPrincipalId;
  @override
  final String? actorDeviceId;
  @override
  final String? actorSessionId;
  @override
  final String action;
  @override
  final AuditDecision decision;
  @override
  final String? denyReason;
  @override
  final String? resourceKind;
  @override
  final String? resourceId;
  @override
  final BuiltMap<String, JsonObject?> details;
  @override
  final String? correlationId;
  @override
  final String prevHash;
  @override
  final String hash;

  factory _$AuditEvent([void Function(AuditEventBuilder)? updates]) =>
      (AuditEventBuilder()..update(updates))._build();

  _$AuditEvent._({
    required this.seq,
    required this.id,
    required this.occurredAt,
    this.actorPrincipalId,
    this.actorDeviceId,
    this.actorSessionId,
    required this.action,
    required this.decision,
    this.denyReason,
    this.resourceKind,
    this.resourceId,
    required this.details,
    this.correlationId,
    required this.prevHash,
    required this.hash,
  }) : super._();
  @override
  AuditEvent rebuild(void Function(AuditEventBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  AuditEventBuilder toBuilder() => AuditEventBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AuditEvent &&
        seq == other.seq &&
        id == other.id &&
        occurredAt == other.occurredAt &&
        actorPrincipalId == other.actorPrincipalId &&
        actorDeviceId == other.actorDeviceId &&
        actorSessionId == other.actorSessionId &&
        action == other.action &&
        decision == other.decision &&
        denyReason == other.denyReason &&
        resourceKind == other.resourceKind &&
        resourceId == other.resourceId &&
        details == other.details &&
        correlationId == other.correlationId &&
        prevHash == other.prevHash &&
        hash == other.hash;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, seq.hashCode);
    _$hash = $jc(_$hash, id.hashCode);
    _$hash = $jc(_$hash, occurredAt.hashCode);
    _$hash = $jc(_$hash, actorPrincipalId.hashCode);
    _$hash = $jc(_$hash, actorDeviceId.hashCode);
    _$hash = $jc(_$hash, actorSessionId.hashCode);
    _$hash = $jc(_$hash, action.hashCode);
    _$hash = $jc(_$hash, decision.hashCode);
    _$hash = $jc(_$hash, denyReason.hashCode);
    _$hash = $jc(_$hash, resourceKind.hashCode);
    _$hash = $jc(_$hash, resourceId.hashCode);
    _$hash = $jc(_$hash, details.hashCode);
    _$hash = $jc(_$hash, correlationId.hashCode);
    _$hash = $jc(_$hash, prevHash.hashCode);
    _$hash = $jc(_$hash, hash.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AuditEvent')
          ..add('seq', seq)
          ..add('id', id)
          ..add('occurredAt', occurredAt)
          ..add('actorPrincipalId', actorPrincipalId)
          ..add('actorDeviceId', actorDeviceId)
          ..add('actorSessionId', actorSessionId)
          ..add('action', action)
          ..add('decision', decision)
          ..add('denyReason', denyReason)
          ..add('resourceKind', resourceKind)
          ..add('resourceId', resourceId)
          ..add('details', details)
          ..add('correlationId', correlationId)
          ..add('prevHash', prevHash)
          ..add('hash', hash))
        .toString();
  }
}

class AuditEventBuilder implements Builder<AuditEvent, AuditEventBuilder> {
  _$AuditEvent? _$v;

  int? _seq;
  int? get seq => _$this._seq;
  set seq(int? seq) => _$this._seq = seq;

  String? _id;
  String? get id => _$this._id;
  set id(String? id) => _$this._id = id;

  DateTime? _occurredAt;
  DateTime? get occurredAt => _$this._occurredAt;
  set occurredAt(DateTime? occurredAt) => _$this._occurredAt = occurredAt;

  String? _actorPrincipalId;
  String? get actorPrincipalId => _$this._actorPrincipalId;
  set actorPrincipalId(String? actorPrincipalId) =>
      _$this._actorPrincipalId = actorPrincipalId;

  String? _actorDeviceId;
  String? get actorDeviceId => _$this._actorDeviceId;
  set actorDeviceId(String? actorDeviceId) =>
      _$this._actorDeviceId = actorDeviceId;

  String? _actorSessionId;
  String? get actorSessionId => _$this._actorSessionId;
  set actorSessionId(String? actorSessionId) =>
      _$this._actorSessionId = actorSessionId;

  String? _action;
  String? get action => _$this._action;
  set action(String? action) => _$this._action = action;

  AuditDecision? _decision;
  AuditDecision? get decision => _$this._decision;
  set decision(AuditDecision? decision) => _$this._decision = decision;

  String? _denyReason;
  String? get denyReason => _$this._denyReason;
  set denyReason(String? denyReason) => _$this._denyReason = denyReason;

  String? _resourceKind;
  String? get resourceKind => _$this._resourceKind;
  set resourceKind(String? resourceKind) => _$this._resourceKind = resourceKind;

  String? _resourceId;
  String? get resourceId => _$this._resourceId;
  set resourceId(String? resourceId) => _$this._resourceId = resourceId;

  MapBuilder<String, JsonObject?>? _details;
  MapBuilder<String, JsonObject?> get details =>
      _$this._details ??= MapBuilder<String, JsonObject?>();
  set details(MapBuilder<String, JsonObject?>? details) =>
      _$this._details = details;

  String? _correlationId;
  String? get correlationId => _$this._correlationId;
  set correlationId(String? correlationId) =>
      _$this._correlationId = correlationId;

  String? _prevHash;
  String? get prevHash => _$this._prevHash;
  set prevHash(String? prevHash) => _$this._prevHash = prevHash;

  String? _hash;
  String? get hash => _$this._hash;
  set hash(String? hash) => _$this._hash = hash;

  AuditEventBuilder() {
    AuditEvent._defaults(this);
  }

  AuditEventBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _seq = $v.seq;
      _id = $v.id;
      _occurredAt = $v.occurredAt;
      _actorPrincipalId = $v.actorPrincipalId;
      _actorDeviceId = $v.actorDeviceId;
      _actorSessionId = $v.actorSessionId;
      _action = $v.action;
      _decision = $v.decision;
      _denyReason = $v.denyReason;
      _resourceKind = $v.resourceKind;
      _resourceId = $v.resourceId;
      _details = $v.details.toBuilder();
      _correlationId = $v.correlationId;
      _prevHash = $v.prevHash;
      _hash = $v.hash;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AuditEvent other) {
    _$v = other as _$AuditEvent;
  }

  @override
  void update(void Function(AuditEventBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AuditEvent build() => _build();

  _$AuditEvent _build() {
    _$AuditEvent _$result;
    try {
      _$result =
          _$v ??
          _$AuditEvent._(
            seq: BuiltValueNullFieldError.checkNotNull(
              seq,
              r'AuditEvent',
              'seq',
            ),
            id: BuiltValueNullFieldError.checkNotNull(id, r'AuditEvent', 'id'),
            occurredAt: BuiltValueNullFieldError.checkNotNull(
              occurredAt,
              r'AuditEvent',
              'occurredAt',
            ),
            actorPrincipalId: actorPrincipalId,
            actorDeviceId: actorDeviceId,
            actorSessionId: actorSessionId,
            action: BuiltValueNullFieldError.checkNotNull(
              action,
              r'AuditEvent',
              'action',
            ),
            decision: BuiltValueNullFieldError.checkNotNull(
              decision,
              r'AuditEvent',
              'decision',
            ),
            denyReason: denyReason,
            resourceKind: resourceKind,
            resourceId: resourceId,
            details: details.build(),
            correlationId: correlationId,
            prevHash: BuiltValueNullFieldError.checkNotNull(
              prevHash,
              r'AuditEvent',
              'prevHash',
            ),
            hash: BuiltValueNullFieldError.checkNotNull(
              hash,
              r'AuditEvent',
              'hash',
            ),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'details';
        details.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'AuditEvent',
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
