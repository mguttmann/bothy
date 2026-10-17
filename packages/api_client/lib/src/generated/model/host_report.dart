//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:api_client/src/generated/model/host_reported_session_user.dart';
import 'package:api_client/src/generated/model/host_doctor_check.dart';
import 'package:api_client/src/generated/model/host_profile.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'host_report.g.dart';

/// Report of profiles, session users and self-checks; never content, paths or values. Sent after every `ready` before the host service accepts `session.open`, and on every change. When `doctor` refuses the start, the host service still logs in, sends one `report` and closes the connection without accepting `session.open` (24 section 2.2).
///
/// Properties:
/// * [profiles]
/// * [sessionUsers]
/// * [doctor] - One entry per check HD1 to HD11; a failed HD6, HD9, HD10 or HD11 has one entry per account named by the rule for names (24, 2.2).
/// * [isServerHost]
/// * [requireLocalConfirm]
/// * [osVersion]
@BuiltValue()
abstract class HostReport implements Built<HostReport, HostReportBuilder> {
  @BuiltValueField(wireName: r'profiles')
  BuiltList<HostProfile> get profiles;

  @BuiltValueField(wireName: r'session_users')
  BuiltList<HostReportedSessionUser> get sessionUsers;

  /// One entry per check HD1 to HD11; a failed HD6, HD9, HD10 or HD11 has one entry per account named by the rule for names (24, 2.2).
  @BuiltValueField(wireName: r'doctor')
  BuiltList<HostDoctorCheck> get doctor;

  @BuiltValueField(wireName: r'is_server_host')
  bool get isServerHost;

  @BuiltValueField(wireName: r'require_local_confirm')
  bool get requireLocalConfirm;

  @BuiltValueField(wireName: r'os_version')
  String? get osVersion;

  HostReport._();

  factory HostReport([void updates(HostReportBuilder b)]) = _$HostReport;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(HostReportBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<HostReport> get serializer => _$HostReportSerializer();
}

class _$HostReportSerializer implements PrimitiveSerializer<HostReport> {
  @override
  final Iterable<Type> types = const [HostReport, _$HostReport];

  @override
  final String wireName = r'HostReport';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    HostReport object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'profiles';
    yield serializers.serialize(
      object.profiles,
      specifiedType: const FullType(BuiltList, [FullType(HostProfile)]),
    );
    yield r'session_users';
    yield serializers.serialize(
      object.sessionUsers,
      specifiedType: const FullType(BuiltList, [
        FullType(HostReportedSessionUser),
      ]),
    );
    yield r'doctor';
    yield serializers.serialize(
      object.doctor,
      specifiedType: const FullType(BuiltList, [FullType(HostDoctorCheck)]),
    );
    yield r'is_server_host';
    yield serializers.serialize(
      object.isServerHost,
      specifiedType: const FullType(bool),
    );
    yield r'require_local_confirm';
    yield serializers.serialize(
      object.requireLocalConfirm,
      specifiedType: const FullType(bool),
    );
    if (object.osVersion != null) {
      yield r'os_version';
      yield serializers.serialize(
        object.osVersion,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    HostReport object, {
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
    required HostReportBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'profiles':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(HostProfile)]),
          ) as BuiltList<HostProfile>;
          result.profiles.replace(valueDes);
          break;
        case r'session_users':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [
              FullType(HostReportedSessionUser),
            ]),
          ) as BuiltList<HostReportedSessionUser>;
          result.sessionUsers.replace(valueDes);
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
        case r'is_server_host':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.isServerHost = valueDes;
          break;
        case r'require_local_confirm':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.requireLocalConfirm = valueDes;
          break;
        case r'os_version':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.osVersion = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  HostReport deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = HostReportBuilder();
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
