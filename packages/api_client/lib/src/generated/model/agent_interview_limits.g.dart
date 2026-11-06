// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'agent_interview_limits.dart';

// **************************************************************************
// BuiltValueGenerator
// **************************************************************************

class _$AgentInterviewLimits extends AgentInterviewLimits {
  @override
  final int instructionsMaxBytes;
  @override
  final int maxSkills;
  @override
  final int skillDescriptionMaxChars;
  @override
  final int skillBodyMaxBytes;
  @override
  final int maxSkillFiles;
  @override
  final int maxMcpServers;
  @override
  final int maxAllowedTools;
  @override
  final num maxCostUsdPerRun;
  @override
  final BuiltList<String> mcpStdioRunners;
  @override
  final int questionMaxChars;

  factory _$AgentInterviewLimits([
    void Function(AgentInterviewLimitsBuilder)? updates,
  ]) => (AgentInterviewLimitsBuilder()..update(updates))._build();

  _$AgentInterviewLimits._({
    required this.instructionsMaxBytes,
    required this.maxSkills,
    required this.skillDescriptionMaxChars,
    required this.skillBodyMaxBytes,
    required this.maxSkillFiles,
    required this.maxMcpServers,
    required this.maxAllowedTools,
    required this.maxCostUsdPerRun,
    required this.mcpStdioRunners,
    required this.questionMaxChars,
  }) : super._();
  @override
  AgentInterviewLimits rebuild(
    void Function(AgentInterviewLimitsBuilder) updates,
  ) => (toBuilder()..update(updates)).build();

  @override
  AgentInterviewLimitsBuilder toBuilder() =>
      AgentInterviewLimitsBuilder()..replace(this);

  @override
  bool operator ==(Object other) {
    if (identical(other, this)) return true;
    return other is AgentInterviewLimits &&
        instructionsMaxBytes == other.instructionsMaxBytes &&
        maxSkills == other.maxSkills &&
        skillDescriptionMaxChars == other.skillDescriptionMaxChars &&
        skillBodyMaxBytes == other.skillBodyMaxBytes &&
        maxSkillFiles == other.maxSkillFiles &&
        maxMcpServers == other.maxMcpServers &&
        maxAllowedTools == other.maxAllowedTools &&
        maxCostUsdPerRun == other.maxCostUsdPerRun &&
        mcpStdioRunners == other.mcpStdioRunners &&
        questionMaxChars == other.questionMaxChars;
  }

  @override
  int get hashCode {
    var _$hash = 0;
    _$hash = $jc(_$hash, instructionsMaxBytes.hashCode);
    _$hash = $jc(_$hash, maxSkills.hashCode);
    _$hash = $jc(_$hash, skillDescriptionMaxChars.hashCode);
    _$hash = $jc(_$hash, skillBodyMaxBytes.hashCode);
    _$hash = $jc(_$hash, maxSkillFiles.hashCode);
    _$hash = $jc(_$hash, maxMcpServers.hashCode);
    _$hash = $jc(_$hash, maxAllowedTools.hashCode);
    _$hash = $jc(_$hash, maxCostUsdPerRun.hashCode);
    _$hash = $jc(_$hash, mcpStdioRunners.hashCode);
    _$hash = $jc(_$hash, questionMaxChars.hashCode);
    _$hash = $jf(_$hash);
    return _$hash;
  }

  @override
  String toString() {
    return (newBuiltValueToStringHelper(r'AgentInterviewLimits')
          ..add('instructionsMaxBytes', instructionsMaxBytes)
          ..add('maxSkills', maxSkills)
          ..add('skillDescriptionMaxChars', skillDescriptionMaxChars)
          ..add('skillBodyMaxBytes', skillBodyMaxBytes)
          ..add('maxSkillFiles', maxSkillFiles)
          ..add('maxMcpServers', maxMcpServers)
          ..add('maxAllowedTools', maxAllowedTools)
          ..add('maxCostUsdPerRun', maxCostUsdPerRun)
          ..add('mcpStdioRunners', mcpStdioRunners)
          ..add('questionMaxChars', questionMaxChars))
        .toString();
  }
}

