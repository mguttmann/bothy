// ignore: unused_import
import 'package:intl/intl.dart' as intl;

import 'app_localizations.dart';

// ignore_for_file: type=lint

/// The translations for German (`de`).
class AppLocalizationsDe extends AppLocalizations {
  AppLocalizationsDe([String locale = 'de']) : super(locale);

  @override
  String get triggerStateReceived => 'Empfangen';

  @override
  String get triggerStateQueued => 'In Warteschlange';

  @override
  String get triggerStateWorking => 'Wird bearbeitet';

  @override
  String get triggerStateDone => 'Erledigt';

  @override
  String get triggerStateFailed => 'Fehlgeschlagen';

  @override
  String get triggerStateFollowupDelivered => 'Nachtrag zugestellt';

  @override
  String triggerStateFailedWithReason(String reason) {
    return 'Fehlgeschlagen: $reason';
  }

  @override
  String get liveBoxStateQueued => 'In Warteschlange';

  @override
  String get liveBoxStateRunning => 'Läuft';

  @override
  String get liveBoxStateWaitingForYou => 'Wartet auf dich';

  @override
  String get liveBoxStateCompleted => 'Erledigt';

  @override
  String get liveBoxStateFailed => 'Fehlgeschlagen';

  @override
  String get liveBoxStateCancelled => 'Abgebrochen';

  @override
  String liveBoxStateWaitingFor(String name) {
    return 'Wartet auf $name';
  }

  @override
  String get liveBoxReasonBillingModeMismatch => 'Backend passt nicht';

  @override
  String get liveBoxReasonAccessRevoked => 'Zugriff entzogen';

  @override
  String get liveBoxReasonStale => 'Abgebrochen nach Stillstand';

  @override
  String get liveBoxReasonAdapterError => 'Fehler im Adapter';

  @override
  String get liveBoxReasonCancelTimeout =>
      'Adapter hat den Abbruch nicht bestätigt';

  @override
  String get liveBoxReasonProjectNotReady => 'Projekt nicht bereit';

  @override
  String liveBoxReasonUserCancelled(String name) {
    return 'Von $name abgebrochen';
  }

  @override
  String get liveBoxStop => 'Stopp';

