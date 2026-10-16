//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'login_client.g.dart';

/// `console`: no `device` or DPoP; cookie, no token (08, 7.5).
class LoginClient extends EnumClass {
  @BuiltValueEnumConst(wireName: r'app')
  static const LoginClient app = _$app;
  @BuiltValueEnumConst(wireName: r'console')
  static const LoginClient console = _$console;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const LoginClient unknownDefaultOpenApi = _$unknownDefaultOpenApi;

  static Serializer<LoginClient> get serializer => _$loginClientSerializer;

  const LoginClient._(String name) : super(name);

  static BuiltSet<LoginClient> get values => _$values;
  static LoginClient valueOf(String name) => _$valueOf(name);
}

/// Optionally, enum_class can generate a mixin to go with your enum for use
/// with Angular. It exposes your enum constants as getters. So, if you mix it
/// in to your Dart component class, the values become available to the
/// corresponding Angular template.
///
/// Trigger mixin generation by writing a line like this one next to your enum.
abstract class LoginClientMixin = Object with _$LoginClientMixin;
