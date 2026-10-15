//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'message_kind.g.dart';

class MessageKind extends EnumClass {
  @BuiltValueEnumConst(wireName: r'text')
  static const MessageKind text = _$text;
  @BuiltValueEnumConst(wireName: r'voice')
  static const MessageKind voice = _$voice;
  @BuiltValueEnumConst(wireName: r'run')
  static const MessageKind run = _$run;
  @BuiltValueEnumConst(wireName: r'card')
  static const MessageKind card = _$card;
  @BuiltValueEnumConst(wireName: r'system')
  static const MessageKind system = _$system;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const MessageKind unknownDefaultOpenApi = _$unknownDefaultOpenApi;

  static Serializer<MessageKind> get serializer => _$messageKindSerializer;

  const MessageKind._(String name) : super(name);

  static BuiltSet<MessageKind> get values => _$values;
  static MessageKind valueOf(String name) => _$valueOf(name);
}

/// Optionally, enum_class can generate a mixin to go with your enum for use
/// with Angular. It exposes your enum constants as getters. So, if you mix it
/// in to your Dart component class, the values become available to the
/// corresponding Angular template.
///
/// Trigger mixin generation by writing a line like this one next to your enum.
abstract class MessageKindMixin = Object with _$MessageKindMixin;
