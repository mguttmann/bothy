// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'run_autonomy_last.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$RunAutonomyLast extends RunAutonomyLast {
  @override
  final String input;
  @override
  final bool idle;

  factory _$RunAutonomyLast([void Function(RunAutonomyLastBuilder)? updates]) =>
      (RunAutonomyLastBuilder()..update(updates))._build();

  _$RunAutonomyLast._({required this.input, required this.idle}) : super._();
  @override
  RunAutonomyLast rebuild(void Function(RunAutonomyLastBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  RunAutonomyLastBuilder toBuilder() => RunAutonomyLastBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is RunAutonomyLast &&
        input == other.input &&
        idle == other.idle;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, input.hashCode);
    _$hash = $jc(_$hash, idle.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'RunAutonomyLast')
          ..add('input', input)
          ..add('idle', idle))
        .toString();
  }
}

class RunAutonomyLastBuilder
    implements Builder<RunAutonomyLast, RunAutonomyLastBuilder> {
  _$RunAutonomyLast? _$v;

  String? _input;
  String? get input => _$this._input;
  set input(String? input) => _$this._input = input;

  bool? _idle;
  bool? get idle => _$this._idle;
  set idle(bool? idle) => _$this._idle = idle;

  RunAutonomyLastBuilder() {
    RunAutonomyLast._defaults(this);
  }

  RunAutonomyLastBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _input = $v.input;
      _idle = $v.idle;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(RunAutonomyLast other) {
    _$v = other as _$RunAutonomyLast;
  }

  @override
  void update(void Function(RunAutonomyLastBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  RunAutonomyLast build() => _build();

  _$RunAutonomyLast _build() {
    final _$result =
        _$v ??
        _$RunAutonomyLast._(
          input: BuiltValueNullFieldError.checkNotNull(
            input,
            r'RunAutonomyLast',
            'input',
          ),
          idle: BuiltValueNullFieldError.checkNotNull(
            idle,
            r'RunAutonomyLast',
            'idle',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
