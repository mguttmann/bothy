// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'interaction_created_event.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const InteractionCreatedEventAudienceEnum
_$interactionCreatedEventAudienceEnum_self =
    const InteractionCreatedEventAudienceEnum._('self');
const InteractionCreatedEventAudienceEnum
_$interactionCreatedEventAudienceEnum_unknownDefaultOpenApi =
    const InteractionCreatedEventAudienceEnum._('unknownDefaultOpenApi');

InteractionCreatedEventAudienceEnum
_$interactionCreatedEventAudienceEnumValueOf(String name) {
  switch (name) {
    case 'self':
      return _$interactionCreatedEventAudienceEnum_self;
    case 'unknownDefaultOpenApi':
      return _$interactionCreatedEventAudienceEnum_unknownDefaultOpenApi;
    default:
      return _$interactionCreatedEventAudienceEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<InteractionCreatedEventAudienceEnum>
_$interactionCreatedEventAudienceEnumValues =
    BuiltSet<InteractionCreatedEventAudienceEnum>(
      const <InteractionCreatedEventAudienceEnum>[
        _$interactionCreatedEventAudienceEnum_self,
        _$interactionCreatedEventAudienceEnum_unknownDefaultOpenApi,
      ],
    );

const InteractionCreatedEventTypeEnum
_$interactionCreatedEventTypeEnum_interactionPeriodCreated =
    const InteractionCreatedEventTypeEnum._('interactionPeriodCreated');
const InteractionCreatedEventTypeEnum
_$interactionCreatedEventTypeEnum_unknownDefaultOpenApi =
    const InteractionCreatedEventTypeEnum._('unknownDefaultOpenApi');

InteractionCreatedEventTypeEnum _$interactionCreatedEventTypeEnumValueOf(
  String name,
) {
  switch (name) {
    case 'interactionPeriodCreated':
      return _$interactionCreatedEventTypeEnum_interactionPeriodCreated;
    case 'unknownDefaultOpenApi':
      return _$interactionCreatedEventTypeEnum_unknownDefaultOpenApi;
    default:
      return _$interactionCreatedEventTypeEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<InteractionCreatedEventTypeEnum>
_$interactionCreatedEventTypeEnumValues =
    BuiltSet<InteractionCreatedEventTypeEnum>(
      const <InteractionCreatedEventTypeEnum>[
        _$interactionCreatedEventTypeEnum_interactionPeriodCreated,
        _$interactionCreatedEventTypeEnum_unknownDefaultOpenApi,
      ],
    );

Serializer<InteractionCreatedEventAudienceEnum>
_$interactionCreatedEventAudienceEnumSerializer =
    _$InteractionCreatedEventAudienceEnumSerializer();
Serializer<InteractionCreatedEventTypeEnum>
_$interactionCreatedEventTypeEnumSerializer =
    _$InteractionCreatedEventTypeEnumSerializer();

class _$InteractionCreatedEventAudienceEnumSerializer
    implements PrimitiveSerializer<InteractionCreatedEventAudienceEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'self': 'self',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'self': 'self',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[
    InteractionCreatedEventAudienceEnum,
  ];
  @override
  final String wireName = 'InteractionCreatedEventAudienceEnum';

  @override
  Object serialize(
    Serializers serializers,
    InteractionCreatedEventAudienceEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  InteractionCreatedEventAudienceEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => InteractionCreatedEventAudienceEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$InteractionCreatedEventTypeEnumSerializer
    implements PrimitiveSerializer<InteractionCreatedEventTypeEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'interactionPeriodCreated': 'interaction.created',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'interaction.created': 'interactionPeriodCreated',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[InteractionCreatedEventTypeEnum];
  @override
  final String wireName = 'InteractionCreatedEventTypeEnum';

  @override
  Object serialize(
    Serializers serializers,
    InteractionCreatedEventTypeEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  InteractionCreatedEventTypeEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => InteractionCreatedEventTypeEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$InteractionCreatedEvent extends InteractionCreatedEvent {
  @override
  final InteractionCreatedPayload payload;
  @override
  final InteractionCreatedEventTypeEnum type;
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

  factory _$InteractionCreatedEvent([
    void Function(InteractionCreatedEventBuilder)? updates,
  ]) => (InteractionCreatedEventBuilder()..update(updates))._build();

  _$InteractionCreatedEvent._({
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
  InteractionCreatedEvent rebuild(
    void Function(InteractionCreatedEventBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  InteractionCreatedEventBuilder toBuilder() =>
      InteractionCreatedEventBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is InteractionCreatedEvent &&
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
    return (newBuiltValueToStringHelper(r'InteractionCreatedEvent')
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

class InteractionCreatedEventBuilder
    implements
        Builder<InteractionCreatedEvent, InteractionCreatedEventBuilder>,
        EventEnvelopeBuilder {
  _$InteractionCreatedEvent? _$v;

  InteractionCreatedPayloadBuilder? _payload;
  InteractionCreatedPayloadBuilder get payload =>
      _$this._payload ??= InteractionCreatedPayloadBuilder();
  set payload(covariant InteractionCreatedPayloadBuilder? payload) =>
      _$this._payload = payload;

  InteractionCreatedEventTypeEnum? _type;
  InteractionCreatedEventTypeEnum? get type => _$this._type;
  set type(covariant InteractionCreatedEventTypeEnum? type) =>
      _$this._type = type;

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

  InteractionCreatedEventBuilder() {
    InteractionCreatedEvent._defaults(this);
  }

  InteractionCreatedEventBuilder get _$this {
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
  void replace(covariant InteractionCreatedEvent other) {
    _$v = other as _$InteractionCreatedEvent;
  }

  @override
  void update(void Function(InteractionCreatedEventBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  InteractionCreatedEvent build() => _build();

  _$InteractionCreatedEvent _build() {
    _$InteractionCreatedEvent _$result;
    try {
      _$result =
          _$v ??
          _$InteractionCreatedEvent._(
            payload: payload.build(),
            type: BuiltValueNullFieldError.checkNotNull(
              type,
              r'InteractionCreatedEvent',
              'type',
            ),
            id: BuiltValueNullFieldError.checkNotNull(
              id,
              r'InteractionCreatedEvent',
              'id',
            ),
            seq: seq,
            schemaVersion: BuiltValueNullFieldError.checkNotNull(
              schemaVersion,
              r'InteractionCreatedEvent',
              'schemaVersion',
            ),
            occurredAt: BuiltValueNullFieldError.checkNotNull(
              occurredAt,
              r'InteractionCreatedEvent',
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
          r'InteractionCreatedEvent',
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
