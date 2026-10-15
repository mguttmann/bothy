// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'agent_session_report.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$AgentSessionReport extends AgentSessionReport {
  @override
  final String externalSessionRef;
  @override
  final String? model;

  factory _$AgentSessionReport([
    void Function(AgentSessionReportBuilder)? updates,
  ]) => (AgentSessionReportBuilder()..update(updates))._build();

  _$AgentSessionReport._({required this.externalSessionRef, this.model})
    : super._();
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
        model == other.model;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, externalSessionRef.hashCode);
    _$hash = $jc(_$hash, model.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AgentSessionReport')
          ..add('externalSessionRef', externalSessionRef)
          ..add('model', model))
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

  AgentSessionReportBuilder() {
    AgentSessionReport._defaults(this);
  }

  AgentSessionReportBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _externalSessionRef = $v.externalSessionRef;
      _model = $v.model;
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
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
