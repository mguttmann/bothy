//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/access_labels.dart';
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/permission_hint.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'permission_hint_page.g.dart';

/// One page. `next_cursor` is absent on the last page.
///
/// Properties:
/// * [items]
/// * [labels]
/// * [nextCursor] - Opaque pagination cursor. Clients must not parse or build cursors.
@BuiltValue()
abstract class PermissionHintPage
    implements Built<PermissionHintPage, PermissionHintPageBuilder> {
  @BuiltValueField(wireName: r'items')
  BuiltList<PermissionHint> get items;

  @BuiltValueField(wireName: r'labels')
  AccessLabels get labels;

  /// Opaque pagination cursor. Clients must not parse or build cursors.
  @BuiltValueField(wireName: r'next_cursor')
  String? get nextCursor;

  PermissionHintPage._();

  factory PermissionHintPage([void updates(PermissionHintPageBuilder b)]) =
      _$PermissionHintPage;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(PermissionHintPageBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<PermissionHintPage> get serializer =>
      _$PermissionHintPageSerializer();
}

class _$PermissionHintPageSerializer
    implements PrimitiveSerializer<PermissionHintPage> {
  @override
  final Iterable<Type> types = const [PermissionHintPage, _$PermissionHintPage];

  @override
  final String wireName = r'PermissionHintPage';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    PermissionHintPage object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'items';
    yield serializers.serialize(
      object.items,
      specifiedType: const FullType(BuiltList, [FullType(PermissionHint)]),
    );
    yield r'labels';
    yield serializers.serialize(
      object.labels,
      specifiedType: const FullType(AccessLabels),
    );
    if (object.nextCursor != null) {
      yield r'next_cursor';
      yield serializers.serialize(
        object.nextCursor,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    PermissionHintPage object, {
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
    required PermissionHintPageBuilder result,
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
              FullType(PermissionHint),
            ]),
          ) as BuiltList<PermissionHint>;
          result.items.replace(valueDes);
          break;
        case r'labels':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(AccessLabels),
          ) as AccessLabels;
          result.labels.replace(valueDes);
          break;
        case r'next_cursor':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.nextCursor = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  PermissionHintPage deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = PermissionHintPageBuilder();
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
