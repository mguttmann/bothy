//
// AUTO-GENERATED FILE, DO NOT MODIFY!
//

// ignore_for_file: unused_element
import 'package:built_collection/built_collection.dart';
import 'package:built_value/built_value.dart';
import 'package:built_value/serializer.dart';

part 'channel_kind.g.dart';

class ChannelKind extends EnumClass {
  @BuiltValueEnumConst(wireName: r'text')
  static const ChannelKind text = _$text;
  @BuiltValueEnumConst(wireName: r'forum')
  static const ChannelKind forum = _$forum;
  @BuiltValueEnumConst(wireName: r'dm')
  static const ChannelKind dm = _$dm;
  @BuiltValueEnumConst(wireName: r'unknown_default_open_api', fallback: true)
  static const ChannelKind unknownDefaultOpenApi = _$unknownDefaultOpenApi;

  static Serializer<ChannelKind> get serializer => _$channelKindSerializer;

  const ChannelKind._(String name) : super(name);

  static BuiltSet<ChannelKind> get values => _$values;
  static ChannelKind valueOf(String name) => _$valueOf(name);
}

/// Optionally, enum_class can generate a mixin to go with your enum for use
/// with Angular. It exposes your enum constants as getters. So, if you mix it
/// in to your Dart component class, the values become available to the
/// corresponding Angular template.
///
/// Trigger mixin generation by writing a line like this one next to your enum.
abstract class ChannelKindMixin = Object with _$ChannelKindMixin;
