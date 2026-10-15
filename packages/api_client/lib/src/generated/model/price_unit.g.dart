// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'price_unit.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const PriceUnit _$inputMtok = const PriceUnit._('inputMtok');
const PriceUnit _$outputMtok = const PriceUnit._('outputMtok');
const PriceUnit _$cacheReadMtok = const PriceUnit._('cacheReadMtok');
const PriceUnit _$cacheWriteMtok = const PriceUnit._('cacheWriteMtok');
const PriceUnit _$cacheWrite1hMtok = const PriceUnit._('cacheWrite1hMtok');
const PriceUnit _$request = const PriceUnit._('request');
const PriceUnit _$unknownDefaultOpenApi = const PriceUnit._(
  'unknownDefaultOpenApi',
);

PriceUnit _$valueOf(String name) {
  switch (name) {
    case 'inputMtok':
      return _$inputMtok;
    case 'outputMtok':
      return _$outputMtok;
    case 'cacheReadMtok':
      return _$cacheReadMtok;
    case 'cacheWriteMtok':
      return _$cacheWriteMtok;
    case 'cacheWrite1hMtok':
      return _$cacheWrite1hMtok;
    case 'request':
      return _$request;
    case 'unknownDefaultOpenApi':
      return _$unknownDefaultOpenApi;
    default:
      return _$unknownDefaultOpenApi;
  }
}

final BuiltSet<PriceUnit> _$values = BuiltSet<PriceUnit>(const <PriceUnit>[
  _$inputMtok,
  _$outputMtok,
  _$cacheReadMtok,
  _$cacheWriteMtok,
  _$cacheWrite1hMtok,
  _$request,
  _$unknownDefaultOpenApi,
]);

class _$PriceUnitMeta {
  const _$PriceUnitMeta();
  PriceUnit get inputMtok => _$inputMtok;
  PriceUnit get outputMtok => _$outputMtok;
  PriceUnit get cacheReadMtok => _$cacheReadMtok;
  PriceUnit get cacheWriteMtok => _$cacheWriteMtok;
  PriceUnit get cacheWrite1hMtok => _$cacheWrite1hMtok;
  PriceUnit get request => _$request;
  PriceUnit get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  PriceUnit valueOf(String name) => _$valueOf(name);
  BuiltSet<PriceUnit> get values => _$values;
}

mixin _$PriceUnitMixin {
  // ignore: non_constant_identifier_names
  _$PriceUnitMeta get PriceUnit => const _$PriceUnitMeta();
}

Serializer<PriceUnit> _$priceUnitSerializer = _$PriceUnitSerializer();

class _$PriceUnitSerializer implements PrimitiveSerializer<PriceUnit> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'inputMtok': 'input_mtok',
    'outputMtok': 'output_mtok',
    'cacheReadMtok': 'cache_read_mtok',
    'cacheWriteMtok': 'cache_write_mtok',
    'cacheWrite1hMtok': 'cache_write_1h_mtok',
    'request': 'request',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'input_mtok': 'inputMtok',
    'output_mtok': 'outputMtok',
    'cache_read_mtok': 'cacheReadMtok',
    'cache_write_mtok': 'cacheWriteMtok',
    'cache_write_1h_mtok': 'cacheWrite1hMtok',
    'request': 'request',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[PriceUnit];
  @override
  final String wireName = 'PriceUnit';

  @override
  Object serialize(
    Serializers serializers,
    PriceUnit object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  PriceUnit deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => PriceUnit.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
