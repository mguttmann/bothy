//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:api_client/src/generated/model/host_session_user.dart';
import 'package:api_client/src/generated/model/llm_credentials.dart';
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/host_platform.dart';
import 'package:api_client/src/generated/model/host_state.dart';
import 'package:api_client/src/generated/model/host_reported_session_user.dart';
import 'package:api_client/src/generated/model/host_doctor_check.dart';
import 'package:api_client/src/generated/model/host_profile.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'host.g.dart';

/// A connected host. Keys are public; the client pins `noise_public_key` per device (24, 5.5).
///
/// Properties:
/// * [id] - UUIDv7 (RFC 9562), lowercase canonical form.
/// * [name]
/// * [state]
/// * [disabledReason] - Present exactly with `state = disabled`. `duplicate_connection` means a second live control connection with the same keys; only re-pairing helps (24, 3.4).
/// * [platform]
/// * [assignedUserId] - Person the host is assigned to (SR10); absent means owners only.
/// * [llmCredentials]
/// * [credentialHolderId] - Present exactly with `llm_credentials = personal`.
/// * [declaredServerHost]
/// * [reportedIsServerHost] - Report of the host service (24, 2.3); absent before the first report.
/// * [noisePublicKey] - Static X25519 key of the host, 32 bytes base64url without padding; absent while the host has no keys (before pairing and after a re-pairing or the fallback of `confirmHost`).
/// * [keyFingerprint] - Fingerprint of the stored keys (24, 4.3), 13 groups of four Crockford base32 characters; absent while the host has no keys.
/// * [keyGeneration]
/// * [profiles]
/// * [doctor] - One entry per check HD1 to HD11; a failed HD6, HD9, HD10 or HD11 has one entry per account named by the rule for names (24, 2.2).
/// * [sessionUsers] - Valid session user assignments (ruling T12), visible like every other member to everyone with `host.read`. Absent when there is none.
/// * [reportedSessionUsers] - Session users the host service reported at pairing and in its last `report` (24 section 2.2). An assignment works only while its name is in this list (SR12); `confirmHost` and `updateHost` accept only these names. Absent before pairing.
/// * [requireLocalConfirm] - Report whether `host_service.require_local_confirm` is on.
/// * [connected] - The control connection of the host service is open (memory of the server).
/// * [hostVersion]
/// * [osVersion]
/// * [lastSeenAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [createdAt] - RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
/// * [version]
@BuiltValue()
abstract class Host implements Built<Host, HostBuilder> {
  /// UUIDv7 (RFC 9562), lowercase canonical form.
  @BuiltValueField(wireName: r'id')
  String get id;

  @BuiltValueField(wireName: r'name')
  String get name;

  @BuiltValueField(wireName: r'state')
  HostState get state;
  // enum stateEnum {  pairing,  confirming,  active,  disabled,  retired,  };

  /// Present exactly with `state = disabled`. `duplicate_connection` means a second live control connection with the same keys; only re-pairing helps (24, 3.4).
  @BuiltValueField(wireName: r'disabled_reason')
  HostDisabledReasonEnum? get disabledReason;
  // enum disabledReasonEnum {  owner,  duplicate_connection,  };

  @BuiltValueField(wireName: r'platform')
  HostPlatform? get platform;
  // enum platformEnum {  macos,  linux,  windows,  };

  /// Person the host is assigned to (SR10); absent means owners only.
  @BuiltValueField(wireName: r'assigned_user_id')
  String? get assignedUserId;

  @BuiltValueField(wireName: r'llm_credentials')
  LlmCredentials get llmCredentials;
  // enum llmCredentialsEnum {  none,  personal,  organization,  };

  /// Present exactly with `llm_credentials = personal`.
  @BuiltValueField(wireName: r'credential_holder_id')
  String? get credentialHolderId;

  @BuiltValueField(wireName: r'declared_server_host')
  bool get declaredServerHost;

  /// Report of the host service (24, 2.3); absent before the first report.
  @BuiltValueField(wireName: r'reported_is_server_host')
  bool? get reportedIsServerHost;

  /// Static X25519 key of the host, 32 bytes base64url without padding; absent while the host has no keys (before pairing and after a re-pairing or the fallback of `confirmHost`).
  @BuiltValueField(wireName: r'noise_public_key')
  String? get noisePublicKey;

  /// Fingerprint of the stored keys (24, 4.3), 13 groups of four Crockford base32 characters; absent while the host has no keys.
  @BuiltValueField(wireName: r'key_fingerprint')
  String? get keyFingerprint;

  @BuiltValueField(wireName: r'key_generation')
  int get keyGeneration;

  @BuiltValueField(wireName: r'profiles')
  BuiltList<HostProfile> get profiles;

