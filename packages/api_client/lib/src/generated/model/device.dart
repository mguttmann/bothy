//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/device_platform.dart';
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/push_registration.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'device.g.dart';

/// Device
///
/// Properties:
/// * [id] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [userId] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [name]
/// * [platform]
/// * [state]
/// * [appVersion]
/// * [keyThumbprint] - RFC 7638 thumbprint of the active `dpop` device key (`cnf.jkt`).
/// * [stepUpKeyThumbprint] - RFC 7638 thumbprint of the active `step_up` device key, if the device has one.
/// * [verificationCode] - Only in responses to the pending device itself (`TokenResponse`, `getDevice`, `listMyDevices` for its own entry); never shown to other devices or owners (08, 5.2).
/// * [revokeReason]
/// * [approvedAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [revokedAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [createdAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [lastSeenAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [pushRegistrations] - Every push registration of the device, active and disabled, ascending by `id`, never with a token (13, 13). Absent when the device has none.
@BuiltValue()
abstract class Device implements Built<Device, DeviceBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'id')
  String get id;

  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'user_id')
  String get userId;

  @BuiltValueField(wireName: r'name')
  String get name;

  @BuiltValueField(wireName: r'platform')
  DevicePlatform get platform;
  // enum platformEnum {  ios,  android,  macos,  windows,  web,  cli,  };

  @BuiltValueField(wireName: r'state')
  DeviceStateEnum get state;
  // enum stateEnum {  pending,  active,  revoked,  };

  @BuiltValueField(wireName: r'app_version')
  String? get appVersion;

  /// RFC 7638 thumbprint of the active `dpop` device key (`cnf.jkt`).
  @BuiltValueField(wireName: r'key_thumbprint')
  String? get keyThumbprint;

  /// RFC 7638 thumbprint of the active `step_up` device key, if the device has one.
  @BuiltValueField(wireName: r'step_up_key_thumbprint')
  String? get stepUpKeyThumbprint;

  /// Only in responses to the pending device itself (`TokenResponse`, `getDevice`, `listMyDevices` for its own entry); never shown to other devices or owners (08, 5.2).
  @BuiltValueField(wireName: r'verification_code')
  String? get verificationCode;

  @BuiltValueField(wireName: r'revoke_reason')
  DeviceRevokeReasonEnum? get revokeReason;
  // enum revokeReasonEnum {  revoked_by_self,  revoked_by_owner,  user_disabled,  inactive,  verification_failed,  pending_expired,  pairing_expired,  };

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'approved_at')
  DateTime? get approvedAt;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'revoked_at')
  DateTime? get revokedAt;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'created_at')
  DateTime get createdAt;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'last_seen_at')
  DateTime? get lastSeenAt;

  /// Every push registration of the device, active and disabled, ascending by `id`, never with a token (13, 13). Absent when the device has none.
  @BuiltValueField(wireName: r'push_registrations')
  BuiltList<PushRegistration>? get pushRegistrations;

  Device._();

  factory Device([void updates(DeviceBuilder b)]) = _$Device;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(DeviceBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<Device> get serializer => _$DeviceSerializer();
}

class _$DeviceSerializer implements PrimitiveSerializer<Device> {
  @override
  final Iterable<Type> types = const [Device, _$Device];

