//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'identity_provider_kind.g.dart';

/// Kind of identity provider (B9).
class IdentityProviderKind extends EnumClass {
  @BuiltValueEnumConst(wireName: r'oidc')
  static const IdentityProviderKind oidc = _$oidc;
  @BuiltValueEnumConst(wireName: r'trusted_header')
  static const IdentityProviderKind trustedHeader = _$trustedHeader;
  @BuiltValueEnumConst(wireName: r'local')
  static const IdentityProviderKind local = _$local;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const IdentityProviderKind unknownDefaultOpenApi =
      _$unknownDefaultOpenApi;

  static Serializer<IdentityProviderKind> get serializer =>
      _$identityProviderKindSerializer;

  const IdentityProviderKind._(String name) : super(name);

  static BuiltSet<IdentityProviderKind> get values => _$values;
  static IdentityProviderKind valueOf(String name) => _$valueOf(name);
}

/// Optionally, enum_class can generate a mixin to go with your enum for use
/// with Angular. It exposes your enum constants as getters. So, if you mix it
/// in to your Dart component class, the values become available to the
/// corresponding Angular template.
///
/// Trigger mixin generation by writing a line like this one next to your enum.
abstract class IdentityProviderKindMixin = Object
    with _$IdentityProviderKindMixin;
