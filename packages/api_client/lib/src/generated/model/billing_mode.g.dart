// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'billing_mode.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const BillingMode _$personal = const BillingMode._('personal');
const BillingMode _$organization = const BillingMode._('organization');
const BillingMode _$unknownDefaultOpenApi = const BillingMode._(
  'unknownDefaultOpenApi',
);

BillingMode _$valueOf(String name) {
  switch (name) {
    case 'personal':
      return _$personal;
    case 'organization':
      return _$organization;
    case 'unknownDefaultOpenApi':
      return _$unknownDefaultOpenApi;
    default:
      return _$unknownDefaultOpenApi;
  }
}

final BuiltSet<BillingMode> _$values = BuiltSet<BillingMode>(
  const <BillingMode>[_$personal, _$organization, _$unknownDefaultOpenApi],
);

class _$BillingModeMeta {
  const _$BillingModeMeta();
  BillingMode get personal => _$personal;
  BillingMode get organization => _$organization;
  BillingMode get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  BillingMode valueOf(String name) => _$valueOf(name);
  BuiltSet<BillingMode> get values => _$values;
}

mixin _$BillingModeMixin {
  // ignore: non_constant_identifier_names
  _$BillingModeMeta get BillingMode => const _$BillingModeMeta();
}

Serializer<BillingMode> _$billingModeSerializer = _$BillingModeSerializer();

class _$BillingModeSerializer implements PrimitiveSerializer<BillingMode> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'personal': 'personal',
    'organization': 'organization',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'personal': 'personal',
    'organization': 'organization',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[BillingMode];
  @override
  final String wireName = 'BillingMode';

  @override
  Object serialize(
    Serializers serializers,
    BillingMode object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  BillingMode deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => BillingMode.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
