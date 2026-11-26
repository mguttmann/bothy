//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/host_platform.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'computer_desktop.g.dart';

/// ComputerDesktop
///
/// Properties:
/// * [hostId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [name]
/// * [platform]
/// * [online]
/// * [locked] - The screen is locked (`report.computer.locked`).
/// * [busy] - Another computer session on the host is not closed.
@BuiltValue()
abstract class ComputerDesktop
    implements Built<ComputerDesktop, ComputerDesktopBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'host_id')
  String get hostId;

  @BuiltValueField(wireName: r'name')
  String get name;

  @BuiltValueField(wireName: r'platform')
  HostPlatform get platform;
  // enum platformEnum {  macos,  linux,  windows,  };

  @BuiltValueField(wireName: r'online')
  bool get online;

  /// The screen is locked (`report.computer.locked`).
  @BuiltValueField(wireName: r'locked')
  bool get locked;

  /// Another computer session on the host is not closed.
  @BuiltValueField(wireName: r'busy')
  bool get busy;

  ComputerDesktop._();

  factory ComputerDesktop([void updates(ComputerDesktopBuilder b)]) =
      _$ComputerDesktop;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(ComputerDesktopBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<ComputerDesktop> get serializer =>
      _$ComputerDesktopSerializer();
}

class _$ComputerDesktopSerializer
    implements PrimitiveSerializer<ComputerDesktop> {
  @override
  final Iterable<Type> types = const [ComputerDesktop, _$ComputerDesktop];

  @override
  final String wireName = r'ComputerDesktop';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    ComputerDesktop object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'host_id';
    yield serializers.serialize(
      object.hostId,
      specifiedType: const FullType(String),
    );
    yield r'name';
    yield serializers.serialize(
      object.name,
      specifiedType: const FullType(String),
    );
    yield r'platform';
    yield serializers.serialize(
      object.platform,
      specifiedType: const FullType(HostPlatform),
    );
    yield r'online';
    yield serializers.serialize(
      object.online,
      specifiedType: const FullType(bool),
    );
    yield r'locked';
    yield serializers.serialize(
      object.locked,
      specifiedType: const FullType(bool),
    );
    yield r'busy';
    yield serializers.serialize(
      object.busy,
      specifiedType: const FullType(bool),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    ComputerDesktop object, {
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
    required ComputerDesktopBuilder result,
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
        case r'name':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.name = valueDes;
          break;
        case r'platform':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(HostPlatform),
          ) as HostPlatform;
          result.platform = valueDes;
          break;
        case r'online':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.online = valueDes;
          break;
        case r'locked':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.locked = valueDes;
          break;
        case r'busy':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.busy = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  ComputerDesktop deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = ComputerDesktopBuilder();
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
