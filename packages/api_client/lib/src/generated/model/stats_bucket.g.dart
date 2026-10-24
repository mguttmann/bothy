// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'stats_bucket.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$StatsBucket extends StatsBucket {
  @override
  final DateTime start;
  @override
  final String? groupKey;
  @override
  final BuiltMap<String, double> metrics;
  @override
  final BuiltList<String>? belowMinGroup;

  factory _$StatsBucket([void Function(StatsBucketBuilder)? updates]) =>
      (StatsBucketBuilder()..update(updates))._build();

  _$StatsBucket._({
    required this.start,
    this.groupKey,
    required this.metrics,
    this.belowMinGroup,
  }) : super._();
  @override
  StatsBucket rebuild(void Function(StatsBucketBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  StatsBucketBuilder toBuilder() => StatsBucketBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is StatsBucket &&
        start == other.start &&
        groupKey == other.groupKey &&
        metrics == other.metrics &&
        belowMinGroup == other.belowMinGroup;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, start.hashCode);
    _$hash = $jc(_$hash, groupKey.hashCode);
    _$hash = $jc(_$hash, metrics.hashCode);
    _$hash = $jc(_$hash, belowMinGroup.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'StatsBucket')
          ..add('start', start)
          ..add('groupKey', groupKey)
          ..add('metrics', metrics)
          ..add('belowMinGroup', belowMinGroup))
        .toString();
  }
}

class StatsBucketBuilder implements Builder<StatsBucket, StatsBucketBuilder> {
  _$StatsBucket? _$v;

  DateTime? _start;
  DateTime? get start => _$this._start;
  set start(DateTime? start) => _$this._start = start;

  String? _groupKey;
  String? get groupKey => _$this._groupKey;
  set groupKey(String? groupKey) => _$this._groupKey = groupKey;

  MapBuilder<String, double>? _metrics;
  MapBuilder<String, double> get metrics =>
      _$this._metrics ??= MapBuilder<String, double>();
  set metrics(MapBuilder<String, double>? metrics) => _$this._metrics = metrics;

  ListBuilder<String>? _belowMinGroup;
  ListBuilder<String> get belowMinGroup =>
      _$this._belowMinGroup ??= ListBuilder<String>();
  set belowMinGroup(ListBuilder<String>? belowMinGroup) =>
      _$this._belowMinGroup = belowMinGroup;

  StatsBucketBuilder() {
    StatsBucket._defaults(this);
  }

  StatsBucketBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _start = $v.start;
      _groupKey = $v.groupKey;
      _metrics = $v.metrics.toBuilder();
      _belowMinGroup = $v.belowMinGroup?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(StatsBucket other) {
    _$v = other as _$StatsBucket;
  }

  @override
  void update(void Function(StatsBucketBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  StatsBucket build() => _build();

  _$StatsBucket _build() {
    _$StatsBucket _$result;
    try {
      _$result =
          _$v ??
          _$StatsBucket._(
            start: BuiltValueNullFieldError.checkNotNull(
              start,
              r'StatsBucket',
              'start',
            ),
            groupKey: groupKey,
            metrics: metrics.build(),
            belowMinGroup: _belowMinGroup?.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'metrics';
        metrics.build();
        _$failedField = 'belowMinGroup';
        _belowMinGroup?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'StatsBucket',
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
