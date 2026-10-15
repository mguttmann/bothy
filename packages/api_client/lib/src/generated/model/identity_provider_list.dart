//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/identity_provider.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'identity_provider_list.g.dart';

/// IdentityProviderList
///
/// Properties:
/// * [items]
@BuiltValue()
abstract class IdentityProviderList
    implements Built<IdentityProviderList, IdentityProviderListBuilder> {
  @BuiltValueField(wireName: r'items')
  BuiltList<IdentityProvider> get items;

  IdentityProviderList._();

  factory IdentityProviderList([void updates(IdentityProviderListBuilder b)]) =
      _$IdentityProviderList;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(IdentityProviderListBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<IdentityProviderList> get serializer =>
      _$IdentityProviderListSerializer();
}

class _$IdentityProviderListSerializer
    implements PrimitiveSerializer<IdentityProviderList> {
  @override
  final Iterable<Type> types = const [
    IdentityProviderList,
    _$IdentityProviderList,
  ];

  @override
  final String wireName = r'IdentityProviderList';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    IdentityProviderList object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'items';
    yield serializers.serialize(
      object.items,
      specifiedType: const FullType(BuiltList, [FullType(IdentityProvider)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    IdentityProviderList object, {
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
    required IdentityProviderListBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'items':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [
              FullType(IdentityProvider),
            ]),
          ) as BuiltList<IdentityProvider>;
          result.items.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  IdentityProviderList deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = IdentityProviderListBuilder();
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