  /// One entry per check HD1 to HD11; a failed HD6, HD9, HD10 or HD11 has one entry per account named by the rule for names (24, 2.2).
  @BuiltValueField(wireName: r'doctor')
  BuiltList<HostDoctorCheck> get doctor;

  /// Valid session user assignments (ruling T12), visible like every other member to everyone with `host.read`. Absent when there is none.
  @BuiltValueField(wireName: r'session_users')
  BuiltList<HostSessionUser>? get sessionUsers;

  /// Session users the host service reported at pairing and in its last `report` (24 section 2.2). An assignment works only while its name is in this list (SR12); `confirmHost` and `updateHost` accept only these names. Absent before pairing.
  @BuiltValueField(wireName: r'reported_session_users')
  BuiltList<HostReportedSessionUser>? get reportedSessionUsers;

  /// Report whether `host_service.require_local_confirm` is on.
  @BuiltValueField(wireName: r'require_local_confirm')
  bool? get requireLocalConfirm;

  /// The control connection of the host service is open (memory of the server).
  @BuiltValueField(wireName: r'connected')
  bool get connected;

  @BuiltValueField(wireName: r'host_version')
  String? get hostVersion;

  @BuiltValueField(wireName: r'os_version')
  String? get osVersion;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'last_seen_at')
  DateTime? get lastSeenAt;

  /// RFC 3339 timestamp in UTC with millisecond precision and suffix `Z`.
  @BuiltValueField(wireName: r'created_at')
  DateTime get createdAt;

  @BuiltValueField(wireName: r'version')
  int get version;

  Host._();

  factory Host([void updates(HostBuilder b)]) = _$Host;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(HostBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<Host> get serializer => _$HostSerializer();
}

class _$HostSerializer implements PrimitiveSerializer<Host> {
  @override
  final Iterable<Type> types = const [Host, _$Host];

