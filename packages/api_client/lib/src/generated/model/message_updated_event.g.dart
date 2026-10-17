// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'message_updated_event.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const MessageUpdatedEventAudienceEnum _$messageUpdatedEventAudienceEnum_self =
    const MessageUpdatedEventAudienceEnum._('self');
const MessageUpdatedEventAudienceEnum
_$messageUpdatedEventAudienceEnum_unknownDefaultOpenApi =
    const MessageUpdatedEventAudienceEnum._('unknownDefaultOpenApi');

MessageUpdatedEventAudienceEnum _$messageUpdatedEventAudienceEnumValueOf(
  String name,
) {
  switch (name) {
    case 'self':
      return _$messageUpdatedEventAudienceEnum_self;
    case 'unknownDefaultOpenApi':
      return _$messageUpdatedEventAudienceEnum_unknownDefaultOpenApi;
    default:
      return _$messageUpdatedEventAudienceEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<MessageUpdatedEventAudienceEnum>
_$messageUpdatedEventAudienceEnumValues =
    BuiltSet<MessageUpdatedEventAudienceEnum>(
      const <MessageUpdatedEventAudienceEnum>[
        _$messageUpdatedEventAudienceEnum_self,
        _$messageUpdatedEventAudienceEnum_unknownDefaultOpenApi,
      ],
    );

const MessageUpdatedEventTypeEnum
_$messageUpdatedEventTypeEnum_messagePeriodUpdated =
    const MessageUpdatedEventTypeEnum._('messagePeriodUpdated');
const MessageUpdatedEventTypeEnum
_$messageUpdatedEventTypeEnum_unknownDefaultOpenApi =
    const MessageUpdatedEventTypeEnum._('unknownDefaultOpenApi');

MessageUpdatedEventTypeEnum _$messageUpdatedEventTypeEnumValueOf(String name) {
  switch (name) {
    case 'messagePeriodUpdated':
      return _$messageUpdatedEventTypeEnum_messagePeriodUpdated;
    case 'unknownDefaultOpenApi':
      return _$messageUpdatedEventTypeEnum_unknownDefaultOpenApi;
    default:
      return _$messageUpdatedEventTypeEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<MessageUpdatedEventTypeEnum>
_$messageUpdatedEventTypeEnumValues = BuiltSet<MessageUpdatedEventTypeEnum>(
  const <MessageUpdatedEventTypeEnum>[
    _$messageUpdatedEventTypeEnum_messagePeriodUpdated,
    _$messageUpdatedEventTypeEnum_unknownDefaultOpenApi,
  ],
);

Serializer<MessageUpdatedEventAudienceEnum>
_$messageUpdatedEventAudienceEnumSerializer =
    _$MessageUpdatedEventAudienceEnumSerializer();
Serializer<MessageUpdatedEventTypeEnum>
_$messageUpdatedEventTypeEnumSerializer =
    _$MessageUpdatedEventTypeEnumSerializer();

class _$MessageUpdatedEventAudienceEnumSerializer
    implements PrimitiveSerializer<MessageUpdatedEventAudienceEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'self': 'self',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'self': 'self',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[MessageUpdatedEventAudienceEnum];
  @override
  final String wireName = 'MessageUpdatedEventAudienceEnum';

  @override
  Object serialize(
    Serializers serializers,
    MessageUpdatedEventAudienceEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  MessageUpdatedEventAudienceEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => MessageUpdatedEventAudienceEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$MessageUpdatedEventTypeEnumSerializer
    implements PrimitiveSerializer<MessageUpdatedEventTypeEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'messagePeriodUpdated': 'message.updated',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'message.updated': 'messagePeriodUpdated',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[MessageUpdatedEventTypeEnum];
  @override
  final String wireName = 'MessageUpdatedEventTypeEnum';

  @override
  Object serialize(
    Serializers serializers,
    MessageUpdatedEventTypeEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  MessageUpdatedEventTypeEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => MessageUpdatedEventTypeEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$MessageUpdatedEvent extends MessageUpdatedEvent {
  @override
  final MessagePayload payload;
  @override
  final MessageUpdatedEventTypeEnum type;
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

  factory _$MessageUpdatedEvent([
    void Function(MessageUpdatedEventBuilder)? updates,
  ]) => (MessageUpdatedEventBuilder()..update(updates))._build();

  _$MessageUpdatedEvent._({
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
  MessageUpdatedEvent rebuild(
    void Function(MessageUpdatedEventBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  MessageUpdatedEventBuilder toBuilder() =>
      MessageUpdatedEventBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is MessageUpdatedEvent &&
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
    return (newBuiltValueToStringHelper(r'MessageUpdatedEvent')
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

class MessageUpdatedEventBuilder
    implements
        Builder<MessageUpdatedEvent, MessageUpdatedEventBuilder>,
        EventEnvelopeBuilder {
  _$MessageUpdatedEvent? _$v;

  MessagePayloadBuilder? _payload;
  MessagePayloadBuilder get payload =>
      _$this._payload ??= MessagePayloadBuilder();
  set payload(covariant MessagePayloadBuilder? payload) =>
      _$this._payload = payload;

  MessageUpdatedEventTypeEnum? _type;
  MessageUpdatedEventTypeEnum? get type => _$this._type;
  set type(covariant MessageUpdatedEventTypeEnum? type) => _$this._type = type;

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

  MessageUpdatedEventBuilder() {
    MessageUpdatedEvent._defaults(this);
  }

  MessageUpdatedEventBuilder get _$this {
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
  void replace(covariant MessageUpdatedEvent other) {
    _$v = other as _$MessageUpdatedEvent;
  }

  @override
  void update(void Function(MessageUpdatedEventBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  MessageUpdatedEvent build() => _build();

  _$MessageUpdatedEvent _build() {
    _$MessageUpdatedEvent _$result;
    try {
      _$result =
          _$v ??
          _$MessageUpdatedEvent._(
            payload: payload.build(),
            type: BuiltValueNullFieldError.checkNotNull(
              type,
              r'MessageUpdatedEvent',
              'type',
            ),
            id: BuiltValueNullFieldError.checkNotNull(
              id,
              r'MessageUpdatedEvent',
              'id',
            ),
            seq: seq,
            schemaVersion: BuiltValueNullFieldError.checkNotNull(
              schemaVersion,
              r'MessageUpdatedEvent',
              'schemaVersion',
            ),
            occurredAt: BuiltValueNullFieldError.checkNotNull(
              occurredAt,
              r'MessageUpdatedEvent',
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
          r'MessageUpdatedEvent',
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
