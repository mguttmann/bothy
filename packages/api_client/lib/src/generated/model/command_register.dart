//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/target_kind.dart';
import 'package:api_client/src/generated/model/command_option.dart';
import 'package:api_client/src/generated/model/command_level.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'command_register.g.dart';

/// CommandRegister
///
/// Properties:
/// * [name]
/// * [description]
/// * [scopeKind]
/// * [scopeId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [options]
/// * [requiredLevel]
/// * [control]
@BuiltValue()
abstract class CommandRegister
    implements Built<CommandRegister, CommandRegisterBuilder> {
  @BuiltValueField(wireName: r'name')
  String get name;

  @BuiltValueField(wireName: r'description')
  String get description;

  @BuiltValueField(wireName: r'scope_kind')
  TargetKind? get scopeKind;
  // enum scopeKindEnum {  instance,  space,  channel,  host,  };

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'scope_id')
  String? get scopeId;

  @BuiltValueField(wireName: r'options')
  BuiltList<CommandOption>? get options;

  @BuiltValueField(wireName: r'required_level')
  CommandLevel? get requiredLevel;
  // enum requiredLevelEnum {  write,  trigger,  };

  @BuiltValueField(wireName: r'control')
  bool? get control;

  CommandRegister._();

  factory CommandRegister([void updates(CommandRegisterBuilder b)]) =
      _$CommandRegister;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(CommandRegisterBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<CommandRegister> get serializer =>
      _$CommandRegisterSerializer();
}

class _$CommandRegisterSerializer
    implements PrimitiveSerializer<CommandRegister> {
  @override
  final Iterable<Type> types = const [CommandRegister, _$CommandRegister];

  @override
  final String wireName = r'CommandRegister';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    CommandRegister object, {
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
    if (object.scopeKind != null) {
      yield r'scope_kind';
      yield serializers.serialize(
        object.scopeKind,
        specifiedType: const FullType(TargetKind),
      );
    }
    if (object.scopeId != null) {
      yield r'scope_id';
      yield serializers.serialize(
        object.scopeId,
        specifiedType: const FullType(String),
      );
    }
    if (object.options != null) {
      yield r'options';
      yield serializers.serialize(
        object.options,
        specifiedType: const FullType(BuiltList, [FullType(CommandOption)]),
      );
    }
    if (object.requiredLevel != null) {
      yield r'required_level';
      yield serializers.serialize(
        object.requiredLevel,
        specifiedType: const FullType(CommandLevel),
      );
    }
    if (object.control != null) {
      yield r'control';
      yield serializers.serialize(
        object.control,
        specifiedType: const FullType(bool),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    CommandRegister object, {
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
    required CommandRegisterBuilder result,
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
        case r'scope_kind':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(TargetKind),
          ) as TargetKind?;
          if (valueDes == null) continue;
          result.scopeKind = valueDes;
          break;
        case r'scope_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.scopeId = valueDes;
          break;
        case r'options':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(CommandOption),
            ]),
          ) as BuiltList<CommandOption>?;
          if (valueDes == null) continue;
          result.options.replace(valueDes);
          break;
        case r'required_level':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(CommandLevel),
          ) as CommandLevel?;
          if (valueDes == null) continue;
          result.requiredLevel = valueDes;
          break;
        case r'control':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(bool),
          ) as bool?;
          if (valueDes == null) continue;
          result.control = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  CommandRegister deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = CommandRegisterBuilder();
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
