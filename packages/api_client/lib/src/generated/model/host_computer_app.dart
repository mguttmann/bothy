//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'host_computer_app.g.dart';

/// Identity of a frontmost app (27 12.3 and 13.1); empty strings before the helper named one. Never a window title or a path.
///
/// Properties:
/// * [id] - Bundle ID or executable name.
/// * [name] - Display name.
/// * [signer] - Team ID or subject of the signer.
@BuiltValue()
abstract class HostComputerApp
    implements Built<HostComputerApp, HostComputerAppBuilder> {
  /// Bundle ID or executable name.
  @BuiltValueField(wireName: r'id')
  String get id;

  /// Display name.
  @BuiltValueField(wireName: r'name')
  String get name;

  /// Team ID or subject of the signer.
  @BuiltValueField(wireName: r'signer')
  String get signer;

  HostComputerApp._();

  factory HostComputerApp([void updates(HostComputerAppBuilder b)]) =
      _$HostComputerApp;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(HostComputerAppBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<HostComputerApp> get serializer =>
      _$HostComputerAppSerializer();
}

class _$HostComputerAppSerializer
    implements PrimitiveSerializer<HostComputerApp> {
  @override
  final Iterable<Type> types = const [HostComputerApp, _$HostComputerApp];

  @override
  final String wireName = r'HostComputerApp';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    HostComputerApp object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'id';
    yield serializers.serialize(
      object.id,
      specifiedType: const FullType(String),
    );
    yield r'name';
    yield serializers.serialize(
      object.name,
      specifiedType: const FullType(String),
    );
    yield r'signer';
    yield serializers.serialize(
      object.signer,
      specifiedType: const FullType(String),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    HostComputerApp object, {
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
    required HostComputerAppBuilder result,
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
        case r'name':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.name = valueDes;
          break;
        case r'signer':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.signer = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  HostComputerApp deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = HostComputerAppBuilder();
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
