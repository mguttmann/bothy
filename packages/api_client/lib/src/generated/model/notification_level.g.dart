// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'notification_level.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const NotificationLevel _$all = const NotificationLevel._('all');
const NotificationLevel _$mentions = const NotificationLevel._('mentions');
const NotificationLevel _$mute = const NotificationLevel._('mute');
const NotificationLevel _$unknownDefaultOpenApi = const NotificationLevel._(
  'unknownDefaultOpenApi',
);

NotificationLevel _$valueOf(String name) {
  switch (name) {
    case 'all':
      return _$all;
    case 'mentions':
      return _$mentions;
    case 'mute':
      return _$mute;
    case 'unknownDefaultOpenApi':
      return _$unknownDefaultOpenApi;
    default:
      return _$unknownDefaultOpenApi;
  }
}

final BuiltSet<NotificationLevel> _$values = BuiltSet<NotificationLevel>(
  const <NotificationLevel>[_$all, _$mentions, _$mute, _$unknownDefaultOpenApi],
);

class _$NotificationLevelMeta {
  const _$NotificationLevelMeta();
  NotificationLevel get all => _$all;
  NotificationLevel get mentions => _$mentions;
  NotificationLevel get mute => _$mute;
  NotificationLevel get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  NotificationLevel valueOf(String name) => _$valueOf(name);
  BuiltSet<NotificationLevel> get values => _$values;
}

mixin _$NotificationLevelMixin {
  // ignore: non_constant_identifier_names
  _$NotificationLevelMeta get NotificationLevel =>
      const _$NotificationLevelMeta();
}

Serializer<NotificationLevel> _$notificationLevelSerializer =
    _$NotificationLevelSerializer();

class _$NotificationLevelSerializer
    implements PrimitiveSerializer<NotificationLevel> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'all': 'all',
    'mentions': 'mentions',
    'mute': 'mute',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'all': 'all',
    'mentions': 'mentions',
    'mute': 'mute',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[NotificationLevel];
  @override
  final String wireName = 'NotificationLevel';

  @override
  Object serialize(
    Serializers serializers,
    NotificationLevel object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  NotificationLevel deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => NotificationLevel.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
