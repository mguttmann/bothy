// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'read_state_payload.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ReadStatePayload extends ReadStatePayload {
  @override
  final ReadState readState;

  factory _$ReadStatePayload([
    void Function(ReadStatePayloadBuilder)? updates,
  ]) => (ReadStatePayloadBuilder()..update(updates))._build();

  _$ReadStatePayload._({required this.readState}) : super._();
  @override
  ReadStatePayload rebuild(void Function(ReadStatePayloadBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ReadStatePayloadBuilder toBuilder() =>
      ReadStatePayloadBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ReadStatePayload && readState == other.readState;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, readState.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'ReadStatePayload',
    )..add('readState', readState)).toString();
  }
}

class ReadStatePayloadBuilder
    implements Builder<ReadStatePayload, ReadStatePayloadBuilder> {
  _$ReadStatePayload? _$v;

  ReadStateBuilder? _readState;
  ReadStateBuilder get readState => _$this._readState ??= ReadStateBuilder();
  set readState(ReadStateBuilder? readState) => _$this._readState = readState;

  ReadStatePayloadBuilder() {
    ReadStatePayload._defaults(this);
  }

  ReadStatePayloadBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _readState = $v.readState.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ReadStatePayload other) {
    _$v = other as _$ReadStatePayload;
  }

  @override
  void update(void Function(ReadStatePayloadBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ReadStatePayload build() => _build();

  _$ReadStatePayload _build() {
    _$ReadStatePayload _$result;
    try {
      _$result = _$v ?? _$ReadStatePayload._(readState: readState.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'readState';
        readState.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'ReadStatePayload',
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
