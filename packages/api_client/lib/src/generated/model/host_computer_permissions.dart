//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'host_computer_permissions.g.dart';

/// HostComputerPermissions
///
/// Properties:
/// * [capture]
/// * [input]
@BuiltValue()
abstract class HostComputerPermissions
    implements Built<HostComputerPermissions, HostComputerPermissionsBuilder> {
  @BuiltValueField(wireName: r'capture')
  HostComputerPermissionsCaptureEnum get capture;
  // enum captureEnum {  granted,  denied,  unknown,  };

  @BuiltValueField(wireName: r'input')
  HostComputerPermissionsInputEnum get input;
  // enum inputEnum {  granted,  denied,  unknown,  };

  HostComputerPermissions._();

  factory HostComputerPermissions([
    void updates(HostComputerPermissionsBuilder b),
  ]) = _$HostComputerPermissions;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(HostComputerPermissionsBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<HostComputerPermissions> get serializer =>
      _$HostComputerPermissionsSerializer();
}

class _$HostComputerPermissionsSerializer
    implements PrimitiveSerializer<HostComputerPermissions> {
  @override
  final Iterable<Type> types = const [
    HostComputerPermissions,
    _$HostComputerPermissions,
  ];

  @override
  final String wireName = r'HostComputerPermissions';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    HostComputerPermissions object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'capture';
    yield serializers.serialize(
      object.capture,
      specifiedType: const FullType(HostComputerPermissionsCaptureEnum),
    );
    yield r'input';
    yield serializers.serialize(
      object.input,
      specifiedType: const FullType(HostComputerPermissionsInputEnum),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    HostComputerPermissions object, {
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
    required HostComputerPermissionsBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'capture':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(HostComputerPermissionsCaptureEnum),
          ) as HostComputerPermissionsCaptureEnum;
          result.capture = valueDes;
          break;
        case r'input':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(HostComputerPermissionsInputEnum),
          ) as HostComputerPermissionsInputEnum;
          result.input = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  HostComputerPermissions deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = HostComputerPermissionsBuilder();
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

class HostComputerPermissionsCaptureEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'granted')
  static const HostComputerPermissionsCaptureEnum granted =
      _$hostComputerPermissionsCaptureEnum_granted;
  @BuiltValueEnumConst(wireName: r'denied')
  static const HostComputerPermissionsCaptureEnum denied =
      _$hostComputerPermissionsCaptureEnum_denied;
  @BuiltValueEnumConst(wireName: r'unknown')
  static const HostComputerPermissionsCaptureEnum unknown =
      _$hostComputerPermissionsCaptureEnum_unknown;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const HostComputerPermissionsCaptureEnum unknownDefaultOpenApi =
      _$hostComputerPermissionsCaptureEnum_unknownDefaultOpenApi;

  static Serializer<HostComputerPermissionsCaptureEnum> get serializer =>
      _$hostComputerPermissionsCaptureEnumSerializer;

  const HostComputerPermissionsCaptureEnum._(String name) : super(name);

  static BuiltSet<HostComputerPermissionsCaptureEnum> get values =>
      _$hostComputerPermissionsCaptureEnumValues;
  static HostComputerPermissionsCaptureEnum valueOf(String name) =>
      _$hostComputerPermissionsCaptureEnumValueOf(name);
}

class HostComputerPermissionsInputEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'granted')
  static const HostComputerPermissionsInputEnum granted =
      _$hostComputerPermissionsInputEnum_granted;
  @BuiltValueEnumConst(wireName: r'denied')
  static const HostComputerPermissionsInputEnum denied =
      _$hostComputerPermissionsInputEnum_denied;
  @BuiltValueEnumConst(wireName: r'unknown')
  static const HostComputerPermissionsInputEnum unknown =
      _$hostComputerPermissionsInputEnum_unknown;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const HostComputerPermissionsInputEnum unknownDefaultOpenApi =
      _$hostComputerPermissionsInputEnum_unknownDefaultOpenApi;

  static Serializer<HostComputerPermissionsInputEnum> get serializer =>
      _$hostComputerPermissionsInputEnumSerializer;

  const HostComputerPermissionsInputEnum._(String name) : super(name);

  static BuiltSet<HostComputerPermissionsInputEnum> get values =>
      _$hostComputerPermissionsInputEnumValues;
  static HostComputerPermissionsInputEnum valueOf(String name) =>
      _$hostComputerPermissionsInputEnumValueOf(name);
}
