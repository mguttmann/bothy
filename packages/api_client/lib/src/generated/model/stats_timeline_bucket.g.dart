// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'stats_timeline_bucket.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$StatsTimelineBucket extends StatsTimelineBucket {
  @override
  final DateTime start;
  @override
  final StatsMetric metric;
  @override
  final String? channelId;
  @override
  final String? agentId;
  @override
  final String dimension;
  @override
  final String? currency;
  @override
  final int count;
  @override
  final int sum;
  @override
  final int good;

  factory _$StatsTimelineBucket([
    void Function(StatsTimelineBucketBuilder)? updates,
  ]) => (StatsTimelineBucketBuilder()..update(updates))._build();

  _$StatsTimelineBucket._({
    required this.start,
    required this.metric,
    this.channelId,
    this.agentId,
    required this.dimension,
    this.currency,
    required this.count,
    required this.sum,
    required this.good,
  }) : super._();
  @override
  StatsTimelineBucket rebuild(
    void Function(StatsTimelineBucketBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  StatsTimelineBucketBuilder toBuilder() =>
      StatsTimelineBucketBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is StatsTimelineBucket &&
        start == other.start &&
        metric == other.metric &&
        channelId == other.channelId &&
        agentId == other.agentId &&
        dimension == other.dimension &&
        currency == other.currency &&
        count == other.count &&
        sum == other.sum &&
        good == other.good;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, start.hashCode);
    _$hash = $jc(_$hash, metric.hashCode);
    _$hash = $jc(_$hash, channelId.hashCode);
    _$hash = $jc(_$hash, agentId.hashCode);
    _$hash = $jc(_$hash, dimension.hashCode);
    _$hash = $jc(_$hash, currency.hashCode);
    _$hash = $jc(_$hash, count.hashCode);
    _$hash = $jc(_$hash, sum.hashCode);
    _$hash = $jc(_$hash, good.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'StatsTimelineBucket')
          ..add('start', start)
          ..add('metric', metric)
          ..add('channelId', channelId)
          ..add('agentId', agentId)
          ..add('dimension', dimension)
          ..add('currency', currency)
          ..add('count', count)
          ..add('sum', sum)
          ..add('good', good))
        .toString();
  }
}

class StatsTimelineBucketBuilder
    implements Builder<StatsTimelineBucket, StatsTimelineBucketBuilder> {
  _$StatsTimelineBucket? _$v;

  DateTime? _start;
  DateTime? get start => _$this._start;
  set start(DateTime? start) => _$this._start = start;

  StatsMetric? _metric;
  StatsMetric? get metric => _$this._metric;
  set metric(StatsMetric? metric) => _$this._metric = metric;

  String? _channelId;
  String? get channelId => _$this._channelId;
  set channelId(String? channelId) => _$this._channelId = channelId;

  String? _agentId;
  String? get agentId => _$this._agentId;
  set agentId(String? agentId) => _$this._agentId = agentId;

  String? _dimension;
  String? get dimension => _$this._dimension;
  set dimension(String? dimension) => _$this._dimension = dimension;

  String? _currency;
  String? get currency => _$this._currency;
  set currency(String? currency) => _$this._currency = currency;

  int? _count;
  int? get count => _$this._count;
  set count(int? count) => _$this._count = count;

  int? _sum;
  int? get sum => _$this._sum;
  set sum(int? sum) => _$this._sum = sum;

  int? _good;
  int? get good => _$this._good;
  set good(int? good) => _$this._good = good;

  StatsTimelineBucketBuilder() {
    StatsTimelineBucket._defaults(this);
  }

  StatsTimelineBucketBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _start = $v.start;
      _metric = $v.metric;
      _channelId = $v.channelId;
      _agentId = $v.agentId;
      _dimension = $v.dimension;
      _currency = $v.currency;
      _count = $v.count;
      _sum = $v.sum;
      _good = $v.good;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(StatsTimelineBucket other) {
    _$v = other as _$StatsTimelineBucket;
  }

  @override
  void update(void Function(StatsTimelineBucketBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  StatsTimelineBucket build() => _build();

  _$StatsTimelineBucket _build() {
    final _$result =
        _$v ??
        _$StatsTimelineBucket._(
          start: BuiltValueNullFieldError.checkNotNull(
            start,
            r'StatsTimelineBucket',
            'start',
          ),
          metric: BuiltValueNullFieldError.checkNotNull(
            metric,
            r'StatsTimelineBucket',
            'metric',
          ),
          channelId: channelId,
          agentId: agentId,
          dimension: BuiltValueNullFieldError.checkNotNull(
            dimension,
            r'StatsTimelineBucket',
            'dimension',
          ),
          currency: currency,
          count: BuiltValueNullFieldError.checkNotNull(
            count,
            r'StatsTimelineBucket',
            'count',
          ),
          sum: BuiltValueNullFieldError.checkNotNull(
            sum,
            r'StatsTimelineBucket',
            'sum',
          ),
          good: BuiltValueNullFieldError.checkNotNull(
            good,
            r'StatsTimelineBucket',
            'good',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
