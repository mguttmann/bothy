// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'interaction_create.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$InteractionCreate extends InteractionCreate {
  @override
  final String clientInteractionId;
  @override
  final BuiltMap<String, JsonObject?>? values;

  factory _$InteractionCreate([
    void Function(InteractionCreateBuilder)? updates,
  ]) => (InteractionCreateBuilder()..update(updates))._build();

  _$InteractionCreate._({required this.clientInteractionId, this.values})
    : super._();
  @override
  InteractionCreate rebuild(void Function(InteractionCreateBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  InteractionCreateBuilder toBuilder() =>
      InteractionCreateBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is InteractionCreate &&
        clientInteractionId == other.clientInteractionId &&
        values == other.values;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, clientInteractionId.hashCode);
    _$hash = $jc(_$hash, values.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'InteractionCreate')
          ..add('clientInteractionId', clientInteractionId)
          ..add('values', values))
        .toString();
  }
}

class InteractionCreateBuilder
    implements Builder<InteractionCreate, InteractionCreateBuilder> {
  _$InteractionCreate? _$v;

  String? _clientInteractionId;
  String? get clientInteractionId => _$this._clientInteractionId;
  set clientInteractionId(String? clientInteractionId) =>
      _$this._clientInteractionId = clientInteractionId;

  MapBuilder<String, JsonObject?>? _values;
  MapBuilder<String, JsonObject?> get values =>
      _$this._values ??= MapBuilder<String, JsonObject?>();
  set values(MapBuilder<String, JsonObject?>? values) =>
      _$this._values = values;

  InteractionCreateBuilder() {
    InteractionCreate._defaults(this);
  }

  InteractionCreateBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _clientInteractionId = $v.clientInteractionId;
      _values = $v.values?.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(InteractionCreate other) {
    _$v = other as _$InteractionCreate;
  }

  @override
  void update(void Function(InteractionCreateBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  InteractionCreate build() => _build();

  _$InteractionCreate _build() {
    _$InteractionCreate _$result;
    try {
      _$result =
          _$v ??
          _$InteractionCreate._(
            clientInteractionId: BuiltValueNullFieldError.checkNotNull(
              clientInteractionId,
              r'InteractionCreate',
              'clientInteractionId',
            ),
            values: _values?.build(),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'values';
        _values?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'InteractionCreate',
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
