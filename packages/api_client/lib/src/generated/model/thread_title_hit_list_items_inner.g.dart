// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'thread_title_hit_list_items_inner.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ThreadTitleHitListItemsInner extends ThreadTitleHitListItemsInner {
  @override
  final String threadId;
  @override
  final String channelId;
  @override
  final String title;
  @override
  final DateTime lastActivityAt;

  factory _$ThreadTitleHitListItemsInner([
    void Function(ThreadTitleHitListItemsInnerBuilder)? updates,
  ]) => (ThreadTitleHitListItemsInnerBuilder()..update(updates))._build();

  _$ThreadTitleHitListItemsInner._({
    required this.threadId,
    required this.channelId,
    required this.title,
    required this.lastActivityAt,
  }) : super._();
  @override
  ThreadTitleHitListItemsInner rebuild(
    void Function(ThreadTitleHitListItemsInnerBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  ThreadTitleHitListItemsInnerBuilder toBuilder() =>
      ThreadTitleHitListItemsInnerBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ThreadTitleHitListItemsInner &&
        threadId == other.threadId &&
        channelId == other.channelId &&
        title == other.title &&
        lastActivityAt == other.lastActivityAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, threadId.hashCode);
    _$hash = $jc(_$hash, channelId.hashCode);
    _$hash = $jc(_$hash, title.hashCode);
    _$hash = $jc(_$hash, lastActivityAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ThreadTitleHitListItemsInner')
          ..add('threadId', threadId)
          ..add('channelId', channelId)
          ..add('title', title)
          ..add('lastActivityAt', lastActivityAt))
        .toString();
  }
}

class ThreadTitleHitListItemsInnerBuilder
    implements
        Builder<
          ThreadTitleHitListItemsInner,
          ThreadTitleHitListItemsInnerBuilder
        > {
  _$ThreadTitleHitListItemsInner? _$v;

  String? _threadId;
  String? get threadId => _$this._threadId;
  set threadId(String? threadId) => _$this._threadId = threadId;

  String? _channelId;
  String? get channelId => _$this._channelId;
  set channelId(String? channelId) => _$this._channelId = channelId;

  String? _title;
  String? get title => _$this._title;
  set title(String? title) => _$this._title = title;

  DateTime? _lastActivityAt;
  DateTime? get lastActivityAt => _$this._lastActivityAt;
  set lastActivityAt(DateTime? lastActivityAt) =>
      _$this._lastActivityAt = lastActivityAt;

  ThreadTitleHitListItemsInnerBuilder() {
    ThreadTitleHitListItemsInner._defaults(this);
  }

  ThreadTitleHitListItemsInnerBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _threadId = $v.threadId;
      _channelId = $v.channelId;
      _title = $v.title;
      _lastActivityAt = $v.lastActivityAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ThreadTitleHitListItemsInner other) {
    _$v = other as _$ThreadTitleHitListItemsInner;
  }

  @override
  void update(void Function(ThreadTitleHitListItemsInnerBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ThreadTitleHitListItemsInner build() => _build();

  _$ThreadTitleHitListItemsInner _build() {
    final _$result =
        _$v ??
        _$ThreadTitleHitListItemsInner._(
          threadId: BuiltValueNullFieldError.checkNotNull(
            threadId,
            r'ThreadTitleHitListItemsInner',
            'threadId',
          ),
          channelId: BuiltValueNullFieldError.checkNotNull(
            channelId,
            r'ThreadTitleHitListItemsInner',
            'channelId',
          ),
          title: BuiltValueNullFieldError.checkNotNull(
            title,
            r'ThreadTitleHitListItemsInner',
            'title',
          ),
          lastActivityAt: BuiltValueNullFieldError.checkNotNull(
            lastActivityAt,
            r'ThreadTitleHitListItemsInner',
            'lastActivityAt',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
