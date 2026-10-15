// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'notification_kind.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const NotificationKind _$message = const NotificationKind._('message');
const NotificationKind _$mention = const NotificationKind._('mention');
const NotificationKind _$dm = const NotificationKind._('dm');
const NotificationKind _$runDone = const NotificationKind._('runDone');
const NotificationKind _$runFailed = const NotificationKind._('runFailed');
const NotificationKind _$inputNeeded = const NotificationKind._('inputNeeded');
const NotificationKind _$security = const NotificationKind._('security');
const NotificationKind _$admin = const NotificationKind._('admin');
const NotificationKind _$unknownDefaultOpenApi = const NotificationKind._(
  'unknownDefaultOpenApi',
);

NotificationKind _$valueOf(String name) {
  switch (name) {
    case 'message':
      return _$message;
    case 'mention':
      return _$mention;
    case 'dm':
      return _$dm;
    case 'runDone':
      return _$runDone;
    case 'runFailed':
      return _$runFailed;
    case 'inputNeeded':
      return _$inputNeeded;
    case 'security':
      return _$security;
    case 'admin':
      return _$admin;
    case 'unknownDefaultOpenApi':
      return _$unknownDefaultOpenApi;
    default:
      return _$unknownDefaultOpenApi;
  }
}

final BuiltSet<NotificationKind> _$values = BuiltSet<NotificationKind>(
  const <NotificationKind>[
    _$message,
    _$mention,
    _$dm,
    _$runDone,
    _$runFailed,
    _$inputNeeded,
    _$security,
    _$admin,
    _$unknownDefaultOpenApi,
  ],
);

class _$NotificationKindMeta {
  const _$NotificationKindMeta();
  NotificationKind get message => _$message;
  NotificationKind get mention => _$mention;
  NotificationKind get dm => _$dm;
  NotificationKind get runDone => _$runDone;
  NotificationKind get runFailed => _$runFailed;
  NotificationKind get inputNeeded => _$inputNeeded;
  NotificationKind get security => _$security;
  NotificationKind get admin => _$admin;
  NotificationKind get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  NotificationKind valueOf(String name) => _$valueOf(name);
  BuiltSet<NotificationKind> get values => _$values;
}

mixin _$NotificationKindMixin {
  // ignore: non_constant_identifier_names
  _$NotificationKindMeta get NotificationKind => const _$NotificationKindMeta();
}

Serializer<NotificationKind> _$notificationKindSerializer =
    _$NotificationKindSerializer();

class _$NotificationKindSerializer
    implements PrimitiveSerializer<NotificationKind> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'message': 'message',
    'mention': 'mention',
    'dm': 'dm',
    'runDone': 'run_done',
    'runFailed': 'run_failed',
    'inputNeeded': 'input_needed',
    'security': 'security',
    'admin': 'admin',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'message': 'message',
    'mention': 'mention',
    'dm': 'dm',
    'run_done': 'runDone',
    'run_failed': 'runFailed',
    'input_needed': 'inputNeeded',
    'security': 'security',
    'admin': 'admin',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[NotificationKind];
  @override
  final String wireName = 'NotificationKind';

  @override
  Object serialize(
    Serializers serializers,
    NotificationKind object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  NotificationKind deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => NotificationKind.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
