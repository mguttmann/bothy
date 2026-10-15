// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'upload_state_completed_parts_inner.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$UploadStateCompletedPartsInner extends UploadStateCompletedPartsInner {
  @override
  final int partNumber;
  @override
  final String etag;

  factory _$UploadStateCompletedPartsInner([
    void Function(UploadStateCompletedPartsInnerBuilder)? updates,
  ]) => (UploadStateCompletedPartsInnerBuilder()..update(updates))._build();

  _$UploadStateCompletedPartsInner._({
    required this.partNumber,
    required this.etag,
  }) : super._();
  @override
  UploadStateCompletedPartsInner rebuild(
    void Function(UploadStateCompletedPartsInnerBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  UploadStateCompletedPartsInnerBuilder toBuilder() =>
      UploadStateCompletedPartsInnerBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is UploadStateCompletedPartsInner &&
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
    return (newBuiltValueToStringHelper(r'UploadStateCompletedPartsInner')
          ..add('partNumber', partNumber)
          ..add('etag', etag))
        .toString();
  }
}

class UploadStateCompletedPartsInnerBuilder
    implements
        Builder<
          UploadStateCompletedPartsInner,
          UploadStateCompletedPartsInnerBuilder
        > {
  _$UploadStateCompletedPartsInner? _$v;

  int? _partNumber;
  int? get partNumber => _$this._partNumber;
  set partNumber(int? partNumber) => _$this._partNumber = partNumber;

  String? _etag;
  String? get etag => _$this._etag;
  set etag(String? etag) => _$this._etag = etag;

  UploadStateCompletedPartsInnerBuilder() {
    UploadStateCompletedPartsInner._defaults(this);
  }

  UploadStateCompletedPartsInnerBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _partNumber = $v.partNumber;
      _etag = $v.etag;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(UploadStateCompletedPartsInner other) {
    _$v = other as _$UploadStateCompletedPartsInner;
  }

  @override
  void update(void Function(UploadStateCompletedPartsInnerBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  UploadStateCompletedPartsInner build() => _build();

  _$UploadStateCompletedPartsInner _build() {
    final _$result =
        _$v ??
        _$UploadStateCompletedPartsInner._(
          partNumber: BuiltValueNullFieldError.checkNotNull(
            partNumber,
            r'UploadStateCompletedPartsInner',
            'partNumber',
          ),
          etag: BuiltValueNullFieldError.checkNotNull(
            etag,
            r'UploadStateCompletedPartsInner',
            'etag',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
