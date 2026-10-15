// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'cost_bucket.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$CostBucket extends CostBucket {
  @override
  final DateTime start;
  @override
  final String? groupKey;
  @override
  final BillingMode billingMode;
  @override
  final int amountMicros;
  @override
  final bool isEstimate;

  factory _$CostBucket([void Function(CostBucketBuilder)? updates]) =>
      (CostBucketBuilder()..update(updates))._build();

  _$CostBucket._({
    required this.start,
    this.groupKey,
    required this.billingMode,
    required this.amountMicros,
    required this.isEstimate,
  }) : super._();
  @override
  CostBucket rebuild(void Function(CostBucketBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  CostBucketBuilder toBuilder() => CostBucketBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is CostBucket &&
        start == other.start &&
        groupKey == other.groupKey &&
        billingMode == other.billingMode &&
        amountMicros == other.amountMicros &&
        isEstimate == other.isEstimate;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, start.hashCode);
    _$hash = $jc(_$hash, groupKey.hashCode);
    _$hash = $jc(_$hash, billingMode.hashCode);
    _$hash = $jc(_$hash, amountMicros.hashCode);
    _$hash = $jc(_$hash, isEstimate.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'CostBucket')
          ..add('start', start)
          ..add('groupKey', groupKey)
          ..add('billingMode', billingMode)
          ..add('amountMicros', amountMicros)
          ..add('isEstimate', isEstimate))
        .toString();
  }
}

class CostBucketBuilder implements Builder<CostBucket, CostBucketBuilder> {
  _$CostBucket? _$v;

  DateTime? _start;
  DateTime? get start => _$this._start;
  set start(DateTime? start) => _$this._start = start;

  String? _groupKey;
  String? get groupKey => _$this._groupKey;
  set groupKey(String? groupKey) => _$this._groupKey = groupKey;

  BillingMode? _billingMode;
  BillingMode? get billingMode => _$this._billingMode;
  set billingMode(BillingMode? billingMode) =>
      _$this._billingMode = billingMode;

  int? _amountMicros;
  int? get amountMicros => _$this._amountMicros;
  set amountMicros(int? amountMicros) => _$this._amountMicros = amountMicros;

  bool? _isEstimate;
  bool? get isEstimate => _$this._isEstimate;
  set isEstimate(bool? isEstimate) => _$this._isEstimate = isEstimate;

  CostBucketBuilder() {
    CostBucket._defaults(this);
  }

  CostBucketBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _start = $v.start;
      _groupKey = $v.groupKey;
      _billingMode = $v.billingMode;
      _amountMicros = $v.amountMicros;
      _isEstimate = $v.isEstimate;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(CostBucket other) {
    _$v = other as _$CostBucket;
  }

  @override
  void update(void Function(CostBucketBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  CostBucket build() => _build();

  _$CostBucket _build() {
    final _$result =
        _$v ??
        _$CostBucket._(
          start: BuiltValueNullFieldError.checkNotNull(
            start,
            r'CostBucket',
            'start',
          ),
          groupKey: groupKey,
          billingMode: BuiltValueNullFieldError.checkNotNull(
            billingMode,
            r'CostBucket',
            'billingMode',
          ),
          amountMicros: BuiltValueNullFieldError.checkNotNull(
            amountMicros,
            r'CostBucket',
            'amountMicros',
          ),
          isEstimate: BuiltValueNullFieldError.checkNotNull(
            isEstimate,
            r'CostBucket',
            'isEstimate',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
