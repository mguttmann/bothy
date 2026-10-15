// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'run_await_input.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$RunAwaitInput extends RunAwaitInput {
  @override
  final String statusLine;

  factory _$RunAwaitInput([void Function(RunAwaitInputBuilder)? updates]) =>
      (RunAwaitInputBuilder()..update(updates))._build();

  _$RunAwaitInput._({required this.statusLine}) : super._();
  @override
  RunAwaitInput rebuild(void Function(RunAwaitInputBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  RunAwaitInputBuilder toBuilder() => RunAwaitInputBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is RunAwaitInput && statusLine == other.statusLine;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, statusLine.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'RunAwaitInput',
    )..add('statusLine', statusLine)).toString();
  }
}

class RunAwaitInputBuilder
    implements Builder<RunAwaitInput, RunAwaitInputBuilder> {
  _$RunAwaitInput? _$v;

  String? _statusLine;
  String? get statusLine => _$this._statusLine;
  set statusLine(String? statusLine) => _$this._statusLine = statusLine;

  RunAwaitInputBuilder() {
    RunAwaitInput._defaults(this);
  }

  RunAwaitInputBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _statusLine = $v.statusLine;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(RunAwaitInput other) {
    _$v = other as _$RunAwaitInput;
  }

  @override
  void update(void Function(RunAwaitInputBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  RunAwaitInput build() => _build();

  _$RunAwaitInput _build() {
    final _$result =
        _$v ??
        _$RunAwaitInput._(
          statusLine: BuiltValueNullFieldError.checkNotNull(
            statusLine,
            r'RunAwaitInput',
            'statusLine',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
