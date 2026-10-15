// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'run_context.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$RunContext extends RunContext {
  @override
  final String runId;
  @override
  final RunContextMessage triggerMessage;
  @override
  final BuiltList<RunContextMessage> threadContext;
  @override
  final BuiltList<RunContextMessage>? channelContext;
  @override
  final int contextFloorSeq;
  @override
  final int? truncatedBeforeSeq;

  factory _$RunContext([void Function(RunContextBuilder)? updates]) =>
      (RunContextBuilder()..update(updates))._build();

  _$RunContext._({
    required this.runId,
    required this.triggerMessage,
    required this.threadContext,
    this.channelContext,
    required this.contextFloorSeq,
    this.truncatedBeforeSeq,
  }) : super._();
  @override
  RunContext rebuild(void Function(RunContextBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  RunContextBuilder toBuilder() => RunContextBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is RunContext &&
        runId == other.runId &&
        triggerMessage == other.triggerMessage &&
        threadContext == other.threadContext &&
        channelContext == other.channelContext &&
        contextFloorSeq == other.contextFloorSeq &&
        truncatedBeforeSeq == other.truncatedBeforeSeq;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, runId.hashCode);
    _$hash = $jc(_$hash, triggerMessage.hashCode);
    _$hash = $jc(_$hash, threadContext.hashCode);
    _$hash = $jc(_$hash, channelContext.hashCode);
    _$hash = $jc(_$hash, contextFloorSeq.hashCode);
    _$hash = $jc(_$hash, truncatedBeforeSeq.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'RunContext')
          ..add('runId', runId)
          ..add('triggerMessage', triggerMessage)
          ..add('threadContext', threadContext)
          ..add('channelContext', channelContext)
          ..add('contextFloorSeq', contextFloorSeq)
          ..add('truncatedBeforeSeq', truncatedBeforeSeq))
        .toString();
  }
}

class RunContextBuilder implements Builder<RunContext, RunContextBuilder> {
  _$RunContext? _$v;

  String? _runId;
  String? get runId => _$this._runId;
  set runId(String? runId) => _$this._runId = runId;

  RunContextMessageBuilder? _triggerMessage;
  RunContextMessageBuilder get triggerMessage =>
      _$this._triggerMessage ??= RunContextMessageBuilder();
  set triggerMessage(RunContextMessageBuilder? triggerMessage) =>
      _$this._triggerMessage = triggerMessage;

  ListBuilder<RunContextMessage>? _threadContext;
  ListBuilder<RunContextMessage> get threadContext =>
      _$this._threadContext ??= ListBuilder<RunContextMessage>();
  set threadContext(ListBuilder<RunContextMessage>? threadContext) =>
      _$this._threadContext = threadContext;

  ListBuilder<RunContextMessage>? _channelContext;
  ListBuilder<RunContextMessage> get channelContext =>
      _$this._channelContext ??= ListBuilder<RunContextMessage>();
  set channelContext(ListBuilder<RunContextMessage>? channelContext) =>
      _$this._channelContext = channelContext;

  int? _contextFloorSeq;
  int? get contextFloorSeq => _$this._contextFloorSeq;
  set contextFloorSeq(int? contextFloorSeq) =>
      _$this._contextFloorSeq = contextFloorSeq;

  int? _truncatedBeforeSeq;
  int? get truncatedBeforeSeq => _$this._truncatedBeforeSeq;
  set truncatedBeforeSeq(int? truncatedBeforeSeq) =>
      _$this._truncatedBeforeSeq = truncatedBeforeSeq;

  RunContextBuilder() {
    RunContext._defaults(this);
  }

  RunContextBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _runId = $v.runId;
      _triggerMessage = $v.triggerMessage.toBuilder();
      _threadContext = $v.threadContext.toBuilder();
      _channelContext = $v.channelContext?.toBuilder();
      _contextFloorSeq = $v.contextFloorSeq;
      _truncatedBeforeSeq = $v.truncatedBeforeSeq;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(RunContext other) {
    _$v = other as _$RunContext;
  }

  @override
  void update(void Function(RunContextBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  RunContext build() => _build();

  _$RunContext _build() {
    _$RunContext _$result;
    try {
      _$result =
          _$v ??
          _$RunContext._(
            runId: BuiltValueNullFieldError.checkNotNull(
              runId,
              r'RunContext',
              'runId',
            ),
            triggerMessage: triggerMessage.build(),
            threadContext: threadContext.build(),
            channelContext: _channelContext?.build(),
            contextFloorSeq: BuiltValueNullFieldError.checkNotNull(
              contextFloorSeq,
              r'RunContext',
              'contextFloorSeq',
            ),
            truncatedBeforeSeq: truncatedBeforeSeq,
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'triggerMessage';
        triggerMessage.build();
        _$failedField = 'threadContext';
        threadContext.build();
        _$failedField = 'channelContext';
        _channelContext?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'RunContext',
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
