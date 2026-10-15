// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'interaction.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const InteractionStateEnum _$interactionStateEnum_received =
    const InteractionStateEnum._('received');
const InteractionStateEnum _$interactionStateEnum_dispatched =
    const InteractionStateEnum._('dispatched');
const InteractionStateEnum _$interactionStateEnum_acknowledged =
    const InteractionStateEnum._('acknowledged');
const InteractionStateEnum _$interactionStateEnum_failed =
    const InteractionStateEnum._('failed');
const InteractionStateEnum _$interactionStateEnum_rejected =
    const InteractionStateEnum._('rejected');
const InteractionStateEnum _$interactionStateEnum_unknownDefaultOpenApi =
    const InteractionStateEnum._('unknownDefaultOpenApi');

InteractionStateEnum _$interactionStateEnumValueOf(String name) {
  switch (name) {
    case 'received':
      return _$interactionStateEnum_received;
    case 'dispatched':
      return _$interactionStateEnum_dispatched;
    case 'acknowledged':
      return _$interactionStateEnum_acknowledged;
    case 'failed':
      return _$interactionStateEnum_failed;
    case 'rejected':
      return _$interactionStateEnum_rejected;
    case 'unknownDefaultOpenApi':
      return _$interactionStateEnum_unknownDefaultOpenApi;
    default:
      return _$interactionStateEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<InteractionStateEnum> _$interactionStateEnumValues =
    BuiltSet<InteractionStateEnum>(const <InteractionStateEnum>[
      _$interactionStateEnum_received,
      _$interactionStateEnum_dispatched,
      _$interactionStateEnum_acknowledged,
      _$interactionStateEnum_failed,
      _$interactionStateEnum_rejected,
      _$interactionStateEnum_unknownDefaultOpenApi,
    ]);

Serializer<InteractionStateEnum> _$interactionStateEnumSerializer =
    _$InteractionStateEnumSerializer();

class _$InteractionStateEnumSerializer
    implements PrimitiveSerializer<InteractionStateEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'received': 'received',
    'dispatched': 'dispatched',
    'acknowledged': 'acknowledged',
    'failed': 'failed',
    'rejected': 'rejected',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'received': 'received',
    'dispatched': 'dispatched',
    'acknowledged': 'acknowledged',
    'failed': 'failed',
    'rejected': 'rejected',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[InteractionStateEnum];
  @override
  final String wireName = 'InteractionStateEnum';

  @override
  Object serialize(
    Serializers serializers,
    InteractionStateEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  InteractionStateEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => InteractionStateEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$Interaction extends Interaction {
  @override
  final String id;
  @override
  final String componentId;
  @override
  final String? customId;
  @override
  final String messageId;
  @override
  final String? threadId;
  @override
  final String? runId;
  @override
  final String principalId;
  @override
  final String clientInteractionId;
  @override
  final BuiltMap<String, JsonObject?> values;
  @override
  final InteractionStateEnum state;
  @override
  final InteractionResponse? response;
  @override
  final DateTime createdAt;

  factory _$Interaction([void Function(InteractionBuilder)? updates]) =>
      (InteractionBuilder()..update(updates))._build();

  _$Interaction._({
    required this.id,
    required this.componentId,
    this.customId,
    required this.messageId,
    this.threadId,
    this.runId,
    required this.principalId,
    required this.clientInteractionId,
    required this.values,
    required this.state,
    this.response,
    required this.createdAt,
  }) : super._();
  @override
  Interaction rebuild(void Function(InteractionBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  InteractionBuilder toBuilder() => InteractionBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is Interaction &&
        id == other.id &&
        componentId == other.componentId &&
        customId == other.customId &&
        messageId == other.messageId &&
        threadId == other.threadId &&
        runId == other.runId &&
        principalId == other.principalId &&
        clientInteractionId == other.clientInteractionId &&
        values == other.values &&
        state == other.state &&
        response == other.response &&
        createdAt == other.createdAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, id.hashCode);
    _$hash = $jc(_$hash, componentId.hashCode);
    _$hash = $jc(_$hash, customId.hashCode);
    _$hash = $jc(_$hash, messageId.hashCode);
    _$hash = $jc(_$hash, threadId.hashCode);
    _$hash = $jc(_$hash, runId.hashCode);
    _$hash = $jc(_$hash, principalId.hashCode);
    _$hash = $jc(_$hash, clientInteractionId.hashCode);
    _$hash = $jc(_$hash, values.hashCode);
    _$hash = $jc(_$hash, state.hashCode);
    _$hash = $jc(_$hash, response.hashCode);
    _$hash = $jc(_$hash, createdAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'Interaction')
          ..add('id', id)
          ..add('componentId', componentId)
          ..add('customId', customId)
          ..add('messageId', messageId)
          ..add('threadId', threadId)
          ..add('runId', runId)
          ..add('principalId', principalId)
          ..add('clientInteractionId', clientInteractionId)
          ..add('values', values)
          ..add('state', state)
          ..add('response', response)
          ..add('createdAt', createdAt))
        .toString();
  }
}

class InteractionBuilder implements Builder<Interaction, InteractionBuilder> {
  _$Interaction? _$v;

  String? _id;
  String? get id => _$this._id;
  set id(String? id) => _$this._id = id;

  String? _componentId;
  String? get componentId => _$this._componentId;
  set componentId(String? componentId) => _$this._componentId = componentId;

  String? _customId;
  String? get customId => _$this._customId;
  set customId(String? customId) => _$this._customId = customId;

  String? _messageId;
  String? get messageId => _$this._messageId;
  set messageId(String? messageId) => _$this._messageId = messageId;

  String? _threadId;
  String? get threadId => _$this._threadId;
  set threadId(String? threadId) => _$this._threadId = threadId;

  String? _runId;
  String? get runId => _$this._runId;
  set runId(String? runId) => _$this._runId = runId;

  String? _principalId;
  String? get principalId => _$this._principalId;
  set principalId(String? principalId) => _$this._principalId = principalId;

  String? _clientInteractionId;
  String? get clientInteractionId => _$this._clientInteractionId;
  set clientInteractionId(String? clientInteractionId) =>
      _$this._clientInteractionId = clientInteractionId;

  MapBuilder<String, JsonObject?>? _values;
  MapBuilder<String, JsonObject?> get values =>
      _$this._values ??= MapBuilder<String, JsonObject?>();
  set values(MapBuilder<String, JsonObject?>? values) =>
      _$this._values = values;

  InteractionStateEnum? _state;
  InteractionStateEnum? get state => _$this._state;
  set state(InteractionStateEnum? state) => _$this._state = state;

  InteractionResponseBuilder? _response;
  InteractionResponseBuilder get response =>
      _$this._response ??= InteractionResponseBuilder();
  set response(InteractionResponseBuilder? response) =>
      _$this._response = response;

  DateTime? _createdAt;
  DateTime? get createdAt => _$this._createdAt;
  set createdAt(DateTime? createdAt) => _$this._createdAt = createdAt;

  InteractionBuilder() {
    Interaction._defaults(this);
  }

  InteractionBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _id = $v.id;
      _componentId = $v.componentId;
      _customId = $v.customId;
      _messageId = $v.messageId;
      _threadId = $v.threadId;
      _runId = $v.runId;
      _principalId = $v.principalId;
      _clientInteractionId = $v.clientInteractionId;
      _values = $v.values.toBuilder();
      _state = $v.state;
      _response = $v.response?.toBuilder();
      _createdAt = $v.createdAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(Interaction other) {
    _$v = other as _$Interaction;
  }

  @override
  void update(void Function(InteractionBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  Interaction build() => _build();

  _$Interaction _build() {
    _$Interaction _$result;
    try {
      _$result =
          _$v ??
          _$Interaction._(
            id: BuiltValueNullFieldError.checkNotNull(id, r'Interaction', 'id'),
            componentId: BuiltValueNullFieldError.checkNotNull(
              componentId,
              r'Interaction',
              'componentId',
            ),
            customId: customId,
            messageId: BuiltValueNullFieldError.checkNotNull(
              messageId,
              r'Interaction',
              'messageId',
            ),
            threadId: threadId,
            runId: runId,
            principalId: BuiltValueNullFieldError.checkNotNull(
              principalId,
              r'Interaction',
              'principalId',
            ),
            clientInteractionId: BuiltValueNullFieldError.checkNotNull(
              clientInteractionId,
              r'Interaction',
              'clientInteractionId',
            ),
            values: values.build(),
            state: BuiltValueNullFieldError.checkNotNull(
              state,
              r'Interaction',
              'state',
            ),
            response: _response?.build(),
            createdAt: BuiltValueNullFieldError.checkNotNull(
              createdAt,
              r'Interaction',
              'createdAt',
            ),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'values';
        values.build();

        _$failedField = 'response';
        _response?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'Interaction',
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
