// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'agent_interview_turn.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const AgentInterviewTurnRoleEnum _$agentInterviewTurnRoleEnum_builder =
    const AgentInterviewTurnRoleEnum._('builder');
const AgentInterviewTurnRoleEnum _$agentInterviewTurnRoleEnum_owner =
    const AgentInterviewTurnRoleEnum._('owner');
const AgentInterviewTurnRoleEnum
_$agentInterviewTurnRoleEnum_unknownDefaultOpenApi =
    const AgentInterviewTurnRoleEnum._('unknownDefaultOpenApi');

AgentInterviewTurnRoleEnum _$agentInterviewTurnRoleEnumValueOf(String name) {
  switch (name) {
    case 'builder':
      return _$agentInterviewTurnRoleEnum_builder;
    case 'owner':
      return _$agentInterviewTurnRoleEnum_owner;
    case 'unknownDefaultOpenApi':
      return _$agentInterviewTurnRoleEnum_unknownDefaultOpenApi;
    default:
      return _$agentInterviewTurnRoleEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<AgentInterviewTurnRoleEnum> _$agentInterviewTurnRoleEnumValues =
    BuiltSet<AgentInterviewTurnRoleEnum>(const <AgentInterviewTurnRoleEnum>[
      _$agentInterviewTurnRoleEnum_builder,
      _$agentInterviewTurnRoleEnum_owner,
      _$agentInterviewTurnRoleEnum_unknownDefaultOpenApi,
    ]);

Serializer<AgentInterviewTurnRoleEnum> _$agentInterviewTurnRoleEnumSerializer =
    _$AgentInterviewTurnRoleEnumSerializer();

class _$AgentInterviewTurnRoleEnumSerializer
    implements PrimitiveSerializer<AgentInterviewTurnRoleEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'builder': 'builder',
    'owner': 'owner',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'builder': 'builder',
    'owner': 'owner',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[AgentInterviewTurnRoleEnum];
  @override
  final String wireName = 'AgentInterviewTurnRoleEnum';

  @override
  Object serialize(
    Serializers serializers,
    AgentInterviewTurnRoleEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  AgentInterviewTurnRoleEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => AgentInterviewTurnRoleEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$AgentInterviewTurn extends AgentInterviewTurn {
  @override
  final AgentInterviewTurnRoleEnum role;
  @override
  final String text;

  factory _$AgentInterviewTurn([
    void Function(AgentInterviewTurnBuilder)? updates,
  ]) => (AgentInterviewTurnBuilder()..update(updates))._build();

  _$AgentInterviewTurn._({required this.role, required this.text}) : super._();
  @override
  AgentInterviewTurn rebuild(
    void Function(AgentInterviewTurnBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  AgentInterviewTurnBuilder toBuilder() =>
      AgentInterviewTurnBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AgentInterviewTurn &&
        role == other.role &&
        text == other.text;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, role.hashCode);
    _$hash = $jc(_$hash, text.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AgentInterviewTurn')
          ..add('role', role)
          ..add('text', text))
        .toString();
  }
}

class AgentInterviewTurnBuilder
    implements Builder<AgentInterviewTurn, AgentInterviewTurnBuilder> {
  _$AgentInterviewTurn? _$v;

  AgentInterviewTurnRoleEnum? _role;
  AgentInterviewTurnRoleEnum? get role => _$this._role;
  set role(AgentInterviewTurnRoleEnum? role) => _$this._role = role;

  String? _text;
  String? get text => _$this._text;
  set text(String? text) => _$this._text = text;

  AgentInterviewTurnBuilder() {
    AgentInterviewTurn._defaults(this);
  }

  AgentInterviewTurnBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _role = $v.role;
      _text = $v.text;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AgentInterviewTurn other) {
    _$v = other as _$AgentInterviewTurn;
  }

  @override
  void update(void Function(AgentInterviewTurnBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AgentInterviewTurn build() => _build();

  _$AgentInterviewTurn _build() {
    final _$result =
        _$v ??
        _$AgentInterviewTurn._(
          role: BuiltValueNullFieldError.checkNotNull(
            role,
            r'AgentInterviewTurn',
            'role',
          ),
          text: BuiltValueNullFieldError.checkNotNull(
            text,
            r'AgentInterviewTurn',
            'text',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
