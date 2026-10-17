// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'channel_behavior.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$ChannelBehavior extends ChannelBehavior {
  @override
  final bool requireMention;
  @override
  final bool threadRequireMention;
  @override
  final bool freeResponse;
  @override
  final bool autoThread;
  @override
  final bool allowMentionedAgents;
  @override
  final int historyBackfillLimit;
  @override
  final String prompt;
  @override
  final BuiltList<String> skills;
  @override
  final ProjectBinding? projectBinding;
  @override
  final int settingsVersion;

  factory _$ChannelBehavior([void Function(ChannelBehaviorBuilder)? updates]) =>
      (ChannelBehaviorBuilder()..update(updates))._build();

  _$ChannelBehavior._({
    required this.requireMention,
    required this.threadRequireMention,
    required this.freeResponse,
    required this.autoThread,
    required this.allowMentionedAgents,
    required this.historyBackfillLimit,
    required this.prompt,
    required this.skills,
    this.projectBinding,
    required this.settingsVersion,
  }) : super._();
  @override
  ChannelBehavior rebuild(void Function(ChannelBehaviorBuilder) updates) =>
      (toBuilder()..update(updates)).build();

  @override
  ChannelBehaviorBuilder toBuilder() => ChannelBehaviorBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is ChannelBehavior &&
        requireMention == other.requireMention &&
        threadRequireMention == other.threadRequireMention &&
        freeResponse == other.freeResponse &&
        autoThread == other.autoThread &&
        allowMentionedAgents == other.allowMentionedAgents &&
        historyBackfillLimit == other.historyBackfillLimit &&
        prompt == other.prompt &&
        skills == other.skills &&
        projectBinding == other.projectBinding &&
        settingsVersion == other.settingsVersion;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, requireMention.hashCode);
    _$hash = $jc(_$hash, threadRequireMention.hashCode);
    _$hash = $jc(_$hash, freeResponse.hashCode);
    _$hash = $jc(_$hash, autoThread.hashCode);
    _$hash = $jc(_$hash, allowMentionedAgents.hashCode);
    _$hash = $jc(_$hash, historyBackfillLimit.hashCode);
    _$hash = $jc(_$hash, prompt.hashCode);
    _$hash = $jc(_$hash, skills.hashCode);
    _$hash = $jc(_$hash, projectBinding.hashCode);
    _$hash = $jc(_$hash, settingsVersion.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'ChannelBehavior')
          ..add('requireMention', requireMention)
          ..add('threadRequireMention', threadRequireMention)
          ..add('freeResponse', freeResponse)
          ..add('autoThread', autoThread)
          ..add('allowMentionedAgents', allowMentionedAgents)
          ..add('historyBackfillLimit', historyBackfillLimit)
          ..add('prompt', prompt)
          ..add('skills', skills)
          ..add('projectBinding', projectBinding)
          ..add('settingsVersion', settingsVersion))
        .toString();
  }
}

class ChannelBehaviorBuilder
    implements Builder<ChannelBehavior, ChannelBehaviorBuilder> {
  _$ChannelBehavior? _$v;

  bool? _requireMention;
  bool? get requireMention => _$this._requireMention;
  set requireMention(bool? requireMention) =>
      _$this._requireMention = requireMention;

  bool? _threadRequireMention;
  bool? get threadRequireMention => _$this._threadRequireMention;
  set threadRequireMention(bool? threadRequireMention) =>
      _$this._threadRequireMention = threadRequireMention;

  bool? _freeResponse;
  bool? get freeResponse => _$this._freeResponse;
  set freeResponse(bool? freeResponse) => _$this._freeResponse = freeResponse;

  bool? _autoThread;
  bool? get autoThread => _$this._autoThread;
  set autoThread(bool? autoThread) => _$this._autoThread = autoThread;

  bool? _allowMentionedAgents;
  bool? get allowMentionedAgents => _$this._allowMentionedAgents;
  set allowMentionedAgents(bool? allowMentionedAgents) =>
      _$this._allowMentionedAgents = allowMentionedAgents;

  int? _historyBackfillLimit;
  int? get historyBackfillLimit => _$this._historyBackfillLimit;
  set historyBackfillLimit(int? historyBackfillLimit) =>
      _$this._historyBackfillLimit = historyBackfillLimit;

  String? _prompt;
  String? get prompt => _$this._prompt;
  set prompt(String? prompt) => _$this._prompt = prompt;

  ListBuilder<String>? _skills;
  ListBuilder<String> get skills => _$this._skills ??= ListBuilder<String>();
  set skills(ListBuilder<String>? skills) => _$this._skills = skills;

  ProjectBindingBuilder? _projectBinding;
  ProjectBindingBuilder get projectBinding =>
      _$this._projectBinding ??= ProjectBindingBuilder();
  set projectBinding(ProjectBindingBuilder? projectBinding) =>
      _$this._projectBinding = projectBinding;

  int? _settingsVersion;
  int? get settingsVersion => _$this._settingsVersion;
  set settingsVersion(int? settingsVersion) =>
      _$this._settingsVersion = settingsVersion;

  ChannelBehaviorBuilder() {
    ChannelBehavior._defaults(this);
  }

  ChannelBehaviorBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _requireMention = $v.requireMention;
      _threadRequireMention = $v.threadRequireMention;
      _freeResponse = $v.freeResponse;
      _autoThread = $v.autoThread;
      _allowMentionedAgents = $v.allowMentionedAgents;
      _historyBackfillLimit = $v.historyBackfillLimit;
      _prompt = $v.prompt;
      _skills = $v.skills.toBuilder();
      _projectBinding = $v.projectBinding?.toBuilder();
      _settingsVersion = $v.settingsVersion;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(ChannelBehavior other) {
    _$v = other as _$ChannelBehavior;
  }

  @override
  void update(void Function(ChannelBehaviorBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  ChannelBehavior build() => _build();

  _$ChannelBehavior _build() {
    _$ChannelBehavior _$result;
    try {
      _$result =
          _$v ??
          _$ChannelBehavior._(
            requireMention: BuiltValueNullFieldError.checkNotNull(
              requireMention,
              r'ChannelBehavior',
              'requireMention',
            ),
            threadRequireMention: BuiltValueNullFieldError.checkNotNull(
              threadRequireMention,
              r'ChannelBehavior',
              'threadRequireMention',
            ),
            freeResponse: BuiltValueNullFieldError.checkNotNull(
              freeResponse,
              r'ChannelBehavior',
              'freeResponse',
            ),
            autoThread: BuiltValueNullFieldError.checkNotNull(
              autoThread,
              r'ChannelBehavior',
              'autoThread',
            ),
            allowMentionedAgents: BuiltValueNullFieldError.checkNotNull(
              allowMentionedAgents,
              r'ChannelBehavior',
              'allowMentionedAgents',
            ),
            historyBackfillLimit: BuiltValueNullFieldError.checkNotNull(
              historyBackfillLimit,
              r'ChannelBehavior',
              'historyBackfillLimit',
            ),
            prompt: BuiltValueNullFieldError.checkNotNull(
              prompt,
              r'ChannelBehavior',
              'prompt',
            ),
            skills: skills.build(),
            projectBinding: _projectBinding?.build(),
            settingsVersion: BuiltValueNullFieldError.checkNotNull(
              settingsVersion,
              r'ChannelBehavior',
              'settingsVersion',
            ),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'skills';
        skills.build();
        _$failedField = 'projectBinding';
        _projectBinding?.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'ChannelBehavior',
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
