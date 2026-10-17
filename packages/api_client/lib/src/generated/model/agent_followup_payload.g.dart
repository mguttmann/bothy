// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'agent_followup_payload.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$AgentFollowupPayload extends AgentFollowupPayload {
  @override
  final String runId;
  @override
  final String threadId;
  @override
  final String messageId;

  factory _$AgentFollowupPayload([
    void Function(AgentFollowupPayloadBuilder)? updates,
  ]) => (AgentFollowupPayloadBuilder()..update(updates))._build();

  _$AgentFollowupPayload._({
    required this.runId,
    required this.threadId,
    required this.messageId,
  }) : super._();
  @override
  AgentFollowupPayload rebuild(
    void Function(AgentFollowupPayloadBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  AgentFollowupPayloadBuilder toBuilder() =>
      AgentFollowupPayloadBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AgentFollowupPayload &&
        runId == other.runId &&
        threadId == other.threadId &&
        messageId == other.messageId;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, runId.hashCode);
    _$hash = $jc(_$hash, threadId.hashCode);
    _$hash = $jc(_$hash, messageId.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AgentFollowupPayload')
          ..add('runId', runId)
          ..add('threadId', threadId)
          ..add('messageId', messageId))
        .toString();
  }
}

class AgentFollowupPayloadBuilder
    implements Builder<AgentFollowupPayload, AgentFollowupPayloadBuilder> {
  _$AgentFollowupPayload? _$v;

  String? _runId;
  String? get runId => _$this._runId;
  set runId(String? runId) => _$this._runId = runId;

  String? _threadId;
  String? get threadId => _$this._threadId;
  set threadId(String? threadId) => _$this._threadId = threadId;

  String? _messageId;
  String? get messageId => _$this._messageId;
  set messageId(String? messageId) => _$this._messageId = messageId;

  AgentFollowupPayloadBuilder() {
    AgentFollowupPayload._defaults(this);
  }

  AgentFollowupPayloadBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _runId = $v.runId;
      _threadId = $v.threadId;
      _messageId = $v.messageId;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AgentFollowupPayload other) {
    _$v = other as _$AgentFollowupPayload;
  }

  @override
  void update(void Function(AgentFollowupPayloadBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AgentFollowupPayload build() => _build();

  _$AgentFollowupPayload _build() {
    final _$result =
        _$v ??
        _$AgentFollowupPayload._(
          runId: BuiltValueNullFieldError.checkNotNull(
            runId,
            r'AgentFollowupPayload',
            'runId',
          ),
          threadId: BuiltValueNullFieldError.checkNotNull(
            threadId,
            r'AgentFollowupPayload',
            'threadId',
          ),
          messageId: BuiltValueNullFieldError.checkNotNull(
            messageId,
            r'AgentFollowupPayload',
            'messageId',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
