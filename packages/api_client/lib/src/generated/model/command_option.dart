//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/command_choice.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'command_option.g.dart';

/// CommandOption
///
/// Properties:
/// * [name]
/// * [description]
/// * [type]
/// * [required_]
/// * [choices]
/// * [autocomplete]
@BuiltValue()
abstract class CommandOption
    implements Built<CommandOption, CommandOptionBuilder> {
  @BuiltValueField(wireName: r'name')
  String get name;

  @BuiltValueField(wireName: r'description')
  String get description;

  @BuiltValueField(wireName: r'type')
  CommandOptionTypeEnum get type;
  // enum typeEnum {  string,  integer,  boolean,  choice,  user,  file,  };

  @BuiltValueField(wireName: r'required')
  bool get required_;

  @BuiltValueField(wireName: r'choices')
  BuiltList<CommandChoice>? get choices;

  @BuiltValueField(wireName: r'autocomplete')
  bool? get autocomplete;

  CommandOption._();

  factory CommandOption([void updates(CommandOptionBuilder b)]) =
      _$CommandOption;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(CommandOptionBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<CommandOption> get serializer =>
      _$CommandOptionSerializer();
}

class _$CommandOptionSerializer implements PrimitiveSerializer<CommandOption> {
  @override
  final Iterable<Type> types = const [CommandOption, _$CommandOption];

  @override
  final String wireName = r'CommandOption';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    CommandOption object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'name';
    yield serializers.serialize(
      object.name,
      specifiedType: const FullType(String),
    );
    yield r'description';
    yield serializers.serialize(
      object.description,
      specifiedType: const FullType(String),
    );
    yield r'type';
    yield serializers.serialize(
      object.type,
      specifiedType: const FullType(CommandOptionTypeEnum),
    );
    yield r'required';
    yield serializers.serialize(
      object.required_,
      specifiedType: const FullType(bool),
    );
    if (object.choices != null) {
      yield r'choices';
      yield serializers.serialize(
        object.choices,
        specifiedType: const FullType(BuiltList, [FullType(CommandChoice)]),
      );
    }
    if (object.autocomplete != null) {
      yield r'autocomplete';
      yield serializers.serialize(
        object.autocomplete,
        specifiedType: const FullType(bool),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    CommandOption object, {
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
    required CommandOptionBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'name':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.name = valueDes;
          break;
        case r'description':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.description = valueDes;
          break;
        case r'type':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(CommandOptionTypeEnum),
          ) as CommandOptionTypeEnum;
          result.type = valueDes;
          break;
        case r'required':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.required_ = valueDes;
          break;
        case r'choices':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(CommandChoice),
            ]),
          ) as BuiltList<CommandChoice>?;
          if (valueDes == null) continue;
          result.choices.replace(valueDes);
          break;
        case r'autocomplete':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(bool),
          ) as bool?;
          if (valueDes == null) continue;
          result.autocomplete = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  CommandOption deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = CommandOptionBuilder();
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

class CommandOptionTypeEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'string')
  static const CommandOptionTypeEnum string = _$commandOptionTypeEnum_string;
  @BuiltValueEnumConst(wireName: r'integer')
  static const CommandOptionTypeEnum integer = _$commandOptionTypeEnum_integer;
  @BuiltValueEnumConst(wireName: r'boolean')
  static const CommandOptionTypeEnum boolean = _$commandOptionTypeEnum_boolean;
  @BuiltValueEnumConst(wireName: r'choice')
  static const CommandOptionTypeEnum choice = _$commandOptionTypeEnum_choice;
  @BuiltValueEnumConst(wireName: r'user')
  static const CommandOptionTypeEnum user = _$commandOptionTypeEnum_user;
  @BuiltValueEnumConst(wireName: r'file')
  static const CommandOptionTypeEnum file = _$commandOptionTypeEnum_file;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const CommandOptionTypeEnum unknownDefaultOpenApi =
      _$commandOptionTypeEnum_unknownDefaultOpenApi;

  static Serializer<CommandOptionTypeEnum> get serializer =>
      _$commandOptionTypeEnumSerializer;

  const CommandOptionTypeEnum._(String name) : super(name);

  static BuiltSet<CommandOptionTypeEnum> get values =>
      _$commandOptionTypeEnumValues;
  static CommandOptionTypeEnum valueOf(String name) =>
      _$commandOptionTypeEnumValueOf(name);
}
