// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'read_state.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ReadState extends ReadState {
  @override
  final String threadId;
  @override
  final String channelId;
  @override
  final int lastReadSeq;
  @override
  final int mentionCount;
  @override
  final int version;
  @override
  final String? updatedByDeviceId;
  @override
  final DateTime? updatedAt;

  factory _$ReadState([void Function(ReadStateBuilder)? updates]) =>
      (ReadStateBuilder()..update(updates))._build();

  _$ReadState._({
    required this.threadId,
    required this.channelId,
    required this.lastReadSeq,
    required this.mentionCount,
    required this.version,
    this.updatedByDeviceId,
    this.updatedAt,
  }) : super._();
  @override
  ReadState rebuild(void Function(ReadStateBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ReadStateBuilder toBuilder() => ReadStateBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ReadState &&
        threadId == other.threadId &&
        channelId == other.channelId &&
        lastReadSeq == other.lastReadSeq &&
        mentionCount == other.mentionCount &&
        version == other.version &&
        updatedByDeviceId == other.updatedByDeviceId &&
        updatedAt == other.updatedAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, threadId.hashCode);
    _$hash = $jc(_$hash, channelId.hashCode);
    _$hash = $jc(_$hash, lastReadSeq.hashCode);
    _$hash = $jc(_$hash, mentionCount.hashCode);
    _$hash = $jc(_$hash, version.hashCode);
    _$hash = $jc(_$hash, updatedByDeviceId.hashCode);
    _$hash = $jc(_$hash, updatedAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ReadState')
          ..add('threadId', threadId)
          ..add('channelId', channelId)
          ..add('lastReadSeq', lastReadSeq)
          ..add('mentionCount', mentionCount)
          ..add('version', version)
          ..add('updatedByDeviceId', updatedByDeviceId)
          ..add('updatedAt', updatedAt))
        .toString();
  }
}

class ReadStateBuilder implements Builder<ReadState, ReadStateBuilder> {
  _$ReadState? _$v;

  String? _threadId;
  String? get threadId => _$this._threadId;
  set threadId(String? threadId) => _$this._threadId = threadId;

  String? _channelId;
  String? get channelId => _$this._channelId;
  set channelId(String? channelId) => _$this._channelId = channelId;

  int? _lastReadSeq;
  int? get lastReadSeq => _$this._lastReadSeq;
  set lastReadSeq(int? lastReadSeq) => _$this._lastReadSeq = lastReadSeq;

  int? _mentionCount;
  int? get mentionCount => _$this._mentionCount;
  set mentionCount(int? mentionCount) => _$this._mentionCount = mentionCount;

  int? _version;
  int? get version => _$this._version;
  set version(int? version) => _$this._version = version;

  String? _updatedByDeviceId;
  String? get updatedByDeviceId => _$this._updatedByDeviceId;
  set updatedByDeviceId(String? updatedByDeviceId) =>
      _$this._updatedByDeviceId = updatedByDeviceId;

  DateTime? _updatedAt;
  DateTime? get updatedAt => _$this._updatedAt;
  set updatedAt(DateTime? updatedAt) => _$this._updatedAt = updatedAt;

  ReadStateBuilder() {
    ReadState._defaults(this);
  }

  ReadStateBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _threadId = $v.threadId;
      _channelId = $v.channelId;
      _lastReadSeq = $v.lastReadSeq;
      _mentionCount = $v.mentionCount;
      _version = $v.version;
      _updatedByDeviceId = $v.updatedByDeviceId;
      _updatedAt = $v.updatedAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ReadState other) {
    _$v = other as _$ReadState;
  }

  @override
  void update(void Function(ReadStateBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ReadState build() => _build();

  _$ReadState _build() {
    final _$result =
        _$v ??
        _$ReadState._(
          threadId: BuiltValueNullFieldError.checkNotNull(
            threadId,
            r'ReadState',
            'threadId',
          ),
          channelId: BuiltValueNullFieldError.checkNotNull(
            channelId,
            r'ReadState',
            'channelId',
          ),
          lastReadSeq: BuiltValueNullFieldError.checkNotNull(
            lastReadSeq,
            r'ReadState',
            'lastReadSeq',
          ),
          mentionCount: BuiltValueNullFieldError.checkNotNull(
            mentionCount,
            r'ReadState',
            'mentionCount',
          ),
          version: BuiltValueNullFieldError.checkNotNull(
            version,
            r'ReadState',
            'version',
          ),
          updatedByDeviceId: updatedByDeviceId,
          updatedAt: updatedAt,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
