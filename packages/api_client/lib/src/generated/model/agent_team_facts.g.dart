// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'agent_team_facts.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const AgentTeamFactsSource_Enum _$agentTeamFactsSourceEnum_owner =
    const AgentTeamFactsSource_Enum._('owner');
const AgentTeamFactsSource_Enum _$agentTeamFactsSourceEnum_member =
    const AgentTeamFactsSource_Enum._('member');
const AgentTeamFactsSource_Enum
_$agentTeamFactsSourceEnum_unknownDefaultOpenApi =
    const AgentTeamFactsSource_Enum._('unknownDefaultOpenApi');

AgentTeamFactsSource_Enum _$agentTeamFactsSourceEnumValueOf(String name) {
  switch (name) {
    case 'owner':
      return _$agentTeamFactsSourceEnum_owner;
    case 'member':
      return _$agentTeamFactsSourceEnum_member;
    case 'unknownDefaultOpenApi':
      return _$agentTeamFactsSourceEnum_unknownDefaultOpenApi;
    default:
      return _$agentTeamFactsSourceEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<AgentTeamFactsSource_Enum> _$agentTeamFactsSourceEnumValues =
    BuiltSet<AgentTeamFactsSource_Enum>(const <AgentTeamFactsSource_Enum>[
      _$agentTeamFactsSourceEnum_owner,
      _$agentTeamFactsSourceEnum_member,
      _$agentTeamFactsSourceEnum_unknownDefaultOpenApi,
    ]);

Serializer<AgentTeamFactsSource_Enum> _$agentTeamFactsSourceEnumSerializer =
    _$AgentTeamFactsSource_EnumSerializer();

class _$AgentTeamFactsSource_EnumSerializer
    implements PrimitiveSerializer<AgentTeamFactsSource_Enum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'owner': 'owner',
    'member': 'member',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'owner': 'owner',
    'member': 'member',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[AgentTeamFactsSource_Enum];
  @override
  final String wireName = 'AgentTeamFactsSource_Enum';

  @override
  Object serialize(
    Serializers serializers,
    AgentTeamFactsSource_Enum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  AgentTeamFactsSource_Enum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => AgentTeamFactsSource_Enum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$AgentTeamFacts extends AgentTeamFacts {
  @override
  final String channelId;
  @override
  final String memberId;
  @override
  final String handle;
  @override
  final AgentTeamFactsSource_Enum source_;
  @override
  final String? sourceHandle;
  @override
  final bool addressed;
  @override
  final BuiltList<AgentTeamMember> members;

  factory _$AgentTeamFacts([void Function(AgentTeamFactsBuilder)? updates]) =>
      (AgentTeamFactsBuilder()..update(updates))._build();

  _$AgentTeamFacts._({
    required this.channelId,
    required this.memberId,
    required this.handle,
    required this.source_,
    this.sourceHandle,
    required this.addressed,
    required this.members,
  }) : super._();
  @override
  AgentTeamFacts rebuild(void Function(AgentTeamFactsBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  AgentTeamFactsBuilder toBuilder() => AgentTeamFactsBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AgentTeamFacts &&
        channelId == other.channelId &&
        memberId == other.memberId &&
        handle == other.handle &&
        source_ == other.source_ &&
        sourceHandle == other.sourceHandle &&
        addressed == other.addressed &&
        members == other.members;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, channelId.hashCode);
    _$hash = $jc(_$hash, memberId.hashCode);
    _$hash = $jc(_$hash, handle.hashCode);
    _$hash = $jc(_$hash, source_.hashCode);
    _$hash = $jc(_$hash, sourceHandle.hashCode);
    _$hash = $jc(_$hash, addressed.hashCode);
    _$hash = $jc(_$hash, members.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AgentTeamFacts')
          ..add('channelId', channelId)
          ..add('memberId', memberId)
          ..add('handle', handle)
          ..add('source_', source_)
          ..add('sourceHandle', sourceHandle)
          ..add('addressed', addressed)
          ..add('members', members))
        .toString();
  }
}

class AgentTeamFactsBuilder
    implements Builder<AgentTeamFacts, AgentTeamFactsBuilder> {
  _$AgentTeamFacts? _$v;

  String? _channelId;
  String? get channelId => _$this._channelId;
  set channelId(String? channelId) => _$this._channelId = channelId;

  String? _memberId;
  String? get memberId => _$this._memberId;
  set memberId(String? memberId) => _$this._memberId = memberId;

  String? _handle;
  String? get handle => _$this._handle;
  set handle(String? handle) => _$this._handle = handle;

  AgentTeamFactsSource_Enum? _source_;
  AgentTeamFactsSource_Enum? get source_ => _$this._source_;
  set source_(AgentTeamFactsSource_Enum? source_) => _$this._source_ = source_;

  String? _sourceHandle;
  String? get sourceHandle => _$this._sourceHandle;
  set sourceHandle(String? sourceHandle) => _$this._sourceHandle = sourceHandle;

  bool? _addressed;
  bool? get addressed => _$this._addressed;
  set addressed(bool? addressed) => _$this._addressed = addressed;

  ListBuilder<AgentTeamMember>? _members;
  ListBuilder<AgentTeamMember> get members =>
      _$this._members ??= ListBuilder<AgentTeamMember>();
  set members(ListBuilder<AgentTeamMember>? members) =>
      _$this._members = members;

  AgentTeamFactsBuilder() {
    AgentTeamFacts._defaults(this);
  }

  AgentTeamFactsBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _channelId = $v.channelId;
      _memberId = $v.memberId;
      _handle = $v.handle;
      _source_ = $v.source_;
      _sourceHandle = $v.sourceHandle;
      _addressed = $v.addressed;
      _members = $v.members.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AgentTeamFacts other) {
    _$v = other as _$AgentTeamFacts;
  }

  @override
  void update(void Function(AgentTeamFactsBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AgentTeamFacts build() => _build();

  _$AgentTeamFacts _build() {
    _$AgentTeamFacts _$result;
    try {
      _$result =
          _$v ??
          _$AgentTeamFacts._(
            channelId: BuiltValueNullFieldError.checkNotNull(
              channelId,
              r'AgentTeamFacts',
              'channelId',
            ),
            memberId: BuiltValueNullFieldError.checkNotNull(
              memberId,
              r'AgentTeamFacts',
              'memberId',
            ),
            handle: BuiltValueNullFieldError.checkNotNull(
              handle,
              r'AgentTeamFacts',
              'handle',
            ),
            source_: BuiltValueNullFieldError.checkNotNull(
              source_,
              r'AgentTeamFacts',
              'source_',
            ),
            sourceHandle: sourceHandle,
            addressed: BuiltValueNullFieldError.checkNotNull(
              addressed,
              r'AgentTeamFacts',
              'addressed',
            ),
            members: members.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'members';
        members.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'AgentTeamFacts',
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
