// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'command_autocomplete_requested_payload.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$CommandAutocompleteRequestedPayload
    extends CommandAutocompleteRequestedPayload {
  @override
  final String requestId;
  @override
  final String commandId;
  @override
  final String channelId;
  @override
  final String? threadId;
  @override
  final String userId;
  @override
  final String option;
  @override
  final String value;
  @override
  final BuiltList<CommandOptionValue>? options;
  @override
  final DateTime expiresAt;

  factory _$CommandAutocompleteRequestedPayload([
    void Function(CommandAutocompleteRequestedPayloadBuilder)? updates,
  ]) =>
      (CommandAutocompleteRequestedPayloadBuilder()..update(updates))._build();

  _$CommandAutocompleteRequestedPayload._({
    required this.requestId,
    required this.commandId,
    required this.channelId,
    this.threadId,
    required this.userId,
    required this.option,
    required this.value,
    this.options,
    required this.expiresAt,
  }) : super._();
  @override
  CommandAutocompleteRequestedPayload rebuild(
    void Function(CommandAutocompleteRequestedPayloadBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  CommandAutocompleteRequestedPayloadBuilder toBuilder() =>
      CommandAutocompleteRequestedPayloadBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is CommandAutocompleteRequestedPayload &&
        requestId == other.requestId &&
        commandId == other.commandId &&
        channelId == other.channelId &&
        threadId == other.threadId &&
        userId == other.userId &&
        option == other.option &&
        value == other.value &&
        options == other.options &&
        expiresAt == other.expiresAt;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, requestId.hashCode);
    _$hash = $jc(_$hash, commandId.hashCode);
    _$hash = $jc(_$hash, channelId.hashCode);
    _$hash = $jc(_$hash, threadId.hashCode);
    _$hash = $jc(_$hash, userId.hashCode);
    _$hash = $jc(_$hash, option.hashCode);
    _$hash = $jc(_$hash, value.hashCode);
    _$hash = $jc(_$hash, options.hashCode);
    _$hash = $jc(_$hash, expiresAt.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'CommandAutocompleteRequestedPayload')
          ..add('requestId', requestId)
          ..add('commandId', commandId)
          ..add('channelId', channelId)
          ..add('threadId', threadId)
          ..add('userId', userId)
          ..add('option', option)
          ..add('value', value)
          ..add('options', options)
          ..add('expiresAt', expiresAt))
        .toString();
  }
}

class CommandAutocompleteRequestedPayloadBuilder
    implements
        Builder<
          CommandAutocompleteRequestedPayload,
          CommandAutocompleteRequestedPayloadBuilder
        > {
  _$CommandAutocompleteRequestedPayload? _$v;

  String? _requestId;
  String? get requestId => _$this._requestId;
  set requestId(String? requestId) => _$this._requestId = requestId;

  String? _commandId;
  String? get commandId => _$this._commandId;
  set commandId(String? commandId) => _$this._commandId = commandId;

  String? _channelId;
  String? get channelId => _$this._channelId;
  set channelId(String? channelId) => _$this._channelId = channelId;

  String? _threadId;
  String? get threadId => _$this._threadId;
  set threadId(String? threadId) => _$this._threadId = threadId;

  String? _userId;
  String? get userId => _$this._userId;
  set userId(String? userId) => _$this._userId = userId;

  String? _option;
  String? get option => _$this._option;
  set option(String? option) => _$this._option = option;

  String? _value;
  String? get value => _$this._value;
  set value(String? value) => _$this._value = value;

  ListBuilder<CommandOptionValue>? _options;
  ListBuilder<CommandOptionValue> get options =>
      _$this._options ??= ListBuilder<CommandOptionValue>();
  set options(ListBuilder<CommandOptionValue>? options) =>
      _$this._options = options;

  DateTime? _expiresAt;
  DateTime? get expiresAt => _$this._expiresAt;
  set expiresAt(DateTime? expiresAt) => _$this._expiresAt = expiresAt;

  CommandAutocompleteRequestedPayloadBuilder() {
    CommandAutocompleteRequestedPayload._defaults(this);
  }

  CommandAutocompleteRequestedPayloadBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _requestId = $v.requestId;
      _commandId = $v.commandId;
      _channelId = $v.channelId;
      _threadId = $v.threadId;
      _userId = $v.userId;
      _option = $v.option;
      _value = $v.value;
      _options = $v.options?.toBuilder();
      _expiresAt = $v.expiresAt;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(CommandAutocompleteRequestedPayload other) {
    _$v = other as _$CommandAutocompleteRequestedPayload;
  }

  @override
  void update(
    void Function(CommandAutocompleteRequestedPayloadBuilder)? updates,
  ) {
    if (updates != null) updates(this);
  }

  @override
  CommandAutocompleteRequestedPayload build() => _build();

  _$CommandAutocompleteRequestedPayload _build() {
    _$CommandAutocompleteRequestedPayload _$result;
    try {
      _$result =
          _$v ??
          _$CommandAutocompleteRequestedPayload._(
            requestId: BuiltValueNullFieldError.checkNotNull(
              requestId,
              r'CommandAutocompleteRequestedPayload',
              'requestId',
            ),
            commandId: BuiltValueNullFieldError.checkNotNull(
              commandId,
              r'CommandAutocompleteRequestedPayload',
              'commandId',
            ),
            channelId: BuiltValueNullFieldError.checkNotNull(
              channelId,
              r'CommandAutocompleteRequestedPayload',
              'channelId',
            ),
            threadId: threadId,
            userId: BuiltValueNullFieldError.checkNotNull(
              userId,
              r'CommandAutocompleteRequestedPayload',
              'userId',
            ),
            option: BuiltValueNullFieldError.checkNotNull(
              option,
              r'CommandAutocompleteRequestedPayload',
              'option',
            ),
            value: BuiltValueNullFieldError.checkNotNull(
              value,
              r'CommandAutocompleteRequestedPayload',
              'value',
            ),
            options: _options?.build(),
            expiresAt: BuiltValueNullFieldError.checkNotNull(
              expiresAt,
              r'CommandAutocompleteRequestedPayload',
              'expiresAt',
            ),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'options';
        _options?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'CommandAutocompleteRequestedPayload',
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