  @override
  final String wireName = r'Device';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    Device object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'id';
    yield serializers.serialize(
      object.id,
      specifiedType: const FullType(String),
    );
    yield r'user_id';
    yield serializers.serialize(
      object.userId,
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
      specifiedType: const FullType(DevicePlatform),
    );
    yield r'state';
    yield serializers.serialize(
      object.state,
      specifiedType: const FullType(DeviceStateEnum),
    );
    if (object.appVersion != null) {
      yield r'app_version';
      yield serializers.serialize(
        object.appVersion,
        specifiedType: const FullType(String),
      );
    }
    if (object.keyThumbprint != null) {
      yield r'key_thumbprint';
      yield serializers.serialize(
        object.keyThumbprint,
        specifiedType: const FullType(String),
      );
    }
    if (object.stepUpKeyThumbprint != null) {
      yield r'step_up_key_thumbprint';
      yield serializers.serialize(
        object.stepUpKeyThumbprint,
        specifiedType: const FullType(String),
      );
    }
    if (object.verificationCode != null) {
      yield r'verification_code';
      yield serializers.serialize(
        object.verificationCode,
        specifiedType: const FullType(String),
      );
    }
    if (object.revokeReason != null) {
      yield r'revoke_reason';
      yield serializers.serialize(
        object.revokeReason,
        specifiedType: const FullType(DeviceRevokeReasonEnum),
      );
    }
    if (object.approvedAt != null) {
      yield r'approved_at';
      yield serializers.serialize(
        object.approvedAt,
        specifiedType: const FullType(DateTime),
      );
    }
    if (object.revokedAt != null) {
      yield r'revoked_at';
      yield serializers.serialize(
        object.revokedAt,
        specifiedType: const FullType(DateTime),
      );
    }
    yield r'created_at';
    yield serializers.serialize(
      object.createdAt,
      specifiedType: const FullType(DateTime),
    );
    if (object.lastSeenAt != null) {
      yield r'last_seen_at';
      yield serializers.serialize(
        object.lastSeenAt,
        specifiedType: const FullType(DateTime),
      );
    }
    if (object.pushRegistrations != null) {
      yield r'push_registrations';
      yield serializers.serialize(
        object.pushRegistrations,
        specifiedType: const FullType(BuiltList, [FullType(PushRegistration)]),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    Device object, {
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
    required DeviceBuilder result,
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
        case r'user_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.userId = valueDes;
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
            specifiedType: const FullType(DevicePlatform),
          ) as DevicePlatform;
          result.platform = valueDes;
          break;
        case r'state':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DeviceStateEnum),
          ) as DeviceStateEnum;
          result.state = valueDes;
          break;
        case r'app_version':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.appVersion = valueDes;
          break;
        case r'key_thumbprint':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.keyThumbprint = valueDes;
          break;
        case r'step_up_key_thumbprint':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.stepUpKeyThumbprint = valueDes;
          break;
        case r'verification_code':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.verificationCode = valueDes;
          break;
        case r'revoke_reason':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DeviceRevokeReasonEnum),
          ) as DeviceRevokeReasonEnum?;
          if (valueDes == null) continue;
          result.revokeReason = valueDes;
          break;
        case r'approved_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
          result.approvedAt = valueDes;
          break;
        case r'revoked_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
          result.revokedAt = valueDes;
          break;
        case r'created_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.createdAt = valueDes;
          break;
        case r'last_seen_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
          result.lastSeenAt = valueDes;
          break;
        case r'push_registrations':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(PushRegistration),
            ]),
          ) as BuiltList<PushRegistration>?;
          if (valueDes == null) continue;
          result.pushRegistrations.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  Device deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = DeviceBuilder();
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

class DeviceStateEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'pending')
  static const DeviceStateEnum pending = _$deviceStateEnum_pending;
  @BuiltValueEnumConst(wireName: r'active')
  static const DeviceStateEnum active = _$deviceStateEnum_active;
  @BuiltValueEnumConst(wireName: r'revoked')
  static const DeviceStateEnum revoked = _$deviceStateEnum_revoked;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const DeviceStateEnum unknownDefaultOpenApi =
      _$deviceStateEnum_unknownDefaultOpenApi;

  static Serializer<DeviceStateEnum> get serializer =>
      _$deviceStateEnumSerializer;

  const DeviceStateEnum._(String name) : super(name);

  static BuiltSet<DeviceStateEnum> get values => _$deviceStateEnumValues;
  static DeviceStateEnum valueOf(String name) => _$deviceStateEnumValueOf(name);
}

class DeviceRevokeReasonEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'revoked_by_self')
  static const DeviceRevokeReasonEnum revokedBySelf =
      _$deviceRevokeReasonEnum_revokedBySelf;
  @BuiltValueEnumConst(wireName: r'revoked_by_owner')
  static const DeviceRevokeReasonEnum revokedByOwner =
      _$deviceRevokeReasonEnum_revokedByOwner;
  @BuiltValueEnumConst(wireName: r'user_disabled')
  static const DeviceRevokeReasonEnum userDisabled =
      _$deviceRevokeReasonEnum_userDisabled;
  @BuiltValueEnumConst(wireName: r'inactive')
  static const DeviceRevokeReasonEnum inactive =
      _$deviceRevokeReasonEnum_inactive;
  @BuiltValueEnumConst(wireName: r'verification_failed')
  static const DeviceRevokeReasonEnum verificationFailed =
      _$deviceRevokeReasonEnum_verificationFailed;
  @BuiltValueEnumConst(wireName: r'pending_expired')
  static const DeviceRevokeReasonEnum pendingExpired =
      _$deviceRevokeReasonEnum_pendingExpired;
  @BuiltValueEnumConst(wireName: r'pairing_expired')
  static const DeviceRevokeReasonEnum pairingExpired =
      _$deviceRevokeReasonEnum_pairingExpired;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const DeviceRevokeReasonEnum unknownDefaultOpenApi =
      _$deviceRevokeReasonEnum_unknownDefaultOpenApi;

  static Serializer<DeviceRevokeReasonEnum> get serializer =>
      _$deviceRevokeReasonEnumSerializer;

  const DeviceRevokeReasonEnum._(String name) : super(name);

  static BuiltSet<DeviceRevokeReasonEnum> get values =>
      _$deviceRevokeReasonEnumValues;
  static DeviceRevokeReasonEnum valueOf(String name) =>
      _$deviceRevokeReasonEnumValueOf(name);
}
