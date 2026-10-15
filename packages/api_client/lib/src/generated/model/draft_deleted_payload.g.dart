// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'draft_deleted_payload.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$DraftDeletedPayload extends DraftDeletedPayload {
  @override
  final String draftId;
  @override
  final String? threadId;
  @override
  final String channelId;

  factory _$DraftDeletedPayload([
    void Function(DraftDeletedPayloadBuilder)? updates,
  ]) => (DraftDeletedPayloadBuilder()..update(updates))._build();

  _$DraftDeletedPayload._({
    required this.draftId,
    this.threadId,
    required this.channelId,
  }) : super._();
  @override
  DraftDeletedPayload rebuild(
    void Function(DraftDeletedPayloadBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  DraftDeletedPayloadBuilder toBuilder() =>
      DraftDeletedPayloadBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is DraftDeletedPayload &&
        draftId == other.draftId &&
        threadId == other.threadId &&
        channelId == other.channelId;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, draftId.hashCode);
    _$hash = $jc(_$hash, threadId.hashCode);
    _$hash = $jc(_$hash, channelId.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'DraftDeletedPayload')
          ..add('draftId', draftId)
          ..add('threadId', threadId)
          ..add('channelId', channelId))
        .toString();
  }
}

class DraftDeletedPayloadBuilder
    implements Builder<DraftDeletedPayload, DraftDeletedPayloadBuilder> {
  _$DraftDeletedPayload? _$v;

  String? _draftId;
  String? get draftId => _$this._draftId;
  set draftId(String? draftId) => _$this._draftId = draftId;

  String? _threadId;
  String? get threadId => _$this._threadId;
  set threadId(String? threadId) => _$this._threadId = threadId;

  String? _channelId;
  String? get channelId => _$this._channelId;
  set channelId(String? channelId) => _$this._channelId = channelId;

  DraftDeletedPayloadBuilder() {
    DraftDeletedPayload._defaults(this);
  }

  DraftDeletedPayloadBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _draftId = $v.draftId;
      _threadId = $v.threadId;
      _channelId = $v.channelId;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(DraftDeletedPayload other) {
    _$v = other as _$DraftDeletedPayload;
  }

  @override
  void update(void Function(DraftDeletedPayloadBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  DraftDeletedPayload build() => _build();

  _$DraftDeletedPayload _build() {
    final _$result =
        _$v ??
        _$DraftDeletedPayload._(
          draftId: BuiltValueNullFieldError.checkNotNull(
            draftId,
            r'DraftDeletedPayload',
            'draftId',
          ),
          threadId: threadId,
          channelId: BuiltValueNullFieldError.checkNotNull(
            channelId,
            r'DraftDeletedPayload',
            'channelId',
          ),
        );
    replace(_$result);
    return _$result;
  }
}

// ignore_for_file: deprecated_member_use_from_same_package,type=lint