  @override
  String liveBoxEarlierSteps(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count frühere Schritte',
      one: '1 früherer Schritt',
    );
    return '$_temp0';
  }

  @override
  String liveBoxStall(int minutes) {
    String _temp0 = intl.Intl.pluralLogic(
      minutes,
      locale: localeName,
      other: 'Seit $minutes Minuten kein Fortschritt',
      one: 'Seit 1 Minute kein Fortschritt',
    );
    return '$_temp0';
  }

  @override
  String get accessNotFoundOrNoAccess => 'Nicht gefunden oder kein Zugriff';

  @override
  String get accessNoAccessAnymore => 'Kein Zugriff mehr';

  @override
  String get messageSending => 'Wird gesendet';

  @override
  String get messageSendingDelayed => 'Senden verzögert';

  @override
  String get connectionOffline => 'Offline';

  @override
  String get relativeDayToday => 'Heute';

  @override
  String get relativeDayYesterday => 'Gestern';

  @override
  String get problemValidationFailed => 'Einige Eingaben sind ungültig';

  @override
  String get problemMalformedRequest =>
      'Die Anfrage konnte nicht gelesen werden';

  @override
  String get problemCursorInvalid =>
      'Die Listenposition ist nicht mehr gültig. Lade die Liste neu';

  @override
  String get problemWebhookHostNotAllowed =>
      'Das Webhook-Ziel steht nicht auf der Erlaubnisliste';

  @override
  String get problemUnauthenticated => 'Bitte melde dich an';

  @override
  String get problemTokenExpired => 'Deine Anmeldung ist abgelaufen';

  @override
  String get problemSessionRevoked => 'Diese Sitzung wurde beendet';

  @override
  String get problemStepUpRequired => 'Bitte bestätige mit deinem Gerät';

  @override
  String get problemUseDpopNonce => 'Bitte versuche es noch einmal';

  @override
  String get problemInvalidDpopProof =>
      'Der Nachweis des Geräts wurde nicht akzeptiert';

  @override
  String get problemForbidden => 'Das darfst du nicht';

  @override
  String get problemScopeMissing => 'Der App fehlt ein nötiger Scope';

  @override
  String get problemDevicePending => 'Dieses Gerät wartet auf Bestätigung';

  @override
  String get problemChannelReadOnly => 'Dieser Kanal ist schreibgeschützt';

  @override
  String get problemFeatureDisabled => 'Diese Funktion ist ausgeschaltet';

  @override
  String get problemAgentInvocationDenied =>
      'Der Agent kann hier nicht gestartet werden';

  @override
  String get problemTerminalDenied =>
      'Das Terminal kann hier nicht geöffnet werden';

  @override
  String get problemInvocationTokenInvalid =>
      'Die Aufrufmarke des Agenten ist ungültig';

  @override
  String get problemNotFound => 'Nicht gefunden oder kein Zugriff';

  @override
  String get problemMethodNotAllowed => 'Diese Anfrage wird nicht unterstützt';

  @override
  String get problemConflict => 'Das widerspricht dem aktuellen Stand';

  @override
  String get problemRequestInProgress => 'Die Anfrage läuft noch';

  @override
  String get problemI1Violation =>
      'Mit dieser Änderung könnten andere einen persönlichen Agenten nutzen';

  @override
  String get problemI6Violation =>
      'Nur wer die persönliche LLM-Anmeldung hält, darf auf diesem Rechner ein Terminal öffnen';

  @override
  String get problemLastOwner => 'Mindestens ein aktiver Owner muss bleiben';

  @override
  String get problemAlreadyStarted => 'Der Lauf hat schon begonnen';

  @override
  String get problemBillingModeMismatch =>
      'Das Backend passt nicht zum Abrechnungsmodus';

  @override
  String get problemToolClassMismatch =>
      'Die gemeldete Werkzeugklasse liegt über der erlaubten';

  @override
  String get problemHostUnavailable => 'Der Rechner ist nicht erreichbar';

  @override
  String get problemFingerprintMismatch =>
      'Der Fingerabdruck passt nicht zum Rechner';

  @override
  String get problemInvalidStateTransition =>
      'Diese Änderung ist im aktuellen Zustand nicht möglich';

  @override
  String get problemBaseVersionConflict =>
      'Der Lauf hat sich inzwischen geändert';

  @override
  String get problemComponentUsed =>
      'Dieses Element wurde schon benutzt, ist abgelaufen oder ausgeschaltet';

  @override
  String get problemOperatorMismatch =>
      'Der Zielkanal hat einen anderen Bediener';

  @override
  String get problemFileNotReady => 'Die Datei ist noch nicht bereit';

  @override
  String get problemFileBlocked =>
      'Die Datei ist gesperrt und wird nie ausgeliefert';

  @override
  String get problemTitleManual => 'Der Titel wurde von Hand gesetzt';

  @override
  String get problemResyncRequired => 'Ein vollständiges Neuladen ist nötig';

  @override
  String get problemMessageDeleted => 'Die Nachricht wurde gelöscht';

  @override
  String get problemGone => 'Das ist abgelaufen';

  @override
  String get problemVersionConflict =>
      'Jemand anderes hat das inzwischen geändert';

  @override
  String get problemPayloadTooLarge => 'Der Inhalt ist zu groß';

  @override
  String get problemUnsupportedMediaType => 'Dieser Dateityp ist nicht erlaubt';

  @override
  String get problemRangeNotSatisfiable =>
      'Der angefragte Ausschnitt liegt außerhalb der Datei';

  @override
  String get problemIdempotencyKeyReuse =>
      'Die Anfrage wurde mit anderem Inhalt wiederholt';

  @override
  String get problemPreconditionRequired => 'Die Versionsprüfung fehlt';

  @override
  String get problemRateLimited =>
      'Zu viele Anfragen. Bitte warte einen Moment';

  @override
  String get problemInternalError =>
      'Auf dem Server ist ein Fehler aufgetreten';

  @override
  String get problemUnavailable => 'Der Server ist gerade nicht verfügbar';

  @override
  String get problemMaintenance =>
      'Wartung: Änderungen pausieren, Lesen geht weiter';

  @override
  String get problemSetupRequired => 'Die Instanz ist noch nicht eingerichtet';

  @override
  String get problemForbiddenCapabilityMissing =>
      'Dafür fehlt dir ein Zusatzrecht';

  @override
  String get problemForbiddenLevelInsufficient =>
      'Deine Stufe reicht dafür nicht aus';

  @override
  String get problemForbiddenOwnerOnly => 'Das kann nur ein Owner';

  @override
  String get problemForbiddenViewAsReadOnly =>
      'Nur Lesen, solange du als andere Person ansiehst';

  @override
  String get problemChannelReadOnlyChannelArchived =>
      'Dieser Kanal ist archiviert';

  @override
  String get problemAgentInvocationDeniedProjectNotReady =>
      'Das Projekt des Agenten ist nicht bereit';

  @override
  String problemFallback(String code) {
    return 'Etwas ist schiefgegangen ($code)';
  }

  @override
  String get vizMetricMessagesCreated => 'Nachrichten';

  @override
  String get vizMetricThreadsCreated => 'Neue Threads';

  @override
  String get vizMetricRunsStarted => 'Gestartete Läufe';

  @override
  String get vizMetricRunsFinished => 'Beendete Läufe';

  @override
  String get vizMetricToolCalls => 'Werkzeugaufrufe';

  @override
  String get vizMetricApprovalCards => 'Freigabekarten';

  @override
  String get vizMetricCost => 'Kosten';

  @override
  String get vizMetricInvocationsDenied => 'Abgelehnte Aufrufe';

  @override
  String get vizMetricBillingModeSwitches => 'Wechsel der Abrechnungsart';

  @override
  String get vizMetricCatchups => 'Nachholvorgänge';

  @override
  String get vizMetricResyncs => 'Neusynchronisierungen';

  @override
  String get vizMetricSlowConsumerCloses => 'Trennungen langsamer Clients';

  @override
  String vizMetricUnknown(String code) {
    return 'Kennzahl $code';
  }

  @override
  String get vizSliApiAvailability => 'API-Verfügbarkeit';

  @override
  String get vizSliDeliveryLatency => 'Zustell-Latenz';

  @override
  String get vizSliReconnect => 'Wiederverbindung';

  @override
  String get vizSliPushAcceptance => 'Push-Annahme';

  @override
  String get vizSliRunStart => 'Lauf-Start';

  @override
  String get vizSliTurnDuration => 'Turn-Dauer';

  @override
  String get vizSloOnTarget => 'im Ziel';

  @override
  String get vizSloBudgetTight => 'Budget knapp';

  @override
  String get vizSloTargetMissed => 'Ziel verfehlt';

  @override
  String get vizSloNotEnoughData => 'zu wenige Daten';

  @override
  String get vizOwnMeasurement => 'Eigene Messung';

  @override
  String vizTrafficLightLabel(String slo, String state) {
    return '$slo: $state, eigene Messung';
  }

  @override
  String vizTrafficLightChanged(String slo, String state) {
    return '$slo ist jetzt $state';
  }

  @override
  String get vizCostEstimate => 'Schätzung';

  @override
  String get vizCostInformationOnly => 'nur zur Information';

  @override
  String get vizBadgePersonal => 'Persönlich';

  @override
  String get vizBadgeOrganization => 'Organisation';

  @override
  String get vizApprovalWaiting => 'Freigabe wartet';

  @override
  String get vizMoreRunsThanShown => 'Mehr Läufe als angezeigt';

  @override
  String get vizRunStateWaitingInput => 'Wartet auf Eingabe';

  @override
  String get vizRunStateUnknown => 'Unbekannter Zustand';

  @override
  String vizStepToolCall(String name) {
    return 'Werkzeug $name';
  }

  @override
  String vizStepStatus(String name) {
    return 'Status $name';
  }

  @override
  String vizStepSubagent(String name) {
    return 'Subagent $name';
  }

  @override
  String vizStepWorkflow(String name) {
    return 'Workflow $name';
  }

  @override
  String vizStepBackgroundJob(String name) {
    return 'Hintergrundjob $name';
  }

  @override
  String vizLaneLabel(String space, String channel, int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count aktive Läufe',
      one: '1 aktiver Lauf',
      zero: 'keine aktiven Läufe',
    );
    return '$space, $channel, $_temp0';
  }

  @override
  String vizBarRowLabel(String row, int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count Läufe',
      one: '1 Lauf',
    );
    return '$row, $_temp0';
  }

  @override
  String get vizChartRates => 'Raten';

  @override
  String get vizChartRuns => 'Läufe';

  @override
  String vizTileUp(String delta) {
    return '$delta mehr als im Vorzeitraum';
  }

  @override
  String vizTileDown(String delta) {
    return '$delta weniger als im Vorzeitraum';
  }

  @override
  String get vizTileSame => 'wie im Vorzeitraum';

  @override
  String get vizNotShown => 'nicht angezeigt';

  @override
  String get vizStale => 'Keine neuen Daten';

  @override
  String get vizHarnessTitle => 'Testumgebung der Visualisierungen';

  @override
  String get vizHarnessTheme => 'Design';

  @override
  String get vizHarnessThemeLight => 'Hell';

  @override
  String get vizHarnessThemeDark => 'Dunkel';

  @override
  String get vizHarnessDensity => 'Dichte';

  @override
  String get vizHarnessDensityComfortable => 'Komfortabel';

  @override
  String get vizHarnessDensityCompact => 'Kompakt';

  @override
  String get vizHarnessDensityTouch => 'Touch';

  @override
  String get vizHarnessTextScale => 'Textgröße';

  @override
  String get vizHarnessLanguage => 'Sprache';

  @override
  String get vizHarnessFixtureState => 'Fixture-Zustand';

  @override
  String get vizHarnessTimeLapse => 'Zeitraffer';

  @override
  String get vizMarkLane => 'Bahn';

  @override
  String get vizMarkRunCard => 'Laufkarte';

  @override
  String get vizMarkArea => 'Fläche';

  @override
  String get vizMarkBar => 'Balken';

  @override
  String get vizMarkTrafficLight => 'Ampel';

  @override
  String get vizMarkMetricTile => 'Kennzahl-Kachel';

  @override
  String get messageShowMore => 'Mehr anzeigen';

  @override
  String get messageEdited => '(bearbeitet)';

  @override
  String get messageDeleted => 'Nachricht gelöscht';

  @override
  String get messageResend => 'Erneut senden';

  @override
  String get messageEdit => 'Bearbeiten';

  @override
  String get messageDiscard => 'Verwerfen';

  @override
  String get messageActionReply => 'Antworten';

  @override
  String get messageActionReact => 'Reagieren';

  @override
  String get messageActionPin => 'Anheften';

  @override
  String get messageActionMore => 'Mehr';

  @override
  String get messageMentionsYou => 'Erwähnt dich';

  @override
  String messageSemanticsLabel(String name, String time, String text) {
    return '$name, $time, $text';
  }

  @override
  String get messageOriginAgent => 'AGENT';

  @override
  String get messageOriginApp => 'APP';

  @override
  String get messageOriginImport => 'IMPORT';

  @override
  String get messageOriginSystem => 'SYSTEM';

  @override
  String messageReplyTo(String name) {
    return 'Antwort auf $name';
  }

  @override
  String messageAttachment(String name) {
    return 'Anhang $name';
  }

  @override
  String messageReaction(int count, String emoji) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count Reaktionen $emoji',
      one: '1 Reaktion $emoji',
    );
    return '$_temp0';
  }

  @override
  String get messageVoicePlay => 'Sprachnachricht abspielen';

  @override
  String get messageVoicePause => 'Sprachnachricht anhalten';

  @override
  String get messageVoiceTranscriptPending => 'Transkript folgt';

  @override
  String messageSystemThreadReopened(String name) {
    return 'Wieder geöffnet durch Nachricht von $name';
  }

  @override
  String get messageSystemResumeFailed =>
      'Die Agenten-Sitzung konnte nicht fortgesetzt werden';

  @override
  String messageSystemUnknown(String code) {
    return 'Systemnachricht ($code)';
  }

  @override
  String get triggerStateCancelled => 'Abgebrochen';

  @override
  String triggerStateCancelledWithReason(String reason) {
    return 'Abgebrochen: $reason';
  }

  @override
  String triggerStateUnknown(String code) {
    return 'Zustand $code';
  }

  @override
  String liveBoxStateUnknown(String code) {
    return 'Zustand $code';
  }

  @override
  String liveBoxStopRun(String agent) {
    return 'Lauf von $agent stoppen';
  }

  @override
  String liveBoxSemanticsLabel(
    String agent,
    String state,
    String duration,
    int percent,
  ) {
    return 'Lauf von $agent: $state, seit $duration, Kontext $percent Prozent';
  }

  @override
  String get liveBoxFollowupHint =>
      'Nachrichten in diesem Thread erreichen den Agenten als Nachtrag';

  @override
  String liveBoxTokens(String count) {
    return '$count Tokens';
  }

  @override
  String liveBoxContext(String percent) {
    return 'Kontext $percent';
  }

  @override
  String liveBoxRan(String duration) {
    return 'Lief $duration';
  }

  @override
  String liveBoxTools(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count Werkzeuge',
      one: '1 Werkzeug',
    );
    return '$_temp0';
  }

  @override
  String liveBoxSubagents(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count Unteragenten',
      one: '1 Unteragent',
    );
    return '$_temp0';
  }

  @override
  String get liveBoxExpand => 'Laufdetails anzeigen';

  @override
  String get liveBoxCollapse => 'Laufdetails ausblenden';

  @override
  String get liveBoxOpen => 'Live-Box öffnen';

  @override
  String liveBoxAnnounceQuestion(String agent) {
    return 'Frage von $agent';
  }

  @override
  String liveBoxAnnounceState(String agent, String state) {
    return 'Lauf von $agent: $state';
  }

  @override
  String get toolChipWaiting => 'wartet';

  @override
  String toolChipRunning(String duration) {
    return 'läuft $duration';
  }

  @override
  String get toolChipFailed => 'fehlgeschlagen';

  @override
  String get toolChipCancelled => 'abgebrochen';

  @override
  String get toolChipCompleted => 'abgeschlossen';

  @override
  String toolChipUnknownStatus(String code) {
    return 'Status $code';
  }

  @override
  String get toolChipKindToolCall => 'Werkzeug';

  @override
  String get toolChipKindStatus => 'Status';

  @override
  String get toolChipKindSubagent => 'Unteragent';

  @override
  String get toolChipKindWorkflow => 'Workflow';

  @override
  String get toolChipKindBackgroundJob => 'Hintergrundjob';

  @override
  String get toolChipKindUnknown => 'Schritt';

  @override
  String toolChipLabel(String kind, String name, String status) {
    return '$kind $name, $status';
  }

  @override
  String get toolChipDetails => 'Details anzeigen';

  @override
  String get threadRowWaitingForYou => 'Wartet auf dich';

  @override
  String get threadRowStateOpen => 'Offen';

  @override
  String get threadRowStateDone => 'Erledigt';

  @override
  String threadRowUnread(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count ungelesen',
      one: '1 ungelesen',
      zero: 'nichts ungelesen',
    );
    return '$_temp0';
  }

  @override
  String threadRowMentions(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count Erwähnungen',
      one: '1 Erwähnung',
    );
    return '$_temp0';
  }

  @override
  String threadRowLabel(
    String title,
    String state,
    String time,
    String unread,
  ) {
    return '$title, $state, $time, $unread';
  }

  @override
  String threadRowOperator(String name) {
    return 'Bediener $name';
  }

  @override
  String get channelRowRunActive => 'Lauf aktiv';

  @override
  String get channelRowMuted => 'stummgeschaltet';

  @override
  String get spaceRowUnread => 'Ungelesen';

  @override
  String spaceRowExpand(String name) {
    return '$name ausklappen';
  }

  @override
  String spaceRowCollapse(String name) {
    return '$name einklappen';
  }

  @override
  String componentsTextFrom(String agent) {
    return 'Text von $agent';
  }

  @override
  String componentsHeaderRequest(String agent) {
    return 'Anfrage von $agent';
  }

  @override
  String componentsHeaderRun(String run) {
    return 'Lauf $run';
  }

  @override
  String componentsHeaderCapability(String capability) {
    return 'Erfordert das Recht \'$capability\'';
  }

  @override
  String componentsApproverRunStarter(String name) {
    return 'Nur $name oder ein Owner kann das freigeben';
  }

  @override
  String get componentsApproverOwner => 'Nur ein Owner kann das freigeben';

  @override
  String get componentsApproverAny =>
      'Alle mit Zugang zu diesem Kanal können antworten';

  @override
  String componentsDeniedCapability(String capability) {
    return 'Dafür fehlt dir das Recht \'$capability\'';
  }

  @override
  String get capabilityApproveActions => 'Aktionen freigeben';

  @override
  String componentsUsed(String name, String choice, String time) {
    return '$name hat \'$choice\' gewählt, $time';
  }

  @override
  String get componentsExpired => 'Abgelaufen';

  @override
  String get componentsDisabled => 'Nicht mehr verfügbar';

  @override
  String get componentsSending => 'Deine Wahl wird gesendet';

  @override
  String get componentsSubmit => 'Absenden';

  @override
  String get componentsSelectPlaceholder => 'Auswählen';

  @override
  String get componentsFieldRequired => 'Pflichtfeld';

  @override
  String componentsFieldTooLong(int max) {
    return 'Höchstens $max Zeichen';
  }

  @override
  String get componentsFieldNumber => 'Zahl eingeben';

  @override
  String get componentsUnsupported => 'Dieses Element braucht eine neuere App';

  @override
  String composerMessageTo(String target) {
    return 'Nachricht an $target';
  }

  @override
  String composerFollowupTo(String agent) {
    return 'Nachtrag an $agent';
  }

  @override
  String composerTyping(String name) {
    return '$name schreibt gerade';
  }

  @override
  String get composerTypingSeveral => 'Mehrere schreiben gerade';

  @override
  String get composerSend => 'Senden';

  @override
  String get composerAttach => 'Datei anhängen';

  @override
  String get composerVoice => 'Sprachnachricht aufnehmen';

  @override
  String composerOptionMissing(String option) {
    return 'Fehlende Option: $option';
  }

  @override
  String get composerOffline =>
      'Offline: deine Nachricht wartet in der Warteschlange';

  @override
  String get composerReadOnly => 'Nur lesen in diesem Kanal';

  @override
  String composerSuggestions(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count Vorschläge',
      one: '1 Vorschlag',
      zero: 'Keine Vorschläge',
    );
    return '$_temp0';
  }

  @override
  String get composerControlCommand => 'Steuerbefehl';

  @override
  String get billingPersonal => 'Persönlich';

  @override
  String get billingOrganization => 'Organisation';

  @override
  String billingUnknown(String code) {
    return 'Backend $code';
  }

  @override
  String billingReported(String mode) {
    return 'Ist: $mode';
  }

  @override
  String billingLabelPersonal(String name) {
    return 'Backend: Persönlich, läuft auf dem Abo von $name';
  }

  @override
  String get billingLabelOrganization => 'Backend: Organisation';

  @override
  String get costEstimate => 'Schätzung';

  @override
  String get costInformationOnly => 'nur Information';

  @override
  String markdownImage(String alt, String domain) {
    return 'Bild: $alt ($domain)';
  }

  @override
  String markdownImageNoDomain(String alt) {
    return 'Bild: $alt';
  }

  @override
  String get markdownCopyCode => 'Code kopieren';

  @override
  String get triggerReasonMultipleAgentsMentioned =>
      'Bitte nur einen Agenten erwähnen.';

  @override
  String get triggerReasonAgentNotAllowedHere =>
      'Dieser Agent antwortet hier nicht.';

  @override
  String get triggerReasonNoTriggerRight =>
      'Du darfst hier keine Agenten auslösen.';

  @override
  String get triggerReasonRunActiveOtherAgent =>
      'Ein anderer Agent arbeitet gerade in diesem Thread.';

  @override
  String get triggerReasonAgentDisabled => 'Agent ist abgeschaltet.';

  @override
  String get triggerReasonBillingModePersonalNotHolder =>
      'Dieser Agent läuft auf einem persönlichen Abo.';

  @override
  String get triggerReasonVoiceUnsupported =>
      'Sprachnachrichten werden hier nicht verstanden.';

  @override
  String get triggerReasonRunActive =>
      'Der Agent arbeitet gerade; Befehl nach dem Lauf erneut senden.';

  @override
  String get liveBoxReasonUserCancelledNoName => 'Von einer Person abgebrochen';

  @override
  String get threadRowStateWaiting => 'Wartet auf Antwort';

  @override
  String get vizViewLiveTopology => 'Live-Topologie';

  @override
  String get vizViewBackend => 'Backend-Ansicht';

  @override
  String get vizViewDelivery => 'Zustellung und Sync live';

  @override
  String get vizViewExecutive => 'Executive-Seite';

  @override
  String get vizViewTimeline => 'Tages-Timeline';

  @override
  String get vizViewTv => 'TV-Modus';

  @override
  String get vizAsTable => 'Als Tabelle';

  @override
  String get vizOpenGrafana => 'In Grafana öffnen';

  @override
  String get vizLoading => 'Wird geladen';

  @override
  String get vizOnlyForOwner => 'Nur für den Eigentümer';

  @override
  String vizSortBy(String column) {
    return 'Sortieren nach $column';
  }

  @override
  String vizFrameTime(String time) {
    return 'Stand $time';
  }

  @override
  String get vizNoActiveRuns => 'Keine aktiven Läufe';

  @override
  String get vizRatesLastMinute => 'Letzte Minute';

  @override
  String get vizNone => 'keine';

  @override
  String get vizColSpace => 'Bereich';

  @override
  String get vizColChannel => 'Kanal';

  @override
  String get vizColAgent => 'Agent';

  @override
  String get vizColState => 'Zustand';

  @override
  String get vizColStep => 'Aktueller Schritt';

  @override
  String get vizColMode => 'Abrechnungsart';

  @override
  String get vizColApproval => 'Bestätigung';

  @override
  String get vizColCost => 'Bisherige Kosten';

  @override
  String get vizColMetric => 'Kennzahl';

  @override
  String get vizColLastMinute => 'Letzte Minute';

  @override
  String get vizColValue => 'Wert';

  @override
  String get vizColNote => 'Hinweis';

  @override
  String get vizColPresence => 'Anwesenheit';

  @override
  String get vizColConfiguredMode => 'Eingestellte Abrechnungsart';

  @override
  String get vizColReportedMode => 'Gemeldete Abrechnungsart';

  @override
  String get vizColSupportedModes => 'Unterstützte Abrechnungsarten';

  @override
  String get vizColSwitches => 'Wechsel der Abrechnungsart';

  @override
  String get vizColI2Denied => 'Vor der Zustellung abgelehnt (I2)';

  @override
  String get vizColI3Denied =>
      'An abweichender Abrechnungsart gescheitert (I3)';

  @override
  String get vizColCosts => 'Kosten je Abrechnungsart';

  @override
  String get vizColRateLimit => 'Ratenlimit';

  @override
  String get vizPresenceOnline => 'Online';

  @override
  String get vizPresenceDegraded => 'Eingeschränkt';

  @override
  String get vizPresenceOffline => 'Offline';

  @override
  String get vizPresenceUnknown => 'Unbekannt';

  @override
  String get vizFollowsRun => 'je Lauf';

  @override
  String vizReportedMismatch(String mode) {
    return 'Gemeldet: $mode';
  }

  @override
  String get vizRateLimitOk => 'im Rahmen';

  @override
  String get vizRateLimitNearLimit => 'nahe am Limit';

  @override
  String get vizRateLimitLimited => 'begrenzt';

  @override
  String get vizRateLimitUnknown => 'unbekannt';

  @override
  String vizRateLimitResets(String time) {
    return 'wieder frei um $time';
  }

  @override
  String get vizSliNoEvents => 'keine Ereignisse';

  @override
  String vizSliValue(int count, String share) {
    final intl.NumberFormat countNumberFormat =
        intl.NumberFormat.decimalPattern(localeName);
    final String countString = countNumberFormat.format(count);

    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$countString Ereignisse',
      one: '1 Ereignis',
    );
    return '$_temp0, $share innerhalb der Schwelle';
  }

  @override
  String vizCatchupValue(String count, String millis) {
    return '$count, zusammen $millis ms';
  }

  @override
  String get vizConnectionsUser => 'Verbindungen von Personen';

  @override
  String get vizConnectionsAgent => 'Verbindungen von Agenten';

  @override
  String get vizConnectionsApp => 'Verbindungen von Apps';

  @override
  String get vizChartHistory => 'Verlauf';

  @override
  String get vizTileUsage => 'Nutzung';

  @override
  String get vizTileRuns => 'Läufe und Fehlerquote';

  @override
  String get vizTileCosts => 'Kosten je Abrechnungsart';

  @override
  String get vizTileSlo => 'SLO-Ampel';

  @override
  String get vizColSlo => 'SLO';

  @override
  String get vizOwnMeasurementNote =>
      'Werte können von Prometheus abweichen: die eigene Messung zählt im Prozess und kann bei einem Absturz ein Schreibintervall verlieren.';

  @override
  String get vizColRun => 'Agent und Kanal';

  @override
  String get vizColStart => 'Beginn';

  @override
  String get vizColEnd => 'Ende';

  @override
  String vizPlayhead(String time) {
    return 'Abspielposition $time';
  }

  @override
  String get vizPlay => 'Abspielen';

  @override
  String get vizPause => 'Anhalten';

  @override
  String vizSpeed(int speed) {
    return '${speed}x';
  }

  @override
  String get vizJump => 'Zu einer Minute springen';

  @override
  String vizTvPosition(String title, int index, int count) {
    return '$title ($index von $count)';
  }

  @override
  String get onboardingConnectTitle => 'Mit deiner Instanz verbinden';

  @override
  String get onboardingServerAddress => 'Serveradresse';

  @override
  String get onboardingConnect => 'Verbinden';

  @override
  String get onboardingConnecting => 'Verbindung wird hergestellt';

  @override
  String get onboardingAddressInvalid =>
      'Gib die vollständige Adresse der Instanz ein, zum Beispiel https://chat.example.com.';

  @override
  String get onboardingNoInstance =>
      'Unter dieser Adresse gibt es keine Instanz. Prüfe die Adresse und versuche es erneut.';

  @override
  String get onboardingCertificateInvalid =>
      'Zertifikat ungültig. Die Verbindung ist nicht sicher, deshalb verbindet sich die App nicht. Bitte den Owner der Instanz, das Zertifikat zu erneuern.';

  @override
  String get onboardingNotSetUp =>
      'Instanz noch nicht eingerichtet. Der Owner richtet sie auf dem Rechner mit dem Einrichtungsassistenten ein (install.sh).';

  @override
  String onboardingAppTooOld(String version) {
    return 'App zu alt. Diese Instanz braucht Version $version oder neuer. Installiere das Update und verbinde dich dann erneut.';
  }

  @override
  String onboardingLoginTitle(String productName) {
    return 'Bei $productName anmelden';
  }

  @override
  String onboardingLoginWithProvider(String name) {
    return 'Mit $name anmelden';
  }

  @override
  String get onboardingOr => 'oder';

  @override
  String get onboardingLoginName => 'Anmeldename';

  @override
  String get onboardingLoginWithPasskey => 'Mit Passkey anmelden';

  @override
  String get onboardingPasswordAndCode => 'Passwort und Einmalcode';

  @override
  String get onboardingPassword => 'Passwort';

  @override
  String get onboardingOneTimeCode => 'Einmalcode';

  @override
  String get onboardingLogIn => 'Anmelden';

  @override
  String get onboardingEnrollmentCodeLink => 'Ich habe einen Einladungs-Code';

  @override
  String get onboardingPairLink => 'Mit anderem Gerät koppeln';

  @override
  String get onboardingNoMethod =>
      'Für diese Instanz ist keine Anmeldung eingerichtet. Wende dich an den Owner der Instanz.';

  @override
  String onboardingServerHost(String address) {
    return 'Server: $address';
  }

  @override
  String get onboardingChangeServer => 'Ändern';

  @override
  String get onboardingBack => 'Zurück';

  @override
  String get onboardingLoginFailed =>
      'Anmeldung fehlgeschlagen. Prüfe deine Angaben und versuche es erneut.';

  @override
  String get onboardingUnreachable =>
      'Die Instanz ist nicht erreichbar. Prüfe das Netzwerk und versuche es erneut.';

  @override
  String get onboardingBrowserCancelled =>
      'Die Anmeldung im Browser wurde abgebrochen. Starte sie erneut.';

  @override
  String get onboardingBrowserExpired =>
      'Die Anmeldung im Browser hat zu lange gedauert. Starte sie erneut.';

  @override
  String get onboardingEnrollTitle => 'Einladungs-Code';

  @override
  String get onboardingEnrollCode => 'Code vom Owner';

  @override
  String get onboardingEnrollHow => 'Wie möchtest du dich anmelden?';

  @override
  String get onboardingEnrollThisDevice => 'Dieses Gerät verwenden';

  @override
  String get onboardingEnrollCreatePasskey => 'Passkey erstellen';

  @override
  String onboardingEnrollLink(String name) {
    return 'Mit $name verknüpfen';
  }

  @override
  String get onboardingContinue => 'Weiter';

  @override
  String get onboardingEnrollOnce => 'Der Code gilt einmal.';

  @override
  String get onboardingPendingTitle => 'Gerät wartet auf Bestätigung';

  @override
  String get onboardingPendingExplain =>
      'Gib diesen Code auf einem deiner aktiven Geräte ein: Einstellungen, Geräte und Sitzungen, Bestätigen. Kein aktives Gerät: frag den Owner.';

  @override
  String get onboardingCancel => 'Abbrechen';

  @override
  String onboardingValidFor(int minutes) {
    String _temp0 = intl.Intl.pluralLogic(
      minutes,
      locale: localeName,
      other: 'Noch $minutes Minuten gültig',
      one: 'Noch 1 Minute gültig',
      zero: 'Noch weniger als eine Minute gültig',
    );
    return '$_temp0';
  }

  @override
  String get onboardingPairTitle => 'Gerät koppeln';

  @override
  String get onboardingPairExplain =>
      'Auf einem aktiven Gerät: Einstellungen, Geräte, Gerät hinzufügen, dann eingeben:';

  @override
  String get onboardingPairNewCode => 'Neuer Code';

  @override
  String get onboardingPairExpired =>
      'Der Code ist abgelaufen. Hol dir einen neuen Code.';

  @override
  String onboardingLockedTitle(String productName) {
    return '$productName ist gesperrt';
  }

  @override
  String get onboardingUnlock => 'Entsperren';

  @override
  String get onboardingUnlockFailed => 'Entsperren fehlgeschlagen.';

  @override
  String get onboardingTryAgain => 'Erneut versuchen';

  @override
  String get onboardingLogOut => 'Abmelden';

  @override
  String get onboardingSignedOutTitle => 'Du bist abgemeldet.';

  @override
  String onboardingSignedOutReason(String reason) {
    return 'Grund: $reason';
  }

  @override
  String get onboardingReasonLoggedOut => 'Du hast dich abgemeldet.';

  @override
  String get onboardingReasonRevokedBySelf =>
      'Dieses Gerät wurde von einem anderen Gerät widerrufen.';

  @override
  String get onboardingReasonRevokedByOwner =>
      'Der Owner hat dieses Gerät widerrufen.';

  @override
  String get onboardingReasonUserDisabled => 'Dein Konto wurde deaktiviert.';

  @override
  String get onboardingReasonInactive =>
      'Dieses Gerät wurde lange nicht verwendet.';

  @override
  String get onboardingReasonRejected =>
      'Die Bestätigung dieses Geräts wurde abgelehnt.';

  @override
  String get onboardingReasonPendingExpired =>
      'Dieses Gerät wurde nicht rechtzeitig bestätigt.';

  @override
  String get onboardingReasonSessionRevoked =>
      'Deine Sitzung wurde auf dem Server beendet.';

  @override
  String get onboardingReasonSessionExpired => 'Deine Sitzung ist abgelaufen.';

  @override
  String get onboardingDataDeleted =>
      'Die Daten dieses Geräts wurden gelöscht.';

  @override
  String get onboardingLogInAgain => 'Erneut anmelden';

  @override
  String get onboardingContactOwner => 'Wende dich an den Owner der Instanz.';

  @override
  String get onboardingCopy => 'Kopieren';

  @override
  String get onboardingCopied => 'Kopiert';

  @override
  String onboardingCodeLabel(String characters) {
    return 'Code: $characters';
  }

  @override
  String get onboardingLoopbackDone =>
      'Die Anmeldung ist abgeschlossen. Du kannst dieses Fenster schließen.';

  @override
  String get shellColumnSidebar => 'Bereiche und Kanäle';

  @override
  String get shellColumnPostList => 'Beitragsliste';

  @override
  String get shellColumnHistory => 'Verlauf';

  @override
  String get shellColumnSession => 'Sitzung';

  @override
  String get shellResizeColumn => 'Spaltenbreite ändern';

  @override
  String get shellUpdateRequired =>
      'Update erforderlich. Installiere die neue Version, um dich wieder zu verbinden.';

  @override
  String get shellStepUpTitle => 'Bestätige, dass du es bist';

  @override
  String get stepUpGenericPurpose => 'Diese Aktion braucht deine Bestätigung.';

  @override
  String get stepUpNext =>
      'Als Nächstes fragt dein Gerät nach Fingerabdruck, Gesicht oder Gerätepasswort.';

  @override
  String get stepUpContinue => 'Weiter';

  @override
  String get selfCancel => 'Abbrechen';

  @override
  String get selfOffline =>
      'Keine Verbindung. Versuche es erneut, wenn du online bist.';

  @override
  String get selfLoadFailed => 'Die Liste konnte nicht geladen werden.';

  @override
  String get selfRetry => 'Erneut versuchen';

  @override
  String stepUpPurposeApproveDevice(String name) {
    return 'Das wartende Gerät $name bestätigen';
  }

  @override
  String get stepUpPurposeAddDevice =>
      'Ein neues Gerät zu deinem Konto hinzufügen';

  @override
  String stepUpPurposeRevokeDevice(String name) {
    return 'Das Gerät $name sperren';
  }

  @override
  String get devicesTitle => 'Geräte';

  @override
  String get devicesAdd => 'Gerät hinzufügen';

  @override
  String get devicesThisDevice => 'Dieses Gerät';

  @override
  String get devicesStatePending => 'Wartet auf Bestätigung';

  @override
  String get devicesStateRevoked => 'Gesperrt';

  @override
  String devicesAppVersion(String version) {
    return 'Version $version';
  }

  @override
  String devicesLastActive(String when) {
    return 'Zuletzt aktiv $when';
  }

  @override
  String get devicesPlatformIos => 'iPhone oder iPad';

  @override
  String get devicesPlatformAndroid => 'Android';

  @override
  String get devicesPlatformMacos => 'Mac';

  @override
  String get devicesPlatformWindows => 'Windows';

  @override
  String get devicesPlatformWeb => 'Web-Konsole';

  @override
  String get devicesPlatformCli => 'Kommandozeile';

  @override
  String get devicesPlatformOther => 'Andere Plattform';

  @override
  String get devicesEnterCode => 'Code eingeben';

  @override
  String get devicesRevoke => 'Sperren';

  @override
  String devicesRevokeTitle(String name) {
    return '$name sperren?';
  }

  @override
  String get devicesRevokeBody =>
      'Das Gerät wird abgemeldet und löscht seine lokalen Daten beim nächsten Kontakt.';

  @override
  String get devicesRevokeThisBody =>
      'Du wirst auf diesem Gerät abgemeldet, und seine lokalen Daten werden gelöscht.';

  @override
  String get devicesRevokeReason => 'Grund (optional)';

  @override
  String get devicesRevokeDefaultReason => 'In den Einstellungen gesperrt';

  @override
  String devicesRevoked(String name) {
    return '$name wurde gesperrt.';
  }

  @override
  String devicesConfirmTitle(String name) {
    return '$name bestätigen';
  }

  @override
  String get devicesConfirmHint =>
      'Gib den Code ein, den das wartende Gerät anzeigt.';

  @override
  String get devicesConfirmCode => 'Bestätigungscode';

  @override
  String get devicesConfirmSubmit => 'Bestätigen';

  @override
  String devicesConfirmed(String name) {
    return '$name ist jetzt aktiv.';
  }

  @override
  String get devicesConfirmRejected =>
      'Das wartende Gerät wurde abgelehnt, weil der Code nicht passte.';

  @override
  String get devicesAddTitle => 'Gerät hinzufügen';

  @override
  String get devicesAddHint =>
      'Gib den Kopplungs-Code ein, den das neue Gerät anzeigt.';

  @override
  String get devicesAddCode => 'Kopplungs-Code';

  @override
  String get devicesAddSubmit => 'Hinzufügen';

  @override
  String devicesAdded(String name) {
    return '$name wurde hinzugefügt.';
  }

  @override
  String get devicesPairingNotFound => 'Keine offene Kopplung mit diesem Code.';

  @override
  String get sessionsTitle => 'Sitzungen';

  @override
  String get sessionsCurrent => 'Diese Sitzung';

  @override
  String get sessionsUnknownDevice => 'Unbekanntes Gerät';

  @override
  String sessionsLastUsed(String when) {
    return 'Zuletzt genutzt $when';
  }

  @override
  String get sessionsMethodOidc => 'Anmeldedienst';

  @override
  String get sessionsMethodTrustedHeader => 'Anmeldung über das Netzwerk';

  @override
  String get sessionsMethodPasskey => 'Passkey';

  @override
  String get sessionsMethodPasswordTotp => 'Passwort und Einmalcode';

  @override
  String get sessionsMethodEnrollmentCode => 'Einladungscode';

  @override
  String get sessionsMethodDevicePairing => 'Kopplung';

  @override
  String get sessionsMethodOther => 'Andere Methode';

  @override
  String get sessionsEnd => 'Sitzung beenden';

  @override
  String get sessionsEndTitle => 'Diese Sitzung beenden?';

  @override
  String get sessionsEndBody => 'Das Gerät dieser Sitzung wird abgemeldet.';

  @override
  String get sessionsEndCurrentBody =>
      'Du wirst auf diesem Gerät abgemeldet, und seine lokalen Daten werden gelöscht.';

  @override
  String get sessionsEnded => 'Die Sitzung wurde beendet.';

  @override
  String get settingsTitle => 'Einstellungen';

  @override
  String get settingsBack => 'Zurück';

  @override
  String get settingsSectionAppearance => 'Darstellung';

  @override
  String get settingsSectionNotifications => 'Benachrichtigungen';

  @override
  String get settingsSectionLanguage => 'Sprache';

  @override
  String get settingsSectionKeyboard => 'Tastatur';

  @override
  String get settingsSectionDevices => 'Geräte und Sitzungen';

  @override
  String get settingsSectionPrivacy => 'Datenschutz';

  @override
  String get settingsSectionAbout => 'Über';

  @override
  String get settingsTheme => 'Design';

  @override
  String get settingsThemeSystem => 'System';

  @override
  String get settingsThemeLight => 'Hell';

  @override
  String get settingsThemeDark => 'Dunkel';

  @override
  String get settingsDensity => 'Dichte';

  @override
  String get settingsDensityComfortable => 'Normal';

  @override
  String get settingsDensityCompact => 'Kompakt';

  @override
  String get settingsMotion => 'Bewegung';

  @override
  String get settingsMotionSystem => 'System';

  @override
  String get settingsMotionFull => 'Voll';

  @override
  String get settingsMotionReduced => 'Reduziert';

  @override
  String get settingsEnterSends => 'Enter sendet';

  @override
  String get settingsEnterSendsHint => 'Umschalt+Enter: neue Zeile';

  @override
  String get settingsColumnWidths => 'Spaltenbreiten';

  @override
  String get settingsColumnWidthsReset => 'Zurücksetzen';

  @override
  String get settingsLanguage => 'Sprache';

  @override
  String get settingsLanguageGerman => 'Deutsch';

  @override
  String get settingsLanguageEnglish => 'Englisch';

  @override
  String get settingsTimeZone => 'Zeitzone';

  @override
  String settingsProfileFailed(String reason) {
    return 'Das Profil konnte nicht gespeichert werden: $reason';
  }

  @override
  String settingsShortcuts(String shortcut) {
    return 'Alle Tastenkürzel: $shortcut';
  }

  @override
  String get settingsCrashReports => 'Absturzberichte senden';

  @override
  String get settingsCrashReportsHint =>
      'Ohne Inhalte: nur App-Version, Plattform und Gerätetyp.';

  @override
  String get settingsClientMetrics => 'Leistungsdaten senden';

  @override
  String get settingsClientMetricsHint => 'Ohne Inhalte, IDs oder Namen.';

  @override
  String get settingsAboutAppVersion => 'App-Version';

  @override
  String get settingsAboutServerVersion => 'Server-Spezifikation';

  @override
  String get settingsAboutLicenses => 'Lizenzen';
}
