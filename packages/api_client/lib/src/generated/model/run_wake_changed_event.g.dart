// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'run_wake_changed_event.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const RunWakeChangedEventAudienceEnum _$runWakeChangedEventAudienceEnum_self =
    const RunWakeChangedEventAudienceEnum._('self');
const RunWakeChangedEventAudienceEnum
_$runWakeChangedEventAudienceEnum_unknownDefaultOpenApi =
    const RunWakeChangedEventAudienceEnum._('unknownDefaultOpenApi');

RunWakeChangedEventAudienceEnum _$runWakeChangedEventAudienceEnumValueOf(
  String name,
) {
  switch (name) {
    case 'self':
      return _$runWakeChangedEventAudienceEnum_self;
    case 'unknownDefaultOpenApi':
      return _$runWakeChangedEventAudienceEnum_unknownDefaultOpenApi;
    default:
      return _$runWakeChangedEventAudienceEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<RunWakeChangedEventAudienceEnum>
_$runWakeChangedEventAudienceEnumValues =
    BuiltSet<RunWakeChangedEventAudienceEnum>(
      const <RunWakeChangedEventAudienceEnum>[
        _$runWakeChangedEventAudienceEnum_self,
        _$runWakeChangedEventAudienceEnum_unknownDefaultOpenApi,
      ],
    );

const RunWakeChangedEventTypeEnum
_$runWakeChangedEventTypeEnum_runPeriodWakeChanged =
    const RunWakeChangedEventTypeEnum._('runPeriodWakeChanged');
const RunWakeChangedEventTypeEnum
_$runWakeChangedEventTypeEnum_unknownDefaultOpenApi =
    const RunWakeChangedEventTypeEnum._('unknownDefaultOpenApi');

RunWakeChangedEventTypeEnum _$runWakeChangedEventTypeEnumValueOf(String name) {
  switch (name) {
    case 'runPeriodWakeChanged':
      return _$runWakeChangedEventTypeEnum_runPeriodWakeChanged;
    case 'unknownDefaultOpenApi':
      return _$runWakeChangedEventTypeEnum_unknownDefaultOpenApi;
    default:
      return _$runWakeChangedEventTypeEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<RunWakeChangedEventTypeEnum>
_$runWakeChangedEventTypeEnumValues = BuiltSet<RunWakeChangedEventTypeEnum>(
  const <RunWakeChangedEventTypeEnum>[
    _$runWakeChangedEventTypeEnum_runPeriodWakeChanged,
    _$runWakeChangedEventTypeEnum_unknownDefaultOpenApi,
  ],
);

Serializer<RunWakeChangedEventAudienceEnum>
_$runWakeChangedEventAudienceEnumSerializer =
    _$RunWakeChangedEventAudienceEnumSerializer();
Serializer<RunWakeChangedEventTypeEnum>
_$runWakeChangedEventTypeEnumSerializer =
    _$RunWakeChangedEventTypeEnumSerializer();

class _$RunWakeChangedEventAudienceEnumSerializer
    implements PrimitiveSerializer<RunWakeChangedEventAudienceEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'self': 'self',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'self': 'self',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[RunWakeChangedEventAudienceEnum];
  @override
  final String wireName = 'RunWakeChangedEventAudienceEnum';

  @override
  Object serialize(
    Serializers serializers,
    RunWakeChangedEventAudienceEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  RunWakeChangedEventAudienceEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => RunWakeChangedEventAudienceEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$RunWakeChangedEventTypeEnumSerializer
    implements PrimitiveSerializer<RunWakeChangedEventTypeEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'runPeriodWakeChanged': 'run.wake_changed',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'run.wake_changed': 'runPeriodWakeChanged',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[RunWakeChangedEventTypeEnum];
  @override
  final String wireName = 'RunWakeChangedEventTypeEnum';

  @override
  Object serialize(
    Serializers serializers,
    RunWakeChangedEventTypeEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  RunWakeChangedEventTypeEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => RunWakeChangedEventTypeEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$RunWakeChangedEvent extends RunWakeChangedEvent {
  @override
  final RunWakeChangedPayload payload;
  @override
  final RunWakeChangedEventTypeEnum type;
  @override
  final String id;
  @override
  final int? seq;
  @override
  final int schemaVersion;
  @override
  final DateTime occurredAt;
  @override
  final EventActor? actor;
  @override
  final EventScope scope;
  @override
  final EventResource resource;
  @override
  final EventEnvelopeAudienceEnum? audience;
  @override
  final bool? redacted;
  @override
  final bool? truncated;
  @override
  final String? correlationId;
  @override
  final String? traceContext;
  @override
  final String? clientMsgId;
  @override
  final int? permissionVersion;
  @override
  final String? invocationToken;
  @override
  final bool? redelivered;
  @override
  final String? syncEpoch;

  factory _$RunWakeChangedEvent([
    void Function(RunWakeChangedEventBuilder)? updates,
  ]) => (RunWakeChangedEventBuilder()..update(updates))._build();

  _$RunWakeChangedEvent._({
    required this.payload,
    required this.type,
    required this.id,
    this.seq,
    required this.schemaVersion,
    required this.occurredAt,
    this.actor,
    required this.scope,
    required this.resource,
    this.audience,
    this.redacted,
    this.truncated,
    this.correlationId,
    this.traceContext,
    this.clientMsgId,
    this.permissionVersion,
    this.invocationToken,
    this.redelivered,
    this.syncEpoch,
  }) : super._();
  @override
  RunWakeChangedEvent rebuild(
    void Function(RunWakeChangedEventBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  RunWakeChangedEventBuilder toBuilder() =>
      RunWakeChangedEventBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is RunWakeChangedEvent &&
        payload == other.payload &&
        type == other.type &&
        id == other.id &&
        seq == other.seq &&
        schemaVersion == other.schemaVersion &&
        occurredAt == other.occurredAt &&
        actor == other.actor &&
        scope == other.scope &&
        resource == other.resource &&
        audience == other.audience &&
        redacted == other.redacted &&
        truncated == other.truncated &&
        correlationId == other.correlationId &&
        traceContext == other.traceContext &&
        clientMsgId == other.clientMsgId &&
        permissionVersion == other.permissionVersion &&
        invocationToken == other.invocationToken &&
        redelivered == other.redelivered &&
        syncEpoch == other.syncEpoch;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, payload.hashCode);
    _$hash = $jc(_$hash, type.hashCode);
    _$hash = $jc(_$hash, id.hashCode);
    _$hash = $jc(_$hash, seq.hashCode);
    _$hash = $jc(_$hash, schemaVersion.hashCode);
    _$hash = $jc(_$hash, occurredAt.hashCode);
    _$hash = $jc(_$hash, actor.hashCode);
    _$hash = $jc(_$hash, scope.hashCode);
    _$hash = $jc(_$hash, resource.hashCode);
    _$hash = $jc(_$hash, audience.hashCode);
    _$hash = $jc(_$hash, redacted.hashCode);
    _$hash = $jc(_$hash, truncated.hashCode);
    _$hash = $jc(_$hash, correlationId.hashCode);
    _$hash = $jc(_$hash, traceContext.hashCode);
    _$hash = $jc(_$hash, clientMsgId.hashCode);
    _$hash = $jc(_$hash, permissionVersion.hashCode);
    _$hash = $jc(_$hash, invocationToken.hashCode);
    _$hash = $jc(_$hash, redelivered.hashCode);
    _$hash = $jc(_$hash, syncEpoch.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'RunWakeChangedEvent')
          ..add('payload', payload)
          ..add('type', type)
          ..add('id', id)
          ..add('seq', seq)
          ..add('schemaVersion', schemaVersion)
          ..add('occurredAt', occurredAt)
          ..add('actor', actor)
          ..add('scope', scope)
          ..add('resource', resource)
          ..add('audience', audience)
          ..add('redacted', redacted)
          ..add('truncated', truncated)
          ..add('correlationId', correlationId)
          ..add('traceContext', traceContext)
          ..add('clientMsgId', clientMsgId)
          ..add('permissionVersion', permissionVersion)
          ..add('invocationToken', invocationToken)
          ..add('redelivered', redelivered)
          ..add('syncEpoch', syncEpoch))
        .toString();
  }
}

class RunWakeChangedEventBuilder
    implements
        Builder<RunWakeChangedEvent, RunWakeChangedEventBuilder>,
        EventEnvelopeBuilder {
  _$RunWakeChangedEvent? _$v;

  RunWakeChangedPayloadBuilder? _payload;
  RunWakeChangedPayloadBuilder get payload =>
      _$this._payload ??= RunWakeChangedPayloadBuilder();
  set payload(covariant RunWakeChangedPayloadBuilder? payload) =>
      _$this._payload = payload;

  RunWakeChangedEventTypeEnum? _type;
  RunWakeChangedEventTypeEnum? get type => _$this._type;
  set type(covariant RunWakeChangedEventTypeEnum? type) => _$this._type = type;

  String? _id;
  String? get id => _$this._id;
  set id(covariant String? id) => _$this._id = id;

  int? _seq;
  int? get seq => _$this._seq;
  set seq(covariant int? seq) => _$this._seq = seq;

  int? _schemaVersion;
  int? get schemaVersion => _$this._schemaVersion;
  set schemaVersion(covariant int? schemaVersion) =>
      _$this._schemaVersion = schemaVersion;

  DateTime? _occurredAt;
  DateTime? get occurredAt => _$this._occurredAt;
  set occurredAt(covariant DateTime? occurredAt) =>
      _$this._occurredAt = occurredAt;

  EventActorBuilder? _actor;
  EventActorBuilder get actor => _$this._actor ??= EventActorBuilder();
  set actor(covariant EventActorBuilder? actor) => _$this._actor = actor;

  EventScopeBuilder? _scope;
  EventScopeBuilder get scope => _$this._scope ??= EventScopeBuilder();
  set scope(covariant EventScopeBuilder? scope) => _$this._scope = scope;

  EventResourceBuilder? _resource;
  EventResourceBuilder get resource =>
      _$this._resource ??= EventResourceBuilder();
  set resource(covariant EventResourceBuilder? resource) =>
      _$this._resource = resource;

  EventEnvelopeAudienceEnum? _audience;
  EventEnvelopeAudienceEnum? get audience => _$this._audience;
  set audience(covariant EventEnvelopeAudienceEnum? audience) =>
      _$this._audience = audience;

  bool? _redacted;
  bool? get redacted => _$this._redacted;
  set redacted(covariant bool? redacted) => _$this._redacted = redacted;

  bool? _truncated;
  bool? get truncated => _$this._truncated;
  set truncated(covariant bool? truncated) => _$this._truncated = truncated;

  String? _correlationId;
  String? get correlationId => _$this._correlationId;
  set correlationId(covariant String? correlationId) =>
      _$this._correlationId = correlationId;

  String? _traceContext;
  String? get traceContext => _$this._traceContext;
  set traceContext(covariant String? traceContext) =>
      _$this._traceContext = traceContext;

  String? _clientMsgId;
  String? get clientMsgId => _$this._clientMsgId;
  set clientMsgId(covariant String? clientMsgId) =>
      _$this._clientMsgId = clientMsgId;

  int? _permissionVersion;
  int? get permissionVersion => _$this._permissionVersion;
  set permissionVersion(covariant int? permissionVersion) =>
      _$this._permissionVersion = permissionVersion;

  String? _invocationToken;
  String? get invocationToken => _$this._invocationToken;
  set invocationToken(covariant String? invocationToken) =>
      _$this._invocationToken = invocationToken;

  bool? _redelivered;
  bool? get redelivered => _$this._redelivered;
  set redelivered(covariant bool? redelivered) =>
      _$this._redelivered = redelivered;

  String? _syncEpoch;
  String? get syncEpoch => _$this._syncEpoch;
  set syncEpoch(covariant String? syncEpoch) => _$this._syncEpoch = syncEpoch;

  RunWakeChangedEventBuilder() {
    RunWakeChangedEvent._defaults(this);
  }

  RunWakeChangedEventBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _payload = $v.payload.toBuilder();
      _type = $v.type;
      _id = $v.id;
      _seq = $v.seq;
      _schemaVersion = $v.schemaVersion;
      _occurredAt = $v.occurredAt;
      _actor = $v.actor?.toBuilder();
      _scope = $v.scope.toBuilder();
      _resource = $v.resource.toBuilder();
      _audience = $v.audience;
      _redacted = $v.redacted;
      _truncated = $v.truncated;
      _correlationId = $v.correlationId;
      _traceContext = $v.traceContext;
      _clientMsgId = $v.clientMsgId;
      _permissionVersion = $v.permissionVersion;
      _invocationToken = $v.invocationToken;
      _redelivered = $v.redelivered;
      _syncEpoch = $v.syncEpoch;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(covariant RunWakeChangedEvent other) {
    _$v = other as _$RunWakeChangedEvent;
  }

  @override
  void update(void Function(RunWakeChangedEventBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  RunWakeChangedEvent build() => _build();

  _$RunWakeChangedEvent _build() {
    _$RunWakeChangedEvent _$result;
    try {
      _$result =
          _$v ??
          _$RunWakeChangedEvent._(
            payload: payload.build(),
            type: BuiltValueNullFieldError.checkNotNull(
              type,
              r'RunWakeChangedEvent',
              'type',
            ),
            id: BuiltValueNullFieldError.checkNotNull(
              id,
              r'RunWakeChangedEvent',
              'id',
            ),
            seq: seq,
            schemaVersion: BuiltValueNullFieldError.checkNotNull(
              schemaVersion,
              r'RunWakeChangedEvent',
              'schemaVersion',
            ),
            occurredAt: BuiltValueNullFieldError.checkNotNull(
              occurredAt,
              r'RunWakeChangedEvent',
              'occurredAt',
            ),
            actor: _actor?.build(),
            scope: scope.build(),
            resource: resource.build(),
            audience: audience,
            redacted: redacted,
            truncated: truncated,
            correlationId: correlationId,
            traceContext: traceContext,
            clientMsgId: clientMsgId,
            permissionVersion: permissionVersion,
            invocationToken: invocationToken,
            redelivered: redelivered,
            syncEpoch: syncEpoch,
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'payload';
        payload.build();

        _$failedField = 'actor';
        _actor?.build();
        _$failedField = 'scope';
        scope.build();
        _$failedField = 'resource';
        resource.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'RunWakeChangedEvent',
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
