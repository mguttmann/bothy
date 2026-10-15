// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'run_cancel.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$RunCancel extends RunCancel {
  @override
  final String? reason;

  factory _$RunCancel([void Function(RunCancelBuilder)? updates]) =>
      (RunCancelBuilder()..update(updates))._build();

  _$RunCancel._({this.reason}) : super._();
  @override
  RunCancel rebuild(void Function(RunCancelBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  RunCancelBuilder toBuilder() => RunCancelBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is RunCancel && reason == other.reason;
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
      r'RunCancel',
    )..add('reason', reason)).toString();
  }
}

class RunCancelBuilder implements Builder<RunCancel, RunCancelBuilder> {
  _$RunCancel? _$v;

  String? _reason;
  String? get reason => _$this._reason;
  set reason(String? reason) => _$this._reason = reason;

  RunCancelBuilder() {
    RunCancel._defaults(this);
  }

  RunCancelBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _reason = $v.reason;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(RunCancel other) {
    _$v = other as _$RunCancel;
  }

  @override
  void update(void Function(RunCancelBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  RunCancel build() => _build();

  _$RunCancel _build() {
    final _$result = _$v ?? _$RunCancel._(reason: reason);
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
