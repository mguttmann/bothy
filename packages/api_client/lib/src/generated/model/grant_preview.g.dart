// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'grant_preview.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

const GrantPreviewTextLocaleEnum _$grantPreviewTextLocaleEnum_de =
    const GrantPreviewTextLocaleEnum._('de');
const GrantPreviewTextLocaleEnum _$grantPreviewTextLocaleEnum_en =
    const GrantPreviewTextLocaleEnum._('en');
const GrantPreviewTextLocaleEnum
_$grantPreviewTextLocaleEnum_unknownDefaultOpenApi =
    const GrantPreviewTextLocaleEnum._('unknownDefaultOpenApi');

GrantPreviewTextLocaleEnum _$grantPreviewTextLocaleEnumValueOf(String name) {
  switch (name) {
    case 'de':
      return _$grantPreviewTextLocaleEnum_de;
    case 'en':
      return _$grantPreviewTextLocaleEnum_en;
    case 'unknownDefaultOpenApi':
      return _$grantPreviewTextLocaleEnum_unknownDefaultOpenApi;
    default:
      return _$grantPreviewTextLocaleEnum_unknownDefaultOpenApi;
  }
}

final BuiltSet<GrantPreviewTextLocaleEnum> _$grantPreviewTextLocaleEnumValues =
    BuiltSet<GrantPreviewTextLocaleEnum>(const <GrantPreviewTextLocaleEnum>[
      _$grantPreviewTextLocaleEnum_de,
      _$grantPreviewTextLocaleEnum_en,
      _$grantPreviewTextLocaleEnum_unknownDefaultOpenApi,
    ]);

Serializer<GrantPreviewTextLocaleEnum> _$grantPreviewTextLocaleEnumSerializer =
    _$GrantPreviewTextLocaleEnumSerializer();

class _$GrantPreviewTextLocaleEnumSerializer
    implements PrimitiveSerializer<GrantPreviewTextLocaleEnum> {
  static const Map<String, Object> _toWire = const <String, Object>{
    'de': 'de',
    'en': 'en',
    'unknownDefaultOpenApi': 'unknown_default_open_api',
  };
  static const Map<Object, String> _fromWire = const <Object, String>{
    'de': 'de',
    'en': 'en',
    'unknown_default_open_api': 'unknownDefaultOpenApi',
  };

  @override
  final Iterable<Type> types = const <Type>[GrantPreviewTextLocaleEnum];
  @override
  final String wireName = 'GrantPreviewTextLocaleEnum';

  @override
  Object serialize(
    Serializers serializers,
    GrantPreviewTextLocaleEnum object, {
    FullType specifiedType = FullType.unspecified,
  }) => _toWire[object.name] ?? object.name;

  @override
  GrantPreviewTextLocaleEnum deserialize(
    Serializers serializers,
    Object serialized, {
    FullType specifiedType = FullType.unspecified,
  }) => GrantPreviewTextLocaleEnum.valueOf(
    _fromWire[serialized] ?? (serialized is String ? serialized : ''),
  );
}

class _$GrantPreview extends GrantPreview {
  @override
  final bool allowed;
  @override
  final String? denialReason;
  @override
  final BuiltList<AccessEffect> effects;
  @override
  final BuiltList<InvariantViolation> invariantViolations;
  @override
  final BuiltList<PreviewHint> hints;
  @override
  final String text;
  @override
  final GrantPreviewTextLocaleEnum textLocale;

  factory _$GrantPreview([void Function(GrantPreviewBuilder)? updates]) =>
      (GrantPreviewBuilder()..update(updates))._build();

