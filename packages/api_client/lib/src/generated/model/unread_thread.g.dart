// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'unread_thread.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$UnreadThread extends UnreadThread {
  @override
  final String threadId;
  @override
  final String channelId;
  @override
  final int unreadCount;
  @override
  final int mentionCount;
  @override
  final int lastMessageSeq;
  @override
  final int lastReadSeq;

  factory _$UnreadThread([void Function(UnreadThreadBuilder)? updates]) =>
      (UnreadThreadBuilder()..update(updates))._build();

  _$UnreadThread._({
    required this.threadId,
    required this.channelId,
    required this.unreadCount,
    required this.mentionCount,
    required this.lastMessageSeq,
    required this.lastReadSeq,
  }) : super._();
  @override
  UnreadThread rebuild(void Function(UnreadThreadBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  UnreadThreadBuilder toBuilder() => UnreadThreadBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is UnreadThread &&
        threadId == other.threadId &&
        channelId == other.channelId &&
        unreadCount == other.unreadCount &&
        mentionCount == other.mentionCount &&
        lastMessageSeq == other.lastMessageSeq &&
        lastReadSeq == other.lastReadSeq;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, threadId.hashCode);
    _$hash = $jc(_$hash, channelId.hashCode);
    _$hash = $jc(_$hash, unreadCount.hashCode);
    _$hash = $jc(_$hash, mentionCount.hashCode);
    _$hash = $jc(_$hash, lastMessageSeq.hashCode);
    _$hash = $jc(_$hash, lastReadSeq.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'UnreadThread')
          ..add('threadId', threadId)
          ..add('channelId', channelId)
          ..add('unreadCount', unreadCount)
          ..add('mentionCount', mentionCount)
          ..add('lastMessageSeq', lastMessageSeq)
          ..add('lastReadSeq', lastReadSeq))
        .toString();
  }
}

class UnreadThreadBuilder
    implements Builder<UnreadThread, UnreadThreadBuilder> {
  _$UnreadThread? _$v;

  String? _threadId;
  String? get threadId => _$this._threadId;
  set threadId(String? threadId) => _$this._threadId = threadId;

  String? _channelId;
  String? get channelId => _$this._channelId;
  set channelId(String? channelId) => _$this._channelId = channelId;

  int? _unreadCount;
  int? get unreadCount => _$this._unreadCount;
  set unreadCount(int? unreadCount) => _$this._unreadCount = unreadCount;

  int? _mentionCount;
  int? get mentionCount => _$this._mentionCount;
  set mentionCount(int? mentionCount) => _$this._mentionCount = mentionCount;

  int? _lastMessageSeq;
  int? get lastMessageSeq => _$this._lastMessageSeq;
  set lastMessageSeq(int? lastMessageSeq) =>
      _$this._lastMessageSeq = lastMessageSeq;

  int? _lastReadSeq;
  int? get lastReadSeq => _$this._lastReadSeq;
  set lastReadSeq(int? lastReadSeq) => _$this._lastReadSeq = lastReadSeq;

  UnreadThreadBuilder() {
    UnreadThread._defaults(this);
  }

  UnreadThreadBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _threadId = $v.threadId;
      _channelId = $v.channelId;
      _unreadCount = $v.unreadCount;
      _mentionCount = $v.mentionCount;
      _lastMessageSeq = $v.lastMessageSeq;
      _lastReadSeq = $v.lastReadSeq;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(UnreadThread other) {
    _$v = other as _$UnreadThread;
  }

  @override
  void update(void Function(UnreadThreadBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  UnreadThread build() => _build();

  _$UnreadThread _build() {
    final _$result =
        _$v ??
        _$UnreadThread._(
          threadId: BuiltValueNullFieldError.checkNotNull(
            threadId,
            r'UnreadThread',
            'threadId',
          ),
          channelId: BuiltValueNullFieldError.checkNotNull(
            channelId,
            r'UnreadThread',
            'channelId',
          ),
          unreadCount: BuiltValueNullFieldError.checkNotNull(
            unreadCount,
            r'UnreadThread',
            'unreadCount',
          ),
          mentionCount: BuiltValueNullFieldError.checkNotNull(
            mentionCount,
            r'UnreadThread',
            'mentionCount',
          ),
          lastMessageSeq: BuiltValueNullFieldError.checkNotNull(
            lastMessageSeq,
            r'UnreadThread',
            'lastMessageSeq',
          ),
          lastReadSeq: BuiltValueNullFieldError.checkNotNull(
            lastReadSeq,
            r'UnreadThread',
            'lastReadSeq',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
