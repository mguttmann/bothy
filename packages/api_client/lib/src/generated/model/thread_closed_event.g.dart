// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'thread_closed_event.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const ThreadClosedEventAudienceEnum _$threadClosedEventAudienceEnum_self =
    const ThreadClosedEventAudienceEnum._('self');
const ThreadClosedEventAudienceEnum
_$threadClosedEventAudienceEnum_unknownDefaultOpenApi =
    const ThreadClosedEventAudienceEnum._('unknownDefaultOpenApi');

ThreadClosedEventAudienceEnum _$threadClosedEventAudienceEnumValueOf(
  String name,
) {
  switch (name) {
    case 'self':
      return _$threadClosedEventAudienceEnum_self;
    case 'unknownDefaultOpenApi':
      return _$threadClosedEventAudienceEnum_unknownDefaultOpenApi;
    default:
      return _$threadClosedEventAudienceEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<ThreadClosedEventAudienceEnum>
_$threadClosedEventAudienceEnumValues = BuiltSet<ThreadClosedEventAudienceEnum>(
  const <ThreadClosedEventAudienceEnum>[
    _$threadClosedEventAudienceEnum_self,
    _$threadClosedEventAudienceEnum_unknownDefaultOpenApi,
  ],
);

const ThreadClosedEventTypeEnum _$threadClosedEventTypeEnum_threadPeriodClosed =
    const ThreadClosedEventTypeEnum._('threadPeriodClosed');
const ThreadClosedEventTypeEnum
_$threadClosedEventTypeEnum_unknownDefaultOpenApi =
    const ThreadClosedEventTypeEnum._('unknownDefaultOpenApi');

ThreadClosedEventTypeEnum _$threadClosedEventTypeEnumValueOf(String name) {
  switch (name) {
    case 'threadPeriodClosed':
      return _$threadClosedEventTypeEnum_threadPeriodClosed;
    case 'unknownDefaultOpenApi':
      return _$threadClosedEventTypeEnum_unknownDefaultOpenApi;
    default:
      return _$threadClosedEventTypeEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<ThreadClosedEventTypeEnum> _$threadClosedEventTypeEnumValues =
    BuiltSet<ThreadClosedEventTypeEnum>(const <ThreadClosedEventTypeEnum>[
      _$threadClosedEventTypeEnum_threadPeriodClosed,
      _$threadClosedEventTypeEnum_unknownDefaultOpenApi,
    ]);

Serializer<ThreadClosedEventAudienceEnum>
_$threadClosedEventAudienceEnumSerializer =
    _$ThreadClosedEventAudienceEnumSerializer();
Serializer<ThreadClosedEventTypeEnum> _$threadClosedEventTypeEnumSerializer =
    _$ThreadClosedEventTypeEnumSerializer();

class _$ThreadClosedEventAudienceEnumSerializer
    implements PrimitiveSerializer<ThreadClosedEventAudienceEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'self': 'self',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'self': 'self',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[ThreadClosedEventAudienceEnum];
  @override
  final String wireName = 'ThreadClosedEventAudienceEnum';

  @override
  Object serialize(
    Serializers serializers,
    ThreadClosedEventAudienceEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  ThreadClosedEventAudienceEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => ThreadClosedEventAudienceEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$ThreadClosedEventTypeEnumSerializer
    implements PrimitiveSerializer<ThreadClosedEventTypeEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'threadPeriodClosed': 'thread.closed',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'thread.closed': 'threadPeriodClosed',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[ThreadClosedEventTypeEnum];
  @override
  final String wireName = 'ThreadClosedEventTypeEnum';

  @override
  Object serialize(
    Serializers serializers,
    ThreadClosedEventTypeEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  ThreadClosedEventTypeEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => ThreadClosedEventTypeEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$ThreadClosedEvent extends ThreadClosedEvent {
  @override
  final ThreadPayload payload;
  @override
  final ThreadClosedEventTypeEnum type;
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

  factory _$ThreadClosedEvent([
    void Function(ThreadClosedEventBuilder)? updates,
  ]) => (ThreadClosedEventBuilder()..update(updates))._build();

  _$ThreadClosedEvent._({
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
  ThreadClosedEvent rebuild(void Function(ThreadClosedEventBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ThreadClosedEventBuilder toBuilder() =>
      ThreadClosedEventBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ThreadClosedEvent &&
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
    return (newBuiltValueToStringHelper(r'ThreadClosedEvent')
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

class ThreadClosedEventBuilder
    implements
        Builder<ThreadClosedEvent, ThreadClosedEventBuilder>,
        EventEnvelopeBuilder {
  _$ThreadClosedEvent? _$v;

  ThreadPayloadBuilder? _payload;
  ThreadPayloadBuilder get payload =>
      _$this._payload ??= ThreadPayloadBuilder();
  set payload(covariant ThreadPayloadBuilder? payload) =>
      _$this._payload = payload;

  ThreadClosedEventTypeEnum? _type;
  ThreadClosedEventTypeEnum? get type => _$this._type;
  set type(covariant ThreadClosedEventTypeEnum? type) => _$this._type = type;

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

  ThreadClosedEventBuilder() {
    ThreadClosedEvent._defaults(this);
  }

  ThreadClosedEventBuilder get _$this {
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
  void replace(covariant ThreadClosedEvent other) {
    _$v = other as _$ThreadClosedEvent;
  }

  @override
  void update(void Function(ThreadClosedEventBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ThreadClosedEvent build() => _build();

  _$ThreadClosedEvent _build() {
    _$ThreadClosedEvent _$result;
    try {
      _$result =
          _$v ??
          _$ThreadClosedEvent._(
            payload: payload.build(),
            type: BuiltValueNullFieldError.checkNotNull(
              type,
              r'ThreadClosedEvent',
              'type',
            ),
            id: BuiltValueNullFieldError.checkNotNull(
              id,
              r'ThreadClosedEvent',
              'id',
            ),
            seq: seq,
            schemaVersion: BuiltValueNullFieldError.checkNotNull(
              schemaVersion,
              r'ThreadClosedEvent',
              'schemaVersion',
            ),
            occurredAt: BuiltValueNullFieldError.checkNotNull(
              occurredAt,
              r'ThreadClosedEvent',
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
          r'ThreadClosedEvent',
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
