// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'app_scope.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const AppScope _$messagesColonRead = const AppScope._('messagesColonRead');
const AppScope _$messagesColonWrite = const AppScope._('messagesColonWrite');
const AppScope _$filesColonRead = const AppScope._('filesColonRead');
const AppScope _$filesColonWrite = const AppScope._('filesColonWrite');
const AppScope _$commandsColonWrite = const AppScope._('commandsColonWrite');
const AppScope _$componentsColonWrite = const AppScope._(
  'componentsColonWrite',
);
const AppScope _$usersColonRead = const AppScope._('usersColonRead');
const AppScope _$eventsColonSubscribe = const AppScope._(
  'eventsColonSubscribe',
);
const AppScope _$costsColonReconcile = const AppScope._('costsColonReconcile');
const AppScope _$unknownDefaultOpenApi = const AppScope._(
  'unknownDefaultOpenApi',
);

AppScope _$valueOf(String name) {
  switch (name) {
    case 'messagesColonRead':
      return _$messagesColonRead;
    case 'messagesColonWrite':
      return _$messagesColonWrite;
    case 'filesColonRead':
      return _$filesColonRead;
    case 'filesColonWrite':
      return _$filesColonWrite;
    case 'commandsColonWrite':
      return _$commandsColonWrite;
    case 'componentsColonWrite':
      return _$componentsColonWrite;
    case 'usersColonRead':
      return _$usersColonRead;
    case 'eventsColonSubscribe':
      return _$eventsColonSubscribe;
    case 'costsColonReconcile':
      return _$costsColonReconcile;
    case 'unknownDefaultOpenApi':
      return _$unknownDefaultOpenApi;
    default:
      return _$unknownDefaultOpenApi;
  }
}

final BuiltSet<AppScope> _$values = BuiltSet<AppScope>(const <AppScope>[
  _$messagesColonRead,
  _$messagesColonWrite,
  _$filesColonRead,
  _$filesColonWrite,
  _$commandsColonWrite,
  _$componentsColonWrite,
  _$usersColonRead,
  _$eventsColonSubscribe,
  _$costsColonReconcile,
  _$unknownDefaultOpenApi,
]);

class _$AppScopeMeta {
  const _$AppScopeMeta();
  AppScope get messagesColonRead => _$messagesColonRead;
  AppScope get messagesColonWrite => _$messagesColonWrite;
  AppScope get filesColonRead => _$filesColonRead;
  AppScope get filesColonWrite => _$filesColonWrite;
  AppScope get commandsColonWrite => _$commandsColonWrite;
  AppScope get componentsColonWrite => _$componentsColonWrite;
  AppScope get usersColonRead => _$usersColonRead;
  AppScope get eventsColonSubscribe => _$eventsColonSubscribe;
  AppScope get costsColonReconcile => _$costsColonReconcile;
  AppScope get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  AppScope valueOf(String name) => _$valueOf(name);
  BuiltSet<AppScope> get values => _$values;
}

mixin _$AppScopeMixin {
  // ignore: non_constant_identifier_names
  _$AppScopeMeta get AppScope => const _$AppScopeMeta();
}

Serializer<AppScope> _$appScopeSerializer = _$AppScopeSerializer();

class _$AppScopeSerializer implements PrimitiveSerializer<AppScope> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'messagesColonRead': 'messages:read',
    'messagesColonWrite': 'messages:write',
    'filesColonRead': 'files:read',
    'filesColonWrite': 'files:write',
    'commandsColonWrite': 'commands:write',
    'componentsColonWrite': 'components:write',
    'usersColonRead': 'users:read',
    'eventsColonSubscribe': 'events:subscribe',
    'costsColonReconcile': 'costs:reconcile',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'messages:read': 'messagesColonRead',
    'messages:write': 'messagesColonWrite',
    'files:read': 'filesColonRead',
    'files:write': 'filesColonWrite',
    'commands:write': 'commandsColonWrite',
    'components:write': 'componentsColonWrite',
    'users:read': 'usersColonRead',
    'events:subscribe': 'eventsColonSubscribe',
    'costs:reconcile': 'costsColonReconcile',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[AppScope];
  @override
  final String wireName = 'AppScope';

  @override
  Object serialize(
    Serializers serializers,
    AppScope object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  AppScope deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => AppScope.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
