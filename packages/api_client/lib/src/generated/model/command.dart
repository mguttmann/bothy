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

part 'command.g.dart';

/// Command
///
/// Properties:
/// * [id] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [ownerPrincipalId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [name]
/// * [description]
/// * [scopeKind]
/// * [scopeId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [options]
/// * [requiredLevel]
/// * [control]
/// * [version]
@BuiltValue()
abstract class Command implements Built<Command, CommandBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'id')
  String get id;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'owner_principal_id')
  String get ownerPrincipalId;

  @BuiltValueField(wireName: r'name')
  String get name;

  @BuiltValueField(wireName: r'description')
  String get description;

  @BuiltValueField(wireName: r'scope_kind')
  TargetKind get scopeKind;
  // enum scopeKindEnum {  instance,  space,  channel,  host,  };

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'scope_id')
  String? get scopeId;

  @BuiltValueField(wireName: r'options')
  BuiltList<CommandOption> get options;

  @BuiltValueField(wireName: r'required_level')
  CommandLevel get requiredLevel;
  // enum requiredLevelEnum {  write,  trigger,  };

  @BuiltValueField(wireName: r'control')
  bool get control;

  @BuiltValueField(wireName: r'version')
  int get version;

  Command._();

  factory Command([void updates(CommandBuilder b)]) = _$Command;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(CommandBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<Command> get serializer => _$CommandSerializer();
}

class _$CommandSerializer implements PrimitiveSerializer<Command> {
  @override
  final Iterable<Type> types = const [Command, _$Command];

  @override
  final String wireName = r'Command';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    Command object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'id';
    yield serializers.serialize(
      object.id,
      specifiedType: const FullType(String),
    );
    yield r'owner_principal_id';
    yield serializers.serialize(
      object.ownerPrincipalId,
      specifiedType: const FullType(String),
    );
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
    yield r'scope_kind';
    yield serializers.serialize(
      object.scopeKind,
      specifiedType: const FullType(TargetKind),
    );
    if (object.scopeId != null) {
      yield r'scope_id';
      yield serializers.serialize(
        object.scopeId,
        specifiedType: const FullType(String),
      );
    }
    yield r'options';
    yield serializers.serialize(
      object.options,
      specifiedType: const FullType(BuiltList, [FullType(CommandOption)]),
    );
    yield r'required_level';
    yield serializers.serialize(
      object.requiredLevel,
      specifiedType: const FullType(CommandLevel),
    );
    yield r'control';
    yield serializers.serialize(
      object.control,
      specifiedType: const FullType(bool),
    );
    yield r'version';
    yield serializers.serialize(
      object.version,
      specifiedType: const FullType(int),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    Command object, {
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
    required CommandBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.id = valueDes;
          break;
        case r'owner_principal_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.ownerPrincipalId = valueDes;
          break;
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
            specifiedType: const FullType(TargetKind),
          ) as TargetKind;
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
            specifiedType: const FullType(BuiltList, [FullType(CommandOption)]),
          ) as BuiltList<CommandOption>;
          result.options.replace(valueDes);
          break;
        case r'required_level':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(CommandLevel),
          ) as CommandLevel;
          result.requiredLevel = valueDes;
          break;
        case r'control':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.control = valueDes;
          break;
        case r'version':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.version = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  Command deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = CommandBuilder();
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
