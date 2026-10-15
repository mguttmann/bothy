// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'command_invoked_payload.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$CommandInvokedPayload extends CommandInvokedPayload {
  @override
  final String invocationId;
  @override
  final String commandId;
  @override
  final String commandName;
  @override
  final bool control;
  @override
  final String channelId;
  @override
  final String? threadId;
  @override
  final String? runId;
  @override
  final PrincipalRef invokedBy;
  @override
  final BuiltList<CommandOptionValue> options;

  factory _$CommandInvokedPayload([
    void Function(CommandInvokedPayloadBuilder)? updates,
  ]) => (CommandInvokedPayloadBuilder()..update(updates))._build();

  _$CommandInvokedPayload._({
    required this.invocationId,
    required this.commandId,
    required this.commandName,
    required this.control,
    required this.channelId,
    this.threadId,
    this.runId,
    required this.invokedBy,
    required this.options,
  }) : super._();
  @override
  CommandInvokedPayload rebuild(
    void Function(CommandInvokedPayloadBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  CommandInvokedPayloadBuilder toBuilder() =>
      CommandInvokedPayloadBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is CommandInvokedPayload &&
        invocationId == other.invocationId &&
        commandId == other.commandId &&
        commandName == other.commandName &&
        control == other.control &&
        channelId == other.channelId &&
        threadId == other.threadId &&
        runId == other.runId &&
        invokedBy == other.invokedBy &&
        options == other.options;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, invocationId.hashCode);
    _$hash = $jc(_$hash, commandId.hashCode);
    _$hash = $jc(_$hash, commandName.hashCode);
    _$hash = $jc(_$hash, control.hashCode);
    _$hash = $jc(_$hash, channelId.hashCode);
    _$hash = $jc(_$hash, threadId.hashCode);
    _$hash = $jc(_$hash, runId.hashCode);
    _$hash = $jc(_$hash, invokedBy.hashCode);
    _$hash = $jc(_$hash, options.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'CommandInvokedPayload')
          ..add('invocationId', invocationId)
          ..add('commandId', commandId)
          ..add('commandName', commandName)
          ..add('control', control)
          ..add('channelId', channelId)
          ..add('threadId', threadId)
          ..add('runId', runId)
          ..add('invokedBy', invokedBy)
          ..add('options', options))
        .toString();
  }
}

class CommandInvokedPayloadBuilder
    implements Builder<CommandInvokedPayload, CommandInvokedPayloadBuilder> {
  _$CommandInvokedPayload? _$v;

  String? _invocationId;
  String? get invocationId => _$this._invocationId;
  set invocationId(String? invocationId) => _$this._invocationId = invocationId;

  String? _commandId;
  String? get commandId => _$this._commandId;
  set commandId(String? commandId) => _$this._commandId = commandId;

  String? _commandName;
  String? get commandName => _$this._commandName;
  set commandName(String? commandName) => _$this._commandName = commandName;

  bool? _control;
  bool? get control => _$this._control;
  set control(bool? control) => _$this._control = control;

  String? _channelId;
  String? get channelId => _$this._channelId;
  set channelId(String? channelId) => _$this._channelId = channelId;

  String? _threadId;
  String? get threadId => _$this._threadId;
  set threadId(String? threadId) => _$this._threadId = threadId;

  String? _runId;
  String? get runId => _$this._runId;
  set runId(String? runId) => _$this._runId = runId;

  PrincipalRefBuilder? _invokedBy;
  PrincipalRefBuilder get invokedBy =>
      _$this._invokedBy ??= PrincipalRefBuilder();
  set invokedBy(PrincipalRefBuilder? invokedBy) =>
      _$this._invokedBy = invokedBy;

  ListBuilder<CommandOptionValue>? _options;
  ListBuilder<CommandOptionValue> get options =>
      _$this._options ??= ListBuilder<CommandOptionValue>();
  set options(ListBuilder<CommandOptionValue>? options) =>
      _$this._options = options;

  CommandInvokedPayloadBuilder() {
    CommandInvokedPayload._defaults(this);
  }

  CommandInvokedPayloadBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _invocationId = $v.invocationId;
      _commandId = $v.commandId;
      _commandName = $v.commandName;
      _control = $v.control;
      _channelId = $v.channelId;
      _threadId = $v.threadId;
      _runId = $v.runId;
      _invokedBy = $v.invokedBy.toBuilder();
      _options = $v.options.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(CommandInvokedPayload other) {
    _$v = other as _$CommandInvokedPayload;
  }

  @override
  void update(void Function(CommandInvokedPayloadBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  CommandInvokedPayload build() => _build();

  _$CommandInvokedPayload _build() {
    _$CommandInvokedPayload _$result;
    try {
      _$result =
          _$v ??
          _$CommandInvokedPayload._(
            invocationId: BuiltValueNullFieldError.checkNotNull(
              invocationId,
              r'CommandInvokedPayload',
              'invocationId',
            ),
            commandId: BuiltValueNullFieldError.checkNotNull(
              commandId,
              r'CommandInvokedPayload',
              'commandId',
            ),
            commandName: BuiltValueNullFieldError.checkNotNull(
              commandName,
              r'CommandInvokedPayload',
              'commandName',
            ),
            control: BuiltValueNullFieldError.checkNotNull(
              control,
              r'CommandInvokedPayload',
              'control',
            ),
            channelId: BuiltValueNullFieldError.checkNotNull(
              channelId,
              r'CommandInvokedPayload',
              'channelId',
            ),
            threadId: threadId,
            runId: runId,
            invokedBy: invokedBy.build(),
            options: options.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'invokedBy';
        invokedBy.build();
        _$failedField = 'options';
        options.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'CommandInvokedPayload',
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
