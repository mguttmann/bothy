// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'stats_series.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$StatsSeries extends StatsSeries {
  @override
  final StatsGranularity granularity;
  @override
  final BuiltList<StatsBucket> buckets;

  factory _$StatsSeries([void Function(StatsSeriesBuilder)? updates]) =>
      (StatsSeriesBuilder()..update(updates))._build();

  _$StatsSeries._({required this.granularity, required this.buckets})
    : super._();
  @override
  StatsSeries rebuild(void Function(StatsSeriesBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  StatsSeriesBuilder toBuilder() => StatsSeriesBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is StatsSeries &&
        granularity == other.granularity &&
        buckets == other.buckets;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, granularity.hashCode);
    _$hash = $jc(_$hash, buckets.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'StatsSeries')
          ..add('granularity', granularity)
          ..add('buckets', buckets))
        .toString();
  }
}

class StatsSeriesBuilder implements Builder<StatsSeries, StatsSeriesBuilder> {
  _$StatsSeries? _$v;

  StatsGranularity? _granularity;
  StatsGranularity? get granularity => _$this._granularity;
  set granularity(StatsGranularity? granularity) =>
      _$this._granularity = granularity;

  ListBuilder<StatsBucket>? _buckets;
  ListBuilder<StatsBucket> get buckets =>
      _$this._buckets ??= ListBuilder<StatsBucket>();
  set buckets(ListBuilder<StatsBucket>? buckets) => _$this._buckets = buckets;

  StatsSeriesBuilder() {
    StatsSeries._defaults(this);
  }

  StatsSeriesBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _granularity = $v.granularity;
      _buckets = $v.buckets.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(StatsSeries other) {
    _$v = other as _$StatsSeries;
  }

  @override
  void update(void Function(StatsSeriesBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  StatsSeries build() => _build();

  _$StatsSeries _build() {
    _$StatsSeries _$result;
    try {
      _$result =
          _$v ??
          _$StatsSeries._(
            granularity: BuiltValueNullFieldError.checkNotNull(
              granularity,
              r'StatsSeries',
              'granularity',
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
          r'StatsSeries',
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
