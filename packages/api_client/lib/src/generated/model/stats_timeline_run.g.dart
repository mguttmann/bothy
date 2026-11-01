// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'stats_timeline_run.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$StatsTimelineRun extends StatsTimelineRun {
  @override
  final String runId;
  @override
  final String channelId;
  @override
  final String agentId;
  @override
  final DateTime queuedAt;
  @override
  final DateTime? startedAt;
  @override
  final DateTime? finishedAt;
  @override
  final RunState state;
  @override
  final StatsRunChat? chat;

  factory _$StatsTimelineRun([
    void Function(StatsTimelineRunBuilder)? updates,
  ]) => (StatsTimelineRunBuilder()..update(updates))._build();

  _$StatsTimelineRun._({
    required this.runId,
    required this.channelId,
    required this.agentId,
    required this.queuedAt,
    this.startedAt,
    this.finishedAt,
    required this.state,
    this.chat,
  }) : super._();
  @override
  StatsTimelineRun rebuild(void Function(StatsTimelineRunBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  StatsTimelineRunBuilder toBuilder() =>
      StatsTimelineRunBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is StatsTimelineRun &&
        runId == other.runId &&
        channelId == other.channelId &&
        agentId == other.agentId &&
        queuedAt == other.queuedAt &&
        startedAt == other.startedAt &&
        finishedAt == other.finishedAt &&
        state == other.state &&
        chat == other.chat;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, runId.hashCode);
    _$hash = $jc(_$hash, channelId.hashCode);
    _$hash = $jc(_$hash, agentId.hashCode);
    _$hash = $jc(_$hash, queuedAt.hashCode);
    _$hash = $jc(_$hash, startedAt.hashCode);
    _$hash = $jc(_$hash, finishedAt.hashCode);
    _$hash = $jc(_$hash, state.hashCode);
    _$hash = $jc(_$hash, chat.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'StatsTimelineRun')
          ..add('runId', runId)
          ..add('channelId', channelId)
          ..add('agentId', agentId)
          ..add('queuedAt', queuedAt)
          ..add('startedAt', startedAt)
          ..add('finishedAt', finishedAt)
          ..add('state', state)
          ..add('chat', chat))
        .toString();
  }
}

class StatsTimelineRunBuilder
    implements Builder<StatsTimelineRun, StatsTimelineRunBuilder> {
  _$StatsTimelineRun? _$v;

  String? _runId;
  String? get runId => _$this._runId;
  set runId(String? runId) => _$this._runId = runId;

  String? _channelId;
  String? get channelId => _$this._channelId;
  set channelId(String? channelId) => _$this._channelId = channelId;

  String? _agentId;
  String? get agentId => _$this._agentId;
  set agentId(String? agentId) => _$this._agentId = agentId;

  DateTime? _queuedAt;
  DateTime? get queuedAt => _$this._queuedAt;
  set queuedAt(DateTime? queuedAt) => _$this._queuedAt = queuedAt;

  DateTime? _startedAt;
  DateTime? get startedAt => _$this._startedAt;
  set startedAt(DateTime? startedAt) => _$this._startedAt = startedAt;

  DateTime? _finishedAt;
  DateTime? get finishedAt => _$this._finishedAt;
  set finishedAt(DateTime? finishedAt) => _$this._finishedAt = finishedAt;

  RunState? _state;
  RunState? get state => _$this._state;
  set state(RunState? state) => _$this._state = state;

  StatsRunChatBuilder? _chat;
  StatsRunChatBuilder get chat => _$this._chat ??= StatsRunChatBuilder();
  set chat(StatsRunChatBuilder? chat) => _$this._chat = chat;

  StatsTimelineRunBuilder() {
    StatsTimelineRun._defaults(this);
  }

  StatsTimelineRunBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _runId = $v.runId;
      _channelId = $v.channelId;
      _agentId = $v.agentId;
      _queuedAt = $v.queuedAt;
      _startedAt = $v.startedAt;
      _finishedAt = $v.finishedAt;
      _state = $v.state;
      _chat = $v.chat?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(StatsTimelineRun other) {
    _$v = other as _$StatsTimelineRun;
  }

  @override
  void update(void Function(StatsTimelineRunBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  StatsTimelineRun build() => _build();

  _$StatsTimelineRun _build() {
    _$StatsTimelineRun _$result;
    try {
      _$result =
          _$v ??
          _$StatsTimelineRun._(
            runId: BuiltValueNullFieldError.checkNotNull(
              runId,
              r'StatsTimelineRun',
              'runId',
            ),
            channelId: BuiltValueNullFieldError.checkNotNull(
              channelId,
              r'StatsTimelineRun',
              'channelId',
            ),
            agentId: BuiltValueNullFieldError.checkNotNull(
              agentId,
              r'StatsTimelineRun',
              'agentId',
            ),
            queuedAt: BuiltValueNullFieldError.checkNotNull(
              queuedAt,
              r'StatsTimelineRun',
              'queuedAt',
            ),
            startedAt: startedAt,
            finishedAt: finishedAt,
            state: BuiltValueNullFieldError.checkNotNull(
              state,
              r'StatsTimelineRun',
              'state',
            ),
            chat: _chat?.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'chat';
        _chat?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'StatsTimelineRun',
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
