//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/host_computer_permissions.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'host_computer.g.dart';

/// The state of the computer helper of a host (schema `host_computer.v1`, docs/spec/27-computer-use.md 12.3): reported by the host service in `report.computer`, stored in `hosts.reported_computer`. Never a path, a key or screen content.
///
/// Properties:
/// * [helper] - The computer helper is connected, absent, or refused by the peer rule (27 9.4).
/// * [enabled] - The switch \"Allow computer control\" of the helper is on.
/// * [generation] - The enablement generation of the host (27 4).
/// * [osAccount] - The operating system account of the desktop session the helper runs in.
/// * [permissions]
/// * [helperVersion] - Version of the computer helper.
/// * [signature] - `verified` when the build of the host service carries a code signing requirement for the helper, else `development`.
/// * [locked] - The screen of the desktop is locked.
@BuiltValue()
abstract class HostComputer
    implements Built<HostComputer, HostComputerBuilder> {
  /// The computer helper is connected, absent, or refused by the peer rule (27 9.4).
  @BuiltValueField(wireName: r'helper')
  HostComputerHelperEnum get helper;
  // enum helperEnum {  connected,  absent,  refused,  };

  /// The switch \"Allow computer control\" of the helper is on.
  @BuiltValueField(wireName: r'enabled')
  bool get enabled;

  /// The enablement generation of the host (27 4).
  @BuiltValueField(wireName: r'generation')
  int get generation;

  /// The operating system account of the desktop session the helper runs in.
  @BuiltValueField(wireName: r'os_account')
  String get osAccount;

  @BuiltValueField(wireName: r'permissions')
  HostComputerPermissions get permissions;

  /// Version of the computer helper.
  @BuiltValueField(wireName: r'helper_version')
  String get helperVersion;

  /// `verified` when the build of the host service carries a code signing requirement for the helper, else `development`.
  @BuiltValueField(wireName: r'signature')
  HostComputerSignatureEnum get signature;
  // enum signatureEnum {  verified,  development,  };

  /// The screen of the desktop is locked.
  @BuiltValueField(wireName: r'locked')
  bool get locked;

  HostComputer._();

  factory HostComputer([void updates(HostComputerBuilder b)]) = _$HostComputer;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(HostComputerBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<HostComputer> get serializer => _$HostComputerSerializer();
}

class _$HostComputerSerializer implements PrimitiveSerializer<HostComputer> {
  @override
  final Iterable<Type> types = const [HostComputer, _$HostComputer];

  @override
  final String wireName = r'HostComputer';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    HostComputer object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'helper';
    yield serializers.serialize(
      object.helper,
      specifiedType: const FullType(HostComputerHelperEnum),
    );
    yield r'enabled';
    yield serializers.serialize(
      object.enabled,
      specifiedType: const FullType(bool),
    );
    yield r'generation';
    yield serializers.serialize(
      object.generation,
      specifiedType: const FullType(int),
    );
    yield r'os_account';
    yield serializers.serialize(
      object.osAccount,
      specifiedType: const FullType(String),
    );
    yield r'permissions';
    yield serializers.serialize(
      object.permissions,
      specifiedType: const FullType(HostComputerPermissions),
    );
    yield r'helper_version';
    yield serializers.serialize(
      object.helperVersion,
      specifiedType: const FullType(String),
    );
    yield r'signature';
    yield serializers.serialize(
      object.signature,
      specifiedType: const FullType(HostComputerSignatureEnum),
    );
    yield r'locked';
    yield serializers.serialize(
      object.locked,
      specifiedType: const FullType(bool),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    HostComputer object, {
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
    required HostComputerBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'helper':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(HostComputerHelperEnum),
          ) as HostComputerHelperEnum;
          result.helper = valueDes;
          break;
        case r'enabled':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.enabled = valueDes;
          break;
        case r'generation':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.generation = valueDes;
          break;
        case r'os_account':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.osAccount = valueDes;
          break;
        case r'permissions':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(HostComputerPermissions),
          ) as HostComputerPermissions;
          result.permissions.replace(valueDes);
          break;
        case r'helper_version':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.helperVersion = valueDes;
          break;
        case r'signature':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(HostComputerSignatureEnum),
          ) as HostComputerSignatureEnum;
          result.signature = valueDes;
          break;
        case r'locked':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.locked = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  HostComputer deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = HostComputerBuilder();
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

/// The computer helper is connected, absent, or refused by the peer rule (27 9.4).
class HostComputerHelperEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'connected')
  static const HostComputerHelperEnum connected =
      _$hostComputerHelperEnum_connected;
  @BuiltValueEnumConst(wireName: r'absent')
  static const HostComputerHelperEnum absent = _$hostComputerHelperEnum_absent;
  @BuiltValueEnumConst(wireName: r'refused')
  static const HostComputerHelperEnum refused =
      _$hostComputerHelperEnum_refused;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const HostComputerHelperEnum unknownDefaultOpenApi =
      _$hostComputerHelperEnum_unknownDefaultOpenApi;

  static Serializer<HostComputerHelperEnum> get serializer =>
      _$hostComputerHelperEnumSerializer;

  const HostComputerHelperEnum._(String name) : super(name);

  static BuiltSet<HostComputerHelperEnum> get values =>
      _$hostComputerHelperEnumValues;
  static HostComputerHelperEnum valueOf(String name) =>
      _$hostComputerHelperEnumValueOf(name);
}

/// `verified` when the build of the host service carries a code signing requirement for the helper, else `development`.
class HostComputerSignatureEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'verified')
  static const HostComputerSignatureEnum verified =
      _$hostComputerSignatureEnum_verified;
  @BuiltValueEnumConst(wireName: r'development')
  static const HostComputerSignatureEnum development =
      _$hostComputerSignatureEnum_development;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const HostComputerSignatureEnum unknownDefaultOpenApi =
      _$hostComputerSignatureEnum_unknownDefaultOpenApi;

  static Serializer<HostComputerSignatureEnum> get serializer =>
      _$hostComputerSignatureEnumSerializer;

  const HostComputerSignatureEnum._(String name) : super(name);

  static BuiltSet<HostComputerSignatureEnum> get values =>
      _$hostComputerSignatureEnumValues;
  static HostComputerSignatureEnum valueOf(String name) =>
      _$hostComputerSignatureEnumValueOf(name);
}
