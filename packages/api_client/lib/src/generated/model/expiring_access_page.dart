//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/access_labels.dart';
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/expiring_access_item.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'expiring_access_page.g.dart';

/// One page. `next_cursor` is absent on the last page.
///
/// Properties:
/// * [items]
/// * [labels]
/// * [nextCursor] - Opaque pagination cursor. Clients must not parse or build cursors.
@BuiltValue()
abstract class ExpiringAccessPage
    implements Built<ExpiringAccessPage, ExpiringAccessPageBuilder> {
  @BuiltValueField(wireName: r'items')
  BuiltList<ExpiringAccessItem> get items;

  @BuiltValueField(wireName: r'labels')
  AccessLabels get labels;

  /// Opaque pagination cursor. Clients must not parse or build cursors.
  @BuiltValueField(wireName: r'next_cursor')
  String? get nextCursor;

  ExpiringAccessPage._();

  factory ExpiringAccessPage([void updates(ExpiringAccessPageBuilder b)]) =
      _$ExpiringAccessPage;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ExpiringAccessPageBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ExpiringAccessPage> get serializer =>
      _$ExpiringAccessPageSerializer();
}

class _$ExpiringAccessPageSerializer
    implements PrimitiveSerializer<ExpiringAccessPage> {
  @override
  final Iterable<Type> types = const [ExpiringAccessPage, _$ExpiringAccessPage];

  @override
  final String wireName = r'ExpiringAccessPage';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ExpiringAccessPage object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'items';
    yield serializers.serialize(
      object.items,
      specifiedType: const FullType(BuiltList, [FullType(ExpiringAccessItem)]),
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
    ExpiringAccessPage object, {
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
    required ExpiringAccessPageBuilder result,
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
              FullType(ExpiringAccessItem),
            ]),
          ) as BuiltList<ExpiringAccessItem>;
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
  ExpiringAccessPage deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ExpiringAccessPageBuilder();
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
