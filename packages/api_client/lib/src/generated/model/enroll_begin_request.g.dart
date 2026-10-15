// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'enroll_begin_request.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$EnrollBeginRequest extends EnrollBeginRequest {
  @override
  final String enrollmentCode;

  factory _$EnrollBeginRequest([
    void Function(EnrollBeginRequestBuilder)? updates,
  ]) => (EnrollBeginRequestBuilder()..update(updates))._build();

  _$EnrollBeginRequest._({required this.enrollmentCode}) : super._();
  @override
  EnrollBeginRequest rebuild(
    void Function(EnrollBeginRequestBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  EnrollBeginRequestBuilder toBuilder() =>
      EnrollBeginRequestBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is EnrollBeginRequest &&
        enrollmentCode == other.enrollmentCode;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, enrollmentCode.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'EnrollBeginRequest',
    )..add('enrollmentCode', enrollmentCode)).toString();
  }
}

class EnrollBeginRequestBuilder
    implements Builder<EnrollBeginRequest, EnrollBeginRequestBuilder> {
  _$EnrollBeginRequest? _$v;

  String? _enrollmentCode;
  String? get enrollmentCode => _$this._enrollmentCode;
  set enrollmentCode(String? enrollmentCode) =>
      _$this._enrollmentCode = enrollmentCode;

  EnrollBeginRequestBuilder() {
    EnrollBeginRequest._defaults(this);
  }

  EnrollBeginRequestBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _enrollmentCode = $v.enrollmentCode;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(EnrollBeginRequest other) {
    _$v = other as _$EnrollBeginRequest;
  }

  @override
  void update(void Function(EnrollBeginRequestBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  EnrollBeginRequest build() => _build();

  _$EnrollBeginRequest _build() {
    final _$result =
        _$v ??
        _$EnrollBeginRequest._(
          enrollmentCode: BuiltValueNullFieldError.checkNotNull(
            enrollmentCode,
            r'EnrollBeginRequest',
            'enrollmentCode',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
