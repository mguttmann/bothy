// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'run_wake_condition.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$RunWakeCondition extends RunWakeCondition {
  @override
  final String? repository;
  @override
  final int? number;
  @override
  final String? headCommit;
  @override
  final String? commandLine;

  factory _$RunWakeCondition([
    void Function(RunWakeConditionBuilder)? updates,
  ]) => (RunWakeConditionBuilder()..update(updates))._build();

  _$RunWakeCondition._({
    this.repository,
    this.number,
    this.headCommit,
    this.commandLine,
  }) : super._();
  @override
  RunWakeCondition rebuild(void Function(RunWakeConditionBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  RunWakeConditionBuilder toBuilder() =>
      RunWakeConditionBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is RunWakeCondition &&
        repository == other.repository &&
        number == other.number &&
        headCommit == other.headCommit &&
        commandLine == other.commandLine;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, repository.hashCode);
    _$hash = $jc(_$hash, number.hashCode);
    _$hash = $jc(_$hash, headCommit.hashCode);
    _$hash = $jc(_$hash, commandLine.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'RunWakeCondition')
          ..add('repository', repository)
          ..add('number', number)
          ..add('headCommit', headCommit)
          ..add('commandLine', commandLine))
        .toString();
  }
}

class RunWakeConditionBuilder
    implements Builder<RunWakeCondition, RunWakeConditionBuilder> {
  _$RunWakeCondition? _$v;

  String? _repository;
  String? get repository => _$this._repository;
  set repository(String? repository) => _$this._repository = repository;

  int? _number;
  int? get number => _$this._number;
  set number(int? number) => _$this._number = number;

  String? _headCommit;
  String? get headCommit => _$this._headCommit;
  set headCommit(String? headCommit) => _$this._headCommit = headCommit;

  String? _commandLine;
  String? get commandLine => _$this._commandLine;
  set commandLine(String? commandLine) => _$this._commandLine = commandLine;

  RunWakeConditionBuilder() {
    RunWakeCondition._defaults(this);
  }

  RunWakeConditionBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _repository = $v.repository;
      _number = $v.number;
      _headCommit = $v.headCommit;
      _commandLine = $v.commandLine;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(RunWakeCondition other) {
    _$v = other as _$RunWakeCondition;
  }

  @override
  void update(void Function(RunWakeConditionBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  RunWakeCondition build() => _build();

  _$RunWakeCondition _build() {
    final _$result =
        _$v ??
        _$RunWakeCondition._(
          repository: repository,
          number: number,
          headCommit: headCommit,
          commandLine: commandLine,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
