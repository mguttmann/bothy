//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'host_doctor_check.g.dart';

/// Result of one self-check of the host service (`host_doctor.v1`, checks HD1 to HD11 in 24, 2.2). Codes and, following the rule for names (24, 2.2), the name of each account a person with a terminal can cause the finding in; never content, paths or values. When HD6 or HD11 fails, the list has one entry per affected session user; when HD10 fails with a wrong peer, one entry per session user of that channel; when HD9 fails, one entry per former session user whose home can be listed, plus one entry without a name for any other user; `check` and `session_user` together are unique in the list.
///
/// Properties:
/// * [check]
/// * [ok]
/// * [code]
/// * [sessionUser] - Only when not passed: with `hd6_llm_login` and `hd11_shared_locations` the session user whose account the finding is in; with `hd10_helper_integrity` and a wrong peer the session user of that channel; with `hd9_foreign_homes` a former session user of this host (bound to a person in the state directory, no longer in `host_service.session_users`) whose home can be listed. The instance operator knows where to look, and the console shows the person of the assignment (24, 2.2 and 7.4). One entry per such account; names of other users are never sent. A name, never a path.
@BuiltValue()
abstract class HostDoctorCheck
    implements Built<HostDoctorCheck, HostDoctorCheckBuilder> {
  @BuiltValueField(wireName: r'check')
  HostDoctorCheckCheckEnum get check;
  // enum checkEnum {  hd1_user,  hd2_groups,  hd3_sudo,  hd4_state_dir,  hd5_server_host,  hd6_llm_login,  hd7_profiles,  hd8_separation,  hd9_foreign_homes,  hd10_helper_integrity,  hd11_shared_locations,  };

  @BuiltValueField(wireName: r'ok')
  bool get ok;

  @BuiltValueField(wireName: r'code')
  String? get code;

  /// Only when not passed: with `hd6_llm_login` and `hd11_shared_locations` the session user whose account the finding is in; with `hd10_helper_integrity` and a wrong peer the session user of that channel; with `hd9_foreign_homes` a former session user of this host (bound to a person in the state directory, no longer in `host_service.session_users`) whose home can be listed. The instance operator knows where to look, and the console shows the person of the assignment (24, 2.2 and 7.4). One entry per such account; names of other users are never sent. A name, never a path.
  @BuiltValueField(wireName: r'session_user')
  String? get sessionUser;

  HostDoctorCheck._();

  factory HostDoctorCheck([void updates(HostDoctorCheckBuilder b)]) =
      _$HostDoctorCheck;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(HostDoctorCheckBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<HostDoctorCheck> get serializer =>
      _$HostDoctorCheckSerializer();
}

class _$HostDoctorCheckSerializer
    implements PrimitiveSerializer<HostDoctorCheck> {
  @override
  final Iterable<Type> types = const [HostDoctorCheck, _$HostDoctorCheck];

  @override
  final String wireName = r'HostDoctorCheck';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    HostDoctorCheck object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'check';
    yield serializers.serialize(
      object.check,
      specifiedType: const FullType(HostDoctorCheckCheckEnum),
    );
    yield r'ok';
    yield serializers.serialize(object.ok, specifiedType: const FullType(bool));
    if (object.code != null) {
      yield r'code';
      yield serializers.serialize(
        object.code,
        specifiedType: const FullType(String),
      );
    }
    if (object.sessionUser != null) {
      yield r'session_user';
      yield serializers.serialize(
        object.sessionUser,
        specifiedType: const FullType(String),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    HostDoctorCheck object, {
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
    required HostDoctorCheckBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'check':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(HostDoctorCheckCheckEnum),
          ) as HostDoctorCheckCheckEnum;
          result.check = valueDes;
          break;
        case r'ok':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.ok = valueDes;
          break;
        case r'code':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.code = valueDes;
          break;
        case r'session_user':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(String),
          ) as String?;
          if (valueDes == null) continue;
          result.sessionUser = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  HostDoctorCheck deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = HostDoctorCheckBuilder();
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

class HostDoctorCheckCheckEnum extends EnumClass {
  @BuiltValueEnumConst(wireName: r'hd1_user')
  static const HostDoctorCheckCheckEnum hd1User =
      _$hostDoctorCheckCheckEnum_hd1User;
  @BuiltValueEnumConst(wireName: r'hd2_groups')
  static const HostDoctorCheckCheckEnum hd2Groups =
      _$hostDoctorCheckCheckEnum_hd2Groups;
  @BuiltValueEnumConst(wireName: r'hd3_sudo')
  static const HostDoctorCheckCheckEnum hd3Sudo =
      _$hostDoctorCheckCheckEnum_hd3Sudo;
  @BuiltValueEnumConst(wireName: r'hd4_state_dir')
  static const HostDoctorCheckCheckEnum hd4StateDir =
      _$hostDoctorCheckCheckEnum_hd4StateDir;
  @BuiltValueEnumConst(wireName: r'hd5_server_host')
  static const HostDoctorCheckCheckEnum hd5ServerHost =
      _$hostDoctorCheckCheckEnum_hd5ServerHost;
  @BuiltValueEnumConst(wireName: r'hd6_llm_login')
  static const HostDoctorCheckCheckEnum hd6LlmLogin =
      _$hostDoctorCheckCheckEnum_hd6LlmLogin;
  @BuiltValueEnumConst(wireName: r'hd7_profiles')
  static const HostDoctorCheckCheckEnum hd7Profiles =
      _$hostDoctorCheckCheckEnum_hd7Profiles;
  @BuiltValueEnumConst(wireName: r'hd8_separation')
  static const HostDoctorCheckCheckEnum hd8Separation =
      _$hostDoctorCheckCheckEnum_hd8Separation;
  @BuiltValueEnumConst(wireName: r'hd9_foreign_homes')
  static const HostDoctorCheckCheckEnum hd9ForeignHomes =
      _$hostDoctorCheckCheckEnum_hd9ForeignHomes;
  @BuiltValueEnumConst(wireName: r'hd10_helper_integrity')
  static const HostDoctorCheckCheckEnum hd10HelperIntegrity =
      _$hostDoctorCheckCheckEnum_hd10HelperIntegrity;
  @BuiltValueEnumConst(wireName: r'hd11_shared_locations')
  static const HostDoctorCheckCheckEnum hd11SharedLocations =
      _$hostDoctorCheckCheckEnum_hd11SharedLocations;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const HostDoctorCheckCheckEnum unknownDefaultOpenApi =
      _$hostDoctorCheckCheckEnum_unknownDefaultOpenApi;

  static Serializer<HostDoctorCheckCheckEnum> get serializer =>
      _$hostDoctorCheckCheckEnumSerializer;

  const HostDoctorCheckCheckEnum._(String name) : super(name);

  static BuiltSet<HostDoctorCheckCheckEnum> get values =>
      _$hostDoctorCheckCheckEnumValues;
  static HostDoctorCheckCheckEnum valueOf(String name) =>
      _$hostDoctorCheckCheckEnumValueOf(name);
}
