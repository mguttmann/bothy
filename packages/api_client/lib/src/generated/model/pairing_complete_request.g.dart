// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'pairing_complete_request.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$PairingCompleteRequest extends PairingCompleteRequest {
  @override
  final String pairingId;

  factory _$PairingCompleteRequest([
    void Function(PairingCompleteRequestBuilder)? updates,
  ]) => (PairingCompleteRequestBuilder()..update(updates))._build();

  _$PairingCompleteRequest._({required this.pairingId}) : super._();
  @override
  PairingCompleteRequest rebuild(
    void Function(PairingCompleteRequestBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  PairingCompleteRequestBuilder toBuilder() =>
      PairingCompleteRequestBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is PairingCompleteRequest && pairingId == other.pairingId;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, pairingId.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(
      r'PairingCompleteRequest',
    )..add('pairingId', pairingId)).toString();
  }
}

class PairingCompleteRequestBuilder
    implements Builder<PairingCompleteRequest, PairingCompleteRequestBuilder> {
  _$PairingCompleteRequest? _$v;

  String? _pairingId;
  String? get pairingId => _$this._pairingId;
  set pairingId(String? pairingId) => _$this._pairingId = pairingId;

  PairingCompleteRequestBuilder() {
    PairingCompleteRequest._defaults(this);
  }

  PairingCompleteRequestBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _pairingId = $v.pairingId;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(PairingCompleteRequest other) {
    _$v = other as _$PairingCompleteRequest;
  }

  @override
  void update(void Function(PairingCompleteRequestBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  PairingCompleteRequest build() => _build();

  _$PairingCompleteRequest _build() {
    final _$result =
        _$v ??
        _$PairingCompleteRequest._(
          pairingId: BuiltValueNullFieldError.checkNotNull(
            pairingId,
            r'PairingCompleteRequest',
            'pairingId',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
