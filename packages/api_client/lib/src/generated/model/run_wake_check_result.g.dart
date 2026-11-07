// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'run_wake_check_result.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$RunWakeCheckResult extends RunWakeCheckResult {
  @override
  final ToolClass toolClass;
  @override
  final BillingMode billingMode;

  factory _$RunWakeCheckResult([
    void Function(RunWakeCheckResultBuilder)? updates,
  ]) => (RunWakeCheckResultBuilder()..update(updates))._build();

  _$RunWakeCheckResult._({required this.toolClass, required this.billingMode})
    : super._();
  @override
  RunWakeCheckResult rebuild(
    void Function(RunWakeCheckResultBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  RunWakeCheckResultBuilder toBuilder() =>
      RunWakeCheckResultBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is RunWakeCheckResult &&
        toolClass == other.toolClass &&
        billingMode == other.billingMode;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, toolClass.hashCode);
    _$hash = $jc(_$hash, billingMode.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'RunWakeCheckResult')
          ..add('toolClass', toolClass)
          ..add('billingMode', billingMode))
        .toString();
  }
}

class RunWakeCheckResultBuilder
    implements Builder<RunWakeCheckResult, RunWakeCheckResultBuilder> {
  _$RunWakeCheckResult? _$v;

  ToolClass? _toolClass;
  ToolClass? get toolClass => _$this._toolClass;
  set toolClass(ToolClass? toolClass) => _$this._toolClass = toolClass;

  BillingMode? _billingMode;
  BillingMode? get billingMode => _$this._billingMode;
  set billingMode(BillingMode? billingMode) =>
      _$this._billingMode = billingMode;

  RunWakeCheckResultBuilder() {
    RunWakeCheckResult._defaults(this);
  }

  RunWakeCheckResultBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _toolClass = $v.toolClass;
      _billingMode = $v.billingMode;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(RunWakeCheckResult other) {
    _$v = other as _$RunWakeCheckResult;
  }

  @override
  void update(void Function(RunWakeCheckResultBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  RunWakeCheckResult build() => _build();

  _$RunWakeCheckResult _build() {
    final _$result =
        _$v ??
        _$RunWakeCheckResult._(
          toolClass: BuiltValueNullFieldError.checkNotNull(
            toolClass,
            r'RunWakeCheckResult',
            'toolClass',
          ),
          billingMode: BuiltValueNullFieldError.checkNotNull(
            billingMode,
            r'RunWakeCheckResult',
            'billingMode',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
