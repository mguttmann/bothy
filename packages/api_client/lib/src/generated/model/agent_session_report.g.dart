// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'agent_session_report.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const AgentSessionReportBindFailedEnum
_$agentSessionReportBindFailedEnum_sessionMissing =
    const AgentSessionReportBindFailedEnum._('sessionMissing');
const AgentSessionReportBindFailedEnum
_$agentSessionReportBindFailedEnum_unknownDefaultOpenApi =
    const AgentSessionReportBindFailedEnum._('unknownDefaultOpenApi');

AgentSessionReportBindFailedEnum _$agentSessionReportBindFailedEnumValueOf(
  String name,
) {
  switch (name) {
    case 'sessionMissing':
      return _$agentSessionReportBindFailedEnum_sessionMissing;
    case 'unknownDefaultOpenApi':
      return _$agentSessionReportBindFailedEnum_unknownDefaultOpenApi;
    default:
      return _$agentSessionReportBindFailedEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<AgentSessionReportBindFailedEnum>
_$agentSessionReportBindFailedEnumValues =
    BuiltSet<AgentSessionReportBindFailedEnum>(
      const <AgentSessionReportBindFailedEnum>[
        _$agentSessionReportBindFailedEnum_sessionMissing,
        _$agentSessionReportBindFailedEnum_unknownDefaultOpenApi,
      ],
    );

Serializer<AgentSessionReportBindFailedEnum>
_$agentSessionReportBindFailedEnumSerializer =
    _$AgentSessionReportBindFailedEnumSerializer();

class _$AgentSessionReportBindFailedEnumSerializer
    implements PrimitiveSerializer<AgentSessionReportBindFailedEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'sessionMissing': 'session_missing',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'session_missing': 'sessionMissing',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[AgentSessionReportBindFailedEnum];
  @override
  final String wireName = 'AgentSessionReportBindFailedEnum';

  @override
  Object serialize(
    Serializers serializers,
    AgentSessionReportBindFailedEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  AgentSessionReportBindFailedEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => AgentSessionReportBindFailedEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$AgentSessionReport extends AgentSessionReport {
  @override
  final String externalSessionRef;
  @override
  final String? model;
  @override
  final bool? started;
  @override
  final AgentSessionReportBindFailedEnum? bindFailed;

  factory _$AgentSessionReport([
    void Function(AgentSessionReportBuilder)? updates,
  ]) => (AgentSessionReportBuilder()..update(updates))._build();

  _$AgentSessionReport._({
    required this.externalSessionRef,
    this.model,
    this.started,
    this.bindFailed,
  }) : super._();
  @override
  AgentSessionReport rebuild(
    void Function(AgentSessionReportBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  AgentSessionReportBuilder toBuilder() =>
      AgentSessionReportBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AgentSessionReport &&
        externalSessionRef == other.externalSessionRef &&
        model == other.model &&
        started == other.started &&
        bindFailed == other.bindFailed;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, externalSessionRef.hashCode);
    _$hash = $jc(_$hash, model.hashCode);
    _$hash = $jc(_$hash, started.hashCode);
    _$hash = $jc(_$hash, bindFailed.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AgentSessionReport')
          ..add('externalSessionRef', externalSessionRef)
          ..add('model', model)
          ..add('started', started)
          ..add('bindFailed', bindFailed))
        .toString();
  }
}

class AgentSessionReportBuilder
    implements Builder<AgentSessionReport, AgentSessionReportBuilder> {
  _$AgentSessionReport? _$v;

  String? _externalSessionRef;
  String? get externalSessionRef => _$this._externalSessionRef;
  set externalSessionRef(String? externalSessionRef) =>
      _$this._externalSessionRef = externalSessionRef;

  String? _model;
  String? get model => _$this._model;
  set model(String? model) => _$this._model = model;

  bool? _started;
  bool? get started => _$this._started;
  set started(bool? started) => _$this._started = started;

  AgentSessionReportBindFailedEnum? _bindFailed;
  AgentSessionReportBindFailedEnum? get bindFailed => _$this._bindFailed;
  set bindFailed(AgentSessionReportBindFailedEnum? bindFailed) =>
      _$this._bindFailed = bindFailed;

  AgentSessionReportBuilder() {
    AgentSessionReport._defaults(this);
  }

  AgentSessionReportBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _externalSessionRef = $v.externalSessionRef;
      _model = $v.model;
      _started = $v.started;
      _bindFailed = $v.bindFailed;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AgentSessionReport other) {
    _$v = other as _$AgentSessionReport;
  }

  @override
  void update(void Function(AgentSessionReportBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AgentSessionReport build() => _build();

  _$AgentSessionReport _build() {
    final _$result =
        _$v ??
        _$AgentSessionReport._(
          externalSessionRef: BuiltValueNullFieldError.checkNotNull(
            externalSessionRef,
            r'AgentSessionReport',
            'externalSessionRef',
          ),
          model: model,
          started: started,
          bindFailed: bindFailed,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
