// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'stats_live_sli.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$StatsLiveSli extends StatsLiveSli {
  @override
  final StatsMetric metric;
  @override
  final int total;
  @override
  final int good;
  @override
  final double? ratio;

  factory _$StatsLiveSli([void Function(StatsLiveSliBuilder)? updates]) =>
      (StatsLiveSliBuilder()..update(updates))._build();

  _$StatsLiveSli._({
    required this.metric,
    required this.total,
    required this.good,
    this.ratio,
  }) : super._();
  @override
  StatsLiveSli rebuild(void Function(StatsLiveSliBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  StatsLiveSliBuilder toBuilder() => StatsLiveSliBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is StatsLiveSli &&
        metric == other.metric &&
        total == other.total &&
        good == other.good &&
        ratio == other.ratio;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, metric.hashCode);
    _$hash = $jc(_$hash, total.hashCode);
    _$hash = $jc(_$hash, good.hashCode);
    _$hash = $jc(_$hash, ratio.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'StatsLiveSli')
          ..add('metric', metric)
          ..add('total', total)
          ..add('good', good)
          ..add('ratio', ratio))
        .toString();
  }
}

class StatsLiveSliBuilder
    implements Builder<StatsLiveSli, StatsLiveSliBuilder> {
  _$StatsLiveSli? _$v;

  StatsMetric? _metric;
  StatsMetric? get metric => _$this._metric;
  set metric(StatsMetric? metric) => _$this._metric = metric;

  int? _total;
  int? get total => _$this._total;
  set total(int? total) => _$this._total = total;

  int? _good;
  int? get good => _$this._good;
  set good(int? good) => _$this._good = good;

  double? _ratio;
  double? get ratio => _$this._ratio;
  set ratio(double? ratio) => _$this._ratio = ratio;

  StatsLiveSliBuilder() {
    StatsLiveSli._defaults(this);
  }

  StatsLiveSliBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _metric = $v.metric;
      _total = $v.total;
      _good = $v.good;
      _ratio = $v.ratio;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(StatsLiveSli other) {
    _$v = other as _$StatsLiveSli;
  }

  @override
  void update(void Function(StatsLiveSliBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  StatsLiveSli build() => _build();

  _$StatsLiveSli _build() {
    final _$result =
        _$v ??
        _$StatsLiveSli._(
          metric: BuiltValueNullFieldError.checkNotNull(
            metric,
            r'StatsLiveSli',
            'metric',
          ),
          total: BuiltValueNullFieldError.checkNotNull(
            total,
            r'StatsLiveSli',
            'total',
          ),
          good: BuiltValueNullFieldError.checkNotNull(
            good,
            r'StatsLiveSli',
            'good',
          ),
          ratio: ratio,
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
