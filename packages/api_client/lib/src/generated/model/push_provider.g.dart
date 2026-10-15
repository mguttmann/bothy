// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'push_provider.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const PushProvider _$apns = const PushProvider._('apns');
const PushProvider _$fcm = const PushProvider._('fcm');
const PushProvider _$unifiedpush = const PushProvider._('unifiedpush');
const PushProvider _$wns = const PushProvider._('wns');
const PushProvider _$unknownDefaultOpenApi = const PushProvider._(
  'unknownDefaultOpenApi',
);

PushProvider _$valueOf(String name) {
  switch (name) {
    case 'apns':
      return _$apns;
    case 'fcm':
      return _$fcm;
    case 'unifiedpush':
      return _$unifiedpush;
    case 'wns':
      return _$wns;
    case 'unknownDefaultOpenApi':
      return _$unknownDefaultOpenApi;
    default:
      return _$unknownDefaultOpenApi;
  }
}

final BuiltSet<PushProvider> _$values = BuiltSet<PushProvider>(
  const <PushProvider>[
    _$apns,
    _$fcm,
    _$unifiedpush,
    _$wns,
    _$unknownDefaultOpenApi,
  ],
);

class _$PushProviderMeta {
  const _$PushProviderMeta();
  PushProvider get apns => _$apns;
  PushProvider get fcm => _$fcm;
  PushProvider get unifiedpush => _$unifiedpush;
  PushProvider get wns => _$wns;
  PushProvider get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  PushProvider valueOf(String name) => _$valueOf(name);
  BuiltSet<PushProvider> get values => _$values;
}

mixin _$PushProviderMixin {
  // ignore: non_constant_identifier_names
  _$PushProviderMeta get PushProvider => const _$PushProviderMeta();
}

Serializer<PushProvider> _$pushProviderSerializer = _$PushProviderSerializer();

class _$PushProviderSerializer implements PrimitiveSerializer<PushProvider> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'apns': 'apns',
    'fcm': 'fcm',
    'unifiedpush': 'unifiedpush',
    'wns': 'wns',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'apns': 'apns',
    'fcm': 'fcm',
    'unifiedpush': 'unifiedpush',
    'wns': 'wns',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[PushProvider];
  @override
  final String wireName = 'PushProvider';

  @override
  Object serialize(
    Serializers serializers,
    PushProvider object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  PushProvider deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => PushProvider.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
