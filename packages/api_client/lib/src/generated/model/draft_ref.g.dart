// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'draft_ref.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$DraftRef extends DraftRef {
  @override
  final String id;
  @override
  final String? threadId;
  @override
  final String channelId;
  @override
  final int version;

  factory _$DraftRef([void Function(DraftRefBuilder)? updates]) =>
      (DraftRefBuilder()..update(updates))._build();

  _$DraftRef._({
    required this.id,
    this.threadId,
    required this.channelId,
    required this.version,
  }) : super._();
  @override
  DraftRef rebuild(void Function(DraftRefBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  DraftRefBuilder toBuilder() => DraftRefBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is DraftRef &&
        id == other.id &&
        threadId == other.threadId &&
        channelId == other.channelId &&
        version == other.version;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, id.hashCode);
    _$hash = $jc(_$hash, threadId.hashCode);
    _$hash = $jc(_$hash, channelId.hashCode);
    _$hash = $jc(_$hash, version.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'DraftRef')
          ..add('id', id)
          ..add('threadId', threadId)
          ..add('channelId', channelId)
          ..add('version', version))
        .toString();
  }
}

class DraftRefBuilder implements Builder<DraftRef, DraftRefBuilder> {
  _$DraftRef? _$v;

  String? _id;
  String? get id => _$this._id;
  set id(String? id) => _$this._id = id;

  String? _threadId;
  String? get threadId => _$this._threadId;
  set threadId(String? threadId) => _$this._threadId = threadId;

  String? _channelId;
  String? get channelId => _$this._channelId;
  set channelId(String? channelId) => _$this._channelId = channelId;

  int? _version;
  int? get version => _$this._version;
  set version(int? version) => _$this._version = version;

  DraftRefBuilder() {
    DraftRef._defaults(this);
  }

  DraftRefBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _id = $v.id;
      _threadId = $v.threadId;
      _channelId = $v.channelId;
      _version = $v.version;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(DraftRef other) {
    _$v = other as _$DraftRef;
  }

  @override
  void update(void Function(DraftRefBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  DraftRef build() => _build();

  _$DraftRef _build() {
    final _$result =
        _$v ??
        _$DraftRef._(
          id: BuiltValueNullFieldError.checkNotNull(id, r'DraftRef', 'id'),
          threadId: threadId,
          channelId: BuiltValueNullFieldError.checkNotNull(
            channelId,
            r'DraftRef',
            'channelId',
          ),
          version: BuiltValueNullFieldError.checkNotNull(
            version,
            r'DraftRef',
            'version',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
