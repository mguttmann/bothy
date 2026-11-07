// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'agent_wake_input_event.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const AgentWakeInputEventAudienceEnum _$agentWakeInputEventAudienceEnum_self =
    const AgentWakeInputEventAudienceEnum._('self');
const AgentWakeInputEventAudienceEnum
_$agentWakeInputEventAudienceEnum_unknownDefaultOpenApi =
    const AgentWakeInputEventAudienceEnum._('unknownDefaultOpenApi');

AgentWakeInputEventAudienceEnum _$agentWakeInputEventAudienceEnumValueOf(
  String name,
) {
  switch (name) {
    case 'self':
      return _$agentWakeInputEventAudienceEnum_self;
    case 'unknownDefaultOpenApi':
      return _$agentWakeInputEventAudienceEnum_unknownDefaultOpenApi;
    default:
      return _$agentWakeInputEventAudienceEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<AgentWakeInputEventAudienceEnum>
_$agentWakeInputEventAudienceEnumValues =
    BuiltSet<AgentWakeInputEventAudienceEnum>(
      const <AgentWakeInputEventAudienceEnum>[
        _$agentWakeInputEventAudienceEnum_self,
        _$agentWakeInputEventAudienceEnum_unknownDefaultOpenApi,
      ],
    );

const AgentWakeInputEventTypeEnum
_$agentWakeInputEventTypeEnum_agentPeriodWakeInput =
    const AgentWakeInputEventTypeEnum._('agentPeriodWakeInput');
const AgentWakeInputEventTypeEnum
_$agentWakeInputEventTypeEnum_unknownDefaultOpenApi =
    const AgentWakeInputEventTypeEnum._('unknownDefaultOpenApi');

AgentWakeInputEventTypeEnum _$agentWakeInputEventTypeEnumValueOf(String name) {
  switch (name) {
    case 'agentPeriodWakeInput':
      return _$agentWakeInputEventTypeEnum_agentPeriodWakeInput;
    case 'unknownDefaultOpenApi':
      return _$agentWakeInputEventTypeEnum_unknownDefaultOpenApi;
    default:
      return _$agentWakeInputEventTypeEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<AgentWakeInputEventTypeEnum>
_$agentWakeInputEventTypeEnumValues = BuiltSet<AgentWakeInputEventTypeEnum>(
  const <AgentWakeInputEventTypeEnum>[
    _$agentWakeInputEventTypeEnum_agentPeriodWakeInput,
    _$agentWakeInputEventTypeEnum_unknownDefaultOpenApi,
  ],
);

Serializer<AgentWakeInputEventAudienceEnum>
_$agentWakeInputEventAudienceEnumSerializer =
    _$AgentWakeInputEventAudienceEnumSerializer();
Serializer<AgentWakeInputEventTypeEnum>
_$agentWakeInputEventTypeEnumSerializer =
    _$AgentWakeInputEventTypeEnumSerializer();

class _$AgentWakeInputEventAudienceEnumSerializer
    implements PrimitiveSerializer<AgentWakeInputEventAudienceEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'self': 'self',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'self': 'self',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[AgentWakeInputEventAudienceEnum];
  @override
  final String wireName = 'AgentWakeInputEventAudienceEnum';

  @override
  Object serialize(
    Serializers serializers,
    AgentWakeInputEventAudienceEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  AgentWakeInputEventAudienceEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => AgentWakeInputEventAudienceEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$AgentWakeInputEventTypeEnumSerializer
    implements PrimitiveSerializer<AgentWakeInputEventTypeEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'agentPeriodWakeInput': 'agent.wake_input',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'agent.wake_input': 'agentPeriodWakeInput',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[AgentWakeInputEventTypeEnum];
  @override
  final String wireName = 'AgentWakeInputEventTypeEnum';

  @override
  Object serialize(
    Serializers serializers,
    AgentWakeInputEventTypeEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  AgentWakeInputEventTypeEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => AgentWakeInputEventTypeEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$AgentWakeInputEvent extends AgentWakeInputEvent {
  @override
  final AgentWakeInputPayload payload;
  @override
  final AgentWakeInputEventTypeEnum type;
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

  factory _$AgentWakeInputEvent([
    void Function(AgentWakeInputEventBuilder)? updates,
  ]) => (AgentWakeInputEventBuilder()..update(updates))._build();

  _$AgentWakeInputEvent._({
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
  AgentWakeInputEvent rebuild(
    void Function(AgentWakeInputEventBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  AgentWakeInputEventBuilder toBuilder() =>
      AgentWakeInputEventBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AgentWakeInputEvent &&
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
    return (newBuiltValueToStringHelper(r'AgentWakeInputEvent')
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

class AgentWakeInputEventBuilder
    implements
        Builder<AgentWakeInputEvent, AgentWakeInputEventBuilder>,
        EventEnvelopeBuilder {
  _$AgentWakeInputEvent? _$v;

  AgentWakeInputPayloadBuilder? _payload;
  AgentWakeInputPayloadBuilder get payload =>
      _$this._payload ??= AgentWakeInputPayloadBuilder();
  set payload(covariant AgentWakeInputPayloadBuilder? payload) =>
      _$this._payload = payload;

  AgentWakeInputEventTypeEnum? _type;
  AgentWakeInputEventTypeEnum? get type => _$this._type;
  set type(covariant AgentWakeInputEventTypeEnum? type) => _$this._type = type;

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

  AgentWakeInputEventBuilder() {
    AgentWakeInputEvent._defaults(this);
  }

  AgentWakeInputEventBuilder get _$this {
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
  void replace(covariant AgentWakeInputEvent other) {
    _$v = other as _$AgentWakeInputEvent;
  }

  @override
  void update(void Function(AgentWakeInputEventBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AgentWakeInputEvent build() => _build();

  _$AgentWakeInputEvent _build() {
    _$AgentWakeInputEvent _$result;
    try {
      _$result =
          _$v ??
          _$AgentWakeInputEvent._(
            payload: payload.build(),
            type: BuiltValueNullFieldError.checkNotNull(
              type,
              r'AgentWakeInputEvent',
              'type',
            ),
            id: BuiltValueNullFieldError.checkNotNull(
              id,
              r'AgentWakeInputEvent',
              'id',
            ),
            seq: seq,
            schemaVersion: BuiltValueNullFieldError.checkNotNull(
              schemaVersion,
              r'AgentWakeInputEvent',
              'schemaVersion',
            ),
            occurredAt: BuiltValueNullFieldError.checkNotNull(
              occurredAt,
              r'AgentWakeInputEvent',
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
          r'AgentWakeInputEvent',
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
