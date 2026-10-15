// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'user_status.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const UserStatus _$pending = const UserStatus._('pending');
const UserStatus _$active = const UserStatus._('active');
const UserStatus _$disabled = const UserStatus._('disabled');
const UserStatus _$deleted = const UserStatus._('deleted');
const UserStatus _$unknownDefaultOpenApi = const UserStatus._(
  'unknownDefaultOpenApi',
);

UserStatus _$valueOf(String name) {
  switch (name) {
    case 'pending':
      return _$pending;
    case 'active':
      return _$active;
    case 'disabled':
      return _$disabled;
    case 'deleted':
      return _$deleted;
    case 'unknownDefaultOpenApi':
      return _$unknownDefaultOpenApi;
    default:
      return _$unknownDefaultOpenApi;
  }
}

final BuiltSet<UserStatus> _$values = BuiltSet<UserStatus>(const <UserStatus>[
  _$pending,
  _$active,
  _$disabled,
  _$deleted,
  _$unknownDefaultOpenApi,
]);

class _$UserStatusMeta {
  const _$UserStatusMeta();
  UserStatus get pending => _$pending;
  UserStatus get active => _$active;
  UserStatus get disabled => _$disabled;
  UserStatus get deleted => _$deleted;
  UserStatus get unknownDefaultOpenApi => _$unknownDefaultOpenApi;
  UserStatus valueOf(String name) => _$valueOf(name);
  BuiltSet<UserStatus> get values => _$values;
}

mixin _$UserStatusMixin {
  // ignore: non_constant_identifier_names
  _$UserStatusMeta get UserStatus => const _$UserStatusMeta();
}

Serializer<UserStatus> _$userStatusSerializer = _$UserStatusSerializer();

class _$UserStatusSerializer implements PrimitiveSerializer<UserStatus> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'pending': 'pending',
    'active': 'active',
    'disabled': 'disabled',
    'deleted': 'deleted',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'pending': 'pending',
    'active': 'active',
    'disabled': 'disabled',
    'deleted': 'deleted',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[UserStatus];
  @override
  final String wireName = 'UserStatus';

  @override
  Object serialize(
    Serializers serializers,
    UserStatus object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  UserStatus deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => UserStatus.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
