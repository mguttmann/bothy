//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'extension_session_create.g.dart';

/// Child session of the calling app session for an app extension with its own refresh family (08, 7.4).
///
/// Properties:
/// * [clientComponent]
@BuiltValue()
abstract class ExtensionSessionCreate
    implements Built<ExtensionSessionCreate, ExtensionSessionCreateBuilder> {
  @BuiltValueField(wireName: r'client_component')
  ExtensionSessionCreateClientComponentEnum get clientComponent;
  // enum clientComponentEnum {  share_ext,  };

  ExtensionSessionCreate._();

  factory ExtensionSessionCreate([
    void updates(ExtensionSessionCreateBuilder b),
  ]) = _$ExtensionSessionCreate;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ExtensionSessionCreateBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ExtensionSessionCreate> get serializer =>
      _$ExtensionSessionCreateSerializer();
}

class _$ExtensionSessionCreateSerializer
    implements PrimitiveSerializer<ExtensionSessionCreate> {
  @override
  final Iterable<Type> types = const [
    ExtensionSessionCreate,
    _$ExtensionSessionCreate,
  ];

  @override
  final String wireName = r'ExtensionSessionCreate';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ExtensionSessionCreate object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'client_component';
    yield serializers.serialize(
      object.clientComponent,
      specifiedType: const FullType(ExtensionSessionCreateClientComponentEnum),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    ExtensionSessionCreate object, {
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
    required ExtensionSessionCreateBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'client_component':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(
              ExtensionSessionCreateClientComponentEnum,
            ),
          ) as ExtensionSessionCreateClientComponentEnum;
          result.clientComponent = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ExtensionSessionCreate deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ExtensionSessionCreateBuilder();
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

class ExtensionSessionCreateClientComponentEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'share_ext')
  static const ExtensionSessionCreateClientComponentEnum shareExt =
      _$extensionSessionCreateClientComponentEnum_shareExt;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const ExtensionSessionCreateClientComponentEnum unknownDefaultOpenApi =
      _$extensionSessionCreateClientComponentEnum_unknownDefaultOpenApi;

  static Serializer<ExtensionSessionCreateClientComponentEnum> get serializer =>
      _$extensionSessionCreateClientComponentEnumSerializer;

  const ExtensionSessionCreateClientComponentEnum._(String name) : super(name);

  static BuiltSet<ExtensionSessionCreateClientComponentEnum> get values =>
      _$extensionSessionCreateClientComponentEnumValues;
  static ExtensionSessionCreateClientComponentEnum valueOf(String name) =>
      _$extensionSessionCreateClientComponentEnumValueOf(name);
}
