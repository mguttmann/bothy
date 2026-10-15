//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/app_scope.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'app_manifest_scopes.g.dart';

/// AppManifestScopes
///
/// Properties:
/// * [required_]
/// * [optional]
@BuiltValue()
abstract class AppManifestScopes
    implements Built<AppManifestScopes, AppManifestScopesBuilder> {
  @BuiltValueField(wireName: r'required')
  BuiltList<AppScope> get required_;

  @BuiltValueField(wireName: r'optional')
  BuiltList<AppScope> get optional;

  AppManifestScopes._();

  factory AppManifestScopes([void updates(AppManifestScopesBuilder b)]) =
      _$AppManifestScopes;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AppManifestScopesBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AppManifestScopes> get serializer =>
      _$AppManifestScopesSerializer();
}

class _$AppManifestScopesSerializer
    implements PrimitiveSerializer<AppManifestScopes> {
  @override
  final Iterable<Type> types = const [AppManifestScopes, _$AppManifestScopes];

  @override
  final String wireName = r'AppManifestScopes';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AppManifestScopes object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'required';
    yield serializers.serialize(
      object.required_,
      specifiedType: const FullType(BuiltList, [FullType(AppScope)]),
    );
    yield r'optional';
    yield serializers.serialize(
      object.optional,
      specifiedType: const FullType(BuiltList, [FullType(AppScope)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    AppManifestScopes object, {
    FullType specifiedType = FullType.unspecified,
  }) {
    return _serializeProperties(
      serializers,
      object,
      specifiedType: specifiedType,
    ).toList();
  }

  void _deserializeProperties(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
    required List<Object?> serializedList,
    required AppManifestScopesBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'required':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(AppScope)]),
          ) as BuiltList<AppScope>;
          result.required_.replace(valueDes);
          break;
        case r'optional':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(AppScope)]),
          ) as BuiltList<AppScope>;
          result.optional.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AppManifestScopes deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AppManifestScopesBuilder();
    final serializedList = (serialized as Iterable<Object?>).toList();
    final unhandled = <Object?>[];
    _deserializeProperties(
      serializers,
      serialized,
      specifiedType: specifiedType,
      serializedList: serializedList,
      unhandled: unhandled,
      result: result,
    );
    return result.build();
  }
}