class AgentInterviewLimitsBuilder
    implements Builder<AgentInterviewLimits, AgentInterviewLimitsBuilder> {
  _$AgentInterviewLimits? _$v;

  int? _instructionsMaxBytes;
  int? get instructionsMaxBytes => _$this._instructionsMaxBytes;
  set instructionsMaxBytes(int? instructionsMaxBytes) =>
      _$this._instructionsMaxBytes = instructionsMaxBytes;

  int? _maxSkills;
  int? get maxSkills => _$this._maxSkills;
  set maxSkills(int? maxSkills) => _$this._maxSkills = maxSkills;

  int? _skillDescriptionMaxChars;
  int? get skillDescriptionMaxChars => _$this._skillDescriptionMaxChars;
  set skillDescriptionMaxChars(int? skillDescriptionMaxChars) =>
      _$this._skillDescriptionMaxChars = skillDescriptionMaxChars;

  int? _skillBodyMaxBytes;
  int? get skillBodyMaxBytes => _$this._skillBodyMaxBytes;
  set skillBodyMaxBytes(int? skillBodyMaxBytes) =>
      _$this._skillBodyMaxBytes = skillBodyMaxBytes;

  int? _maxSkillFiles;
  int? get maxSkillFiles => _$this._maxSkillFiles;
  set maxSkillFiles(int? maxSkillFiles) =>
      _$this._maxSkillFiles = maxSkillFiles;

  int? _maxMcpServers;
  int? get maxMcpServers => _$this._maxMcpServers;
  set maxMcpServers(int? maxMcpServers) =>
      _$this._maxMcpServers = maxMcpServers;

  int? _maxAllowedTools;
  int? get maxAllowedTools => _$this._maxAllowedTools;
  set maxAllowedTools(int? maxAllowedTools) =>
      _$this._maxAllowedTools = maxAllowedTools;

  num? _maxCostUsdPerRun;
  num? get maxCostUsdPerRun => _$this._maxCostUsdPerRun;
  set maxCostUsdPerRun(num? maxCostUsdPerRun) =>
      _$this._maxCostUsdPerRun = maxCostUsdPerRun;

  ListBuilder<String>? _mcpStdioRunners;
  ListBuilder<String> get mcpStdioRunners =>
      _$this._mcpStdioRunners ??= ListBuilder<String>();
  set mcpStdioRunners(ListBuilder<String>? mcpStdioRunners) =>
      _$this._mcpStdioRunners = mcpStdioRunners;

  int? _questionMaxChars;
  int? get questionMaxChars => _$this._questionMaxChars;
  set questionMaxChars(int? questionMaxChars) =>
      _$this._questionMaxChars = questionMaxChars;

  AgentInterviewLimitsBuilder() {
    AgentInterviewLimits._defaults(this);
  }

  AgentInterviewLimitsBuilder get _$this {
    final $v = _$v;
    if ($v != null) {
      _instructionsMaxBytes = $v.instructionsMaxBytes;
      _maxSkills = $v.maxSkills;
      _skillDescriptionMaxChars = $v.skillDescriptionMaxChars;
      _skillBodyMaxBytes = $v.skillBodyMaxBytes;
      _maxSkillFiles = $v.maxSkillFiles;
      _maxMcpServers = $v.maxMcpServers;
      _maxAllowedTools = $v.maxAllowedTools;
      _maxCostUsdPerRun = $v.maxCostUsdPerRun;
      _mcpStdioRunners = $v.mcpStdioRunners.toBuilder();
      _questionMaxChars = $v.questionMaxChars;
      _$v = null;
    }
    return this;
  }

  @override
  void replace(AgentInterviewLimits other) {
    _$v = other as _$AgentInterviewLimits;
  }

  @override
  void update(void Function(AgentInterviewLimitsBuilder)? updates) {
    if (updates != null) updates(this);
  }

  @override
  AgentInterviewLimits build() => _build();

  _$AgentInterviewLimits _build() {
    _$AgentInterviewLimits _$result;
    try {
      _$result =
          _$v ??
          _$AgentInterviewLimits._(
            instructionsMaxBytes: BuiltValueNullFieldError.checkNotNull(
              instructionsMaxBytes,
              r'AgentInterviewLimits',
              'instructionsMaxBytes',
            ),
            maxSkills: BuiltValueNullFieldError.checkNotNull(
              maxSkills,
              r'AgentInterviewLimits',
              'maxSkills',
            ),
            skillDescriptionMaxChars: BuiltValueNullFieldError.checkNotNull(
              skillDescriptionMaxChars,
              r'AgentInterviewLimits',
              'skillDescriptionMaxChars',
            ),
            skillBodyMaxBytes: BuiltValueNullFieldError.checkNotNull(
              skillBodyMaxBytes,
              r'AgentInterviewLimits',
              'skillBodyMaxBytes',
            ),
            maxSkillFiles: BuiltValueNullFieldError.checkNotNull(
              maxSkillFiles,
              r'AgentInterviewLimits',
              'maxSkillFiles',
            ),
            maxMcpServers: BuiltValueNullFieldError.checkNotNull(
              maxMcpServers,
              r'AgentInterviewLimits',
              'maxMcpServers',
            ),
            maxAllowedTools: BuiltValueNullFieldError.checkNotNull(
              maxAllowedTools,
              r'AgentInterviewLimits',
              'maxAllowedTools',
            ),
            maxCostUsdPerRun: BuiltValueNullFieldError.checkNotNull(
              maxCostUsdPerRun,
              r'AgentInterviewLimits',
              'maxCostUsdPerRun',
            ),
            mcpStdioRunners: mcpStdioRunners.build(),
            questionMaxChars: BuiltValueNullFieldError.checkNotNull(
              questionMaxChars,
              r'AgentInterviewLimits',
              'questionMaxChars',
            ),
          );
    } catch (_) {
      late String _$failedField;
      try {
        _$failedField = 'mcpStdioRunners';
        mcpStdioRunners.build();
      } catch (e) {
        throw BuiltValueNestedFieldError(
          r'AgentInterviewLimits',
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
