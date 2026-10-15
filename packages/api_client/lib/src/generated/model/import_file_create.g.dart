// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'import_file_create.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ImportFileCreate extends ImportFileCreate {
  @override
  final String originKey;
  @override
  final String channelId;
  @override
  final String authorId;
  @override
  final String name;
  @override
  final int sizeBytes;
  @override
  final String? declaredMimeType;

  factory _$ImportFileCreate([
    void Function(ImportFileCreateBuilder)? updates,
  ]) => (ImportFileCreateBuilder()..update(updates))._build();

  _$ImportFileCreate._({
    required this.originKey,
    required this.channelId,
    required this.authorId,
    required this.name,
    required this.sizeBytes,
    this.declaredMimeType,
  }) : super._();
  @override
  ImportFileCreate rebuild(void Function(ImportFileCreateBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ImportFileCreateBuilder toBuilder() =>
      ImportFileCreateBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ImportFileCreate &&
        originKey == other.originKey &&
        channelId == other.channelId &&
        authorId == other.authorId &&
        name == other.name &&
        sizeBytes == other.sizeBytes &&
        declaredMimeType == other.declaredMimeType;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, originKey.hashCode);
    _$hash = $jc(_$hash, channelId.hashCode);
    _$hash = $jc(_$hash, authorId.hashCode);
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jc(_$hash, sizeBytes.hashCode);
    _$hash = $jc(_$hash, declaredMimeType.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ImportFileCreate')
          ..add('originKey', originKey)
          ..add('channelId', channelId)
          ..add('authorId', authorId)
          ..add('name', name)
          ..add('sizeBytes', sizeBytes)
          ..add('declaredMimeType', declaredMimeType))
        .toString();
  }
}

class ImportFileCreateBuilder
    implements Builder<ImportFileCreate, ImportFileCreateBuilder> {
  _$ImportFileCreate? _$v;

  String? _originKey;
  String? get originKey => _$this._originKey;
  set originKey(String? originKey) => _$this._originKey = originKey;

  String? _channelId;
  String? get channelId => _$this._channelId;
  set channelId(String? channelId) => _$this._channelId = channelId;

  String? _authorId;
  String? get authorId => _$this._authorId;
  set authorId(String? authorId) => _$this._authorId = authorId;

  String? _name;
  String? get name => _$this._name;
  set name(String? name) => _$this._name = name;

  int? _sizeBytes;
  int? get sizeBytes => _$this._sizeBytes;
  set sizeBytes(int? sizeBytes) => _$this._sizeBytes = sizeBytes;

  String? _declaredMimeType;
  String? get declaredMimeType => _$this._declaredMimeType;
  set declaredMimeType(String? declaredMimeType) =>
      _$this._declaredMimeType = declaredMimeType;

  ImportFileCreateBuilder() {
    ImportFileCreate._defaults(this);
  }

  ImportFileCreateBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _originKey = $v.originKey;
      _channelId = $v.channelId;
      _authorId = $v.authorId;
      _name = $v.name;
      _sizeBytes = $v.sizeBytes;
      _declaredMimeType = $v.declaredMimeType;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ImportFileCreate other) {
    _$v = other as _$ImportFileCreate;
  }

  @override
  void update(void Function(ImportFileCreateBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ImportFileCreate build() => _build();

  _$ImportFileCreate _build() {
    final _$result =
        _$v ??
        _$ImportFileCreate._(
          originKey: BuiltValueNullFieldError.checkNotNull(
            originKey,
            r'ImportFileCreate',
            'originKey',
          ),
          channelId: BuiltValueNullFieldError.checkNotNull(
            channelId,
            r'ImportFileCreate',
            'channelId',
          ),
          authorId: BuiltValueNullFieldError.checkNotNull(
            authorId,
            r'ImportFileCreate',
            'authorId',
          ),
          name: BuiltValueNullFieldError.checkNotNull(
            name,
            r'ImportFileCreate',
            'name',
          ),
          sizeBytes: BuiltValueNullFieldError.checkNotNull(
            sizeBytes,
            r'ImportFileCreate',
            'sizeBytes',
          ),
          declaredMimeType: declaredMimeType,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
