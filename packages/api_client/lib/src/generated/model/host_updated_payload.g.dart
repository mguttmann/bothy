// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'host_updated_payload.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$HostUpdatedPayload extends HostUpdatedPayload {
  @override
  final Host host;
  @override
  final bool? keyChanged;

  factory _$HostUpdatedPayload([
    void Function(HostUpdatedPayloadBuilder)? updates,
  ]) => (HostUpdatedPayloadBuilder()..update(updates))._build();

  _$HostUpdatedPayload._({required this.host, this.keyChanged}) : super._();
  @override
  HostUpdatedPayload rebuild(
    void Function(HostUpdatedPayloadBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  HostUpdatedPayloadBuilder toBuilder() =>
      HostUpdatedPayloadBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is HostUpdatedPayload &&
        host == other.host &&
        keyChanged == other.keyChanged;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, host.hashCode);
    _$hash = $jc(_$hash, keyChanged.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'HostUpdatedPayload')
          ..add('host', host)
          ..add('keyChanged', keyChanged))
        .toString();
  }
}

class HostUpdatedPayloadBuilder
    implements Builder<HostUpdatedPayload, HostUpdatedPayloadBuilder> {
  _$HostUpdatedPayload? _$v;

  HostBuilder? _host;
  HostBuilder get host => _$this._host ??= HostBuilder();
  set host(HostBuilder? host) => _$this._host = host;

  bool? _keyChanged;
  bool? get keyChanged => _$this._keyChanged;
  set keyChanged(bool? keyChanged) => _$this._keyChanged = keyChanged;

  HostUpdatedPayloadBuilder() {
    HostUpdatedPayload._defaults(this);
  }

  HostUpdatedPayloadBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _host = $v.host.toBuilder();
      _keyChanged = $v.keyChanged;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(HostUpdatedPayload other) {
    _$v = other as _$HostUpdatedPayload;
  }

  @override
  void update(void Function(HostUpdatedPayloadBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  HostUpdatedPayload build() => _build();

  _$HostUpdatedPayload _build() {
    _$HostUpdatedPayload _$result;
    try {
      _$result =
          _$v ??
          _$HostUpdatedPayload._(host: host.build(), keyChanged: keyChanged);
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'host';
        host.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'HostUpdatedPayload',
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
