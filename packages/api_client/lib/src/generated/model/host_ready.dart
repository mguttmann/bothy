//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/llm_credentials.dart';
import 'package:api_client/src/generated/model/host_state.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'host_ready.g.dart';

/// Sent after login and again on every change of `llm_credentials`; the host service treats each `ready` as the current declaration (24, 2.2 and 7.4).
///
/// Properties:
/// * [hostId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [state]
/// * [llmCredentials]
@BuiltValue()
abstract class HostReady implements Built<HostReady, HostReadyBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'host_id')
  String get hostId;

  @BuiltValueField(wireName: r'state')
  HostState get state;
  // enum stateEnum {  pairing,  confirming,  active,  disabled,  retired,  };

  @BuiltValueField(wireName: r'llm_credentials')
  LlmCredentials get llmCredentials;
  // enum llmCredentialsEnum {  none,  personal,  organization,  };

  HostReady._();

  factory HostReady([void updates(HostReadyBuilder b)]) = _$HostReady;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(HostReadyBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<HostReady> get serializer => _$HostReadySerializer();
}

class _$HostReadySerializer implements PrimitiveSerializer<HostReady> {
  @override
  final Iterable<Type> types = const [HostReady, _$HostReady];

  @override
  final String wireName = r'HostReady';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    HostReady object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'host_id';
    yield serializers.serialize(
      object.hostId,
      specifiedType: const FullType(String),
    );
    yield r'state';
    yield serializers.serialize(
      object.state,
      specifiedType: const FullType(HostState),
    );
    yield r'llm_credentials';
    yield serializers.serialize(
      object.llmCredentials,
      specifiedType: const FullType(LlmCredentials),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    HostReady object, {
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
    required HostReadyBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'host_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.hostId = valueDes;
          break;
        case r'state':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(HostState),
          ) as HostState;
          result.state = valueDes;
          break;
        case r'llm_credentials':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(LlmCredentials),
          ) as LlmCredentials;
          result.llmCredentials = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  HostReady deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = HostReadyBuilder();
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
