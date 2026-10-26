//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'app_lock_policy.g.dart';

/// The app lock policy of the phone apps (08 9): the server cannot check the lock, so it only delivers the two values the app enforces. Both come from the process configuration, not from an instance setting.
///
/// Properties:
/// * [required_] - Whether the owner requires the app lock on every device (`security.app_lock_required`); the app then shows the switch on and disabled, and a device without a device credential shows no content.
/// * [afterMs] - Time without use after which an app with the lock on asks for the system authentication again (`client.app_lock_after`), in milliseconds.
@BuiltValue()
abstract class AppLockPolicy
    implements Built<AppLockPolicy, AppLockPolicyBuilder> {
  /// Whether the owner requires the app lock on every device (`security.app_lock_required`); the app then shows the switch on and disabled, and a device without a device credential shows no content.
  @BuiltValueField(wireName: r'required')
  bool get required_;

  /// Time without use after which an app with the lock on asks for the system authentication again (`client.app_lock_after`), in milliseconds.
  @BuiltValueField(wireName: r'after_ms')
  int get afterMs;

  AppLockPolicy._();

  factory AppLockPolicy([void updates(AppLockPolicyBuilder b)]) =
      _$AppLockPolicy;

  @BuiltValueHook(initializeBuilder: true)
  static void _defaults(AppLockPolicyBuilder b) => b;

  @BuiltValueSerializer(custom: true)
  static Serializer<AppLockPolicy> get serializer =>
      _$AppLockPolicySerializer();
}

class _$AppLockPolicySerializer implements PrimitiveSerializer<AppLockPolicy> {
  @override
  final Iterable<Type> types = const [AppLockPolicy, _$AppLockPolicy];

  @override
  final String wireName = r'AppLockPolicy';

  Iterable<Object?> _serializeProperties(
    Serializers serializers,
    AppLockPolicy object, {
    FullType specifiedType = FullType.unspecified,
  }) sync* {
    yield r'required';
    yield serializers.serialize(
      object.required_,
      specifiedType: const FullType(bool),
    );
    yield r'after_ms';
    yield serializers.serialize(
      object.afterMs,
      specifiedType: const FullType(int),
    );
  }

  @override
  Object serialize(
    Serializers serializers,
    AppLockPolicy object, {
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
    required AppLockPolicyBuilder result,
    required List<Object?> unhandled,
  }) {
    for (var i = 0; i < serializedList.length; i += 2) {
      final key = serializedList[i] as String;
      final value = serializedList[i + 1];
      switch (key) {
        case r'required':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(bool),
          ) as bool;
          result.required_ = valueDes;
          break;
        case r'after_ms':
          final valueDes = serializers.deserialize(
            value,
            specifiedType: const FullType(int),
          ) as int;
          result.afterMs = valueDes;
          break;
        default:
          unhandled.add(key);
          unhandled.add(value);
          break;
      }
    }
  }

  @override
  AppLockPolicy deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) {
    final result = AppLockPolicyBuilder();
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
