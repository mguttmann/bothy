// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'reaction_removed_event.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const ReactionRemovedEventAudienceEnum _$reactionRemovedEventAudienceEnum_self =
    const ReactionRemovedEventAudienceEnum._('self');
const ReactionRemovedEventAudienceEnum
_$reactionRemovedEventAudienceEnum_unknownDefaultOpenApi =
    const ReactionRemovedEventAudienceEnum._('unknownDefaultOpenApi');

ReactionRemovedEventAudienceEnum _$reactionRemovedEventAudienceEnumValueOf(
  String name,
) {
  switch (name) {
    case 'self':
      return _$reactionRemovedEventAudienceEnum_self;
    case 'unknownDefaultOpenApi':
      return _$reactionRemovedEventAudienceEnum_unknownDefaultOpenApi;
    default:
      return _$reactionRemovedEventAudienceEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<ReactionRemovedEventAudienceEnum>
_$reactionRemovedEventAudienceEnumValues =
    BuiltSet<ReactionRemovedEventAudienceEnum>(
      const <ReactionRemovedEventAudienceEnum>[
        _$reactionRemovedEventAudienceEnum_self,
        _$reactionRemovedEventAudienceEnum_unknownDefaultOpenApi,
      ],
    );

const ReactionRemovedEventTypeEnum
_$reactionRemovedEventTypeEnum_reactionPeriodRemoved =
    const ReactionRemovedEventTypeEnum._('reactionPeriodRemoved');
const ReactionRemovedEventTypeEnum
_$reactionRemovedEventTypeEnum_unknownDefaultOpenApi =
    const ReactionRemovedEventTypeEnum._('unknownDefaultOpenApi');

ReactionRemovedEventTypeEnum _$reactionRemovedEventTypeEnumValueOf(
  String name,
) {
  switch (name) {
    case 'reactionPeriodRemoved':
      return _$reactionRemovedEventTypeEnum_reactionPeriodRemoved;
    case 'unknownDefaultOpenApi':
      return _$reactionRemovedEventTypeEnum_unknownDefaultOpenApi;
    default:
      return _$reactionRemovedEventTypeEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<ReactionRemovedEventTypeEnum>
_$reactionRemovedEventTypeEnumValues = BuiltSet<ReactionRemovedEventTypeEnum>(
  const <ReactionRemovedEventTypeEnum>[
    _$reactionRemovedEventTypeEnum_reactionPeriodRemoved,
    _$reactionRemovedEventTypeEnum_unknownDefaultOpenApi,
  ],
);

Serializer<ReactionRemovedEventAudienceEnum>
_$reactionRemovedEventAudienceEnumSerializer =
    _$ReactionRemovedEventAudienceEnumSerializer();
Serializer<ReactionRemovedEventTypeEnum>
_$reactionRemovedEventTypeEnumSerializer =
    _$ReactionRemovedEventTypeEnumSerializer();

class _$ReactionRemovedEventAudienceEnumSerializer
    implements PrimitiveSerializer<ReactionRemovedEventAudienceEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'self': 'self',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'self': 'self',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[ReactionRemovedEventAudienceEnum];
  @override
  final String wireName = 'ReactionRemovedEventAudienceEnum';

  @override
  Object serialize(
    Serializers serializers,
    ReactionRemovedEventAudienceEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  ReactionRemovedEventAudienceEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => ReactionRemovedEventAudienceEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$ReactionRemovedEventTypeEnumSerializer
    implements PrimitiveSerializer<ReactionRemovedEventTypeEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'reactionPeriodRemoved': 'reaction.removed',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'reaction.removed': 'reactionPeriodRemoved',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[ReactionRemovedEventTypeEnum];
  @override
  final String wireName = 'ReactionRemovedEventTypeEnum';

  @override
  Object serialize(
    Serializers serializers,
    ReactionRemovedEventTypeEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  ReactionRemovedEventTypeEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => ReactionRemovedEventTypeEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$ReactionRemovedEvent extends ReactionRemovedEvent {
  @override
  final ReactionPayload payload;
  @override
  final ReactionRemovedEventTypeEnum type;
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

  factory _$ReactionRemovedEvent([
    void Function(ReactionRemovedEventBuilder)? updates,
  ]) => (ReactionRemovedEventBuilder()..update(updates))._build();

  _$ReactionRemovedEvent._({
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
  ReactionRemovedEvent rebuild(
    void Function(ReactionRemovedEventBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  ReactionRemovedEventBuilder toBuilder() =>
      ReactionRemovedEventBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ReactionRemovedEvent &&
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
    return (newBuiltValueToStringHelper(r'ReactionRemovedEvent')
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

class ReactionRemovedEventBuilder
    implements
        Builder<ReactionRemovedEvent, ReactionRemovedEventBuilder>,
        EventEnvelopeBuilder {
  _$ReactionRemovedEvent? _$v;

  ReactionPayloadBuilder? _payload;
  ReactionPayloadBuilder get payload =>
      _$this._payload ??= ReactionPayloadBuilder();
  set payload(covariant ReactionPayloadBuilder? payload) =>
      _$this._payload = payload;

  ReactionRemovedEventTypeEnum? _type;
  ReactionRemovedEventTypeEnum? get type => _$this._type;
  set type(covariant ReactionRemovedEventTypeEnum? type) => _$this._type = type;

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

  ReactionRemovedEventBuilder() {
    ReactionRemovedEvent._defaults(this);
  }

  ReactionRemovedEventBuilder get _$this {
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
  void replace(covariant ReactionRemovedEvent other) {
    _$v = other as _$ReactionRemovedEvent;
  }

  @override
  void update(void Function(ReactionRemovedEventBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ReactionRemovedEvent build() => _build();

  _$ReactionRemovedEvent _build() {
    _$ReactionRemovedEvent _$result;
    try {
      _$result =
          _$v ??
          _$ReactionRemovedEvent._(
            payload: payload.build(),
            type: BuiltValueNullFieldError.checkNotNull(
              type,
              r'ReactionRemovedEvent',
              'type',
            ),
            id: BuiltValueNullFieldError.checkNotNull(
              id,
              r'ReactionRemovedEvent',
              'id',
            ),
            seq: seq,
            schemaVersion: BuiltValueNullFieldError.checkNotNull(
              schemaVersion,
              r'ReactionRemovedEvent',
              'schemaVersion',
            ),
            occurredAt: BuiltValueNullFieldError.checkNotNull(
              occurredAt,
              r'ReactionRemovedEvent',
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
          r'ReactionRemovedEvent',
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
