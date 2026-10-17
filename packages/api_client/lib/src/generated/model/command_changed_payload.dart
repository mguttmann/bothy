//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/command.dart';
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'command_changed_payload.g.dart';

/// CommandChangedPayload
///
/// Properties:
/// * [command]
/// * [change]
@BuiltValue()
abstract class CommandChangedPayload
    implements Built<CommandChangedPayload, CommandChangedPayloadBuilder> {
  @BuiltValueField(wireName: r'command')
  Command get command;

  @BuiltValueField(wireName: r'change')
  CommandChangedPayloadChangeEnum get change;
  // enum changeEnum {  registered,  updated,  removed,  };

  CommandChangedPayload._();

  factory CommandChangedPayload([
    void updates(CommandChangedPayloadBuilder b),
  ]) = _$CommandChangedPayload;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(CommandChangedPayloadBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<CommandChangedPayload> get serializer =>
      _$CommandChangedPayloadSerializer();
}

class _$CommandChangedPayloadSerializer
    implements PrimitiveSerializer<CommandChangedPayload> {
  @override
  final Iterable<Type> types = const [
    CommandChangedPayload,
    _$CommandChangedPayload,
  ];

  @override
  final String wireName = r'CommandChangedPayload';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    CommandChangedPayload object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'command';
    yield serializers.serialize(
      object.command,
      specifiedType: const FullType(Command),
    );
    yield r'change';
    yield serializers.serialize(
      object.change,
      specifiedType: const FullType(CommandChangedPayloadChangeEnum),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    CommandChangedPayload object, {
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
    required CommandChangedPayloadBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'command':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(Command),
          ) as Command;
          result.command.replace(valueDes);
          break;
        case r'change':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(CommandChangedPayloadChangeEnum),
          ) as CommandChangedPayloadChangeEnum;
          result.change = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  CommandChangedPayload deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = CommandChangedPayloadBuilder();
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

class CommandChangedPayloadChangeEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'registered')
  static const CommandChangedPayloadChangeEnum registered =
      _$commandChangedPayloadChangeEnum_registered;
  @BuiltValueEnumConst(wireName: r'updated')
  static const CommandChangedPayloadChangeEnum updated =
      _$commandChangedPayloadChangeEnum_updated;
  @BuiltValueEnumConst(wireName: r'removed')
  static const CommandChangedPayloadChangeEnum removed =
      _$commandChangedPayloadChangeEnum_removed;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const CommandChangedPayloadChangeEnum unknownDefaultOpenApi =
      _$commandChangedPayloadChangeEnum_unknownDefaultOpenApi;

  static Serializer<CommandChangedPayloadChangeEnum> get serializer =>
      _$commandChangedPayloadChangeEnumSerializer;

  const CommandChangedPayloadChangeEnum._(String name) : super(name);

  static BuiltSet<CommandChangedPayloadChangeEnum> get values =>
      _$commandChangedPayloadChangeEnumValues;
  static CommandChangedPayloadChangeEnum valueOf(String name) =>
      _$commandChangedPayloadChangeEnumValueOf(name);
}
