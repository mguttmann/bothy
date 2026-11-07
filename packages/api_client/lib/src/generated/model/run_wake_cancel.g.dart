// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'run_wake_cancel.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$RunWakeCancel extends RunWakeCancel {
  @override
  final String? reason;

  factory _$RunWakeCancel([void Function(RunWakeCancelBuilder)? updates]) =>
      (RunWakeCancelBuilder()..update(updates))._build();

  _$RunWakeCancel._({this.reason}) : super._();
  @override
  RunWakeCancel rebuild(void Function(RunWakeCancelBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  RunWakeCancelBuilder toBuilder() => RunWakeCancelBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is RunWakeCancel && reason == other.reason;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, reason.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'RunWakeCancel',
    )..add('reason', reason)).toString();
  }
}

class RunWakeCancelBuilder
    implements Builder<RunWakeCancel, RunWakeCancelBuilder> {
  _$RunWakeCancel? _$v;

  String? _reason;
  String? get reason => _$this._reason;
  set reason(String? reason) => _$this._reason = reason;

  RunWakeCancelBuilder() {
    RunWakeCancel._defaults(this);
  }

  RunWakeCancelBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _reason = $v.reason;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(RunWakeCancel other) {
    _$v = other as _$RunWakeCancel;
  }

  @override
  void update(void Function(RunWakeCancelBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  RunWakeCancel build() => _build();

  _$RunWakeCancel _build() {
    final _$result = _$v ?? _$RunWakeCancel._(reason: reason);
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
