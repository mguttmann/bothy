// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'run_plan_items_inner.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const RunPlanItemsInnerStatusEnum _$runPlanItemsInnerStatusEnum_pending =
    const RunPlanItemsInnerStatusEnum._('pending');
const RunPlanItemsInnerStatusEnum _$runPlanItemsInnerStatusEnum_inProgress =
    const RunPlanItemsInnerStatusEnum._('inProgress');
const RunPlanItemsInnerStatusEnum _$runPlanItemsInnerStatusEnum_completed =
    const RunPlanItemsInnerStatusEnum._('completed');
const RunPlanItemsInnerStatusEnum
_$runPlanItemsInnerStatusEnum_unknownDefaultOpenApi =
    const RunPlanItemsInnerStatusEnum._('unknownDefaultOpenApi');

RunPlanItemsInnerStatusEnum _$runPlanItemsInnerStatusEnumValueOf(String name) {
  switch (name) {
    case 'pending':
      return _$runPlanItemsInnerStatusEnum_pending;
    case 'inProgress':
      return _$runPlanItemsInnerStatusEnum_inProgress;
    case 'completed':
      return _$runPlanItemsInnerStatusEnum_completed;
    case 'unknownDefaultOpenApi':
      return _$runPlanItemsInnerStatusEnum_unknownDefaultOpenApi;
    default:
      return _$runPlanItemsInnerStatusEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<RunPlanItemsInnerStatusEnum>
_$runPlanItemsInnerStatusEnumValues = BuiltSet<RunPlanItemsInnerStatusEnum>(
  const <RunPlanItemsInnerStatusEnum>[
    _$runPlanItemsInnerStatusEnum_pending,
    _$runPlanItemsInnerStatusEnum_inProgress,
    _$runPlanItemsInnerStatusEnum_completed,
    _$runPlanItemsInnerStatusEnum_unknownDefaultOpenApi,
  ],
);

Serializer<RunPlanItemsInnerStatusEnum>
_$runPlanItemsInnerStatusEnumSerializer =
    _$RunPlanItemsInnerStatusEnumSerializer();

class _$RunPlanItemsInnerStatusEnumSerializer
    implements PrimitiveSerializer<RunPlanItemsInnerStatusEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'pending': 'pending',
    'inProgress': 'in_progress',
    'completed': 'completed',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'pending': 'pending',
    'in_progress': 'inProgress',
    'completed': 'completed',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[RunPlanItemsInnerStatusEnum];
  @override
  final String wireName = 'RunPlanItemsInnerStatusEnum';

  @override
  Object serialize(
    Serializers serializers,
    RunPlanItemsInnerStatusEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  RunPlanItemsInnerStatusEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => RunPlanItemsInnerStatusEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$RunPlanItemsInner extends RunPlanItemsInner {
  @override
  final String id;
  @override
  final String text;
  @override
  final RunPlanItemsInnerStatusEnum status;

  factory _$RunPlanItemsInner([
    void Function(RunPlanItemsInnerBuilder)? updates,
  ]) => (RunPlanItemsInnerBuilder()..update(updates))._build();

  _$RunPlanItemsInner._({
    required this.id,
    required this.text,
    required this.status,
  }) : super._();
  @override
  RunPlanItemsInner rebuild(void Function(RunPlanItemsInnerBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  RunPlanItemsInnerBuilder toBuilder() =>
      RunPlanItemsInnerBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is RunPlanItemsInner &&
        id == other.id &&
        text == other.text &&
        status == other.status;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, id.hashCode);
    _$hash = $jc(_$hash, text.hashCode);
    _$hash = $jc(_$hash, status.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'RunPlanItemsInner')
          ..add('id', id)
          ..add('text', text)
          ..add('status', status))
        .toString();
  }
}

class RunPlanItemsInnerBuilder
    implements Builder<RunPlanItemsInner, RunPlanItemsInnerBuilder> {
  _$RunPlanItemsInner? _$v;

  String? _id;
  String? get id => _$this._id;
  set id(String? id) => _$this._id = id;

  String? _text;
  String? get text => _$this._text;
  set text(String? text) => _$this._text = text;

  RunPlanItemsInnerStatusEnum? _status;
  RunPlanItemsInnerStatusEnum? get status => _$this._status;
  set status(RunPlanItemsInnerStatusEnum? status) => _$this._status = status;

  RunPlanItemsInnerBuilder() {
    RunPlanItemsInner._defaults(this);
  }

  RunPlanItemsInnerBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _id = $v.id;
      _text = $v.text;
      _status = $v.status;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(RunPlanItemsInner other) {
    _$v = other as _$RunPlanItemsInner;
  }

  @override
  void update(void Function(RunPlanItemsInnerBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  RunPlanItemsInner build() => _build();

  _$RunPlanItemsInner _build() {
    final _$result =
        _$v ??
        _$RunPlanItemsInner._(
          id: BuiltValueNullFieldError.checkNotNull(
            id,
            r'RunPlanItemsInner',
            'id',
          ),
          text: BuiltValueNullFieldError.checkNotNull(
            text,
            r'RunPlanItemsInner',
            'text',
          ),
          status: BuiltValueNullFieldError.checkNotNull(
            status,
            r'RunPlanItemsInner',
            'status',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
