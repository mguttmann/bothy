// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'stats_run_chat.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$StatsRunChat extends StatsRunChat {
  @override
  final String threadId;
  @override
  final String channelName;
  @override
  final String? threadTitle;

  factory _$StatsRunChat([void Function(StatsRunChatBuilder)? updates]) =>
      (StatsRunChatBuilder()..update(updates))._build();

  _$StatsRunChat._({
    required this.threadId,
    required this.channelName,
    this.threadTitle,
  }) : super._();
  @override
  StatsRunChat rebuild(void Function(StatsRunChatBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  StatsRunChatBuilder toBuilder() => StatsRunChatBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is StatsRunChat &&
        threadId == other.threadId &&
        channelName == other.channelName &&
        threadTitle == other.threadTitle;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, threadId.hashCode);
    _$hash = $jc(_$hash, channelName.hashCode);
    _$hash = $jc(_$hash, threadTitle.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'StatsRunChat')
          ..add('threadId', threadId)
          ..add('channelName', channelName)
          ..add('threadTitle', threadTitle))
        .toString();
  }
}

class StatsRunChatBuilder
    implements Builder<StatsRunChat, StatsRunChatBuilder> {
  _$StatsRunChat? _$v;

  String? _threadId;
  String? get threadId => _$this._threadId;
  set threadId(String? threadId) => _$this._threadId = threadId;

  String? _channelName;
  String? get channelName => _$this._channelName;
  set channelName(String? channelName) => _$this._channelName = channelName;

  String? _threadTitle;
  String? get threadTitle => _$this._threadTitle;
  set threadTitle(String? threadTitle) => _$this._threadTitle = threadTitle;

  StatsRunChatBuilder() {
    StatsRunChat._defaults(this);
  }

  StatsRunChatBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _threadId = $v.threadId;
      _channelName = $v.channelName;
      _threadTitle = $v.threadTitle;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(StatsRunChat other) {
    _$v = other as _$StatsRunChat;
  }

  @override
  void update(void Function(StatsRunChatBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  StatsRunChat build() => _build();

  _$StatsRunChat _build() {
    final _$result =
        _$v ??
        _$StatsRunChat._(
          threadId: BuiltValueNullFieldError.checkNotNull(
            threadId,
            r'StatsRunChat',
            'threadId',
          ),
          channelName: BuiltValueNullFieldError.checkNotNull(
            channelName,
            r'StatsRunChat',
            'channelName',
          ),
          threadTitle: threadTitle,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
