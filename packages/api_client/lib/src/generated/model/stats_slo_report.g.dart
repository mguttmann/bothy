// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'stats_slo_report.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const StatsSloReportMeasuredByEnum _$statsSloReportMeasuredByEnum_own =
    const StatsSloReportMeasuredByEnum._('own');
const StatsSloReportMeasuredByEnum
_$statsSloReportMeasuredByEnum_unknownDefaultOpenApi =
    const StatsSloReportMeasuredByEnum._('unknownDefaultOpenApi');

StatsSloReportMeasuredByEnum _$statsSloReportMeasuredByEnumValueOf(
  String name,
) {
  switch (name) {
    case 'own':
      return _$statsSloReportMeasuredByEnum_own;
    case 'unknownDefaultOpenApi':
      return _$statsSloReportMeasuredByEnum_unknownDefaultOpenApi;
    default:
      return _$statsSloReportMeasuredByEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<StatsSloReportMeasuredByEnum>
_$statsSloReportMeasuredByEnumValues = BuiltSet<StatsSloReportMeasuredByEnum>(
  const <StatsSloReportMeasuredByEnum>[
    _$statsSloReportMeasuredByEnum_own,
    _$statsSloReportMeasuredByEnum_unknownDefaultOpenApi,
  ],
);

Serializer<StatsSloReportMeasuredByEnum>
_$statsSloReportMeasuredByEnumSerializer =
    _$StatsSloReportMeasuredByEnumSerializer();

class _$StatsSloReportMeasuredByEnumSerializer
    implements PrimitiveSerializer<StatsSloReportMeasuredByEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'own': 'own',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'own': 'own',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[StatsSloReportMeasuredByEnum];
  @override
  final String wireName = 'StatsSloReportMeasuredByEnum';

  @override
  Object serialize(
    Serializers serializers,
    StatsSloReportMeasuredByEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  StatsSloReportMeasuredByEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => StatsSloReportMeasuredByEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$StatsSloReport extends StatsSloReport {
  @override
  final DateTime at;
  @override
  final StatsSloReportMeasuredByEnum measuredBy;
  @override
  final BuiltList<StatsSloEntry> items;

  factory _$StatsSloReport([void Function(StatsSloReportBuilder)? updates]) =>
      (StatsSloReportBuilder()..update(updates))._build();

  _$StatsSloReport._({
    required this.at,
    required this.measuredBy,
    required this.items,
  }) : super._();
  @override
  StatsSloReport rebuild(void Function(StatsSloReportBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  StatsSloReportBuilder toBuilder() => StatsSloReportBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is StatsSloReport &&
        at == other.at &&
        measuredBy == other.measuredBy &&
        items == other.items;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, at.hashCode);
    _$hash = $jc(_$hash, measuredBy.hashCode);
    _$hash = $jc(_$hash, items.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'StatsSloReport')
          ..add('at', at)
          ..add('measuredBy', measuredBy)
          ..add('items', items))
        .toString();
  }
}

class StatsSloReportBuilder
    implements Builder<StatsSloReport, StatsSloReportBuilder> {
  _$StatsSloReport? _$v;

  DateTime? _at;
  DateTime? get at => _$this._at;
  set at(DateTime? at) => _$this._at = at;

  StatsSloReportMeasuredByEnum? _measuredBy;
  StatsSloReportMeasuredByEnum? get measuredBy => _$this._measuredBy;
  set measuredBy(StatsSloReportMeasuredByEnum? measuredBy) =>
      _$this._measuredBy = measuredBy;

  ListBuilder<StatsSloEntry>? _items;
  ListBuilder<StatsSloEntry> get items =>
      _$this._items ??= ListBuilder<StatsSloEntry>();
  set items(ListBuilder<StatsSloEntry>? items) => _$this._items = items;

  StatsSloReportBuilder() {
    StatsSloReport._defaults(this);
  }

  StatsSloReportBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _at = $v.at;
      _measuredBy = $v.measuredBy;
      _items = $v.items.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(StatsSloReport other) {
    _$v = other as _$StatsSloReport;
  }

  @override
  void update(void Function(StatsSloReportBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  StatsSloReport build() => _build();

  _$StatsSloReport _build() {
    _$StatsSloReport _$result;
    try {
      _$result =
          _$v ??
          _$StatsSloReport._(
            at: BuiltValueNullFieldError.checkNotNull(
              at,
              r'StatsSloReport',
              'at',
            ),
            measuredBy: BuiltValueNullFieldError.checkNotNull(
              measuredBy,
              r'StatsSloReport',
              'measuredBy',
            ),
            items: items.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'items';
        items.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'StatsSloReport',
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
