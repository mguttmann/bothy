//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'terminal_session_attach.g.dart';

/// TerminalSessionAttach
///
/// Properties:
/// * [cols]
/// * [rows]
/// * [clientEphemeralKeyHash]
/// * [pinChanged] - Same as in `TerminalSessionOpen`; recorded in the audit entry of `terminal.attach`.
@BuiltValue()
abstract class TerminalSessionAttach
    implements Built<TerminalSessionAttach, TerminalSessionAttachBuilder> {
  @BuiltValueField(wireName: r'cols')
  int get cols;

  @BuiltValueField(wireName: r'rows')
  int get rows;

  @BuiltValueField(wireName: r'client_ephemeral_key_hash')
  String get clientEphemeralKeyHash;

  /// Same as in `TerminalSessionOpen`; recorded in the audit entry of `terminal.attach`.
  @BuiltValueField(wireName: r'pin_changed')
  bool? get pinChanged;

  TerminalSessionAttach._();

  factory TerminalSessionAttach([
    void updates(TerminalSessionAttachBuilder b),
  ]) = _$TerminalSessionAttach;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(TerminalSessionAttachBuilder b) =>
      b..pinChanged = false;

  @BuiltValueSerializer(custom: true)
  static Serializer<TerminalSessionAttach> get serializer =>
      _$TerminalSessionAttachSerializer();
}

class _$TerminalSessionAttachSerializer
    implements PrimitiveSerializer<TerminalSessionAttach> {
  @override
  final Iterable<Type> types = const [
    TerminalSessionAttach,
    _$TerminalSessionAttach,
  ];

  @override
  final String wireName = r'TerminalSessionAttach';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    TerminalSessionAttach object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
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
    TerminalSessionAttach object, {
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
    required TerminalSessionAttachBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
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
  TerminalSessionAttach deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = TerminalSessionAttachBuilder();
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
