// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'interaction_created_payload.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$InteractionCreatedPayload extends InteractionCreatedPayload {
  @override
  final Interaction interaction;

  factory _$InteractionCreatedPayload([
    void Function(InteractionCreatedPayloadBuilder)? updates,
  ]) => (InteractionCreatedPayloadBuilder()..update(updates))._build();

  _$InteractionCreatedPayload._({required this.interaction}) : super._();
  @override
  InteractionCreatedPayload rebuild(
    void Function(InteractionCreatedPayloadBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  InteractionCreatedPayloadBuilder toBuilder() =>
      InteractionCreatedPayloadBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is InteractionCreatedPayload &&
        interaction == other.interaction;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, interaction.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'InteractionCreatedPayload',
    )..add('interaction', interaction)).toString();
  }
}

class InteractionCreatedPayloadBuilder
    implements
        Builder<InteractionCreatedPayload, InteractionCreatedPayloadBuilder> {
  _$InteractionCreatedPayload? _$v;

  InteractionBuilder? _interaction;
  InteractionBuilder get interaction =>
      _$this._interaction ??= InteractionBuilder();
  set interaction(InteractionBuilder? interaction) =>
      _$this._interaction = interaction;

  InteractionCreatedPayloadBuilder() {
    InteractionCreatedPayload._defaults(this);
  }

  InteractionCreatedPayloadBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _interaction = $v.interaction.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(InteractionCreatedPayload other) {
    _$v = other as _$InteractionCreatedPayload;
  }

  @override
  void update(void Function(InteractionCreatedPayloadBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  InteractionCreatedPayload build() => _build();

  _$InteractionCreatedPayload _build() {
    _$InteractionCreatedPayload _$result;
    try {
      _$result =
          _$v ??
          _$InteractionCreatedPayload._(interaction: interaction.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'interaction';
        interaction.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'InteractionCreatedPayload',
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
