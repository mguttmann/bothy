//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'user_status.g.dart';

class UserStatus extends EnumClass {
  @BuiltValueEnumConst(wireName: r'pending')
  static const UserStatus pending = _$pending;
  @BuiltValueEnumConst(wireName: r'active')
  static const UserStatus active = _$active;
  @BuiltValueEnumConst(wireName: r'disabled')
  static const UserStatus disabled = _$disabled;
  @BuiltValueEnumConst(wireName: r'deleted')
  static const UserStatus deleted = _$deleted;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const UserStatus unknownDefaultOpenApi = _$unknownDefaultOpenApi;

  static Serializer<UserStatus> get serializer => _$userStatusSerializer;

  const UserStatus._(String name) : super(name);

  static BuiltSet<UserStatus> get values => _$values;
  static UserStatus valueOf(String name) => _$valueOf(name);
}

/// Optionally, enum_class can generate a mixin to go with your enum for use
/// with Angular. It exposes your enum constants as getters. So, if you mix it
/// in to your Dart component class, the values become available to the
/// corresponding Angular template.
///
/// Trigger mixin generation by writing a line like this one next to your enum.
abstract class UserStatusMixin = Object with _$UserStatusMixin;
