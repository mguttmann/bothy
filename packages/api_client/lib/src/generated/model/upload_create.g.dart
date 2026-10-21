// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'upload_create.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$UploadCreate extends UploadCreate {
  @override
  final String name;
  @override
  final int sizeBytes;
  @override
  final String? declaredMimeType;
  @override
  final String sha256;

  factory _$UploadCreate([void Function(UploadCreateBuilder)? updates]) =>
      (UploadCreateBuilder()..update(updates))._build();

  _$UploadCreate._({
    required this.name,
    required this.sizeBytes,
    this.declaredMimeType,
    required this.sha256,
  }) : super._();
  @override
  UploadCreate rebuild(void Function(UploadCreateBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  UploadCreateBuilder toBuilder() => UploadCreateBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is UploadCreate &&
        name == other.name &&
        sizeBytes == other.sizeBytes &&
        declaredMimeType == other.declaredMimeType &&
        sha256 == other.sha256;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jc(_$hash, sizeBytes.hashCode);
    _$hash = $jc(_$hash, declaredMimeType.hashCode);
    _$hash = $jc(_$hash, sha256.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'UploadCreate')
          ..add('name', name)
          ..add('sizeBytes', sizeBytes)
          ..add('declaredMimeType', declaredMimeType)
          ..add('sha256', sha256))
        .toString();
  }
}

class UploadCreateBuilder
    implements Builder<UploadCreate, UploadCreateBuilder> {
  _$UploadCreate? _$v;

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

  String? _sha256;
  String? get sha256 => _$this._sha256;
  set sha256(String? sha256) => _$this._sha256 = sha256;

  UploadCreateBuilder() {
    UploadCreate._defaults(this);
  }

  UploadCreateBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _name = $v.name;
      _sizeBytes = $v.sizeBytes;
      _declaredMimeType = $v.declaredMimeType;
      _sha256 = $v.sha256;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(UploadCreate other) {
    _$v = other as _$UploadCreate;
  }

  @override
  void update(void Function(UploadCreateBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  UploadCreate build() => _build();

  _$UploadCreate _build() {
    final _$result =
        _$v ??
        _$UploadCreate._(
          name: BuiltValueNullFieldError.checkNotNull(
            name,
            r'UploadCreate',
            'name',
          ),
          sizeBytes: BuiltValueNullFieldError.checkNotNull(
            sizeBytes,
            r'UploadCreate',
            'sizeBytes',
          ),
          declaredMimeType: declaredMimeType,
          sha256: BuiltValueNullFieldError.checkNotNull(
            sha256,
            r'UploadCreate',
            'sha256',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
