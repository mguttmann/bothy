//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/tool_class.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'tool_class_set.g.dart';

/// ToolClassSet
///
/// Properties:
/// * [toolClass]
/// * [reason]
@BuiltValue()
abstract class ToolClassSet
    implements Built<ToolClassSet, ToolClassSetBuilder> {
  @BuiltValueField(wireName: r'tool_class')
  ToolClass get toolClass;
  // enum toolClassEnum {  chat,  scoped,  host,  };

  @BuiltValueField(wireName: r'reason')
  String get reason;

  ToolClassSet._();

  factory ToolClassSet([void updates(ToolClassSetBuilder b)]) = _$ToolClassSet;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ToolClassSetBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ToolClassSet> get serializer => _$ToolClassSetSerializer();
}

class _$ToolClassSetSerializer implements PrimitiveSerializer<ToolClassSet> {
  @override
  final Iterable<Type> types = const [ToolClassSet, _$ToolClassSet];

  @override
  final String wireName = r'ToolClassSet';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ToolClassSet object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'tool_class';
    yield serializers.serialize(
      object.toolClass,
      specifiedType: const FullType(ToolClass),
    );
    yield r'reason';
    yield serializers.serialize(
      object.reason,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    ToolClassSet object, {
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
    required ToolClassSetBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'tool_class':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(ToolClass),
          ) as ToolClass;
          result.toolClass = valueDes;
          break;
        case r'reason':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.reason = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ToolClassSet deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ToolClassSetBuilder();
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
