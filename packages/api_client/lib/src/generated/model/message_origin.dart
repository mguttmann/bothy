//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'message_origin.g.dart';

class MessageOrigin extends EnumClass {
  @BuiltValueEnumConst(wireName: r'human')
  static const MessageOrigin human = _$human;
  @BuiltValueEnumConst(wireName: r'agent')
  static const MessageOrigin agent = _$agent;
  @BuiltValueEnumConst(wireName: r'app')
  static const MessageOrigin app = _$app;
  @BuiltValueEnumConst(wireName: r'import')
  static const MessageOrigin import_ = _$import_;
  @BuiltValueEnumConst(wireName: r'system')
  static const MessageOrigin system = _$system;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const MessageOrigin unknownDefaultOpenApi = _$unknownDefaultOpenApi;

  static Serializer<MessageOrigin> get serializer => _$messageOriginSerializer;

  const MessageOrigin._(String name) : super(name);

  static BuiltSet<MessageOrigin> get values => _$values;
  static MessageOrigin valueOf(String name) => _$valueOf(name);
}

/// Optionally, enum_class can generate a mixin to go with your enum for use
/// with Angular. It exposes your enum constants as getters. So, if you mix it
/// in to your Dart component class, the values become available to the
/// corresponding Angular template.
///
/// Trigger mixin generation by writing a line like this one next to your enum.
abstract class MessageOriginMixin = Object with _$MessageOriginMixin;
