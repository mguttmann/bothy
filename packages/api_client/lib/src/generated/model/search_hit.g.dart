// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'search_hit.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$SearchHit extends SearchHit {
  @override
  final SearchHitKind kind;
  @override
  final double? score;
  @override
  final Message? message;
  @override
  final Thread? thread;
  @override
  final File? file;
  @override
  final String? threadTitle;
  @override
  final String? channelName;
  @override
  final String? spaceName;
  @override
  final BuiltList<SearchHitSegmentsInner> segments;

  factory _$SearchHit([void Function(SearchHitBuilder)? updates]) =>
      (SearchHitBuilder()..update(updates))._build();

  _$SearchHit._({
    required this.kind,
    this.score,
    this.message,
    this.thread,
    this.file,
    this.threadTitle,
    this.channelName,
    this.spaceName,
    required this.segments,
  }) : super._();
  @override
  SearchHit rebuild(void Function(SearchHitBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  SearchHitBuilder toBuilder() => SearchHitBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is SearchHit &&
        kind == other.kind &&
        score == other.score &&
        message == other.message &&
        thread == other.thread &&
        file == other.file &&
        threadTitle == other.threadTitle &&
        channelName == other.channelName &&
        spaceName == other.spaceName &&
        segments == other.segments;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, kind.hashCode);
    _$hash = $jc(_$hash, score.hashCode);
    _$hash = $jc(_$hash, message.hashCode);
    _$hash = $jc(_$hash, thread.hashCode);
    _$hash = $jc(_$hash, file.hashCode);
    _$hash = $jc(_$hash, threadTitle.hashCode);
    _$hash = $jc(_$hash, channelName.hashCode);
    _$hash = $jc(_$hash, spaceName.hashCode);
    _$hash = $jc(_$hash, segments.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'SearchHit')
          ..add('kind', kind)
          ..add('score', score)
          ..add('message', message)
          ..add('thread', thread)
          ..add('file', file)
          ..add('threadTitle', threadTitle)
          ..add('channelName', channelName)
          ..add('spaceName', spaceName)
          ..add('segments', segments))
        .toString();
  }
}

class SearchHitBuilder implements Builder<SearchHit, SearchHitBuilder> {
  _$SearchHit? _$v;

  SearchHitKind? _kind;
  SearchHitKind? get kind => _$this._kind;
  set kind(SearchHitKind? kind) => _$this._kind = kind;

  double? _score;
  double? get score => _$this._score;
  set score(double? score) => _$this._score = score;

  MessageBuilder? _message;
  MessageBuilder get message => _$this._message ??= MessageBuilder();
  set message(MessageBuilder? message) => _$this._message = message;

  ThreadBuilder? _thread;
  ThreadBuilder get thread => _$this._thread ??= ThreadBuilder();
  set thread(ThreadBuilder? thread) => _$this._thread = thread;

  FileBuilder? _file;
  FileBuilder get file => _$this._file ??= FileBuilder();
  set file(FileBuilder? file) => _$this._file = file;

  String? _threadTitle;
  String? get threadTitle => _$this._threadTitle;
  set threadTitle(String? threadTitle) => _$this._threadTitle = threadTitle;

  String? _channelName;
  String? get channelName => _$this._channelName;
  set channelName(String? channelName) => _$this._channelName = channelName;

  String? _spaceName;
  String? get spaceName => _$this._spaceName;
  set spaceName(String? spaceName) => _$this._spaceName = spaceName;

  ListBuilder<SearchHitSegmentsInner>? _segments;
  ListBuilder<SearchHitSegmentsInner> get segments =>
      _$this._segments ??= ListBuilder<SearchHitSegmentsInner>();
  set segments(ListBuilder<SearchHitSegmentsInner>? segments) =>
      _$this._segments = segments;

  SearchHitBuilder() {
    SearchHit._defaults(this);
  }

  SearchHitBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _kind = $v.kind;
      _score = $v.score;
      _message = $v.message?.toBuilder();
      _thread = $v.thread?.toBuilder();
      _file = $v.file?.toBuilder();
      _threadTitle = $v.threadTitle;
      _channelName = $v.channelName;
      _spaceName = $v.spaceName;
      _segments = $v.segments.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(SearchHit other) {
    _$v = other as _$SearchHit;
  }

  @override
  void update(void Function(SearchHitBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  SearchHit build() => _build();

  _$SearchHit _build() {
    _$SearchHit _$result;
    try {
      _$result =
          _$v ??
          _$SearchHit._(
            kind: BuiltValueNullFieldError.checkNotNull(
              kind,
              r'SearchHit',
              'kind',
            ),
            score: score,
            message: _message?.build(),
            thread: _thread?.build(),
            file: _file?.build(),
            threadTitle: threadTitle,
            channelName: channelName,
            spaceName: spaceName,
            segments: segments.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'message';
        _message?.build();
        _$failedField = 'thread';
        _thread?.build();
        _$failedField = 'file';
        _file?.build();

        _$failedField = 'segments';
        segments.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'SearchHit',
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
