//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/computer_desktop.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'computer_desktop_list.g.dart';

/// ComputerDesktopList
///
/// Properties:
/// * [items]
@BuiltValue()
abstract class ComputerDesktopList
    implements Built<ComputerDesktopList, ComputerDesktopListBuilder> {
  @BuiltValueField(wireName: r'items')
  BuiltList<ComputerDesktop> get items;

  ComputerDesktopList._();

  factory ComputerDesktopList([void updates(ComputerDesktopListBuilder b)]) =
      _$ComputerDesktopList;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ComputerDesktopListBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ComputerDesktopList> get serializer =>
      _$ComputerDesktopListSerializer();
}

class _$ComputerDesktopListSerializer
    implements PrimitiveSerializer<ComputerDesktopList> {
  @override
  final Iterable<Type> types = const [
    ComputerDesktopList,
    _$ComputerDesktopList,
  ];

  @override
  final String wireName = r'ComputerDesktopList';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ComputerDesktopList object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'items';
    yield serializers.serialize(
      object.items,
      specifiedType: const FullType(BuiltList, [FullType(ComputerDesktop)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    ComputerDesktopList object, {
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
    required ComputerDesktopListBuilder result,
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
              FullType(ComputerDesktop),
            ]),
          ) as BuiltList<ComputerDesktop>;
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
  ComputerDesktopList deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ComputerDesktopListBuilder();
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
