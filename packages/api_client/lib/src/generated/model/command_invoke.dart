//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/command_option_value.dart';
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'command_invoke.g.dart';

/// CommandInvoke
///
/// Properties:
/// * [threadId] - Absent invokes in the main thread or creates a thread per `auto_thread`.
/// * [options]
@BuiltValue()
abstract class CommandInvoke
    implements Built<CommandInvoke, CommandInvokeBuilder> {
  /// Absent invokes in the main thread or creates a thread per `auto_thread`.
  @BuiltValueField(wireName: r'thread_id')
  String? get threadId;

  @BuiltValueField(wireName: r'options')
  BuiltList<CommandOptionValue>? get options;

  CommandInvoke._();

  factory CommandInvoke([void updates(CommandInvokeBuilder b)]) =
      _$CommandInvoke;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(CommandInvokeBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<CommandInvoke> get serializer =>
      _$CommandInvokeSerializer();
}

class _$CommandInvokeSerializer implements PrimitiveSerializer<CommandInvoke> {
  @override
  final Iterable<Type> types = const [CommandInvoke, _$CommandInvoke];

  @override
  final String wireName = r'CommandInvoke';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    CommandInvoke object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    if (object.threadId != null) {
      yield r'thread_id';
      yield serializers.serialize(
        object.threadId,
        specifiedType: const FullType(String),
      );
    }
    if (object.options != null) {
      yield r'options';
      yield serializers.serialize(
        object.options,
        specifiedType: const FullType(BuiltList, [
          FullType(CommandOptionValue),
        ]),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    CommandInvoke object, {
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
    required CommandInvokeBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'thread_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.threadId = valueDes;
          break;
        case r'options':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(CommandOptionValue),
            ]),
          ) as BuiltList<CommandOptionValue>?;
          if (valueDes == null) continue;
          result.options.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  CommandInvoke deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = CommandInvokeBuilder();
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
