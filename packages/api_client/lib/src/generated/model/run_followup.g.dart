// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'run_followup.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const RunFollowupStateEnum _$runFollowupStateEnum_queued =
    const RunFollowupStateEnum._('queued');
const RunFollowupStateEnum _$runFollowupStateEnum_delivered =
    const RunFollowupStateEnum._('delivered');
const RunFollowupStateEnum _$runFollowupStateEnum_absorbed =
    const RunFollowupStateEnum._('absorbed');
const RunFollowupStateEnum _$runFollowupStateEnum_deferred_ =
    const RunFollowupStateEnum._('deferred_');
const RunFollowupStateEnum _$runFollowupStateEnum_discarded =
    const RunFollowupStateEnum._('discarded');
const RunFollowupStateEnum _$runFollowupStateEnum_unknownDefaultOpenApi =
    const RunFollowupStateEnum._('unknownDefaultOpenApi');

RunFollowupStateEnum _$runFollowupStateEnumValueOf(String name) {
  switch (name) {
    case 'queued':
      return _$runFollowupStateEnum_queued;
    case 'delivered':
      return _$runFollowupStateEnum_delivered;
    case 'absorbed':
      return _$runFollowupStateEnum_absorbed;
    case 'deferred_':
      return _$runFollowupStateEnum_deferred_;
    case 'discarded':
      return _$runFollowupStateEnum_discarded;
    case 'unknownDefaultOpenApi':
      return _$runFollowupStateEnum_unknownDefaultOpenApi;
    default:
      return _$runFollowupStateEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<RunFollowupStateEnum> _$runFollowupStateEnumValues =
    BuiltSet<RunFollowupStateEnum>(const <RunFollowupStateEnum>[
      _$runFollowupStateEnum_queued,
      _$runFollowupStateEnum_delivered,
      _$runFollowupStateEnum_absorbed,
      _$runFollowupStateEnum_deferred_,
      _$runFollowupStateEnum_discarded,
      _$runFollowupStateEnum_unknownDefaultOpenApi,
    ]);

Serializer<RunFollowupStateEnum> _$runFollowupStateEnumSerializer =
    _$RunFollowupStateEnumSerializer();

class _$RunFollowupStateEnumSerializer
    implements PrimitiveSerializer<RunFollowupStateEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'queued': 'queued',
    'delivered': 'delivered',
    'absorbed': 'absorbed',
    'deferred_': 'deferred',
    'discarded': 'discarded',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'queued': 'queued',
    'delivered': 'delivered',
    'absorbed': 'absorbed',
    'deferred': 'deferred_',
    'discarded': 'discarded',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[RunFollowupStateEnum];
  @override
  final String wireName = 'RunFollowupStateEnum';

  @override
  Object serialize(
    Serializers serializers,
    RunFollowupStateEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  RunFollowupStateEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => RunFollowupStateEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$RunFollowup extends RunFollowup {
  @override
  final String messageId;
  @override
  final RunFollowupStateEnum state;

  factory _$RunFollowup([void Function(RunFollowupBuilder)? updates]) =>
      (RunFollowupBuilder()..update(updates))._build();

  _$RunFollowup._({required this.messageId, required this.state}) : super._();
  @override
  RunFollowup rebuild(void Function(RunFollowupBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  RunFollowupBuilder toBuilder() => RunFollowupBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is RunFollowup &&
        messageId == other.messageId &&
        state == other.state;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, messageId.hashCode);
    _$hash = $jc(_$hash, state.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'RunFollowup')
          ..add('messageId', messageId)
          ..add('state', state))
        .toString();
  }
}

class RunFollowupBuilder implements Builder<RunFollowup, RunFollowupBuilder> {
  _$RunFollowup? _$v;

  String? _messageId;
  String? get messageId => _$this._messageId;
  set messageId(String? messageId) => _$this._messageId = messageId;

  RunFollowupStateEnum? _state;
  RunFollowupStateEnum? get state => _$this._state;
  set state(RunFollowupStateEnum? state) => _$this._state = state;

  RunFollowupBuilder() {
    RunFollowup._defaults(this);
  }

  RunFollowupBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _messageId = $v.messageId;
      _state = $v.state;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(RunFollowup other) {
    _$v = other as _$RunFollowup;
  }

  @override
  void update(void Function(RunFollowupBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  RunFollowup build() => _build();

  _$RunFollowup _build() {
    final _$result =
        _$v ??
        _$RunFollowup._(
          messageId: BuiltValueNullFieldError.checkNotNull(
            messageId,
            r'RunFollowup',
            'messageId',
          ),
          state: BuiltValueNullFieldError.checkNotNull(
            state,
            r'RunFollowup',
            'state',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
