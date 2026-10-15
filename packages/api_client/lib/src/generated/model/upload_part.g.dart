// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'upload_part.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$UploadPart extends UploadPart {
  @override
  final int partNumber;
  @override
  final String url;
  @override
  final BuiltList<HeaderPair> headers;

  factory _$UploadPart([void Function(UploadPartBuilder)? updates]) =>
      (UploadPartBuilder()..update(updates))._build();

  _$UploadPart._({
    required this.partNumber,
    required this.url,
    required this.headers,
  }) : super._();
  @override
  UploadPart rebuild(void Function(UploadPartBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  UploadPartBuilder toBuilder() => UploadPartBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is UploadPart &&
        partNumber == other.partNumber &&
        url == other.url &&
        headers == other.headers;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, partNumber.hashCode);
    _$hash = $jc(_$hash, url.hashCode);
    _$hash = $jc(_$hash, headers.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'UploadPart')
          ..add('partNumber', partNumber)
          ..add('url', url)
          ..add('headers', headers))
        .toString();
  }
}

class UploadPartBuilder implements Builder<UploadPart, UploadPartBuilder> {
  _$UploadPart? _$v;

  int? _partNumber;
  int? get partNumber => _$this._partNumber;
  set partNumber(int? partNumber) => _$this._partNumber = partNumber;

  String? _url;
  String? get url => _$this._url;
  set url(String? url) => _$this._url = url;

  ListBuilder<HeaderPair>? _headers;
  ListBuilder<HeaderPair> get headers =>
      _$this._headers ??= ListBuilder<HeaderPair>();
  set headers(ListBuilder<HeaderPair>? headers) => _$this._headers = headers;

  UploadPartBuilder() {
    UploadPart._defaults(this);
  }

  UploadPartBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _partNumber = $v.partNumber;
      _url = $v.url;
      _headers = $v.headers.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(UploadPart other) {
    _$v = other as _$UploadPart;
  }

  @override
  void update(void Function(UploadPartBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  UploadPart build() => _build();

  _$UploadPart _build() {
    _$UploadPart _$result;
    try {
      _$result =
          _$v ??
          _$UploadPart._(
            partNumber: BuiltValueNullFieldError.checkNotNull(
              partNumber,
              r'UploadPart',
              'partNumber',
            ),
            url: BuiltValueNullFieldError.checkNotNull(
              url,
              r'UploadPart',
              'url',
            ),
            headers: headers.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'headers';
        headers.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'UploadPart',
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
