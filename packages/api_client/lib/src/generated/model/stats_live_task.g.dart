// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'stats_live_task.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const StatsLiveTaskKindEnum _$statsLiveTaskKindEnum_subagent =
    const StatsLiveTaskKindEnum._('subagent');
const StatsLiveTaskKindEnum _$statsLiveTaskKindEnum_workflow =
    const StatsLiveTaskKindEnum._('workflow');
const StatsLiveTaskKindEnum _$statsLiveTaskKindEnum_backgroundJob =
    const StatsLiveTaskKindEnum._('backgroundJob');
const StatsLiveTaskKindEnum _$statsLiveTaskKindEnum_unknownDefaultOpenApi =
    const StatsLiveTaskKindEnum._('unknownDefaultOpenApi');

StatsLiveTaskKindEnum _$statsLiveTaskKindEnumValueOf(String name) {
  switch (name) {
    case 'subagent':
      return _$statsLiveTaskKindEnum_subagent;
    case 'workflow':
      return _$statsLiveTaskKindEnum_workflow;
    case 'backgroundJob':
      return _$statsLiveTaskKindEnum_backgroundJob;
    case 'unknownDefaultOpenApi':
      return _$statsLiveTaskKindEnum_unknownDefaultOpenApi;
    default:
      return _$statsLiveTaskKindEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<StatsLiveTaskKindEnum> _$statsLiveTaskKindEnumValues =
    BuiltSet<StatsLiveTaskKindEnum>(const <StatsLiveTaskKindEnum>[
      _$statsLiveTaskKindEnum_subagent,
      _$statsLiveTaskKindEnum_workflow,
      _$statsLiveTaskKindEnum_backgroundJob,
      _$statsLiveTaskKindEnum_unknownDefaultOpenApi,
    ]);

const StatsLiveTaskStateEnum _$statsLiveTaskStateEnum_running =
    const StatsLiveTaskStateEnum._('running');
const StatsLiveTaskStateEnum _$statsLiveTaskStateEnum_completed =
    const StatsLiveTaskStateEnum._('completed');
const StatsLiveTaskStateEnum _$statsLiveTaskStateEnum_failed =
    const StatsLiveTaskStateEnum._('failed');
const StatsLiveTaskStateEnum _$statsLiveTaskStateEnum_cancelled =
    const StatsLiveTaskStateEnum._('cancelled');
const StatsLiveTaskStateEnum _$statsLiveTaskStateEnum_unknownDefaultOpenApi =
    const StatsLiveTaskStateEnum._('unknownDefaultOpenApi');

StatsLiveTaskStateEnum _$statsLiveTaskStateEnumValueOf(String name) {
  switch (name) {
    case 'running':
      return _$statsLiveTaskStateEnum_running;
    case 'completed':
      return _$statsLiveTaskStateEnum_completed;
    case 'failed':
      return _$statsLiveTaskStateEnum_failed;
    case 'cancelled':
      return _$statsLiveTaskStateEnum_cancelled;
    case 'unknownDefaultOpenApi':
      return _$statsLiveTaskStateEnum_unknownDefaultOpenApi;
    default:
      return _$statsLiveTaskStateEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<StatsLiveTaskStateEnum> _$statsLiveTaskStateEnumValues =
    BuiltSet<StatsLiveTaskStateEnum>(const <StatsLiveTaskStateEnum>[
      _$statsLiveTaskStateEnum_running,
      _$statsLiveTaskStateEnum_completed,
      _$statsLiveTaskStateEnum_failed,
      _$statsLiveTaskStateEnum_cancelled,
      _$statsLiveTaskStateEnum_unknownDefaultOpenApi,
    ]);

Serializer<StatsLiveTaskKindEnum> _$statsLiveTaskKindEnumSerializer =
    _$StatsLiveTaskKindEnumSerializer();
Serializer<StatsLiveTaskStateEnum> _$statsLiveTaskStateEnumSerializer =
    _$StatsLiveTaskStateEnumSerializer();

class _$StatsLiveTaskKindEnumSerializer
    implements PrimitiveSerializer<StatsLiveTaskKindEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'subagent': 'subagent',
    'workflow': 'workflow',
    'backgroundJob': 'background_job',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'subagent': 'subagent',
    'workflow': 'workflow',
    'background_job': 'backgroundJob',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[StatsLiveTaskKindEnum];
  @override
  final String wireName = 'StatsLiveTaskKindEnum';

  @override
  Object serialize(
    Serializers serializers,
    StatsLiveTaskKindEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  StatsLiveTaskKindEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => StatsLiveTaskKindEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$StatsLiveTaskStateEnumSerializer
    implements PrimitiveSerializer<StatsLiveTaskStateEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'running': 'running',
    'completed': 'completed',
    'failed': 'failed',
    'cancelled': 'cancelled',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'running': 'running',
    'completed': 'completed',
    'failed': 'failed',
    'cancelled': 'cancelled',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[StatsLiveTaskStateEnum];
  @override
  final String wireName = 'StatsLiveTaskStateEnum';

  @override
  Object serialize(
    Serializers serializers,
    StatsLiveTaskStateEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  StatsLiveTaskStateEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => StatsLiveTaskStateEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$StatsLiveTask extends StatsLiveTask {
  @override
  final String id;
  @override
  final String? parentId;
  @override
  final StatsLiveTaskKindEnum kind;
  @override
  final String name;
  @override
  final StatsLiveTaskStateEnum state;
  @override
  final DateTime startedAt;
  @override
  final DateTime? finishedAt;
  @override
  final String? tool;
  @override
  final String? label;

  factory _$StatsLiveTask([void Function(StatsLiveTaskBuilder)? updates]) =>
      (StatsLiveTaskBuilder()..update(updates))._build();

  _$StatsLiveTask._({
    required this.id,
    this.parentId,
    required this.kind,
    required this.name,
    required this.state,
    required this.startedAt,
    this.finishedAt,
    this.tool,
    this.label,
  }) : super._();
  @override
  StatsLiveTask rebuild(void Function(StatsLiveTaskBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  StatsLiveTaskBuilder toBuilder() => StatsLiveTaskBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is StatsLiveTask &&
        id == other.id &&
        parentId == other.parentId &&
        kind == other.kind &&
        name == other.name &&
        state == other.state &&
        startedAt == other.startedAt &&
        finishedAt == other.finishedAt &&
        tool == other.tool &&
        label == other.label;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, id.hashCode);
    _$hash = $jc(_$hash, parentId.hashCode);
    _$hash = $jc(_$hash, kind.hashCode);
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jc(_$hash, state.hashCode);
    _$hash = $jc(_$hash, startedAt.hashCode);
    _$hash = $jc(_$hash, finishedAt.hashCode);
    _$hash = $jc(_$hash, tool.hashCode);
    _$hash = $jc(_$hash, label.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'StatsLiveTask')
          ..add('id', id)
          ..add('parentId', parentId)
          ..add('kind', kind)
          ..add('name', name)
          ..add('state', state)
          ..add('startedAt', startedAt)
          ..add('finishedAt', finishedAt)
          ..add('tool', tool)
          ..add('label', label))
        .toString();
  }
}

class StatsLiveTaskBuilder
    implements Builder<StatsLiveTask, StatsLiveTaskBuilder> {
  _$StatsLiveTask? _$v;

  String? _id;
  String? get id => _$this._id;
  set id(String? id) => _$this._id = id;

  String? _parentId;
  String? get parentId => _$this._parentId;
  set parentId(String? parentId) => _$this._parentId = parentId;

  StatsLiveTaskKindEnum? _kind;
  StatsLiveTaskKindEnum? get kind => _$this._kind;
  set kind(StatsLiveTaskKindEnum? kind) => _$this._kind = kind;

  String? _name;
  String? get name => _$this._name;
  set name(String? name) => _$this._name = name;

  StatsLiveTaskStateEnum? _state;
  StatsLiveTaskStateEnum? get state => _$this._state;
  set state(StatsLiveTaskStateEnum? state) => _$this._state = state;

  DateTime? _startedAt;
  DateTime? get startedAt => _$this._startedAt;
  set startedAt(DateTime? startedAt) => _$this._startedAt = startedAt;

  DateTime? _finishedAt;
  DateTime? get finishedAt => _$this._finishedAt;
  set finishedAt(DateTime? finishedAt) => _$this._finishedAt = finishedAt;

  String? _tool;
  String? get tool => _$this._tool;
  set tool(String? tool) => _$this._tool = tool;

  String? _label;
  String? get label => _$this._label;
  set label(String? label) => _$this._label = label;

  StatsLiveTaskBuilder() {
    StatsLiveTask._defaults(this);
  }

  StatsLiveTaskBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _id = $v.id;
      _parentId = $v.parentId;
      _kind = $v.kind;
      _name = $v.name;
      _state = $v.state;
      _startedAt = $v.startedAt;
      _finishedAt = $v.finishedAt;
      _tool = $v.tool;
      _label = $v.label;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(StatsLiveTask other) {
    _$v = other as _$StatsLiveTask;
  }

  @override
  void update(void Function(StatsLiveTaskBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  StatsLiveTask build() => _build();

  _$StatsLiveTask _build() {
    final _$result =
        _$v ??
        _$StatsLiveTask._(
          id: BuiltValueNullFieldError.checkNotNull(id, r'StatsLiveTask', 'id'),
          parentId: parentId,
          kind: BuiltValueNullFieldError.checkNotNull(
            kind,
            r'StatsLiveTask',
            'kind',
          ),
          name: BuiltValueNullFieldError.checkNotNull(
            name,
            r'StatsLiveTask',
            'name',
          ),
          state: BuiltValueNullFieldError.checkNotNull(
            state,
            r'StatsLiveTask',
            'state',
          ),
          startedAt: BuiltValueNullFieldError.checkNotNull(
            startedAt,
            r'StatsLiveTask',
            'startedAt',
          ),
          finishedAt: finishedAt,
          tool: tool,
          label: label,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
