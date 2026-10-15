// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'upload_part_result.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$UploadPartResult extends UploadPartResult {
  @override
  final int partNumber;
  @override
  final String etag;

  factory _$UploadPartResult([
    void Function(UploadPartResultBuilder)? updates,
  ]) => (UploadPartResultBuilder()..update(updates))._build();

  _$UploadPartResult._({required this.partNumber, required this.etag})
    : super._();
  @override
  UploadPartResult rebuild(void Function(UploadPartResultBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  UploadPartResultBuilder toBuilder() =>
      UploadPartResultBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is UploadPartResult &&
        partNumber == other.partNumber &&
        etag == other.etag;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, partNumber.hashCode);
    _$hash = $jc(_$hash, etag.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'UploadPartResult')
          ..add('partNumber', partNumber)
          ..add('etag', etag))
        .toString();
  }
}

class UploadPartResultBuilder
    implements Builder<UploadPartResult, UploadPartResultBuilder> {
  _$UploadPartResult? _$v;

  int? _partNumber;
  int? get partNumber => _$this._partNumber;
  set partNumber(int? partNumber) => _$this._partNumber = partNumber;

  String? _etag;
  String? get etag => _$this._etag;
  set etag(String? etag) => _$this._etag = etag;

  UploadPartResultBuilder() {
    UploadPartResult._defaults(this);
  }

  UploadPartResultBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _partNumber = $v.partNumber;
      _etag = $v.etag;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(UploadPartResult other) {
    _$v = other as _$UploadPartResult;
  }

  @override
  void update(void Function(UploadPartResultBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  UploadPartResult build() => _build();

  _$UploadPartResult _build() {
    final _$result =
        _$v ??
        _$UploadPartResult._(
          partNumber: BuiltValueNullFieldError.checkNotNull(
            partNumber,
            r'UploadPartResult',
            'partNumber',
          ),
          etag: BuiltValueNullFieldError.checkNotNull(
            etag,
            r'UploadPartResult',
            'etag',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
