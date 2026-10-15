// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'stats_slo_entry.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$StatsSloEntry extends StatsSloEntry {
  @override
  final String slo;
  @override
  final StatsMetric metric;
  @override
  final double objective;
  @override
  final int? thresholdMs;
  @override
  final int periodDays;
  @override
  final int total;
  @override
  final int good;
  @override
  final double? ratio;
  @override
  final double? budgetRemaining;
  @override
  final StatsSloState state;

  factory _$StatsSloEntry([void Function(StatsSloEntryBuilder)? updates]) =>
      (StatsSloEntryBuilder()..update(updates))._build();

  _$StatsSloEntry._({
    required this.slo,
    required this.metric,
    required this.objective,
    this.thresholdMs,
    required this.periodDays,
    required this.total,
    required this.good,
    this.ratio,
    this.budgetRemaining,
    required this.state,
  }) : super._();
  @override
  StatsSloEntry rebuild(void Function(StatsSloEntryBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  StatsSloEntryBuilder toBuilder() => StatsSloEntryBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is StatsSloEntry &&
        slo == other.slo &&
        metric == other.metric &&
        objective == other.objective &&
        thresholdMs == other.thresholdMs &&
        periodDays == other.periodDays &&
        total == other.total &&
        good == other.good &&
        ratio == other.ratio &&
        budgetRemaining == other.budgetRemaining &&
        state == other.state;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, slo.hashCode);
    _$hash = $jc(_$hash, metric.hashCode);
    _$hash = $jc(_$hash, objective.hashCode);
    _$hash = $jc(_$hash, thresholdMs.hashCode);
    _$hash = $jc(_$hash, periodDays.hashCode);
    _$hash = $jc(_$hash, total.hashCode);
    _$hash = $jc(_$hash, good.hashCode);
    _$hash = $jc(_$hash, ratio.hashCode);
    _$hash = $jc(_$hash, budgetRemaining.hashCode);
    _$hash = $jc(_$hash, state.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'StatsSloEntry')
          ..add('slo', slo)
          ..add('metric', metric)
          ..add('objective', objective)
          ..add('thresholdMs', thresholdMs)
          ..add('periodDays', periodDays)
          ..add('total', total)
          ..add('good', good)
          ..add('ratio', ratio)
          ..add('budgetRemaining', budgetRemaining)
          ..add('state', state))
        .toString();
  }
}

class StatsSloEntryBuilder
    implements Builder<StatsSloEntry, StatsSloEntryBuilder> {
  _$StatsSloEntry? _$v;

  String? _slo;
  String? get slo => _$this._slo;
  set slo(String? slo) => _$this._slo = slo;

  StatsMetric? _metric;
  StatsMetric? get metric => _$this._metric;
  set metric(StatsMetric? metric) => _$this._metric = metric;

  double? _objective;
  double? get objective => _$this._objective;
  set objective(double? objective) => _$this._objective = objective;

  int? _thresholdMs;
  int? get thresholdMs => _$this._thresholdMs;
  set thresholdMs(int? thresholdMs) => _$this._thresholdMs = thresholdMs;

  int? _periodDays;
  int? get periodDays => _$this._periodDays;
  set periodDays(int? periodDays) => _$this._periodDays = periodDays;

  int? _total;
  int? get total => _$this._total;
  set total(int? total) => _$this._total = total;

  int? _good;
  int? get good => _$this._good;
  set good(int? good) => _$this._good = good;

  double? _ratio;
  double? get ratio => _$this._ratio;
  set ratio(double? ratio) => _$this._ratio = ratio;

  double? _budgetRemaining;
  double? get budgetRemaining => _$this._budgetRemaining;
  set budgetRemaining(double? budgetRemaining) =>
      _$this._budgetRemaining = budgetRemaining;

  StatsSloState? _state;
  StatsSloState? get state => _$this._state;
  set state(StatsSloState? state) => _$this._state = state;

  StatsSloEntryBuilder() {
    StatsSloEntry._defaults(this);
  }

  StatsSloEntryBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _slo = $v.slo;
      _metric = $v.metric;
      _objective = $v.objective;
      _thresholdMs = $v.thresholdMs;
      _periodDays = $v.periodDays;
      _total = $v.total;
      _good = $v.good;
      _ratio = $v.ratio;
      _budgetRemaining = $v.budgetRemaining;
      _state = $v.state;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(StatsSloEntry other) {
    _$v = other as _$StatsSloEntry;
  }

  @override
  void update(void Function(StatsSloEntryBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  StatsSloEntry build() => _build();

  _$StatsSloEntry _build() {
    final _$result =
        _$v ??
        _$StatsSloEntry._(
          slo: BuiltValueNullFieldError.checkNotNull(
            slo,
            r'StatsSloEntry',
            'slo',
          ),
          metric: BuiltValueNullFieldError.checkNotNull(
            metric,
            r'StatsSloEntry',
            'metric',
          ),
          objective: BuiltValueNullFieldError.checkNotNull(
            objective,
            r'StatsSloEntry',
            'objective',
          ),
          thresholdMs: thresholdMs,
          periodDays: BuiltValueNullFieldError.checkNotNull(
            periodDays,
            r'StatsSloEntry',
            'periodDays',
          ),
          total: BuiltValueNullFieldError.checkNotNull(
            total,
            r'StatsSloEntry',
            'total',
          ),
          good: BuiltValueNullFieldError.checkNotNull(
            good,
            r'StatsSloEntry',
            'good',
          ),
          ratio: ratio,
          budgetRemaining: budgetRemaining,
          state: BuiltValueNullFieldError.checkNotNull(
            state,
            r'StatsSloEntry',
            'state',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
