// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'gdpr_request_payload.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$GdprRequestPayload extends GdprRequestPayload {
  @override
  final GdprRequest request;

  factory _$GdprRequestPayload([
    void Function(GdprRequestPayloadBuilder)? updates,
  ]) => (GdprRequestPayloadBuilder()..update(updates))._build();

  _$GdprRequestPayload._({required this.request}) : super._();
  @override
  GdprRequestPayload rebuild(
    void Function(GdprRequestPayloadBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  GdprRequestPayloadBuilder toBuilder() =>
      GdprRequestPayloadBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is GdprRequestPayload && request == other.request;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, request.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'GdprRequestPayload',
    )..add('request', request)).toString();
  }
}

class GdprRequestPayloadBuilder
    implements Builder<GdprRequestPayload, GdprRequestPayloadBuilder> {
  _$GdprRequestPayload? _$v;

  GdprRequestBuilder? _request;
  GdprRequestBuilder get request => _$this._request ??= GdprRequestBuilder();
  set request(GdprRequestBuilder? request) => _$this._request = request;

  GdprRequestPayloadBuilder() {
    GdprRequestPayload._defaults(this);
  }

  GdprRequestPayloadBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _request = $v.request.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(GdprRequestPayload other) {
    _$v = other as _$GdprRequestPayload;
  }

  @override
  void update(void Function(GdprRequestPayloadBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  GdprRequestPayload build() => _build();

  _$GdprRequestPayload _build() {
    _$GdprRequestPayload _$result;
    try {
      _$result = _$v ?? _$GdprRequestPayload._(request: request.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'request';
        request.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'GdprRequestPayload',
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