  _$GrantPreview._({
    required this.allowed,
    this.denialReason,
    required this.effects,
    required this.invariantViolations,
    required this.hints,
    required this.text,
    required this.textLocale,
  }) : super._();
  @override
  GrantPreview rebuild(void Function(GrantPreviewBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  GrantPreviewBuilder toBuilder() => GrantPreviewBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is GrantPreview &&
        allowed == other.allowed &&
        denialReason == other.denialReason &&
        effects == other.effects &&
        invariantViolations == other.invariantViolations &&
        hints == other.hints &&
        text == other.text &&
        textLocale == other.textLocale;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, allowed.hashCode);
    _$hash = $jc(_$hash, denialReason.hashCode);
    _$hash = $jc(_$hash, effects.hashCode);
    _$hash = $jc(_$hash, invariantViolations.hashCode);
    _$hash = $jc(_$hash, hints.hashCode);
    _$hash = $jc(_$hash, text.hashCode);
    _$hash = $jc(_$hash, textLocale.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'GrantPreview')
          ..add('allowed', allowed)
          ..add('denialReason', denialReason)
          ..add('effects', effects)
          ..add('invariantViolations', invariantViolations)
          ..add('hints', hints)
          ..add('text', text)
          ..add('textLocale', textLocale))
        .toString();
  }
}

class GrantPreviewBuilder
    implements Builder<GrantPreview, GrantPreviewBuilder> {
  _$GrantPreview? _$v;

  bool? _allowed;
  bool? get allowed => _$this._allowed;
  set allowed(bool? allowed) => _$this._allowed = allowed;

  String? _denialReason;
  String? get denialReason => _$this._denialReason;
  set denialReason(String? denialReason) => _$this._denialReason = denialReason;

  ListBuilder<AccessEffect>? _effects;
  ListBuilder<AccessEffect> get effects =>
      _$this._effects ??= ListBuilder<AccessEffect>();
  set effects(ListBuilder<AccessEffect>? effects) => _$this._effects = effects;

  ListBuilder<InvariantViolation>? _invariantViolations;
  ListBuilder<InvariantViolation> get invariantViolations =>
      _$this._invariantViolations ??= ListBuilder<InvariantViolation>();
  set invariantViolations(
    ListBuilder<InvariantViolation>? invariantViolations,
  ) => _$this._invariantViolations = invariantViolations;

  ListBuilder<PreviewHint>? _hints;
  ListBuilder<PreviewHint> get hints =>
      _$this._hints ??= ListBuilder<PreviewHint>();
  set hints(ListBuilder<PreviewHint>? hints) => _$this._hints = hints;

  String? _text;
  String? get text => _$this._text;
  set text(String? text) => _$this._text = text;

  GrantPreviewTextLocaleEnum? _textLocale;
  GrantPreviewTextLocaleEnum? get textLocale => _$this._textLocale;
  set textLocale(GrantPreviewTextLocaleEnum? textLocale) =>
      _$this._textLocale = textLocale;

  GrantPreviewBuilder() {
    GrantPreview._defaults(this);
  }

  GrantPreviewBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _allowed = $v.allowed;
      _denialReason = $v.denialReason;
      _effects = $v.effects.toBuilder();
      _invariantViolations = $v.invariantViolations.toBuilder();
      _hints = $v.hints.toBuilder();
      _text = $v.text;
      _textLocale = $v.textLocale;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(GrantPreview other) {
    _$v = other as _$GrantPreview;
  }

  @override
  void update(void Function(GrantPreviewBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  GrantPreview build() => _build();

  _$GrantPreview _build() {
    _$GrantPreview _$result;
    try {
      _$result =
          _$v ??
          _$GrantPreview._(
            allowed: BuiltValueNullFieldError.checkNotNull(
              allowed,
              r'GrantPreview',
              'allowed',
            ),
            denialReason: denialReason,
            effects: effects.build(),
            invariantViolations: invariantViolations.build(),
            hints: hints.build(),
            text: BuiltValueNullFieldError.checkNotNull(
              text,
              r'GrantPreview',
              'text',
            ),
            textLocale: BuiltValueNullFieldError.checkNotNull(
              textLocale,
              r'GrantPreview',
              'textLocale',
            ),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'effects';
        effects.build();
        _$failedField = 'invariantViolations';
        invariantViolations.build();
        _$failedField = 'hints';
        hints.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'GrantPreview',
          _$failedField,
          e.toString(),
        );
      }
      rethrow;
    }
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
