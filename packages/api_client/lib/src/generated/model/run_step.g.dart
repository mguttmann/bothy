// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'run_step.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const RunStepKindEnum _$runStepKindEnum_toolCall = const RunStepKindEnum._(
  'toolCall',
);
const RunStepKindEnum _$runStepKindEnum_status = const RunStepKindEnum._(
  'status',
);
const RunStepKindEnum _$runStepKindEnum_subagent = const RunStepKindEnum._(
  'subagent',
);
const RunStepKindEnum _$runStepKindEnum_workflow = const RunStepKindEnum._(
  'workflow',
);
const RunStepKindEnum _$runStepKindEnum_backgroundJob = const RunStepKindEnum._(
  'backgroundJob',
);
const RunStepKindEnum _$runStepKindEnum_unknownDefaultOpenApi =
    const RunStepKindEnum._('unknownDefaultOpenApi');

RunStepKindEnum _$runStepKindEnumValueOf(String name) {
  switch (name) {
    case 'toolCall':
      return _$runStepKindEnum_toolCall;
    case 'status':
      return _$runStepKindEnum_status;
    case 'subagent':
      return _$runStepKindEnum_subagent;
    case 'workflow':
      return _$runStepKindEnum_workflow;
    case 'backgroundJob':
      return _$runStepKindEnum_backgroundJob;
    case 'unknownDefaultOpenApi':
      return _$runStepKindEnum_unknownDefaultOpenApi;
    default:
      return _$runStepKindEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<RunStepKindEnum> _$runStepKindEnumValues =
    BuiltSet<RunStepKindEnum>(const <RunStepKindEnum>[
      _$runStepKindEnum_toolCall,
      _$runStepKindEnum_status,
      _$runStepKindEnum_subagent,
      _$runStepKindEnum_workflow,
      _$runStepKindEnum_backgroundJob,
      _$runStepKindEnum_unknownDefaultOpenApi,
    ]);

const RunStepStatusEnum _$runStepStatusEnum_pending = const RunStepStatusEnum._(
  'pending',
);
const RunStepStatusEnum _$runStepStatusEnum_running = const RunStepStatusEnum._(
  'running',
);
const RunStepStatusEnum _$runStepStatusEnum_completed =
    const RunStepStatusEnum._('completed');
const RunStepStatusEnum _$runStepStatusEnum_failed = const RunStepStatusEnum._(
  'failed',
);
const RunStepStatusEnum _$runStepStatusEnum_cancelled =
    const RunStepStatusEnum._('cancelled');
const RunStepStatusEnum _$runStepStatusEnum_unknownDefaultOpenApi =
    const RunStepStatusEnum._('unknownDefaultOpenApi');

RunStepStatusEnum _$runStepStatusEnumValueOf(String name) {
  switch (name) {
    case 'pending':
      return _$runStepStatusEnum_pending;
    case 'running':
      return _$runStepStatusEnum_running;
    case 'completed':
      return _$runStepStatusEnum_completed;
    case 'failed':
      return _$runStepStatusEnum_failed;
    case 'cancelled':
      return _$runStepStatusEnum_cancelled;
    case 'unknownDefaultOpenApi':
      return _$runStepStatusEnum_unknownDefaultOpenApi;
    default:
      return _$runStepStatusEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<RunStepStatusEnum> _$runStepStatusEnumValues =
    BuiltSet<RunStepStatusEnum>(const <RunStepStatusEnum>[
      _$runStepStatusEnum_pending,
      _$runStepStatusEnum_running,
      _$runStepStatusEnum_completed,
      _$runStepStatusEnum_failed,
      _$runStepStatusEnum_cancelled,
      _$runStepStatusEnum_unknownDefaultOpenApi,
    ]);

Serializer<RunStepKindEnum> _$runStepKindEnumSerializer =
    _$RunStepKindEnumSerializer();
Serializer<RunStepStatusEnum> _$runStepStatusEnumSerializer =
    _$RunStepStatusEnumSerializer();

class _$RunStepKindEnumSerializer
    implements PrimitiveSerializer<RunStepKindEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'toolCall': 'tool_call',
    'status': 'status',
    'subagent': 'subagent',
    'workflow': 'workflow',
    'backgroundJob': 'background_job',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'tool_call': 'toolCall',
    'status': 'status',
    'subagent': 'subagent',
    'workflow': 'workflow',
    'background_job': 'backgroundJob',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[RunStepKindEnum];
  @override
  final String wireName = 'RunStepKindEnum';

  @override
  Object serialize(
    Serializers serializers,
    RunStepKindEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  RunStepKindEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => RunStepKindEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$RunStepStatusEnumSerializer
    implements PrimitiveSerializer<RunStepStatusEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'pending': 'pending',
    'running': 'running',
    'completed': 'completed',
    'failed': 'failed',
    'cancelled': 'cancelled',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'pending': 'pending',
    'running': 'running',
    'completed': 'completed',
    'failed': 'failed',
    'cancelled': 'cancelled',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[RunStepStatusEnum];
  @override
  final String wireName = 'RunStepStatusEnum';

  @override
  Object serialize(
    Serializers serializers,
    RunStepStatusEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  RunStepStatusEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => RunStepStatusEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$RunStep extends RunStep {
  @override
  final String id;
  @override
  final int ordinal;
  @override
  final String? parentStepId;
  @override
  final RunStepKindEnum kind;
  @override
  final String name;
  @override
  final String title;
  @override
  final RunStepStatusEnum status;
  @override
  final BuiltMap<String, JsonObject?>? detail;
  @override
  final DateTime startedAt;
  @override
  final DateTime? finishedAt;

  factory _$RunStep([void Function(RunStepBuilder)? updates]) =>
      (RunStepBuilder()..update(updates))._build();

  _$RunStep._({
    required this.id,
    required this.ordinal,
    this.parentStepId,
    required this.kind,
    required this.name,
    required this.title,
    required this.status,
    this.detail,
    required this.startedAt,
    this.finishedAt,
  }) : super._();
  @override
  RunStep rebuild(void Function(RunStepBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  RunStepBuilder toBuilder() => RunStepBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is RunStep &&
        id == other.id &&
        ordinal == other.ordinal &&
        parentStepId == other.parentStepId &&
        kind == other.kind &&
        name == other.name &&
        title == other.title &&
        status == other.status &&
        detail == other.detail &&
        startedAt == other.startedAt &&
        finishedAt == other.finishedAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, id.hashCode);
    _$hash = $jc(_$hash, ordinal.hashCode);
    _$hash = $jc(_$hash, parentStepId.hashCode);
    _$hash = $jc(_$hash, kind.hashCode);
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jc(_$hash, title.hashCode);
    _$hash = $jc(_$hash, status.hashCode);
    _$hash = $jc(_$hash, detail.hashCode);
    _$hash = $jc(_$hash, startedAt.hashCode);
    _$hash = $jc(_$hash, finishedAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'RunStep')
          ..add('id', id)
          ..add('ordinal', ordinal)
          ..add('parentStepId', parentStepId)
          ..add('kind', kind)
          ..add('name', name)
          ..add('title', title)
          ..add('status', status)
          ..add('detail', detail)
          ..add('startedAt', startedAt)
          ..add('finishedAt', finishedAt))
        .toString();
  }
}

class RunStepBuilder implements Builder<RunStep, RunStepBuilder> {
  _$RunStep? _$v;

  String? _id;
  String? get id => _$this._id;
  set id(String? id) => _$this._id = id;

  int? _ordinal;
  int? get ordinal => _$this._ordinal;
  set ordinal(int? ordinal) => _$this._ordinal = ordinal;

  String? _parentStepId;
  String? get parentStepId => _$this._parentStepId;
  set parentStepId(String? parentStepId) => _$this._parentStepId = parentStepId;

  RunStepKindEnum? _kind;
  RunStepKindEnum? get kind => _$this._kind;
  set kind(RunStepKindEnum? kind) => _$this._kind = kind;

  String? _name;
  String? get name => _$this._name;
  set name(String? name) => _$this._name = name;

  String? _title;
  String? get title => _$this._title;
  set title(String? title) => _$this._title = title;

  RunStepStatusEnum? _status;
  RunStepStatusEnum? get status => _$this._status;
  set status(RunStepStatusEnum? status) => _$this._status = status;

  MapBuilder<String, JsonObject?>? _detail;
  MapBuilder<String, JsonObject?> get detail =>
      _$this._detail ??= MapBuilder<String, JsonObject?>();
  set detail(MapBuilder<String, JsonObject?>? detail) =>
      _$this._detail = detail;

  DateTime? _startedAt;
  DateTime? get startedAt => _$this._startedAt;
  set startedAt(DateTime? startedAt) => _$this._startedAt = startedAt;

  DateTime? _finishedAt;
  DateTime? get finishedAt => _$this._finishedAt;
  set finishedAt(DateTime? finishedAt) => _$this._finishedAt = finishedAt;

  RunStepBuilder() {
    RunStep._defaults(this);
  }

  RunStepBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _id = $v.id;
      _ordinal = $v.ordinal;
      _parentStepId = $v.parentStepId;
      _kind = $v.kind;
      _name = $v.name;
      _title = $v.title;
      _status = $v.status;
      _detail = $v.detail?.toBuilder();
      _startedAt = $v.startedAt;
      _finishedAt = $v.finishedAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(RunStep other) {
    _$v = other as _$RunStep;
  }

  @override
  void update(void Function(RunStepBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  RunStep build() => _build();

  _$RunStep _build() {
    _$RunStep _$result;
    try {
      _$result =
          _$v ??
          _$RunStep._(
            id: BuiltValueNullFieldError.checkNotNull(id, r'RunStep', 'id'),
            ordinal: BuiltValueNullFieldError.checkNotNull(
              ordinal,
              r'RunStep',
              'ordinal',
            ),
            parentStepId: parentStepId,
            kind: BuiltValueNullFieldError.checkNotNull(
              kind,
              r'RunStep',
              'kind',
            ),
            name: BuiltValueNullFieldError.checkNotNull(
              name,
              r'RunStep',
              'name',
            ),
            title: BuiltValueNullFieldError.checkNotNull(
              title,
              r'RunStep',
              'title',
            ),
            status: BuiltValueNullFieldError.checkNotNull(
              status,
              r'RunStep',
              'status',
            ),
            detail: _detail?.build(),
            startedAt: BuiltValueNullFieldError.checkNotNull(
              startedAt,
              r'RunStep',
              'startedAt',
            ),
            finishedAt: finishedAt,
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'detail';
        _detail?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'RunStep',
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
