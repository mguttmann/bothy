// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'run_start.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$RunStart extends RunStart {
  @override
  final String invocationToken;
  @override
  final BillingMode billingModeUsed;
  @override
  final ToolClass toolClassUsed;
  @override
  final String? externalSessionRef;
  @override
  final String? model;
  @override
  final BillingEvidence? billingEvidence;

  factory _$RunStart([void Function(RunStartBuilder)? updates]) =>
      (RunStartBuilder()..update(updates))._build();

  _$RunStart._({
    required this.invocationToken,
    required this.billingModeUsed,
    required this.toolClassUsed,
    this.externalSessionRef,
    this.model,
    this.billingEvidence,
  }) : super._();
  @override
  RunStart rebuild(void Function(RunStartBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  RunStartBuilder toBuilder() => RunStartBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is RunStart &&
        invocationToken == other.invocationToken &&
        billingModeUsed == other.billingModeUsed &&
        toolClassUsed == other.toolClassUsed &&
        externalSessionRef == other.externalSessionRef &&
        model == other.model &&
        billingEvidence == other.billingEvidence;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, invocationToken.hashCode);
    _$hash = $jc(_$hash, billingModeUsed.hashCode);
    _$hash = $jc(_$hash, toolClassUsed.hashCode);
    _$hash = $jc(_$hash, externalSessionRef.hashCode);
    _$hash = $jc(_$hash, model.hashCode);
    _$hash = $jc(_$hash, billingEvidence.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'RunStart')
          ..add('invocationToken', invocationToken)
          ..add('billingModeUsed', billingModeUsed)
          ..add('toolClassUsed', toolClassUsed)
          ..add('externalSessionRef', externalSessionRef)
          ..add('model', model)
          ..add('billingEvidence', billingEvidence))
        .toString();
  }
}

class RunStartBuilder implements Builder<RunStart, RunStartBuilder> {
  _$RunStart? _$v;

  String? _invocationToken;
  String? get invocationToken => _$this._invocationToken;
  set invocationToken(String? invocationToken) =>
      _$this._invocationToken = invocationToken;

  BillingMode? _billingModeUsed;
  BillingMode? get billingModeUsed => _$this._billingModeUsed;
  set billingModeUsed(BillingMode? billingModeUsed) =>
      _$this._billingModeUsed = billingModeUsed;

  ToolClass? _toolClassUsed;
  ToolClass? get toolClassUsed => _$this._toolClassUsed;
  set toolClassUsed(ToolClass? toolClassUsed) =>
      _$this._toolClassUsed = toolClassUsed;

  String? _externalSessionRef;
  String? get externalSessionRef => _$this._externalSessionRef;
  set externalSessionRef(String? externalSessionRef) =>
      _$this._externalSessionRef = externalSessionRef;

  String? _model;
  String? get model => _$this._model;
  set model(String? model) => _$this._model = model;

  BillingEvidenceBuilder? _billingEvidence;
  BillingEvidenceBuilder get billingEvidence =>
      _$this._billingEvidence ??= BillingEvidenceBuilder();
  set billingEvidence(BillingEvidenceBuilder? billingEvidence) =>
      _$this._billingEvidence = billingEvidence;

  RunStartBuilder() {
    RunStart._defaults(this);
  }

  RunStartBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _invocationToken = $v.invocationToken;
      _billingModeUsed = $v.billingModeUsed;
      _toolClassUsed = $v.toolClassUsed;
      _externalSessionRef = $v.externalSessionRef;
      _model = $v.model;
      _billingEvidence = $v.billingEvidence?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(RunStart other) {
    _$v = other as _$RunStart;
  }

  @override
  void update(void Function(RunStartBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  RunStart build() => _build();

  _$RunStart _build() {
    _$RunStart _$result;
    try {
      _$result =
          _$v ??
          _$RunStart._(
            invocationToken: BuiltValueNullFieldError.checkNotNull(
              invocationToken,
              r'RunStart',
              'invocationToken',
            ),
            billingModeUsed: BuiltValueNullFieldError.checkNotNull(
              billingModeUsed,
              r'RunStart',
              'billingModeUsed',
            ),
            toolClassUsed: BuiltValueNullFieldError.checkNotNull(
              toolClassUsed,
              r'RunStart',
              'toolClassUsed',
            ),
            externalSessionRef: externalSessionRef,
            model: model,
            billingEvidence: _billingEvidence?.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'billingEvidence';
        _billingEvidence?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'RunStart',
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
