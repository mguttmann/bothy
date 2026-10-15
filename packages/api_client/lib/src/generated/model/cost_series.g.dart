// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'cost_series.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$CostSeries extends CostSeries {
  @override
  final StatsGranularity granularity;
  @override
  final String currency;
  @override
  final BuiltList<CostBucket> buckets;

  factory _$CostSeries([void Function(CostSeriesBuilder)? updates]) =>
      (CostSeriesBuilder()..update(updates))._build();

  _$CostSeries._({
    required this.granularity,
    required this.currency,
    required this.buckets,
  }) : super._();
  @override
  CostSeries rebuild(void Function(CostSeriesBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  CostSeriesBuilder toBuilder() => CostSeriesBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is CostSeries &&
        granularity == other.granularity &&
        currency == other.currency &&
        buckets == other.buckets;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, granularity.hashCode);
    _$hash = $jc(_$hash, currency.hashCode);
    _$hash = $jc(_$hash, buckets.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'CostSeries')
          ..add('granularity', granularity)
          ..add('currency', currency)
          ..add('buckets', buckets))
        .toString();
  }
}

class CostSeriesBuilder implements Builder<CostSeries, CostSeriesBuilder> {
  _$CostSeries? _$v;

  StatsGranularity? _granularity;
  StatsGranularity? get granularity => _$this._granularity;
  set granularity(StatsGranularity? granularity) =>
      _$this._granularity = granularity;

  String? _currency;
  String? get currency => _$this._currency;
  set currency(String? currency) => _$this._currency = currency;

  ListBuilder<CostBucket>? _buckets;
  ListBuilder<CostBucket> get buckets =>
      _$this._buckets ??= ListBuilder<CostBucket>();
  set buckets(ListBuilder<CostBucket>? buckets) => _$this._buckets = buckets;

  CostSeriesBuilder() {
    CostSeries._defaults(this);
  }

  CostSeriesBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _granularity = $v.granularity;
      _currency = $v.currency;
      _buckets = $v.buckets.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(CostSeries other) {
    _$v = other as _$CostSeries;
  }

  @override
  void update(void Function(CostSeriesBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  CostSeries build() => _build();

  _$CostSeries _build() {
    _$CostSeries _$result;
    try {
      _$result =
          _$v ??
          _$CostSeries._(
            granularity: BuiltValueNullFieldError.checkNotNull(
              granularity,
              r'CostSeries',
              'granularity',
            ),
            currency: BuiltValueNullFieldError.checkNotNull(
              currency,
              r'CostSeries',
              'currency',
            ),
            buckets: buckets.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'buckets';
        buckets.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'CostSeries',
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
