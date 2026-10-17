//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/sidebar_state.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'sidebar_payload.g.dart';

/// SidebarPayload
///
/// Properties:
/// * [state]
@BuiltValue()
abstract class SidebarPayload
    implements Built<SidebarPayload, SidebarPayloadBuilder> {
  @BuiltValueField(wireName: r'state')
  SidebarState get state;

  SidebarPayload._();

  factory SidebarPayload([void updates(SidebarPayloadBuilder b)]) =
      _$SidebarPayload;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(SidebarPayloadBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<SidebarPayload> get serializer =>
      _$SidebarPayloadSerializer();
}

class _$SidebarPayloadSerializer
    implements PrimitiveSerializer<SidebarPayload> {
  @override
  final Iterable<Type> types = const [SidebarPayload, _$SidebarPayload];

  @override
  final String wireName = r'SidebarPayload';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    SidebarPayload object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'state';
    yield serializers.serialize(
      object.state,
      specifiedType: const FullType(SidebarState),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    SidebarPayload object, {
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
    required SidebarPayloadBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'state':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(SidebarState),
          ) as SidebarState;
          result.state.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  SidebarPayload deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = SidebarPayloadBuilder();
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
