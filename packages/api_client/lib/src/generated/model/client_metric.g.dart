// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'client_metric.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const ClientMetricKindEnum _$clientMetricKindEnum_histogram =
    const ClientMetricKindEnum._('histogram');
const ClientMetricKindEnum _$clientMetricKindEnum_counter =
    const ClientMetricKindEnum._('counter');
const ClientMetricKindEnum _$clientMetricKindEnum_gauge =
    const ClientMetricKindEnum._('gauge');
const ClientMetricKindEnum _$clientMetricKindEnum_unknownDefaultOpenApi =
    const ClientMetricKindEnum._('unknownDefaultOpenApi');

ClientMetricKindEnum _$clientMetricKindEnumValueOf(String name) {
  switch (name) {
    case 'histogram':
      return _$clientMetricKindEnum_histogram;
    case 'counter':
      return _$clientMetricKindEnum_counter;
    case 'gauge':
      return _$clientMetricKindEnum_gauge;
    case 'unknownDefaultOpenApi':
      return _$clientMetricKindEnum_unknownDefaultOpenApi;
    default:
      return _$clientMetricKindEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<ClientMetricKindEnum> _$clientMetricKindEnumValues =
    BuiltSet<ClientMetricKindEnum>(const <ClientMetricKindEnum>[
      _$clientMetricKindEnum_histogram,
      _$clientMetricKindEnum_counter,
      _$clientMetricKindEnum_gauge,
      _$clientMetricKindEnum_unknownDefaultOpenApi,
    ]);

Serializer<ClientMetricKindEnum> _$clientMetricKindEnumSerializer =
    _$ClientMetricKindEnumSerializer();

class _$ClientMetricKindEnumSerializer
    implements PrimitiveSerializer<ClientMetricKindEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'histogram': 'histogram',
    'counter': 'counter',
    'gauge': 'gauge',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'histogram': 'histogram',
    'counter': 'counter',
    'gauge': 'gauge',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[ClientMetricKindEnum];
  @override
  final String wireName = 'ClientMetricKindEnum';

  @override
  Object serialize(
    Serializers serializers,
    ClientMetricKindEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  ClientMetricKindEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => ClientMetricKindEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$ClientMetric extends ClientMetric {
  @override
  final String name;
  @override
  final ClientMetricKindEnum kind;
  @override
  final int? closeCode;
  @override
  final double? value;
  @override
  final BuiltList<ClientMetricBucketsInner>? buckets;

  factory _$ClientMetric([void Function(ClientMetricBuilder)? updates]) =>
      (ClientMetricBuilder()..update(updates))._build();

  _$ClientMetric._({
    required this.name,
    required this.kind,
    this.closeCode,
    this.value,
    this.buckets,
  }) : super._();
  @override
  ClientMetric rebuild(void Function(ClientMetricBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ClientMetricBuilder toBuilder() => ClientMetricBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ClientMetric &&
        name == other.name &&
        kind == other.kind &&
        closeCode == other.closeCode &&
        value == other.value &&
        buckets == other.buckets;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, name.hashCode);
    _$hash = $jc(_$hash, kind.hashCode);
    _$hash = $jc(_$hash, closeCode.hashCode);
    _$hash = $jc(_$hash, value.hashCode);
    _$hash = $jc(_$hash, buckets.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ClientMetric')
          ..add('name', name)
          ..add('kind', kind)
          ..add('closeCode', closeCode)
          ..add('value', value)
          ..add('buckets', buckets))
        .toString();
  }
}

class ClientMetricBuilder
    implements Builder<ClientMetric, ClientMetricBuilder> {
  _$ClientMetric? _$v;

  String? _name;
  String? get name => _$this._name;
  set name(String? name) => _$this._name = name;

  ClientMetricKindEnum? _kind;
  ClientMetricKindEnum? get kind => _$this._kind;
  set kind(ClientMetricKindEnum? kind) => _$this._kind = kind;

  int? _closeCode;
  int? get closeCode => _$this._closeCode;
  set closeCode(int? closeCode) => _$this._closeCode = closeCode;

  double? _value;
  double? get value => _$this._value;
  set value(double? value) => _$this._value = value;

  ListBuilder<ClientMetricBucketsInner>? _buckets;
  ListBuilder<ClientMetricBucketsInner> get buckets =>
      _$this._buckets ??= ListBuilder<ClientMetricBucketsInner>();
  set buckets(ListBuilder<ClientMetricBucketsInner>? buckets) =>
      _$this._buckets = buckets;

  ClientMetricBuilder() {
    ClientMetric._defaults(this);
  }

  ClientMetricBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _name = $v.name;
      _kind = $v.kind;
      _closeCode = $v.closeCode;
      _value = $v.value;
      _buckets = $v.buckets?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ClientMetric other) {
    _$v = other as _$ClientMetric;
  }

  @override
  void update(void Function(ClientMetricBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ClientMetric build() => _build();

  _$ClientMetric _build() {
    _$ClientMetric _$result;
    try {
      _$result =
          _$v ??
          _$ClientMetric._(
            name: BuiltValueNullFieldError.checkNotNull(
              name,
              r'ClientMetric',
              'name',
            ),
            kind: BuiltValueNullFieldError.checkNotNull(
              kind,
              r'ClientMetric',
              'kind',
            ),
            closeCode: closeCode,
            value: value,
            buckets: _buckets?.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'buckets';
        _buckets?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'ClientMetric',
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
