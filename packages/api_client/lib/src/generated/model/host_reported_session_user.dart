//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'host_reported_session_user.g.dart';

/// A session user the host service reports (`host_reported_session_users.v1`, 24 section 2.2), with the display names of its own working directories of the `claude` profile (ruling T13). Names only, never paths.
///
/// Properties:
/// * [osUser] - Operating system user name from `host_service.session_users`.
/// * [workdirs] - Display names of the working directories of this session user in `host_service.claude_workdirs` after check HD7, in their order (a directory listed under two session users is dropped from both lists); empty means no `claude` profile for this session user. A client sends the index into its own list, never a path; the host service resolves the index in exactly this list (24, 2.2 and 5.3).
@BuiltValue()
abstract class HostReportedSessionUser
    implements Built<HostReportedSessionUser, HostReportedSessionUserBuilder> {
  /// Operating system user name from `host_service.session_users`.
  @BuiltValueField(wireName: r'os_user')
  String get osUser;

  /// Display names of the working directories of this session user in `host_service.claude_workdirs` after check HD7, in their order (a directory listed under two session users is dropped from both lists); empty means no `claude` profile for this session user. A client sends the index into its own list, never a path; the host service resolves the index in exactly this list (24, 2.2 and 5.3).
  @BuiltValueField(wireName: r'workdirs')
  BuiltList<String> get workdirs;

  HostReportedSessionUser._();

  factory HostReportedSessionUser([
    void updates(HostReportedSessionUserBuilder b),
  ]) = _$HostReportedSessionUser;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(HostReportedSessionUserBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<HostReportedSessionUser> get serializer =>
      _$HostReportedSessionUserSerializer();
}

class _$HostReportedSessionUserSerializer
    implements PrimitiveSerializer<HostReportedSessionUser> {
  @override
  final Iterable<Type> types = const [
    HostReportedSessionUser,
    _$HostReportedSessionUser,
  ];

  @override
  final String wireName = r'HostReportedSessionUser';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    HostReportedSessionUser object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'os_user';
    yield serializers.serialize(
      object.osUser,
      specifiedType: const FullType(String),
    );
    yield r'workdirs';
    yield serializers.serialize(
      object.workdirs,
      specifiedType: const FullType(BuiltList, [FullType(String)]),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    HostReportedSessionUser object, {
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
    required HostReportedSessionUserBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'os_user':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.osUser = valueDes;
          break;
        case r'workdirs':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(BuiltList, [FullType(String)]),
          ) as BuiltList<String>;
          result.workdirs.replace(valueDes);
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  HostReportedSessionUser deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = HostReportedSessionUserBuilder();
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