  @override
  final String wireName = r'Host';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    Host object, {
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
    yield r'state';
    yield serializers.serialize(
      object.state,
      specifiedType: const FullType(HostState),
    );
    if (object.disabledReason != null) {
      yield r'disabled_reason';
      yield serializers.serialize(
        object.disabledReason,
        specifiedType: const FullType(HostDisabledReasonEnum),
      );
    }
    if (object.platform != null) {
      yield r'platform';
      yield serializers.serialize(
        object.platform,
        specifiedType: const FullType(HostPlatform),
      );
    }
    if (object.assignedUserId != null) {
      yield r'assigned_user_id';
      yield serializers.serialize(
        object.assignedUserId,
        specifiedType: const FullType(String),
      );
    }
    yield r'llm_credentials';
    yield serializers.serialize(
      object.llmCredentials,
      specifiedType: const FullType(LlmCredentials),
    );
    if (object.credentialHolderId != null) {
      yield r'credential_holder_id';
      yield serializers.serialize(
        object.credentialHolderId,
        specifiedType: const FullType(String),
      );
    }
    yield r'declared_server_host';
    yield serializers.serialize(
      object.declaredServerHost,
      specifiedType: const FullType(bool),
    );
    if (object.reportedIsServerHost != null) {
      yield r'reported_is_server_host';
      yield serializers.serialize(
        object.reportedIsServerHost,
        specifiedType: const FullType(bool),
      );
    }
    if (object.noisePublicKey != null) {
      yield r'noise_public_key';
      yield serializers.serialize(
        object.noisePublicKey,
        specifiedType: const FullType(String),
      );
    }
    if (object.keyFingerprint != null) {
      yield r'key_fingerprint';
      yield serializers.serialize(
        object.keyFingerprint,
        specifiedType: const FullType(String),
      );
    }
    yield r'key_generation';
    yield serializers.serialize(
      object.keyGeneration,
      specifiedType: const FullType(int),
    );
    yield r'profiles';
    yield serializers.serialize(
      object.profiles,
      specifiedType: const FullType(BuiltList, [FullType(HostProfile)]),
    );
    yield r'doctor';
    yield serializers.serialize(
      object.doctor,
      specifiedType: const FullType(BuiltList, [FullType(HostDoctorCheck)]),
    );
    if (object.sessionUsers != null) {
      yield r'session_users';
      yield serializers.serialize(
        object.sessionUsers,
        specifiedType: const FullType(BuiltList, [FullType(HostSessionUser)]),
      );
    }
    if (object.reportedSessionUsers != null) {
      yield r'reported_session_users';
      yield serializers.serialize(
        object.reportedSessionUsers,
        specifiedType: const FullType(BuiltList, [
          FullType(HostReportedSessionUser),
        ]),
      );
    }
    if (object.requireLocalConfirm != null) {
      yield r'require_local_confirm';
      yield serializers.serialize(
        object.requireLocalConfirm,
        specifiedType: const FullType(bool),
      );
    }
    yield r'connected';
    yield serializers.serialize(
      object.connected,
      specifiedType: const FullType(bool),
    );
    if (object.hostVersion != null) {
      yield r'host_version';
      yield serializers.serialize(
        object.hostVersion,
        specifiedType: const FullType(String),
      );
    }
    if (object.osVersion != null) {
      yield r'os_version';
      yield serializers.serialize(
        object.osVersion,
        specifiedType: const FullType(String),
      );
    }
    if (object.lastSeenAt != null) {
      yield r'last_seen_at';
      yield serializers.serialize(
        object.lastSeenAt,
        specifiedType: const FullType(DateTime),
      );
    }
    yield r'created_at';
    yield serializers.serialize(
      object.createdAt,
      specifiedType: const FullType(DateTime),
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
    Host object, {
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
    required HostBuilder result,
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
        case r'state':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(HostState),
          ) as HostState;
          result.state = valueDes;
          break;
        case r'disabled_reason':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(HostDisabledReasonEnum),
          ) as HostDisabledReasonEnum?;
          if (valueDes == null) continue;
          result.disabledReason = valueDes;
          break;
        case r'platform':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(HostPlatform),
          ) as HostPlatform?;
          if (valueDes == null) continue;
          result.platform = valueDes;
          break;
        case r'assigned_user_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.assignedUserId = valueDes;
          break;
        case r'llm_credentials':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(LlmCredentials),
          ) as LlmCredentials;
          result.llmCredentials = valueDes;
          break;
        case r'credential_holder_id':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.credentialHolderId = valueDes;
          break;
        case r'declared_server_host':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.declaredServerHost = valueDes;
          break;
        case r'reported_is_server_host':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(bool),
          ) as bool?;
          if (valueDes == null) continue;
          result.reportedIsServerHost = valueDes;
          break;
        case r'noise_public_key':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.noisePublicKey = valueDes;
          break;
        case r'key_fingerprint':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.keyFingerprint = valueDes;
          break;
        case r'key_generation':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.keyGeneration = valueDes;
          break;
        case r'profiles':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(HostProfile)]),
          ) as BuiltList<HostProfile>;
          result.profiles.replace(valueDes);
          break;
        case r'doctor':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [
              FullType(HostDoctorCheck),
            ]),
          ) as BuiltList<HostDoctorCheck>;
          result.doctor.replace(valueDes);
          break;
        case r'session_users':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(HostSessionUser),
            ]),
          ) as BuiltList<HostSessionUser>?;
          if (valueDes == null) continue;
          result.sessionUsers.replace(valueDes);
          break;
        case r'reported_session_users':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(BuiltList, [
              FullType(HostReportedSessionUser),
            ]),
          ) as BuiltList<HostReportedSessionUser>?;
          if (valueDes == null) continue;
          result.reportedSessionUsers.replace(valueDes);
          break;
        case r'require_local_confirm':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(bool),
          ) as bool?;
          if (valueDes == null) continue;
          result.requireLocalConfirm = valueDes;
          break;
        case r'connected':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.connected = valueDes;
          break;
        case r'host_version':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.hostVersion = valueDes;
          break;
        case r'os_version':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.osVersion = valueDes;
          break;
        case r'last_seen_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(DateTime),
          ) as DateTime?;
          if (valueDes == null) continue;
          result.lastSeenAt = valueDes;
          break;
        case r'created_at':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(DateTime),
          ) as DateTime;
          result.createdAt = valueDes;
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
  Host deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = HostBuilder();
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

/// Present exactly with `state = disabled`. `duplicate_connection` means a second live control connection with the same keys; only re-pairing helps (24, 3.4).
class HostDisabledReasonEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'owner')
  static const HostDisabledReasonEnum owner = _$hostDisabledReasonEnum_owner;
  @BuiltValueEnumConst(wireName: r'duplicate_connection')
  static const HostDisabledReasonEnum duplicateConnection =
      _$hostDisabledReasonEnum_duplicateConnection;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const HostDisabledReasonEnum unknownDefaultOpenApi =
      _$hostDisabledReasonEnum_unknownDefaultOpenApi;

  static Serializer<HostDisabledReasonEnum> get serializer =>
      _$hostDisabledReasonEnumSerializer;

  const HostDisabledReasonEnum._(String name) : super(name);

  static BuiltSet<HostDisabledReasonEnum> get values =>
      _$hostDisabledReasonEnumValues;
  static HostDisabledReasonEnum valueOf(String name) =>
      _$hostDisabledReasonEnumValueOf(name);
}
