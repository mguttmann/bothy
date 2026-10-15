// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'stats_live_rate.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$StatsLiveRate extends StatsLiveRate {
  @override
  final StatsMetric metric;
  @override
  final String? dimension;
  @override
  final int lastMinute;
  @override
  final int currentMinute;
  @override
  final int? lastMinuteSum;
  @override
  final String? currency;

  factory _$StatsLiveRate([void Function(StatsLiveRateBuilder)? updates]) =>
      (StatsLiveRateBuilder()..update(updates))._build();

  _$StatsLiveRate._({
    required this.metric,
    this.dimension,
    required this.lastMinute,
    required this.currentMinute,
    this.lastMinuteSum,
    this.currency,
  }) : super._();
  @override
  StatsLiveRate rebuild(void Function(StatsLiveRateBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  StatsLiveRateBuilder toBuilder() => StatsLiveRateBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is StatsLiveRate &&
        metric == other.metric &&
        dimension == other.dimension &&
        lastMinute == other.lastMinute &&
        currentMinute == other.currentMinute &&
        lastMinuteSum == other.lastMinuteSum &&
        currency == other.currency;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, metric.hashCode);
    _$hash = $jc(_$hash, dimension.hashCode);
    _$hash = $jc(_$hash, lastMinute.hashCode);
    _$hash = $jc(_$hash, currentMinute.hashCode);
    _$hash = $jc(_$hash, lastMinuteSum.hashCode);
    _$hash = $jc(_$hash, currency.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'StatsLiveRate')
          ..add('metric', metric)
          ..add('dimension', dimension)
          ..add('lastMinute', lastMinute)
          ..add('currentMinute', currentMinute)
          ..add('lastMinuteSum', lastMinuteSum)
          ..add('currency', currency))
        .toString();
  }
}

class StatsLiveRateBuilder
    implements Builder<StatsLiveRate, StatsLiveRateBuilder> {
  _$StatsLiveRate? _$v;

  StatsMetric? _metric;
  StatsMetric? get metric => _$this._metric;
  set metric(StatsMetric? metric) => _$this._metric = metric;

  String? _dimension;
  String? get dimension => _$this._dimension;
  set dimension(String? dimension) => _$this._dimension = dimension;

  int? _lastMinute;
  int? get lastMinute => _$this._lastMinute;
  set lastMinute(int? lastMinute) => _$this._lastMinute = lastMinute;

  int? _currentMinute;
  int? get currentMinute => _$this._currentMinute;
  set currentMinute(int? currentMinute) =>
      _$this._currentMinute = currentMinute;

  int? _lastMinuteSum;
  int? get lastMinuteSum => _$this._lastMinuteSum;
  set lastMinuteSum(int? lastMinuteSum) =>
      _$this._lastMinuteSum = lastMinuteSum;

  String? _currency;
  String? get currency => _$this._currency;
  set currency(String? currency) => _$this._currency = currency;

  StatsLiveRateBuilder() {
    StatsLiveRate._defaults(this);
  }

  StatsLiveRateBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _metric = $v.metric;
      _dimension = $v.dimension;
      _lastMinute = $v.lastMinute;
      _currentMinute = $v.currentMinute;
      _lastMinuteSum = $v.lastMinuteSum;
      _currency = $v.currency;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(StatsLiveRate other) {
    _$v = other as _$StatsLiveRate;
  }

  @override
  void update(void Function(StatsLiveRateBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  StatsLiveRate build() => _build();

  _$StatsLiveRate _build() {
    final _$result =
        _$v ??
        _$StatsLiveRate._(
          metric: BuiltValueNullFieldError.checkNotNull(
            metric,
            r'StatsLiveRate',
            'metric',
          ),
          dimension: dimension,
          lastMinute: BuiltValueNullFieldError.checkNotNull(
            lastMinute,
            r'StatsLiveRate',
            'lastMinute',
          ),
          currentMinute: BuiltValueNullFieldError.checkNotNull(
            currentMinute,
            r'StatsLiveRate',
            'currentMinute',
          ),
          lastMinuteSum: lastMinuteSum,
          currency: currency,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
