// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'command_invocation.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$CommandInvocation extends CommandInvocation {
  @override
  final String invocationId;
  @override
  final String commandId;
  @override
  final String channelId;
  @override
  final String? threadId;
  @override
  final String? runId;
  @override
  final bool accepted;

  factory _$CommandInvocation([
    void Function(CommandInvocationBuilder)? updates,
  ]) => (CommandInvocationBuilder()..update(updates))._build();

  _$CommandInvocation._({
    required this.invocationId,
    required this.commandId,
    required this.channelId,
    this.threadId,
    this.runId,
    required this.accepted,
  }) : super._();
  @override
  CommandInvocation rebuild(void Function(CommandInvocationBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  CommandInvocationBuilder toBuilder() =>
      CommandInvocationBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is CommandInvocation &&
        invocationId == other.invocationId &&
        commandId == other.commandId &&
        channelId == other.channelId &&
        threadId == other.threadId &&
        runId == other.runId &&
        accepted == other.accepted;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, invocationId.hashCode);
    _$hash = $jc(_$hash, commandId.hashCode);
    _$hash = $jc(_$hash, channelId.hashCode);
    _$hash = $jc(_$hash, threadId.hashCode);
    _$hash = $jc(_$hash, runId.hashCode);
    _$hash = $jc(_$hash, accepted.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'CommandInvocation')
          ..add('invocationId', invocationId)
          ..add('commandId', commandId)
          ..add('channelId', channelId)
          ..add('threadId', threadId)
          ..add('runId', runId)
          ..add('accepted', accepted))
        .toString();
  }
}

class CommandInvocationBuilder
    implements Builder<CommandInvocation, CommandInvocationBuilder> {
  _$CommandInvocation? _$v;

  String? _invocationId;
  String? get invocationId => _$this._invocationId;
  set invocationId(String? invocationId) => _$this._invocationId = invocationId;

  String? _commandId;
  String? get commandId => _$this._commandId;
  set commandId(String? commandId) => _$this._commandId = commandId;

  String? _channelId;
  String? get channelId => _$this._channelId;
  set channelId(String? channelId) => _$this._channelId = channelId;

  String? _threadId;
  String? get threadId => _$this._threadId;
  set threadId(String? threadId) => _$this._threadId = threadId;

  String? _runId;
  String? get runId => _$this._runId;
  set runId(String? runId) => _$this._runId = runId;

  bool? _accepted;
  bool? get accepted => _$this._accepted;
  set accepted(bool? accepted) => _$this._accepted = accepted;

  CommandInvocationBuilder() {
    CommandInvocation._defaults(this);
  }

  CommandInvocationBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _invocationId = $v.invocationId;
      _commandId = $v.commandId;
      _channelId = $v.channelId;
      _threadId = $v.threadId;
      _runId = $v.runId;
      _accepted = $v.accepted;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(CommandInvocation other) {
    _$v = other as _$CommandInvocation;
  }

  @override
  void update(void Function(CommandInvocationBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  CommandInvocation build() => _build();

  _$CommandInvocation _build() {
    final _$result =
        _$v ??
        _$CommandInvocation._(
          invocationId: BuiltValueNullFieldError.checkNotNull(
            invocationId,
            r'CommandInvocation',
            'invocationId',
          ),
          commandId: BuiltValueNullFieldError.checkNotNull(
            commandId,
            r'CommandInvocation',
            'commandId',
          ),
          channelId: BuiltValueNullFieldError.checkNotNull(
            channelId,
            r'CommandInvocation',
            'channelId',
          ),
          threadId: threadId,
          runId: runId,
          accepted: BuiltValueNullFieldError.checkNotNull(
            accepted,
            r'CommandInvocation',
            'accepted',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
