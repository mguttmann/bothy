// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'notification_pref_payload.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$NotificationPrefPayload extends NotificationPrefPayload {
  @override
  final NotificationPreference preference;
  @override
  final bool? deleted;

  factory _$NotificationPrefPayload([
    void Function(NotificationPrefPayloadBuilder)? updates,
  ]) => (NotificationPrefPayloadBuilder()..update(updates))._build();

  _$NotificationPrefPayload._({required this.preference, this.deleted})
    : super._();
  @override
  NotificationPrefPayload rebuild(
    void Function(NotificationPrefPayloadBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  NotificationPrefPayloadBuilder toBuilder() =>
      NotificationPrefPayloadBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is NotificationPrefPayload &&
        preference == other.preference &&
        deleted == other.deleted;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, preference.hashCode);
    _$hash = $jc(_$hash, deleted.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'NotificationPrefPayload')
          ..add('preference', preference)
          ..add('deleted', deleted))
        .toString();
  }
}

class NotificationPrefPayloadBuilder
    implements
        Builder<NotificationPrefPayload, NotificationPrefPayloadBuilder> {
  _$NotificationPrefPayload? _$v;

  NotificationPreferenceBuilder? _preference;
  NotificationPreferenceBuilder get preference =>
      _$this._preference ??= NotificationPreferenceBuilder();
  set preference(NotificationPreferenceBuilder? preference) =>
      _$this._preference = preference;

  bool? _deleted;
  bool? get deleted => _$this._deleted;
  set deleted(bool? deleted) => _$this._deleted = deleted;

  NotificationPrefPayloadBuilder() {
    NotificationPrefPayload._defaults(this);
  }

  NotificationPrefPayloadBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _preference = $v.preference.toBuilder();
      _deleted = $v.deleted;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(NotificationPrefPayload other) {
    _$v = other as _$NotificationPrefPayload;
  }

  @override
  void update(void Function(NotificationPrefPayloadBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  NotificationPrefPayload build() => _build();

  _$NotificationPrefPayload _build() {
    _$NotificationPrefPayload _$result;
    try {
      _$result =
          _$v ??
          _$NotificationPrefPayload._(
            preference: preference.build(),
            deleted: deleted,
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'preference';
        preference.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'NotificationPrefPayload',
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
