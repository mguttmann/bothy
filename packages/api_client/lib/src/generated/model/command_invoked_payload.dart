//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/command_option_value.dart';
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/principal_ref.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'command_invoked_payload.g.dart';

/// CommandInvokedPayload
///
/// Properties:
/// * [invocationId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [commandId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [commandName]
/// * [control]
/// * [channelId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [threadId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [runId] - Active run of the command owner in the thread at execution time; absent when the owner has none (R-AZ-25). Never the run of another agent. Always absent for app commands, because apps have no runs.
/// * [invokedBy]
/// * [options]
@BuiltValue()
abstract class CommandInvokedPayload
    implements Built<CommandInvokedPayload, CommandInvokedPayloadBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'invocation_id')
  String get invocationId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'command_id')
  String get commandId;

  @BuiltValueField(wireName: r'command_name')
  String get commandName;

  @BuiltValueField(wireName: r'control')
  bool get control;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'channel_id')
  String get channelId;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'thread_id')
  String? get threadId;

  /// Active run of the command owner in the thread at execution time; absent when the owner has none (R-AZ-25). Never the run of another agent. Always absent for app commands, because apps have no runs.
  @BuiltValueField(wireName: r'run_id')
  String? get runId;

  @BuiltValueField(wireName: r'invoked_by')
  PrincipalRef get invokedBy;

  @BuiltValueField(wireName: r'options')
  BuiltList<CommandOptionValue> get options;

  CommandInvokedPayload._();

  factory CommandInvokedPayload([
    void updates(CommandInvokedPayloadBuilder b),
  ]) = _$CommandInvokedPayload;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(CommandInvokedPayloadBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<CommandInvokedPayload> get serializer =>
      _$CommandInvokedPayloadSerializer();
}

class _$CommandInvokedPayloadSerializer
    implements PrimitiveSerializer<CommandInvokedPayload> {
  @override
  final Iterable<Type> types = const [
    CommandInvokedPayload,
    _$CommandInvokedPayload,
  ];

  @override
  final String wireName = r'CommandInvokedPayload';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    CommandInvokedPayload object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'invocation_id';
    yield serializers.serialize(
      object.invocationId,
      specifiedType: const FullType(String),
    );
    yield r'command_id';
    yield serializers.serialize(
      object.commandId,
      specifiedType: const FullType(String),
    );
    yield r'command_name';
    yield serializers.serialize(
      object.commandName,
      specifiedType: const FullType(String),
    );
    yield r'control';
    yield serializers.serialize(
      object.control,
      specifiedType: const FullType(bool),
    );
    yield r'channel_id';
    yield serializers.serialize(
      object.channelId,
      specifiedType: const FullType(String),
    );
    if (object.threadId != null) {
      yield r'thread_id';
      yield serializers.serialize(
        object.threadId,
        specifiedType: const FullType(String),
      );
    }
    if (object.runId != null) {
      yield r'run_id';
      yield serializers.serialize(
        object.runId,
        specifiedType: const FullType(String),
      );
    }
    yield r'invoked_by';
    yield serializers.serialize(
      object.invokedBy,
      specifiedType: const FullType(PrincipalRef),
    );
    yield r'options';
    yield serializers.serialize(
      object.options,
      specifiedType: const FullType(BuiltList, [FullType(CommandOptionValue)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    CommandInvokedPayload object, {
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
    required CommandInvokedPayloadBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'invocation_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.invocationId = valueDes;
          break;
        case r'command_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.commandId = valueDes;
          break;
        case r'command_name':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.commandName = valueDes;
          break;
        case r'control':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.control = valueDes;
          break;
        case r'channel_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.channelId = valueDes;
          break;
        case r'thread_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.threadId = valueDes;
          break;
        case r'run_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.runId = valueDes;
          break;
        case r'invoked_by':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(PrincipalRef),
          ) as PrincipalRef;
          result.invokedBy.replace(valueDes);
          break;
        case r'options':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [
              FullType(CommandOptionValue),
            ]),
          ) as BuiltList<CommandOptionValue>;
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
  CommandInvokedPayload deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = CommandInvokedPayloadBuilder();
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
