//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'terminal_session_open.g.dart';

/// TerminalSessionOpen
///
/// Properties:
/// * [profile]
/// * [workdirIndex] - Only with a `claude` profile: index into `workdirs` of the caller's own entry in `reported_session_users` (ruling T13). Another index gives 403 `terminal_denied` with `reason: profile_unknown`. The session token carries it as `wdi` (24, 5.3 and 11.1).
/// * [cols]
/// * [rows]
/// * [clientEphemeralKeyHash] - SHA-256 of the ephemeral X25519 public key the device sends in Noise message 1, base64url without padding; becomes the claim `cek`.
/// * [pinChanged] - `true` when the person confirmed a host key that differs from the key this device pinned earlier (docs/spec/24-terminal.md section 5.5, docs/spec/15-client-architektur.md section 23.3); recorded in the audit entry of `terminal.open`.
@BuiltValue()
abstract class TerminalSessionOpen
    implements Built<TerminalSessionOpen, TerminalSessionOpenBuilder> {
  @BuiltValueField(wireName: r'profile')
  String get profile;

  /// Only with a `claude` profile: index into `workdirs` of the caller's own entry in `reported_session_users` (ruling T13). Another index gives 403 `terminal_denied` with `reason: profile_unknown`. The session token carries it as `wdi` (24, 5.3 and 11.1).
  @BuiltValueField(wireName: r'workdir_index')
  int? get workdirIndex;

  @BuiltValueField(wireName: r'cols')
  int get cols;

  @BuiltValueField(wireName: r'rows')
  int get rows;

  /// SHA-256 of the ephemeral X25519 public key the device sends in Noise message 1, base64url without padding; becomes the claim `cek`.
  @BuiltValueField(wireName: r'client_ephemeral_key_hash')
  String get clientEphemeralKeyHash;

  /// `true` when the person confirmed a host key that differs from the key this device pinned earlier (docs/spec/24-terminal.md section 5.5, docs/spec/15-client-architektur.md section 23.3); recorded in the audit entry of `terminal.open`.
  @BuiltValueField(wireName: r'pin_changed')
  bool? get pinChanged;

  TerminalSessionOpen._();

  factory TerminalSessionOpen([void updates(TerminalSessionOpenBuilder b)]) =
      _$TerminalSessionOpen;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(TerminalSessionOpenBuilder b) => b..pinChanged = false;

  @BuiltValueSerializer(custom: true)
  static Serializer<TerminalSessionOpen> get serializer =>
      _$TerminalSessionOpenSerializer();
}

class _$TerminalSessionOpenSerializer
    implements PrimitiveSerializer<TerminalSessionOpen> {
  @override
  final Iterable<Type> types = const [
    TerminalSessionOpen,
    _$TerminalSessionOpen,
  ];

  @override
  final String wireName = r'TerminalSessionOpen';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    TerminalSessionOpen object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'profile';
    yield serializers.serialize(
      object.profile,
      specifiedType: const FullType(String),
    );
    if (object.workdirIndex != null) {
      yield r'workdir_index';
      yield serializers.serialize(
        object.workdirIndex,
        specifiedType: const FullType(int),
      );
    }
    yield r'cols';
    yield serializers.serialize(
      object.cols,
      specifiedType: const FullType(int),
    );
    yield r'rows';
    yield serializers.serialize(
      object.rows,
      specifiedType: const FullType(int),
    );
    yield r'client_ephemeral_key_hash';
    yield serializers.serialize(
      object.clientEphemeralKeyHash,
      specifiedType: const FullType(String),
    );
    if (object.pinChanged != null) {
      yield r'pin_changed';
      yield serializers.serialize(
        object.pinChanged,
        specifiedType: const FullType(bool),
      );
    }
  }

  @override
  Object serialize(
    Serializers serializers,
    TerminalSessionOpen object, {
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
    required TerminalSessionOpenBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'profile':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.profile = valueDes;
          break;
        case r'workdir_index':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(int),
          ) as int?;
          if (valueDes == null) continue;
          result.workdirIndex = valueDes;
          break;
        case r'cols':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.cols = valueDes;
          break;
        case r'rows':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.rows = valueDes;
          break;
        case r'client_ephemeral_key_hash':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(String),
          ) as String;
          result.clientEphemeralKeyHash = valueDes;
          break;
        case r'pin_changed':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType.nullable(bool),
          ) as bool?;
          if (valueDes == null) continue;
          result.pinChanged = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  TerminalSessionOpen deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = TerminalSessionOpenBuilder();
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
