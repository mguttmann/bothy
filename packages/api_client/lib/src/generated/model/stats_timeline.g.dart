// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'stats_timeline.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$StatsTimeline extends StatsTimeline {
  @override
  final DateTime from;
  @override
  final DateTime to;
  @override
  final BuiltList<StatsTimelineBucket> buckets;
  @override
  final BuiltList<StatsTimelineRun> runs;
  @override
  final bool runsTruncated;

  factory _$StatsTimeline([void Function(StatsTimelineBuilder)? updates]) =>
      (StatsTimelineBuilder()..update(updates))._build();

  _$StatsTimeline._({
    required this.from,
    required this.to,
    required this.buckets,
    required this.runs,
    required this.runsTruncated,
  }) : super._();
  @override
  StatsTimeline rebuild(void Function(StatsTimelineBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  StatsTimelineBuilder toBuilder() => StatsTimelineBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is StatsTimeline &&
        from == other.from &&
        to == other.to &&
        buckets == other.buckets &&
        runs == other.runs &&
        runsTruncated == other.runsTruncated;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, from.hashCode);
    _$hash = $jc(_$hash, to.hashCode);
    _$hash = $jc(_$hash, buckets.hashCode);
    _$hash = $jc(_$hash, runs.hashCode);
    _$hash = $jc(_$hash, runsTruncated.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'StatsTimeline')
          ..add('from', from)
          ..add('to', to)
          ..add('buckets', buckets)
          ..add('runs', runs)
          ..add('runsTruncated', runsTruncated))
        .toString();
  }
}

class StatsTimelineBuilder
    implements Builder<StatsTimeline, StatsTimelineBuilder> {
  _$StatsTimeline? _$v;

  DateTime? _from;
  DateTime? get from => _$this._from;
  set from(DateTime? from) => _$this._from = from;

  DateTime? _to;
  DateTime? get to => _$this._to;
  set to(DateTime? to) => _$this._to = to;

  ListBuilder<StatsTimelineBucket>? _buckets;
  ListBuilder<StatsTimelineBucket> get buckets =>
      _$this._buckets ??= ListBuilder<StatsTimelineBucket>();
  set buckets(ListBuilder<StatsTimelineBucket>? buckets) =>
      _$this._buckets = buckets;

  ListBuilder<StatsTimelineRun>? _runs;
  ListBuilder<StatsTimelineRun> get runs =>
      _$this._runs ??= ListBuilder<StatsTimelineRun>();
  set runs(ListBuilder<StatsTimelineRun>? runs) => _$this._runs = runs;

  bool? _runsTruncated;
  bool? get runsTruncated => _$this._runsTruncated;
  set runsTruncated(bool? runsTruncated) =>
      _$this._runsTruncated = runsTruncated;

  StatsTimelineBuilder() {
    StatsTimeline._defaults(this);
  }

  StatsTimelineBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _from = $v.from;
      _to = $v.to;
      _buckets = $v.buckets.toBuilder();
      _runs = $v.runs.toBuilder();
      _runsTruncated = $v.runsTruncated;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(StatsTimeline other) {
    _$v = other as _$StatsTimeline;
  }

  @override
  void update(void Function(StatsTimelineBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  StatsTimeline build() => _build();

  _$StatsTimeline _build() {
    _$StatsTimeline _$result;
    try {
      _$result =
          _$v ??
          _$StatsTimeline._(
            from: BuiltValueNullFieldError.checkNotNull(
              from,
              r'StatsTimeline',
              'from',
            ),
            to: BuiltValueNullFieldError.checkNotNull(
              to,
              r'StatsTimeline',
              'to',
            ),
            buckets: buckets.build(),
            runs: runs.build(),
            runsTruncated: BuiltValueNullFieldError.checkNotNull(
              runsTruncated,
              r'StatsTimeline',
              'runsTruncated',
            ),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'buckets';
        buckets.build();
        _$failedField = 'runs';
        runs.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'StatsTimeline',
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
