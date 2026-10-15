// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'pin_payload.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$PinPayload extends PinPayload {
  @override
  final Pin pin;

  factory _$PinPayload([void Function(PinPayloadBuilder)? updates]) =>
      (PinPayloadBuilder()..update(updates))._build();

  _$PinPayload._({required this.pin}) : super._();
  @override
  PinPayload rebuild(void Function(PinPayloadBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  PinPayloadBuilder toBuilder() => PinPayloadBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is PinPayload && pin == other.pin;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, pin.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'PinPayload',
    )..add('pin', pin)).toString();
  }
}

class PinPayloadBuilder implements Builder<PinPayload, PinPayloadBuilder> {
  _$PinPayload? _$v;

  PinBuilder? _pin;
  PinBuilder get pin => _$this._pin ??= PinBuilder();
  set pin(PinBuilder? pin) => _$this._pin = pin;

  PinPayloadBuilder() {
    PinPayload._defaults(this);
  }

  PinPayloadBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _pin = $v.pin.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(PinPayload other) {
    _$v = other as _$PinPayload;
  }

  @override
  void update(void Function(PinPayloadBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  PinPayload build() => _build();

  _$PinPayload _build() {
    _$PinPayload _$result;
    try {
      _$result = _$v ?? _$PinPayload._(pin: pin.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'pin';
        pin.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'PinPayload',
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
