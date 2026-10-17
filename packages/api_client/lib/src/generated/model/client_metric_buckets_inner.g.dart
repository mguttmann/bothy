// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'client_metric_buckets_inner.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ClientMetricBucketsInner extends ClientMetricBucketsInner {
  @override
  final double le;
  @override
  final int count;

  factory _$ClientMetricBucketsInner([
    void Function(ClientMetricBucketsInnerBuilder)? updates,
  ]) => (ClientMetricBucketsInnerBuilder()..update(updates))._build();

  _$ClientMetricBucketsInner._({required this.le, required this.count})
    : super._();
  @override
  ClientMetricBucketsInner rebuild(
    void Function(ClientMetricBucketsInnerBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  ClientMetricBucketsInnerBuilder toBuilder() =>
      ClientMetricBucketsInnerBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ClientMetricBucketsInner &&
        le == other.le &&
        count == other.count;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, le.hashCode);
    _$hash = $jc(_$hash, count.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ClientMetricBucketsInner')
          ..add('le', le)
          ..add('count', count))
        .toString();
  }
}

class ClientMetricBucketsInnerBuilder
    implements
        Builder<ClientMetricBucketsInner, ClientMetricBucketsInnerBuilder> {
  _$ClientMetricBucketsInner? _$v;

  double? _le;
  double? get le => _$this._le;
  set le(double? le) => _$this._le = le;

  int? _count;
  int? get count => _$this._count;
  set count(int? count) => _$this._count = count;

  ClientMetricBucketsInnerBuilder() {
    ClientMetricBucketsInner._defaults(this);
  }

  ClientMetricBucketsInnerBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _le = $v.le;
      _count = $v.count;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ClientMetricBucketsInner other) {
    _$v = other as _$ClientMetricBucketsInner;
  }

  @override
  void update(void Function(ClientMetricBucketsInnerBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ClientMetricBucketsInner build() => _build();

  _$ClientMetricBucketsInner _build() {
    final _$result =
        _$v ??
        _$ClientMetricBucketsInner._(
          le: BuiltValueNullFieldError.checkNotNull(
            le,
            r'ClientMetricBucketsInner',
            'le',
          ),
          count: BuiltValueNullFieldError.checkNotNull(
            count,
            r'ClientMetricBucketsInner',
            'count',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
