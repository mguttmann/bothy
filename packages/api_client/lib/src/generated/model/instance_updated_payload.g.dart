// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'instance_updated_payload.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$InstanceUpdatedPayload extends InstanceUpdatedPayload {
  @override
  final BuiltList<String> changedKeys;

  factory _$InstanceUpdatedPayload([
    void Function(InstanceUpdatedPayloadBuilder)? updates,
  ]) => (InstanceUpdatedPayloadBuilder()..update(updates))._build();

  _$InstanceUpdatedPayload._({required this.changedKeys}) : super._();
  @override
  InstanceUpdatedPayload rebuild(
    void Function(InstanceUpdatedPayloadBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  InstanceUpdatedPayloadBuilder toBuilder() =>
      InstanceUpdatedPayloadBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is InstanceUpdatedPayload && changedKeys == other.changedKeys;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, changedKeys.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'InstanceUpdatedPayload',
    )..add('changedKeys', changedKeys)).toString();
  }
}

class InstanceUpdatedPayloadBuilder
    implements Builder<InstanceUpdatedPayload, InstanceUpdatedPayloadBuilder> {
  _$InstanceUpdatedPayload? _$v;

  ListBuilder<String>? _changedKeys;
  ListBuilder<String> get changedKeys =>
      _$this._changedKeys ??= ListBuilder<String>();
  set changedKeys(ListBuilder<String>? changedKeys) =>
      _$this._changedKeys = changedKeys;

  InstanceUpdatedPayloadBuilder() {
    InstanceUpdatedPayload._defaults(this);
  }

  InstanceUpdatedPayloadBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _changedKeys = $v.changedKeys.toBuilder();
      _$v = null;
    }
    return this;
  }

  @override
  void replace(InstanceUpdatedPayload other) {
    _$v = other as _$InstanceUpdatedPayload;
  }

  @override
  void update(void Function(InstanceUpdatedPayloadBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  InstanceUpdatedPayload build() => _build();

  _$InstanceUpdatedPayload _build() {
    _$InstanceUpdatedPayload _$result;
    try {
      _$result =
          _$v ?? _$InstanceUpdatedPayload._(changedKeys: changedKeys.build());
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'changedKeys';
        changedKeys.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'InstanceUpdatedPayload',
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
