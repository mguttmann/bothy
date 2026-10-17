// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'agent_session.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const AgentSessionEndReasonEnum _$agentSessionEndReasonEnum_reset =
    const AgentSessionEndReasonEnum._('reset');
const AgentSessionEndReasonEnum _$agentSessionEndReasonEnum_rebind =
    const AgentSessionEndReasonEnum._('rebind');
const AgentSessionEndReasonEnum _$agentSessionEndReasonEnum_import_ =
    const AgentSessionEndReasonEnum._('import_');
const AgentSessionEndReasonEnum _$agentSessionEndReasonEnum_agentChanged =
    const AgentSessionEndReasonEnum._('agentChanged');
const AgentSessionEndReasonEnum
_$agentSessionEndReasonEnum_unknownDefaultOpenApi =
    const AgentSessionEndReasonEnum._('unknownDefaultOpenApi');

AgentSessionEndReasonEnum _$agentSessionEndReasonEnumValueOf(String name) {
  switch (name) {
    case 'reset':
      return _$agentSessionEndReasonEnum_reset;
    case 'rebind':
      return _$agentSessionEndReasonEnum_rebind;
    case 'import_':
      return _$agentSessionEndReasonEnum_import_;
    case 'agentChanged':
      return _$agentSessionEndReasonEnum_agentChanged;
    case 'unknownDefaultOpenApi':
      return _$agentSessionEndReasonEnum_unknownDefaultOpenApi;
    default:
      return _$agentSessionEndReasonEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<AgentSessionEndReasonEnum> _$agentSessionEndReasonEnumValues =
    BuiltSet<AgentSessionEndReasonEnum>(const <AgentSessionEndReasonEnum>[
      _$agentSessionEndReasonEnum_reset,
      _$agentSessionEndReasonEnum_rebind,
      _$agentSessionEndReasonEnum_import_,
      _$agentSessionEndReasonEnum_agentChanged,
      _$agentSessionEndReasonEnum_unknownDefaultOpenApi,
    ]);

Serializer<AgentSessionEndReasonEnum> _$agentSessionEndReasonEnumSerializer =
    _$AgentSessionEndReasonEnumSerializer();

class _$AgentSessionEndReasonEnumSerializer
    implements PrimitiveSerializer<AgentSessionEndReasonEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'reset': 'reset',
    'rebind': 'rebind',
    'import_': 'import',
    'agentChanged': 'agent_changed',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'reset': 'reset',
    'rebind': 'rebind',
    'import': 'import_',
    'agent_changed': 'agentChanged',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[AgentSessionEndReasonEnum];
  @override
  final String wireName = 'AgentSessionEndReasonEnum';

  @override
  Object serialize(
    Serializers serializers,
    AgentSessionEndReasonEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  AgentSessionEndReasonEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => AgentSessionEndReasonEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$AgentSession extends AgentSession {
  @override
  final String id;
  @override
  final String threadId;
  @override
  final String agentId;
  @override
  final String? externalSessionRef;
  @override
  final bool isCurrent;
  @override
  final bool started;
  @override
  final String? model;
  @override
  final Money cost;
  @override
  final DateTime createdAt;
  @override
  final DateTime? endedAt;
  @override
  final AgentSessionEndReasonEnum? endReason;
  @override
  final int version;

  factory _$AgentSession([void Function(AgentSessionBuilder)? updates]) =>
      (AgentSessionBuilder()..update(updates))._build();

  _$AgentSession._({
    required this.id,
    required this.threadId,
    required this.agentId,
    this.externalSessionRef,
    required this.isCurrent,
    required this.started,
    this.model,
    required this.cost,
    required this.createdAt,
    this.endedAt,
    this.endReason,
    required this.version,
  }) : super._();
  @override
  AgentSession rebuild(void Function(AgentSessionBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  AgentSessionBuilder toBuilder() => AgentSessionBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AgentSession &&
        id == other.id &&
        threadId == other.threadId &&
        agentId == other.agentId &&
        externalSessionRef == other.externalSessionRef &&
        isCurrent == other.isCurrent &&
        started == other.started &&
        model == other.model &&
        cost == other.cost &&
        createdAt == other.createdAt &&
        endedAt == other.endedAt &&
        endReason == other.endReason &&
        version == other.version;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, id.hashCode);
    _$hash = $jc(_$hash, threadId.hashCode);
    _$hash = $jc(_$hash, agentId.hashCode);
    _$hash = $jc(_$hash, externalSessionRef.hashCode);
    _$hash = $jc(_$hash, isCurrent.hashCode);
    _$hash = $jc(_$hash, started.hashCode);
    _$hash = $jc(_$hash, model.hashCode);
    _$hash = $jc(_$hash, cost.hashCode);
    _$hash = $jc(_$hash, createdAt.hashCode);
    _$hash = $jc(_$hash, endedAt.hashCode);
    _$hash = $jc(_$hash, endReason.hashCode);
    _$hash = $jc(_$hash, version.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AgentSession')
          ..add('id', id)
          ..add('threadId', threadId)
          ..add('agentId', agentId)
          ..add('externalSessionRef', externalSessionRef)
          ..add('isCurrent', isCurrent)
          ..add('started', started)
          ..add('model', model)
          ..add('cost', cost)
          ..add('createdAt', createdAt)
          ..add('endedAt', endedAt)
          ..add('endReason', endReason)
          ..add('version', version))
        .toString();
  }
}

class AgentSessionBuilder
    implements Builder<AgentSession, AgentSessionBuilder> {
  _$AgentSession? _$v;

  String? _id;
  String? get id => _$this._id;
  set id(String? id) => _$this._id = id;

  String? _threadId;
  String? get threadId => _$this._threadId;
  set threadId(String? threadId) => _$this._threadId = threadId;

  String? _agentId;
  String? get agentId => _$this._agentId;
  set agentId(String? agentId) => _$this._agentId = agentId;

  String? _externalSessionRef;
  String? get externalSessionRef => _$this._externalSessionRef;
  set externalSessionRef(String? externalSessionRef) =>
      _$this._externalSessionRef = externalSessionRef;

  bool? _isCurrent;
  bool? get isCurrent => _$this._isCurrent;
  set isCurrent(bool? isCurrent) => _$this._isCurrent = isCurrent;

  bool? _started;
  bool? get started => _$this._started;
  set started(bool? started) => _$this._started = started;

  String? _model;
  String? get model => _$this._model;
  set model(String? model) => _$this._model = model;

  MoneyBuilder? _cost;
  MoneyBuilder get cost => _$this._cost ??= MoneyBuilder();
  set cost(MoneyBuilder? cost) => _$this._cost = cost;

  DateTime? _createdAt;
  DateTime? get createdAt => _$this._createdAt;
  set createdAt(DateTime? createdAt) => _$this._createdAt = createdAt;

  DateTime? _endedAt;
  DateTime? get endedAt => _$this._endedAt;
  set endedAt(DateTime? endedAt) => _$this._endedAt = endedAt;

  AgentSessionEndReasonEnum? _endReason;
  AgentSessionEndReasonEnum? get endReason => _$this._endReason;
  set endReason(AgentSessionEndReasonEnum? endReason) =>
      _$this._endReason = endReason;

  int? _version;
  int? get version => _$this._version;
  set version(int? version) => _$this._version = version;

  AgentSessionBuilder() {
    AgentSession._defaults(this);
  }

  AgentSessionBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _id = $v.id;
      _threadId = $v.threadId;
      _agentId = $v.agentId;
      _externalSessionRef = $v.externalSessionRef;
      _isCurrent = $v.isCurrent;
      _started = $v.started;
      _model = $v.model;
      _cost = $v.cost.toBuilder();
      _createdAt = $v.createdAt;
      _endedAt = $v.endedAt;
      _endReason = $v.endReason;
      _version = $v.version;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AgentSession other) {
    _$v = other as _$AgentSession;
  }

  @override
  void update(void Function(AgentSessionBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AgentSession build() => _build();

  _$AgentSession _build() {
    _$AgentSession _$result;
    try {
      _$result =
          _$v ??
          _$AgentSession._(
            id: BuiltValueNullFieldError.checkNotNull(
              id,
              r'AgentSession',
              'id',
            ),
            threadId: BuiltValueNullFieldError.checkNotNull(
              threadId,
              r'AgentSession',
              'threadId',
            ),
            agentId: BuiltValueNullFieldError.checkNotNull(
              agentId,
              r'AgentSession',
              'agentId',
            ),
            externalSessionRef: externalSessionRef,
            isCurrent: BuiltValueNullFieldError.checkNotNull(
              isCurrent,
              r'AgentSession',
              'isCurrent',
            ),
            started: BuiltValueNullFieldError.checkNotNull(
              started,
              r'AgentSession',
              'started',
            ),
            model: model,
            cost: cost.build(),
            createdAt: BuiltValueNullFieldError.checkNotNull(
              createdAt,
              r'AgentSession',
              'createdAt',
            ),
            endedAt: endedAt,
            endReason: endReason,
            version: BuiltValueNullFieldError.checkNotNull(
              version,
              r'AgentSession',
              'version',
            ),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'cost';
        cost.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'AgentSession',
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
