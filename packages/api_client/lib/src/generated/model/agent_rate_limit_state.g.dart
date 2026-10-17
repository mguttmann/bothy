// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'agent_rate_limit_state.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const AgentRateLimitStateStateEnum _$agentRateLimitStateStateEnum_ok =
    const AgentRateLimitStateStateEnum._('ok');
const AgentRateLimitStateStateEnum _$agentRateLimitStateStateEnum_nearLimit =
    const AgentRateLimitStateStateEnum._('nearLimit');
const AgentRateLimitStateStateEnum _$agentRateLimitStateStateEnum_limited =
    const AgentRateLimitStateStateEnum._('limited');
const AgentRateLimitStateStateEnum _$agentRateLimitStateStateEnum_unknown =
    const AgentRateLimitStateStateEnum._('unknown');
const AgentRateLimitStateStateEnum
_$agentRateLimitStateStateEnum_unknownDefaultOpenApi =
    const AgentRateLimitStateStateEnum._('unknownDefaultOpenApi');

AgentRateLimitStateStateEnum _$agentRateLimitStateStateEnumValueOf(
  String name,
) {
  switch (name) {
    case 'ok':
      return _$agentRateLimitStateStateEnum_ok;
    case 'nearLimit':
      return _$agentRateLimitStateStateEnum_nearLimit;
    case 'limited':
      return _$agentRateLimitStateStateEnum_limited;
    case 'unknown':
      return _$agentRateLimitStateStateEnum_unknown;
    case 'unknownDefaultOpenApi':
      return _$agentRateLimitStateStateEnum_unknownDefaultOpenApi;
    default:
      return _$agentRateLimitStateStateEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<AgentRateLimitStateStateEnum>
_$agentRateLimitStateStateEnumValues = BuiltSet<AgentRateLimitStateStateEnum>(
  const <AgentRateLimitStateStateEnum>[
    _$agentRateLimitStateStateEnum_ok,
    _$agentRateLimitStateStateEnum_nearLimit,
    _$agentRateLimitStateStateEnum_limited,
    _$agentRateLimitStateStateEnum_unknown,
    _$agentRateLimitStateStateEnum_unknownDefaultOpenApi,
  ],
);

Serializer<AgentRateLimitStateStateEnum>
_$agentRateLimitStateStateEnumSerializer =
    _$AgentRateLimitStateStateEnumSerializer();

class _$AgentRateLimitStateStateEnumSerializer
    implements PrimitiveSerializer<AgentRateLimitStateStateEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'ok': 'ok',
    'nearLimit': 'near_limit',
    'limited': 'limited',
    'unknown': 'unknown',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'ok': 'ok',
    'near_limit': 'nearLimit',
    'limited': 'limited',
    'unknown': 'unknown',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[AgentRateLimitStateStateEnum];
  @override
  final String wireName = 'AgentRateLimitStateStateEnum';

  @override
  Object serialize(
    Serializers serializers,
    AgentRateLimitStateStateEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  AgentRateLimitStateStateEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => AgentRateLimitStateStateEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$AgentRateLimitState extends AgentRateLimitState {
  @override
  final AgentRateLimitStateStateEnum state;
  @override
  final DateTime? resetsAt;

  factory _$AgentRateLimitState([
    void Function(AgentRateLimitStateBuilder)? updates,
  ]) => (AgentRateLimitStateBuilder()..update(updates))._build();

  _$AgentRateLimitState._({required this.state, this.resetsAt}) : super._();
  @override
  AgentRateLimitState rebuild(
    void Function(AgentRateLimitStateBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  AgentRateLimitStateBuilder toBuilder() =>
      AgentRateLimitStateBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AgentRateLimitState &&
        state == other.state &&
        resetsAt == other.resetsAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, state.hashCode);
    _$hash = $jc(_$hash, resetsAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AgentRateLimitState')
          ..add('state', state)
          ..add('resetsAt', resetsAt))
        .toString();
  }
}

class AgentRateLimitStateBuilder
    implements Builder<AgentRateLimitState, AgentRateLimitStateBuilder> {
  _$AgentRateLimitState? _$v;

  AgentRateLimitStateStateEnum? _state;
  AgentRateLimitStateStateEnum? get state => _$this._state;
  set state(AgentRateLimitStateStateEnum? state) => _$this._state = state;

  DateTime? _resetsAt;
  DateTime? get resetsAt => _$this._resetsAt;
  set resetsAt(DateTime? resetsAt) => _$this._resetsAt = resetsAt;

  AgentRateLimitStateBuilder() {
    AgentRateLimitState._defaults(this);
  }

  AgentRateLimitStateBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _state = $v.state;
      _resetsAt = $v.resetsAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AgentRateLimitState other) {
    _$v = other as _$AgentRateLimitState;
  }

  @override
  void update(void Function(AgentRateLimitStateBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AgentRateLimitState build() => _build();

  _$AgentRateLimitState _build() {
    final _$result =
        _$v ??
        _$AgentRateLimitState._(
          state: BuiltValueNullFieldError.checkNotNull(
            state,
            r'AgentRateLimitState',
            'state',
          ),
          resetsAt: resetsAt,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
