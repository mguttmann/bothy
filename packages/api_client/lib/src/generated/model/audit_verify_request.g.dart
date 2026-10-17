// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'audit_verify_request.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$AuditVerifyRequest extends AuditVerifyRequest {
  @override
  final int? fromSeq;
  @override
  final int? toSeq;

  factory _$AuditVerifyRequest([
    void Function(AuditVerifyRequestBuilder)? updates,
  ]) => (AuditVerifyRequestBuilder()..update(updates))._build();

  _$AuditVerifyRequest._({this.fromSeq, this.toSeq}) : super._();
  @override
  AuditVerifyRequest rebuild(
    void Function(AuditVerifyRequestBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  AuditVerifyRequestBuilder toBuilder() =>
      AuditVerifyRequestBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AuditVerifyRequest &&
        fromSeq == other.fromSeq &&
        toSeq == other.toSeq;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, fromSeq.hashCode);
    _$hash = $jc(_$hash, toSeq.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AuditVerifyRequest')
          ..add('fromSeq', fromSeq)
          ..add('toSeq', toSeq))
        .toString();
  }
}

class AuditVerifyRequestBuilder
    implements Builder<AuditVerifyRequest, AuditVerifyRequestBuilder> {
  _$AuditVerifyRequest? _$v;

  int? _fromSeq;
  int? get fromSeq => _$this._fromSeq;
  set fromSeq(int? fromSeq) => _$this._fromSeq = fromSeq;

  int? _toSeq;
  int? get toSeq => _$this._toSeq;
  set toSeq(int? toSeq) => _$this._toSeq = toSeq;

  AuditVerifyRequestBuilder() {
    AuditVerifyRequest._defaults(this);
  }

  AuditVerifyRequestBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _fromSeq = $v.fromSeq;
      _toSeq = $v.toSeq;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AuditVerifyRequest other) {
    _$v = other as _$AuditVerifyRequest;
  }

  @override
  void update(void Function(AuditVerifyRequestBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AuditVerifyRequest build() => _build();

  _$AuditVerifyRequest _build() {
    final _$result =
        _$v ?? _$AuditVerifyRequest._(fromSeq: fromSeq, toSeq: toSeq);
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
