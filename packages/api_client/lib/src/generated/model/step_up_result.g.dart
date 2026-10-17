// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'step_up_result.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$StepUpResult extends StepUpResult {
  @override
  final DateTime stepUpAt;
  @override
  final DateTime validUntil;

  factory _$StepUpResult([void Function(StepUpResultBuilder)? updates]) =>
      (StepUpResultBuilder()..update(updates))._build();

  _$StepUpResult._({required this.stepUpAt, required this.validUntil})
    : super._();
  @override
  StepUpResult rebuild(void Function(StepUpResultBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  StepUpResultBuilder toBuilder() => StepUpResultBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is StepUpResult &&
        stepUpAt == other.stepUpAt &&
        validUntil == other.validUntil;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, stepUpAt.hashCode);
    _$hash = $jc(_$hash, validUntil.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'StepUpResult')
          ..add('stepUpAt', stepUpAt)
          ..add('validUntil', validUntil))
        .toString();
  }
}

class StepUpResultBuilder
    implements Builder<StepUpResult, StepUpResultBuilder> {
  _$StepUpResult? _$v;

  DateTime? _stepUpAt;
  DateTime? get stepUpAt => _$this._stepUpAt;
  set stepUpAt(DateTime? stepUpAt) => _$this._stepUpAt = stepUpAt;

  DateTime? _validUntil;
  DateTime? get validUntil => _$this._validUntil;
  set validUntil(DateTime? validUntil) => _$this._validUntil = validUntil;

  StepUpResultBuilder() {
    StepUpResult._defaults(this);
  }

  StepUpResultBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _stepUpAt = $v.stepUpAt;
      _validUntil = $v.validUntil;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(StepUpResult other) {
    _$v = other as _$StepUpResult;
  }

  @override
  void update(void Function(StepUpResultBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  StepUpResult build() => _build();

  _$StepUpResult _build() {
    final _$result =
        _$v ??
        _$StepUpResult._(
          stepUpAt: BuiltValueNullFieldError.checkNotNull(
            stepUpAt,
            r'StepUpResult',
            'stepUpAt',
          ),
          validUntil: BuiltValueNullFieldError.checkNotNull(
            validUntil,
            r'StepUpResult',
            'validUntil',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
