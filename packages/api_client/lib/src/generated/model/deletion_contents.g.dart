// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'deletion_contents.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$DeletionContents extends DeletionContents {
  @override
  final int channels;
  @override
  final int threads;
  @override
  final int messages;
  @override
  final int files;
  @override
  final int sessions;

  factory _$DeletionContents([
    void Function(DeletionContentsBuilder)? updates,
  ]) => (DeletionContentsBuilder()..update(updates))._build();

  _$DeletionContents._({
    required this.channels,
    required this.threads,
    required this.messages,
    required this.files,
    required this.sessions,
  }) : super._();
  @override
  DeletionContents rebuild(void Function(DeletionContentsBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  DeletionContentsBuilder toBuilder() =>
      DeletionContentsBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is DeletionContents &&
        channels == other.channels &&
        threads == other.threads &&
        messages == other.messages &&
        files == other.files &&
        sessions == other.sessions;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, channels.hashCode);
    _$hash = $jc(_$hash, threads.hashCode);
    _$hash = $jc(_$hash, messages.hashCode);
    _$hash = $jc(_$hash, files.hashCode);
    _$hash = $jc(_$hash, sessions.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'DeletionContents')
          ..add('channels', channels)
          ..add('threads', threads)
          ..add('messages', messages)
          ..add('files', files)
          ..add('sessions', sessions))
        .toString();
  }
}

class DeletionContentsBuilder
    implements Builder<DeletionContents, DeletionContentsBuilder> {
  _$DeletionContents? _$v;

  int? _channels;
  int? get channels => _$this._channels;
  set channels(int? channels) => _$this._channels = channels;

  int? _threads;
  int? get threads => _$this._threads;
  set threads(int? threads) => _$this._threads = threads;

  int? _messages;
  int? get messages => _$this._messages;
  set messages(int? messages) => _$this._messages = messages;

  int? _files;
  int? get files => _$this._files;
  set files(int? files) => _$this._files = files;

  int? _sessions;
  int? get sessions => _$this._sessions;
  set sessions(int? sessions) => _$this._sessions = sessions;

  DeletionContentsBuilder() {
    DeletionContents._defaults(this);
  }

  DeletionContentsBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _channels = $v.channels;
      _threads = $v.threads;
      _messages = $v.messages;
      _files = $v.files;
      _sessions = $v.sessions;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(DeletionContents other) {
    _$v = other as _$DeletionContents;
  }

  @override
  void update(void Function(DeletionContentsBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  DeletionContents build() => _build();

  _$DeletionContents _build() {
    final _$result =
        _$v ??
        _$DeletionContents._(
          channels: BuiltValueNullFieldError.checkNotNull(
            channels,
            r'DeletionContents',
            'channels',
          ),
          threads: BuiltValueNullFieldError.checkNotNull(
            threads,
            r'DeletionContents',
            'threads',
          ),
          messages: BuiltValueNullFieldError.checkNotNull(
            messages,
            r'DeletionContents',
            'messages',
          ),
          files: BuiltValueNullFieldError.checkNotNull(
            files,
            r'DeletionContents',
            'files',
          ),
          sessions: BuiltValueNullFieldError.checkNotNull(
            sessions,
            r'DeletionContents',
            'sessions',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
