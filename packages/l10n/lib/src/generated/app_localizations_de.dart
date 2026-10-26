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

  @override
  String get sidebarJumpTo => 'Springen zu';

  @override
  String get sidebarRunningNow => 'Läuft gerade';

  @override
  String get sidebarLiveGroup => 'Live und Statistik';

  @override
  String get sidebarLive => 'Live';

  @override
  String get sidebarSettings => 'Einstellungen';

  @override
  String sidebarOwnAvatar(String name) {
    return '$name, angemeldet';
  }

  @override
  String get sidebarMoveUp => 'Nach oben verschieben';

  @override
  String get sidebarMoveDown => 'Nach unten verschieben';

  @override
  String get sidebarNoSpaces => 'Noch keine Bereiche';

  @override
  String get postListNewPost => 'Neuer Beitrag';

  @override
  String get postListFilterOpen => 'Offen';

  @override
  String get postListFilterWaitingForMe => 'Wartet auf mich';

  @override
  String get postListFilterTags => 'Tags';

  @override
  String postListFilterTagsCount(int count) {
    return 'Tags ($count)';
  }

  @override
  String get postListSortLastActivity => 'Letzte Aktivität';

  @override
  String get postListSortCreated => 'Neueste zuerst';

  @override
  String postListDone(int count) {
    return 'Erledigt ($count)';
  }

  @override
  String postListDoneMore(int count) {
    return 'Erledigt ($count+)';
  }

  @override
  String get postListEmpty => 'Kein offener Beitrag hier';

  @override
  String postListCreateFailed(String title) {
    return '„$title“ konnte nicht gepostet werden';
  }

  @override
  String get postListTryAgain => 'Erneut versuchen';

  @override
  String get postListDiscard => 'Verwerfen';

  @override
  String get postListLoadFailed =>
      'Weitere Beiträge konnten nicht geladen werden';

  @override
  String get postListEdit => 'Bearbeiten';

  @override
  String postListFailureReason(String reason) {
    return 'Grund: $reason';
  }

  @override
  String postListLabel(String forum) {
    return 'Beitragsliste von $forum';
  }

  @override
  String get newPostTitle => 'Titel';

  @override
  String get newPostMessage => 'Erste Nachricht';

  @override
  String get newPostTags => 'Tags';

  @override
  String get newPostCreate => 'Beitrag erstellen';

  @override
  String get newPostCancel => 'Abbrechen';

  @override
  String get adminTitle => 'Verwaltung';

  @override
  String get adminBack => 'Zurück zum Chat';

  @override
  String get adminNavPeople => 'Personen';

  @override
  String get adminNavGroups => 'Gruppen';

  @override
  String get adminNavDevices => 'Geräte';

  @override
  String adminNavDevicesWaiting(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: 'Geräte ($count warten)',
      one: 'Geräte (1 wartet)',
      zero: 'Geräte',
    );
    return '$_temp0';
  }

  @override
  String get adminReason => 'Grund (steht im Audit-Log)';

  @override
  String get adminDefaultReason => 'In der Verwaltung geändert';

  @override
  String get adminLoadMore => 'Mehr laden';

  @override
  String get adminOneTimeSecret =>
      'Der Code wurde einmal angezeigt und lässt sich nicht erneut anzeigen. Stelle einen neuen Code aus.';

  @override
  String get adminI1Explained =>
      'Ändere zuerst unter Agenten den Modus des betroffenen Agenten und aktiviere die Person dann erneut.';

  @override
  String get adminPickPerson => 'Person wählen';

  @override
  String get adminRevoke => 'Widerrufen';

  @override
  String get adminOwnerBadge => 'Owner';

  @override
  String get adminStatusActive => 'Aktiv';

  @override
  String get adminStatusPending => 'Wartet auf Anmeldung';

  @override
  String get adminStatusDisabled => 'Gesperrt';

  @override
  String get adminStatusDeleted => 'Gelöscht';

  @override
  String get adminPeopleTitle => 'Personen';

  @override
  String get adminPeopleAdd => 'Person anlegen';

  @override
  String get adminPeopleSearch => 'Suchen';

  @override
  String get adminPeopleStatusAll => 'Alle Zustände';

  @override
  String get adminPeopleGroupAll => 'Alle Gruppen';

  @override
  String get adminPeopleGroupNote =>
      'Der Gruppenfilter wirkt nur auf die auf dieser Seite geladenen Personen.';

  @override
  String get adminPeopleEmpty => 'Keine Person passt.';

  @override
  String adminPersonCreated(String name) {
    return '$name wurde angelegt.';
  }

  @override
  String get adminCreatePersonTitle => 'Person anlegen';

  @override
  String get adminCreatePersonBody =>
      'Die Person bekommt ein Konto. Mit lokalem Konto erscheint der Einladungs-Code im nächsten Schritt genau einmal.';

  @override
  String get adminCreatePersonName => 'Anzeigename';

  @override
  String get adminCreatePersonHandle => 'Handle';

  @override
  String get adminCreatePersonHandleInvalid =>
      'Verwende Kleinbuchstaben, Ziffern, Punkte, Bindestriche und Unterstriche.';

  @override
  String get adminCreatePersonLocal => 'Lokales Konto mit Einladungs-Code';

  @override
  String get adminCreatePersonSubmit => 'Anlegen';

  @override
  String adminCodeTitle(String name) {
    return 'Einladungs-Code für $name';
  }

  @override
  String get adminCodeOnce =>
      'Dieser Code wird nur einmal angezeigt. Gib ihn persönlich oder telefonisch weiter.';

  @override
  String get adminCodePurposeFirstLogin => 'Für die erste Anmeldung';

  @override
  String get adminCodePurposeRecovery => 'Für die Wiederherstellung';

  @override
  String get adminCodePurposeIdentityLink =>
      'Zum Verknüpfen eines Anmeldedienstes';

  @override
  String adminCodeExpires(String when) {
    return 'Gültig bis $when';
  }

  @override
  String get adminCodeClose => 'Schließen';

  @override
  String get adminPersonBack => 'Alle Personen';

  @override
  String get adminPersonDisable => 'Sperren';

  @override
  String get adminPersonEnable => 'Entsperren';

  @override
  String get adminPersonNewCode => 'Neuer Einladungs-Code';

  @override
  String adminDisableTitle(String name) {
    return '$name sperren?';
  }

  @override
  String adminDisableBody(String name) {
    return '$name wird gesperrt. Alle Sitzungen von $name enden sofort und alle Geräte werden widerrufen.';
  }

  @override
  String adminPersonDisabled(String name) {
    return '$name ist gesperrt. Alle Sitzungen wurden beendet.';
  }

  @override
  String adminEnableTitle(String name) {
    return '$name entsperren?';
  }

  @override
  String adminEnableBody(String name) {
    return '$name kann sich wieder anmelden. Frühere Geräte bleiben widerrufen.';
  }

  @override
  String adminPersonEnabled(String name) {
    return '$name ist entsperrt.';
  }

  @override
  String adminNewCodeTitle(String name) {
    return 'Neuen Einladungs-Code für $name?';
  }

  @override
  String adminNewCodeBody(String name) {
    return 'Ein neuer Code wird ausgestellt und einmal angezeigt. Jeder ältere Code von $name wird ungültig.';
  }

  @override
  String get adminCredentialsTitle => 'Anmeldedaten';

  @override
  String get adminCredentialsPasswordSet => 'Lokales Passwort gesetzt';

  @override
  String get adminCredentialsPasswordNone => 'Kein lokales Passwort';

  @override
  String get adminCredentialsTotpSet => 'Einmalcode-App eingerichtet';

  @override
  String get adminCredentialsTotpNone => 'Keine Einmalcode-App';

  @override
  String get adminCredentialsIdentities => 'Verknüpfte Anmeldedienste';

  @override
  String get adminCredentialsPasskeys => 'Passkeys';

  @override
  String get adminCredentialsNone => 'Keine';

  @override
  String adminPasskeyAdded(String when) {
    return 'Hinzugefügt $when';
  }

  @override
  String get adminUnlink => 'Trennen';

  @override
  String adminUnlinkTitle(String subject) {
    return '$subject trennen?';
  }

  @override
  String adminUnlinkBody(String name) {
    return '$name kann sich mit diesem Anmeldedienst nicht mehr anmelden.';
  }

  @override
  String adminUnlinked(String subject) {
    return '$subject wurde getrennt.';
  }

  @override
  String adminRevokePasskeyTitle(String passkey) {
    return 'Passkey $passkey widerrufen?';
  }

  @override
  String adminRevokePasskeyBody(String name) {
    return '$name kann sich mit diesem Passkey nicht mehr anmelden.';
  }

  @override
  String adminPasskeyRevoked(String passkey) {
    return 'Der Passkey $passkey wurde widerrufen.';
  }

  @override
  String get adminEndSessionBody =>
      'Das Gerät dieser Sitzung wird sofort abgemeldet.';

  @override
  String get adminSessionEnded => 'Die Sitzung wurde beendet.';

  @override
  String get adminOwnersTitle => 'Owner';

  @override
  String get adminOwnersAdd => 'Owner hinzufügen';

  @override
  String get adminOwnersAddTitle => 'Owner hinzufügen';

  @override
  String get adminOwnersAddBody => 'Die Person erhält alle Rechte der Instanz.';

  @override
  String adminOwnersAdded(String name) {
    return '$name ist jetzt Owner.';
  }

  @override
  String get adminOwnersRevoke => 'Entziehen';

  @override
  String adminOwnersRevokeTitle(String name) {
    return 'Owner-Rechte von $name entziehen?';
  }

  @override
  String adminOwnersRevokeBody(String name) {
    return '$name behält das Konto, verliert aber die Owner-Rechte.';
  }

  @override
  String adminOwnersRevoked(String name) {
    return '$name ist nicht mehr Owner.';
  }

  @override
  String adminOwnerSince(String when, String reason) {
    return 'Seit $when: $reason';
  }

  @override
  String get adminGroupsTitle => 'Gruppen';

  @override
  String get adminGroupsCreate => 'Gruppe anlegen';

  @override
  String get adminGroupsEmpty => 'Noch keine Gruppen.';

  @override
  String adminGroupMembers(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count Mitglieder',
      one: '1 Mitglied',
      zero: 'Keine Mitglieder',
    );
    return '$_temp0';
  }

  @override
  String adminGroupCreated(String name) {
    return 'Die Gruppe $name wurde angelegt.';
  }

  @override
  String get adminGroupCreateTitle => 'Gruppe anlegen';

  @override
  String get adminGroupCreateBody =>
      'Eine neue Gruppe ohne Mitglieder wird angelegt.';

  @override
  String get adminGroupName => 'Name';

  @override
  String get adminGroupDescription => 'Beschreibung (optional)';

  @override
  String get adminGroupBack => 'Alle Gruppen';

  @override
  String get adminGroupFromProvider =>
      'Der Anmeldedienst verwaltet die Mitglieder dieser Gruppe.';

  @override
  String get adminGroupRename => 'Umbenennen';

  @override
  String adminGroupRenameTitle(String name) {
    return '$name umbenennen';
  }

  @override
  String get adminGroupRenameBody =>
      'Die Gruppe behält ihre Mitglieder und ihre Rechte.';

  @override
  String adminGroupRenamed(String name) {
    return 'Die Gruppe heißt jetzt $name.';
  }

  @override
  String get adminGroupDelete => 'Gruppe löschen';

  @override
  String adminGroupDeleteTitle(String name) {
    return '$name löschen?';
  }

  @override
  String get adminGroupDeleteBody =>
      'Die Gruppe wird entfernt und die Rechte, die sie gab, enden. Die Mitglieder behalten ihre Konten.';

  @override
  String get adminMembersTitle => 'Mitglieder';

  @override
  String get adminMembersEmpty => 'Noch keine Mitglieder.';

  @override
  String get adminMemberAdd => 'Mitglied hinzufügen';

  @override
  String adminMemberAddTitle(String group) {
    return 'Mitglied zu $group hinzufügen';
  }

  @override
  String get adminMemberAddBody =>
      'Die Person erhält die Rechte der Gruppe, bis zum angegebenen Tag, wenn einer gesetzt ist.';

  @override
  String get adminMemberUntil => 'Bis (optional, JJJJ-MM-TT)';

  @override
  String get adminMemberUntilInvalid => 'Gib ein Datum als JJJJ-MM-TT ein.';

  @override
  String adminMemberAdded(String name, String group) {
    return '$name wurde zu $group hinzugefügt.';
  }

  @override
  String adminMemberExpires(String when) {
    return 'Bis $when';
  }

  @override
  String get adminMemberRemove => 'Entfernen';

  @override
  String adminMemberRemoveTitle(String name, String group) {
    return '$name aus $group entfernen?';
  }

  @override
  String adminMemberRemoveBody(String name) {
    return '$name verliert die Rechte der Gruppe.';
  }

  @override
  String adminMemberRemoved(String name, String group) {
    return '$name wurde aus $group entfernt.';
  }

  @override
  String get adminWaitingTitle => 'Wartende Geräte';

  @override
  String get adminWaitingEmpty => 'Kein Gerät wartet.';

  @override
  String adminWaitingDevice(String person, String device) {
    return '$person: $device';
  }

  @override
  String adminWaitingLoggedIn(String when) {
    return 'Angemeldet $when';
  }

  @override
  String adminWaitingVia(String method) {
    return 'über $method';
  }

  @override
  String get adminWaitingNoOrigin => 'Herkunft unbekannt';

  @override
  String get adminApprove => 'Bestätigen';

  @override
  String get adminReject => 'Ablehnen';

  @override
  String adminApproveTitle(String device, String person) {
    return '$device von $person bestätigen';
  }

  @override
  String adminApproveHint(String person) {
    return 'Frage $person nach dem Code, den das wartende Gerät zeigt, und gib ihn hier ein.';
  }

  @override
  String adminApproved(String device, String person) {
    return '$device von $person ist jetzt aktiv.';
  }

  @override
  String adminRejectTitle(String device) {
    return '$device ablehnen?';
  }

  @override
  String adminRejectBody(String person) {
    return 'Das wartende Gerät von $person wird widerrufen und kann sich nicht anmelden.';
  }

  @override
  String adminRejected(String device) {
    return '$device wurde abgelehnt.';
  }

  @override
  String stepUpPurposeCreatePerson(String name) {
    return 'Person $name anlegen';
  }

  @override
  String stepUpPurposeDisablePerson(String name) {
    return '$name sperren';
  }

  @override
  String stepUpPurposeEnablePerson(String name) {
    return '$name entsperren';
  }

  @override
  String stepUpPurposeNewCode(String name) {
    return 'Einladungs-Code für $name ausstellen';
  }

  @override
  String stepUpPurposeUnlinkIdentity(String name) {
    return 'Anmeldedienst von $name trennen';
  }

  @override
  String stepUpPurposeRevokePasskey(String name) {
    return 'Passkey von $name widerrufen';
  }

  @override
  String stepUpPurposeAddOwner(String name) {
    return '$name zum Owner machen';
  }

  @override
  String stepUpPurposeRevokeOwner(String name) {
    return 'Owner-Rechte von $name entziehen';
  }

  @override
  String stepUpPurposeCreateGroup(String name) {
    return 'Gruppe $name anlegen';
  }

  @override
  String stepUpPurposeRenameGroup(String name) {
    return 'Gruppe $name umbenennen';
  }

  @override
  String stepUpPurposeDeleteGroup(String name) {
    return 'Gruppe $name löschen';
  }

  @override
  String stepUpPurposeAddMember(String name, String group) {
    return '$name zu $group hinzufügen';
  }

  @override
  String stepUpPurposeRemoveMember(String name, String group) {
    return '$name aus $group entfernen';
  }

  @override
  String stepUpPurposeApproveDeviceOf(String device, String person) {
    return 'Gerät $device von $person bestätigen';
  }

  @override
  String stepUpPurposeRejectDevice(String device, String person) {
    return 'Gerät $device von $person ablehnen';
  }

  @override
  String get adminNavAudit => 'Audit';

  @override
  String get auditTitle => 'Audit-Log';

  @override
  String get auditFilterAction => 'Aktion';

  @override
  String get auditFilterActorAll => 'Alle Personen';

  @override
  String get auditResultAll => 'Alle Ergebnisse';

  @override
  String get auditResultAllow => 'erlaubt';

  @override
  String get auditResultDeny => 'abgelehnt';

  @override
  String get auditResultError => 'Fehler';

  @override
  String get auditFilterResourceKind => 'Ressourcenart';

  @override
  String get auditFilterResourceId => 'Ressourcen-ID';

  @override
  String get auditFilterFrom => 'Von (JJJJ-MM-TT)';

  @override
  String get auditFilterTo => 'Bis (JJJJ-MM-TT)';

  @override
  String get auditFilterApply => 'Filter anwenden';

  @override
  String get auditFilterInvalidDate => 'Datum als JJJJ-MM-TT eingeben.';

  @override
  String get auditColTime => 'Zeit';

  @override
  String get auditColWho => 'Wer';

  @override
  String get auditColResult => 'Ergebnis';

  @override
  String get auditColResource => 'Ressource';

  @override
  String get auditColReason => 'Grund';

  @override
  String get auditEmpty => 'Keine Einträge zu diesen Filtern.';

  @override
  String get auditStepUpNeeded =>
      'Das Audit-Log zu lesen braucht deine Bestätigung.';

  @override
  String get auditCheckChain => 'Kette prüfen';

  @override
  String auditChainOk(int seq) {
    return 'Kette geprüft bis Nr. $seq: unverändert.';
  }

  @override
  String auditChainBroken(int seq, String reason) {
    return 'Kette gebrochen bei Nr. $seq: $reason';
  }

  @override
  String auditChainRange(int from, int to, int count) {
    return 'Geprüft Nr. $from bis $to; verglichene Prüfpunkte: $count.';
  }

  @override
  String get auditChainReasonHash => 'der Eintrag passt nicht zu seinem Hash.';

  @override
  String get auditChainReasonLink =>
      'der Eintrag verweist nicht auf den Eintrag davor.';

  @override
  String get auditChainReasonGap => 'ein Eintrag fehlt.';

  @override
  String get auditChainReasonCheckpoint => 'ein Prüfpunkt passt nicht.';

  @override
  String get auditChainReasonCheckpointInvalid => 'ein Prüfpunkt ist ungültig.';

  @override
  String auditChainReasonOther(String code) {
    return 'Grund $code.';
  }

  @override
  String get auditChainWarningNoSink =>
      'Warnung: Es ist kein Ziel für Prüfpunkte eingerichtet, Prüfpunkte gibt es nur in der Datenbank.';

  @override
  String auditChainWarning(String code) {
    return 'Warnung: $code';
  }

  @override
  String get auditExport => 'Exportieren';

  @override
  String get auditExportCsv => 'Als CSV exportieren';

  @override
  String get auditExportNeedsRange =>
      'Für den Export einen Zeitraum (von und bis) wählen.';

  @override
  String get auditExportDone => 'Export gespeichert.';

  @override
  String get auditExportFailed =>
      'Der Export brach ab; die Datei ist unvollständig.';

  @override
  String get stepUpPurposeAuditRead => 'Audit-Log lesen';

  @override
  String get stepUpPurposeAuditVerify => 'Hash-Kette des Audit-Logs prüfen';

  @override
  String get stepUpPurposeAuditExport => 'Audit-Log exportieren';

  @override
  String auditDetailTitle(int seq) {
    return 'Eintrag Nr. $seq';
  }

  @override
  String get auditDetailCorrelation => 'Korrelations-ID';

  @override
  String get auditDetailDenyReason => 'Ablehnungsgrund';

  @override
  String get auditDetailDetails => 'Details';

  @override
  String get auditDetailHash => 'Hash';

  @override
  String get auditDetailPrevHash => 'Vorheriger Hash';

  @override
  String get auditDetailClose => 'Schließen';

  @override
  String get auditActionAuditRead => 'Audit-Log lesen';

  @override
  String get auditActionAuditVerify => 'Audit-Kette prüfen';

  @override
  String get auditActionGrantCreate => 'Freigabe anlegen';

  @override
  String get auditActionGrantRevoke => 'Freigabe widerrufen';

  @override
  String get auditActionOwnerAdd => 'Owner hinzufügen';

  @override
  String get auditActionOwnerRemove => 'Owner entfernen';

  @override
  String get auditActionUserCreate => 'Person anlegen';

  @override
  String get auditActionUserDisable => 'Person deaktivieren';

  @override
  String get auditActionUserEnable => 'Person aktivieren';

  @override
  String get auditActionDeviceApprove => 'Gerät bestätigen';

  @override
  String get auditActionDeviceRevoke => 'Gerät widerrufen';

  @override
  String get auditActionGroupCreate => 'Gruppe anlegen';

  @override
  String get auditActionStepUp => 'Mit Step-up bestätigen';

  @override
  String get adminNavGrants => 'Freigaben';

  @override
  String get adminGrantsTitle => 'Freigaben';

  @override
  String get adminGrantsTarget => 'Ziel';

  @override
  String get adminGrantsPickTarget => 'Ziel wählen';

  @override
  String get adminTargetInstance => 'Instanz';

  @override
  String adminTargetSpace(String name) {
    return 'Bereich $name';
  }

  @override
  String adminTargetChannel(String path) {
    return 'Kanal $path';
  }

  @override
  String get adminGrantsCreate => 'Freigabe anlegen';

  @override
  String get adminGrantsWhoHasAccess => 'Wer hat Zugriff?';

  @override
  String get adminGrantsEmpty => 'Keine Freigabe auf diesem Ziel.';

  @override
  String adminGrantsUntil(String day) {
    return 'bis $day';
  }

  @override
  String adminGrantsReason(String reason) {
    return 'Grund: $reason';
  }

  @override
  String adminGrantCreated(String name) {
    return '$name hat die Freigabe erhalten.';
  }

  @override
  String adminGrantRevoked(String name) {
    return 'Die Freigabe für $name wurde widerrufen.';
  }

  @override
  String get adminLevelRead => 'Lesen';

  @override
  String get adminLevelWrite => 'Schreiben';

  @override
  String get adminLevelTrigger => 'Agenten auslösen';

  @override
  String get adminLevelNone => 'Keine Stufe';

  @override
  String get adminCapabilityUpload => 'Dateien hochladen';

  @override
  String get adminCapabilityExport => 'Exportieren';

  @override
  String get adminCapabilityManage => 'Verwalten';

  @override
  String get adminCapabilityTerminal => 'Terminal';

  @override
  String adminWizardStep(int step, int total) {
    return 'Schritt $step von $total';
  }

  @override
  String get adminWizardWho => 'Wer';

  @override
  String get adminWizardWhere => 'Wo';

  @override
  String get adminWizardLevel => 'Stufe und Zusatzrechte';

  @override
  String get adminWizardKindPerson => 'Person';

  @override
  String get adminWizardKindGroup => 'Gruppe';

  @override
  String get adminWizardKindAgent => 'Agent';

  @override
  String get adminWizardSearch => 'Nach Namen suchen';

  @override
  String get adminWizardNoMatch => 'Nichts gefunden.';

  @override
  String get adminWizardCapabilities => 'Zusatzrechte';

  @override
  String get adminWizardUntil => 'Gültig bis (JJJJ-MM-TT, optional)';

  @override
  String get adminReasonRequired => 'Grund (Pflicht, steht im Audit-Log)';

  @override
  String get adminWizardBack => 'Zurück';

  @override
  String get adminWizardNext => 'Weiter';

  @override
  String get adminWizardConfirm => 'Bestätigen';

  @override
  String get adminPreviewTitle => 'Vorschau in Klartext';

  @override
  String get adminPreviewLoading => 'Wirkung wird geprüft';

  @override
  String get adminPreviewEffects => 'Wer betroffen ist';

  @override
  String adminPreviewAdded(String level) {
    return 'Erhält $level';
  }

  @override
  String adminPreviewRemoved(String level) {
    return 'Verliert $level';
  }

  @override
  String adminPreviewChanged(String before, String after) {
    return '$before wird zu $after';
  }

  @override
  String adminPreviewCapabilities(String list) {
    return 'Zusätzlich: $list';
  }

  @override
  String get adminPreviewRejected => 'Würde abgelehnt';

  @override
  String adminPreviewAgent(String agent) {
    return 'Agent: $agent';
  }

  @override
  String adminPreviewAffected(String people) {
    return 'Betroffen: $people';
  }

  @override
  String get adminPreviewI1Hint =>
      'Stelle den Agenten zuerst auf Organisation um.';

  @override
  String adminPreviewDenied(String reason) {
    return 'Die Freigabe würde abgelehnt: $reason';
  }

  @override
  String get adminPreviewHints => 'Hinweise';

  @override
  String adminHintReadHorizon(String person, String channel, String agent) {
    return 'Lesehorizont (H1): $person kann Inhalte aus $channel über Agent $agent erfahren';
  }

  @override
  String adminHintTriggerBlocked(String agent) {
    return 'Auslösen gesperrt: $agent kann von diesem Prinzipal nicht ausgelöst werden (SR2, SR3)';
  }

  @override
  String adminHintGroupReach(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: 'Diese Freigabe erreicht $count Personen',
      one: 'Diese Freigabe erreicht 1 Person',
    );
    return '$_temp0';
  }

  @override
  String get adminHintExpiryMissing =>
      'Kein Ablaufdatum: Die Freigabe für eine Person außerhalb von Gruppen endet nicht';

  @override
  String get adminHintTerminalNotEffective =>
      'Die Freigabe wirkt auf diesem Rechner nicht (SR10, SR12)';

  @override
  String adminHintUnknown(String code) {
    return 'Hinweis $code';
  }

  @override
  String adminRevokeGrantTitle(String name) {
    return 'Freigabe für $name widerrufen?';
  }

  @override
  String get adminRevokeGrantBody =>
      'Der Widerruf wirkt sofort: Der Zugriff endet jetzt.';

  @override
  String adminAccessTitle(String target) {
    return 'Wer hat Zugriff? $target';
  }

  @override
  String get adminAccessEmpty => 'Niemand hat Zugriff.';

  @override
  String get adminSourceDirect => 'Direkte Freigabe';

  @override
  String adminSourceGroup(String group) {
    return 'Über Gruppe $group';
  }

  @override
  String adminSourceInheritedSpace(String space) {
    return 'Geerbt vom Bereich $space';
  }

  @override
  String get adminSourceInheritedInstance => 'Geerbt von der Instanz';

  @override
  String get adminSourceDm => 'Teilnehmer';

  @override
  String get adminSourceOperator => 'Bediener dieses Kanals';

  @override
  String adminSourceViaAgent(String agent) {
    return 'Kann Inhalte hier über Agent $agent erfahren (H1)';
  }

  @override
  String adminSourceGuardBlocked(String rule) {
    return 'Agenten auslösen, aber das Auslösen ist gesperrt ($rule)';
  }

  @override
  String adminSourceGuardAgent(String agent) {
    return 'Agent $agent';
  }

  @override
  String get adminSourceUnknown => 'Andere Herkunft';

  @override
  String adminSourceMemberUntil(String day) {
    return 'Mitglied bis $day';
  }

  @override
  String get adminAccessViewAs => 'Als Person ansehen';

  @override
  String adminViewAsBanner(String name) {
    return 'Ansicht als $name (nur lesen). Nichts wird gesendet. Protokolliert.';
  }

  @override
  String adminViewAsUntil(String when) {
    return 'Die Ansicht endet $when';
  }

  @override
  String get adminViewAsExit => 'Beenden';

  @override
  String get adminViewAsSpaces => 'Bereichsliste';

  @override
  String get adminViewAsChannels => 'Kanalliste';

  @override
  String get adminViewAsNothing => 'Nichts sichtbar.';

  @override
  String stepUpPurposeCreateGrant(String name, String target) {
    return 'Freigabe für $name auf $target anlegen';
  }

  @override
  String stepUpPurposeRevokeGrant(String name) {
    return 'Freigabe für $name widerrufen';
  }

  @override
  String stepUpPurposeViewAs(String name) {
    return 'Als $name ansehen';
  }

  @override
  String get problemForbiddenDelegationLevel =>
      'Du kannst nicht mehr als deine eigene Stufe vergeben';

  @override
  String get problemForbiddenDelegationCapabilities =>
      'Du kannst keine Zusatzrechte vergeben, die du nicht hast';

  @override
  String get problemForbiddenSubjectKind =>
      'Für Agenten oder Apps nicht erlaubt';

  @override
  String get adminNavHosts => 'Rechner';

  @override
  String get hostsAdd => 'Rechner hinzufügen';

  @override
  String get hostsEmpty => 'Noch keine Rechner.';

  @override
  String get hostOwnersOnly => 'Nur Owner';

  @override
  String get hostStatePairing => 'Kopplung';

  @override
  String get hostStateConfirming => 'Wartet auf Bestätigung';

  @override
  String get hostStateActive => 'aktiv';

  @override
  String get hostStateDisabled => 'deaktiviert';

  @override
  String get hostStateDuplicate =>
      'deaktiviert: eine zweite Verbindung nutzte dieselben Schlüssel, nur eine neue Kopplung hilft';

  @override
  String get hostStateRetired => 'stillgelegt';

  @override
  String get hostStateUnknown => 'unbekannt';

  @override
  String get hostConnected => 'verbunden';

  @override
  String get hostDisconnected => 'getrennt';

  @override
  String hostLastSeen(String when) {
    return 'zuletzt gesehen $when';
  }

  @override
  String get hostBack => 'Zurück zu den Rechnern';

  @override
  String get hostFieldAssignee => 'Zugewiesen an';

  @override
  String get hostFieldPlatform => 'Plattform';

  @override
  String get hostFieldConnection => 'Verbindung';

  @override
  String get hostFieldFingerprint => 'Fingerabdruck';

  @override
  String get hostFieldLlmLogin => 'LLM-Anmeldung';

  @override
  String get hostFieldServerHost => 'Server-Rechner';

  @override
  String get hostFieldChecks => 'Prüfungen';

  @override
  String get hostFieldProfiles => 'Profile';

  @override
  String get hostFieldGrants => 'Freigaben';

  @override
  String get hostFieldSessionUsers => 'Sitzungsbenutzer';

  @override
  String get hostFieldSessions => 'Terminal-Sitzungen';

  @override
  String hostPlatformValue(String platform, String version) {
    return '$platform, Version $version';
  }

  @override
  String get hostNotReported => 'Noch nicht gemeldet';

  @override
  String hostFingerprintValue(String value, int generation) {
    return '$value   Schlüsselgeneration $generation';
  }

  @override
  String get hostNoKeys => 'Keine Schlüssel';

  @override
  String get hostLlmNone => 'Keine';

  @override
  String get hostLlmPersonal => 'Persönlich';

  @override
  String hostLlmPersonalOf(String name) {
    return 'Persönlich von $name';
  }

  @override
  String get hostLlmOrganization => 'Organisation';

  @override
  String get hostLlmI6Explained =>
      'Persönlich: nur diese Person darf hier ein Terminal öffnen, auch keine anderen Owner.';

  @override
  String get hostLlmHolder => 'Inhaber der Anmeldung';

  @override
  String hostLlmTitle(String name) {
    return 'LLM-Anmeldung auf $name';
  }

  @override
  String get hostLlmBody =>
      'Ein Wechsel auf Persönlich oder ein neuer Inhaber beendet die Terminal-Sitzungen anderer Personen auf diesem Rechner.';

  @override
  String hostLlmChanged(String name) {
    return 'Die LLM-Anmeldung von $name ist geändert.';
  }

  @override
  String get hostServerHostQuestion =>
      'Ist dieser Rechner der Server der Instanz?';

  @override
  String get hostYes => 'Ja';

  @override
  String get hostNo => 'Nein';

  @override
  String hostServerHostDeclared(String answer) {
    return 'Angegeben: $answer';
  }

  @override
  String hostServerHostReported(String answer) {
    return 'Meldung des Host-Dienstes auf dem Rechner: $answer';
  }

  @override
  String hostDoctorOk(String check) {
    return '$check ok';
  }

  @override
  String hostDoctorFailed(String check, String code) {
    return '$check nicht bestanden: $code';
  }

  @override
  String hostDoctorAssigned(String osUser, String name) {
    return '$osUser (zugewiesen an $name)';
  }

  @override
  String hostDoctorFormer(String osUser, String name) {
    return '$osUser (früher zugewiesen an $name)';
  }

  @override
  String hostDoctorUnknown(String osUser) {
    return '$osUser (keine Zuweisung bekannt)';
  }

  @override
  String get hostDoctorNone => 'Noch keine Prüfergebnisse.';

  @override
  String get hostDoctorAuditFailed =>
      'Das Audit dieses Rechners war nicht lesbar, deshalb stehen frühere Sitzungsbenutzer ohne ihre Person da.';

  @override
  String hostWorkdirs(String folders) {
    return 'Arbeitsordner: $folders';
  }

  @override
  String get hostWorkdirsNone => 'Arbeitsordner: keine';

  @override
  String hostSessionUsersUnassigned(String osUser) {
    return 'Gemeldet, nicht zugewiesen: $osUser';
  }

  @override
  String get hostSessionUsersSr12 =>
      'Ohne Sitzungsbenutzer kein Terminal (SR12).';

  @override
  String get hostSessionUsersOnlyReported =>
      'Nur Namen, die der Rechner bei der Kopplung oder in seiner letzten Meldung gemeldet hat.';

  @override
  String get hostSessionUserAssign => 'Zuweisen';

  @override
  String get hostSessionUserAssignTitle => 'Sitzungsbenutzer zuweisen';

  @override
  String get hostSessionUserAssignBody =>
      'Ein einmal vergebener Name bleibt auf diesem Rechner an seine Person gebunden.';

  @override
  String get hostSessionUserPerson => 'Person';

  @override
  String get hostSessionUserName => 'Gemeldeter Name';

  @override
  String hostSessionUserAssigned(String name, String osUser) {
    return '$name hat jetzt den Sitzungsbenutzer $osUser.';
  }

  @override
  String get hostSessionUserEnd => 'Beenden';

  @override
  String hostSessionUserEndTitle(String name) {
    return 'Zuweisung von $name beenden?';
  }

  @override
  String hostSessionUserEndBody(String name, String osUser) {
    return '$name kann auf diesem Rechner kein Terminal mehr öffnen. Der Name $osUser bleibt auf diesem Rechner vergeben und geht nie an eine andere Person.';
  }

  @override
  String hostSessionUserEnded(String name, String osUser) {
    return 'Die Zuweisung von $name ist beendet; $osUser bleibt vergeben.';
  }

  @override
  String get hostSessionUserTaken =>
      'Dieser Name ist vergeben: ein einmal vergebener Name bleibt auf diesem Rechner an seine Person gebunden, auch nach dem Ende der Zuweisung, und jede Person hat einen Sitzungsbenutzer je Rechner.';

  @override
  String get hostSessionUserNotOnHost =>
      'Der Rechner hat diesen Namen nicht gemeldet. Erst muss der Rechner den Namen melden, dann lässt er sich zuweisen.';

  @override
  String get hostSessionRowsTitle => 'Sitzungsbenutzer je Person';

  @override
  String get hostSessionRowsAdd => 'Sitzungsbenutzer hinzufügen';

  @override
  String get hostSessionRowRemove => 'Entfernen';

  @override
  String get hostChange => 'Ändern';

  @override
  String hostAssigneeTitle(String name) {
    return 'Zuweisung von $name';
  }

  @override
  String get hostAssigneeBody =>
      'Eine Person, die kein Owner ist, bekommt ein Terminal nur auf dem Rechner, der ihr zugewiesen ist (SR10).';

  @override
  String hostAssigneeChanged(String name) {
    return 'Die Zuweisung von $name ist geändert.';
  }

  @override
  String get hostGrantsAdd => 'Terminal freigeben';

  @override
  String get hostGrantsNone => 'Keine Freigaben auf diesem Rechner.';

  @override
  String get hostNotEffective => 'Nicht wirksam';

  @override
  String hostSessionsCount(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count offene Terminal-Sitzungen',
      one: '1 offene Terminal-Sitzung',
      zero: 'Keine offenen Terminal-Sitzungen',
    );
    return '$_temp0';
  }

  @override
  String get hostRepair => 'Neu koppeln';

  @override
  String hostRepairTitle(String name) {
    return '$name neu koppeln?';
  }

  @override
  String get hostRepairBody =>
      'Die Schlüssel des Rechners verfallen sofort, seine Verbindungen schließen und seine Terminal-Sitzungen enden. Ein neuer Kopplungs-Code folgt; die Schlüsselgeneration steigt mit der nächsten Kopplung.';

  @override
  String get hostPair => 'Kopplung fortsetzen';

  @override
  String get hostDisable => 'Deaktivieren';

  @override
  String hostDisableTitle(String name) {
    return '$name deaktivieren?';
  }

  @override
  String get hostDisableBody =>
      'Alle Terminal-Sitzungen auf diesem Rechner enden jetzt. Niemand kann hier ein Terminal öffnen, bis er wieder aktiviert ist.';

  @override
  String get hostEnable => 'Aktivieren';

  @override
  String hostEnableTitle(String name) {
    return '$name aktivieren?';
  }

  @override
  String get hostEnableBody =>
      'Terminal-Sitzungen sind für die Personen, die sie öffnen dürfen, wieder möglich.';

  @override
  String get hostRetire => 'Stilllegen';

  @override
  String hostRetireTitle(String name) {
    return '$name stilllegen?';
  }

  @override
  String get hostRetireBody =>
      'Alle Terminal-Sitzungen enden jetzt, und der Rechner ist für immer stillgelegt. Er lässt sich weder koppeln noch wieder aktivieren.';

  @override
  String hostStateChanged(String name, String state) {
    return '$name ist jetzt $state.';
  }

  @override
  String get hostPairingTitle => 'Rechner koppeln';

  @override
  String get hostCreateName => 'Name';

  @override
  String get hostCreateAssignee => 'Zugewiesen an (optional)';

  @override
  String get hostCreateSubmit => 'Anlegen und Code holen';

  @override
  String get hostPairingCommandIntro =>
      'Diesen Befehl auf dem Rechner als Benutzer des Host-Dienstes eingeben (ein Einmal-Code, der bald abläuft):';

  @override
  String hostPairingExpiresIn(String time) {
    return 'Läuft ab in $time';
  }

  @override
  String get hostPairingExpired =>
      'Der Kopplungs-Code ist abgelaufen. Einen neuen Code ausstellen.';

  @override
  String get hostPairingNewCode => 'Neuer Code';

  @override
  String get hostPairingNotYet =>
      'Der Rechner hat sich noch nicht gemeldet. Den Befehl auf dem Rechner eingeben, dann erneut versuchen.';

  @override
  String get hostPairingNeedsCode =>
      'Dieser Rechner wartet auf einen Kopplungs-Code. Einen neuen Code ausstellen, um fortzufahren.';

  @override
  String get hostPairingClose => 'Schließen';

  @override
  String hostFingerprintLabel(int count) {
    return 'Fingerabdruck vom Rechner eingeben (erste $count Zeichen)';
  }

  @override
  String get hostFingerprintCompare =>
      'Mit der Ausgabe auf dem Rechner vergleichen und erneut eingeben.';

  @override
  String hostAttemptsLeft(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count Versuche übrig',
      one: '1 Versuch übrig',
    );
    return '$_temp0';
  }

  @override
  String get hostFingerprintFallback =>
      'Keine Versuche mehr: der Rechner ist zurück in der Kopplung, seine Schlüssel sind verworfen. Einen neuen Kopplungs-Code ausstellen.';

  @override
  String get hostLlmQuestion => 'LLM-Anmeldung auf diesem Rechner';

  @override
  String get hostConfirmSubmit => 'Mit Passkey / Gerät bestätigen';

  @override
  String hostConfirmed(String name) {
    return '$name ist aktiv.';
  }

  @override
  String stepUpPurposeCreateHost(String name) {
    return 'Den Rechner $name anlegen';
  }

  @override
  String stepUpPurposePairingCode(String name) {
    return 'Einen Kopplungs-Code für $name ausstellen';
  }

  @override
  String stepUpPurposeConfirmHost(String name) {
    return 'Den Rechner $name bestätigen';
  }

  @override
  String stepUpPurposeUpdateHost(String name) {
    return 'Den Rechner $name ändern';
  }

  @override
  String stepUpPurposeHostLlmLogin(String name) {
    return 'Die LLM-Anmeldung von $name ändern';
  }

  @override
  String stepUpPurposeHostState(String name) {
    return 'Den Zustand von $name ändern';
  }

  @override
  String stepUpPurposeHostAudit(String name) {
    return 'Das Audit von $name lesen';
  }

  @override
  String adminTargetHost(String name) {
    return 'Rechner $name';
  }

  @override
  String get adminTargetsHosts => 'Rechner';

  @override
  String get threadNoOperator => 'Kein Agent antwortet hier';

  @override
  String threadOperator(String name) {
    return 'Bediener: $name';
  }

  @override
  String get threadTitleAutomatic => 'automatisch';

  @override
  String get threadDone => 'Erledigt';

  @override
  String get threadReopen => 'Wieder öffnen';

  @override
  String get threadMove => 'Verschieben';

  @override
  String get threadPin => 'Anheften';

  @override
  String get threadColumn => 'Spalte';

  @override
  String get threadEditTitle => 'Titel bearbeiten';

  @override
  String get threadTitleLabel => 'Titel';

  @override
  String get threadSave => 'Speichern';

  @override
  String get threadCancel => 'Abbrechen';

  @override
  String threadStarter(String title, int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count Antworten',
      one: '1 Antwort',
    );
    return 'Thread: $title, $_temp0';
  }

  @override
  String get threadLoadingOlder => 'Frühere Nachrichten werden geladen';

  @override
  String get threadLoadingNewer => 'Fehlende Nachrichten werden geladen';

  @override
  String get threadEmpty => 'Noch keine Nachrichten';

  @override
  String get threadNoAccess => 'Kein Zugriff mehr';

  @override
  String get threadNoAccessWaiting =>
      'Nicht gesendete Nachrichten zum Kopieren:';

  @override
  String get messageActionStartThread => 'Thread starten';

  @override
  String get messageActionUnpin => 'Lösen';

  @override
  String get messageActionVersions => 'Fassungen ansehen';

  @override
  String get messageActionDelete => 'Löschen';

  @override
  String get messageActionMarkUnread => 'Als ungelesen markieren';

  @override
  String get messageActionCopyText => 'Text kopieren';

  @override
  String get messageActionCopyLink => 'Link kopieren';

  @override
  String get messageMenuLabel => 'Aktionen der Nachricht';

  @override
  String get messageDeleteTitle => 'Nachricht löschen?';

  @override
  String get messageDeleteBody =>
      'Die Nachricht wird für alle durch \"Nachricht gelöscht\" ersetzt.';

  @override
  String get messageVersionsTitle => 'Fassungen';

  @override
  String get messageVersionsCurrent => 'Aktuelle Fassung';

  @override
  String messageVersionsRevision(int number) {
    return 'Fassung $number';
  }

  @override
  String get messageVersionsEmpty => 'Keine früheren Fassungen';

  @override
  String messageVersionsFailed(String reason) {
    return 'Die Fassungen konnten nicht geladen werden: $reason';
  }

  @override
  String get messageConflictTitle => 'Die Nachricht wurde anderswo geändert';

  @override
  String get messageConflictMine => 'Deine Fassung';

  @override
  String get messageConflictTheirs => 'Aktuelle Fassung';

  @override
  String get messageConflictKeepMine => 'Meine behalten';

  @override
  String get messageConflictTakeTheirs => 'Aktuelle übernehmen';

  @override
  String get messageEditLabel => 'Nachricht bearbeiten';

  @override
  String get messageReactionTitle => 'Reagieren';

  @override
  String get messageReactionInput => 'Beliebiges Zeichen';

  @override
  String get messageReactionAdd => 'Hinzufügen';

  @override
  String get messageCopied => 'Kopiert';

  @override
  String outboxFailedAction(String reason) {
    return 'Nicht gespeichert: $reason';
  }

  @override
  String get outboxCompare => 'Vergleichen';

  @override
  String get outboxCopy => 'Kopieren';

  @override
  String composerTooLong(int limit) {
    return 'Die Nachricht ist zu lang: höchstens $limit Byte';
  }

  @override
  String get composerDraftOverwritten =>
      'Dieser Entwurf hat eine neuere Fassung eines anderen Geräts ersetzt';

  @override
  String composerReplyTo(String name) {
    return 'Antwort an $name';
  }

  @override
  String get composerCancelReply => 'Antwort abbrechen';

  @override
  String get composerTemplates => 'Vorlagen';

  @override
  String get composerTemplatesPersonal => 'Deine Vorlagen';

  @override
  String get composerTemplatesChannel => 'Vorlagen dieses Kanals';

  @override
  String get composerTemplatesEmpty => 'Keine Vorlagen';

  @override
  String get composerTemplatesSave => 'Als Vorlage speichern';

  @override
  String get composerTemplatesName => 'Name';

  @override
  String sessionPinned(int count) {
    return 'Angeheftet ($count)';
  }

  @override
  String get sessionPinnedEmpty => 'Keine angehefteten Nachrichten';

  @override
  String get sessionNoThread => 'Öffne einen Thread, um seine Sitzung zu sehen';

  @override
  String get threadMoveNoTarget => 'Kein Kanal mit demselben Bediener';

  @override
  String get threadMoveReason => 'Grund';

  @override
  String get consoleNavLive => 'Live';

  @override
  String get consoleNavStats => 'Statistik';

  @override
  String get consoleNavHosts => 'Rechner und Terminal';

  @override
  String get consoleNavPeople => 'Personen';

  @override
  String get consoleNavGroups => 'Gruppen und Freigaben';

  @override
  String get consoleNavDevices => 'Geräte';

  @override
  String get consoleNavAgents => 'Agenten und Kosten';

  @override
  String get consoleNavApps => 'Apps und Webhooks';

  @override
  String get consoleNavAudit => 'Audit';

  @override
  String get consoleNavImport => 'Import';

  @override
  String get consoleNavInstance => 'Instanz und Branding';

  @override
  String get consoleNavStatus => 'Status';

  @override
  String get consoleNavGrafana => 'Betrieb (Grafana)';

  @override
  String get consoleGrafanaNote =>
      'Diese Seite zeigt Grafana. Sie folgt den Berechtigungen von Grafana, nicht denen dieser Instanz.';

  @override
  String get consoleGrafanaOpen => 'In neuem Tab öffnen';

  @override
  String get consoleGrafanaFrameLabel => 'Grafana-Dashboard';

  @override
  String get consoleGrafanaUnavailable =>
      'Für diese Konsole ist kein Grafana-Dashboard eingerichtet.';

  @override
  String get consoleNavigationLabel => 'Navigation der Konsole';

  @override
  String consoleTitle(String product) {
    return '$product Konsole';
  }

  @override
  String get consolePlaceholderPage =>
      'Diese Seite kommt mit einer späteren Version der Konsole.';

  @override
  String consoleSignedInAs(String name) {
    return 'Angemeldet: $name';
  }

  @override
  String get consoleLogout => 'Abmelden';

  @override
  String consoleThreadInApp(String id) {
    return 'Thread $id: Im Browser gibt es keinen Chat. Öffne ihn in der App.';
  }

  @override
  String get consoleCopyThreadId => 'Thread-ID kopieren';

  @override
  String get consoleCopied => 'Kopiert';

  @override
  String get consoleWorking => 'Einen Moment';

  @override
  String consoleLoginTitle(String product) {
    return 'Bei $product anmelden';
  }

  @override
  String consoleLoginWithProvider(String provider) {
    return 'Mit $provider anmelden';
  }

  @override
  String get consoleLoginOr => 'oder';

  @override
  String get consoleLoginName => 'Anmeldename';

  @override
  String get consoleLoginWithPasskey => 'Mit Passkey anmelden';

  @override
  String get consoleLoginPasswordAndCode => 'Passwort und Einmal-Code';

  @override
  String get consoleLoginPassword => 'Passwort';

  @override
  String get consoleLoginOneTimeCode => 'Einmal-Code';

  @override
  String get consoleLoginSubmit => 'Anmelden';

  @override
  String get consoleLoginNoMethod =>
      'Für diese Instanz ist keine Anmeldung eingerichtet. Bitte den Eigentümer, eine einzurichten.';

  @override
  String get consoleLoginFailed =>
      'Die Anmeldung ist fehlgeschlagen. Prüfe Anmeldename, Passwort und Einmal-Code und versuche es erneut.';

  @override
  String get consoleLoginPasskeyFailed =>
      'Der Passkey wurde nicht angenommen. Versuche es erneut oder wähle einen anderen Weg.';

  @override
  String get consoleLoginOidcFailed =>
      'Der Anmeldedienst hat die Anmeldung nicht bestätigt. Starte die Anmeldung erneut.';

  @override
  String get consoleLoginOidcCancelled =>
      'Die Anmeldung wurde beim Anmeldedienst abgebrochen. Starte sie erneut oder wähle einen anderen Weg.';

  @override
  String get consoleLoginOidcExpired =>
      'Die Anmeldung hat zu lange gedauert. Starte sie erneut.';

  @override
  String get consoleLoginUnavailable =>
      'Der Server ist nicht erreichbar. Prüfe die Verbindung und versuche es erneut.';

  @override
  String get consoleCallbackWorking => 'Die Anmeldung wird abgeschlossen';

  @override
  String get consoleStepUpTitle => 'Bestätige, dass du es bist';

  @override
  String get consoleStepUpExplanation =>
      'Vor jeder Verwaltung verlangt die Konsole eine frische Bestätigung.';

  @override
  String get consoleStepUpPasskey => 'Mit Passkey bestätigen';

  @override
  String consoleStepUpOidc(String provider) {
    return 'Erneut mit $provider anmelden';
  }

  @override
  String get consoleStepUpPassword => 'Mit Passwort und Einmal-Code bestätigen';

  @override
  String get consoleStepUpConfirm => 'Bestätigen';

  @override
  String get consoleStepUpFailed =>
      'Die Bestätigung ist fehlgeschlagen. Versuche es erneut oder wähle einen anderen Weg.';

  @override
  String get consoleStepUpNoMethod =>
      'Für dich ist kein Weg zur Bestätigung eingerichtet. Bitte den Eigentümer, dir einen Passkey einzurichten.';

  @override
  String get consoleLoggedOutTitle => 'Du bist abgemeldet.';

  @override
  String consoleLoggedOutReason(String reason) {
    return 'Grund: $reason';
  }

  @override
  String get consoleLoggedOutByYou =>
      'Du hast dich von der Konsole abgemeldet.';

  @override
  String get consoleLogInAgain => 'Erneut anmelden';

  @override
  String get consoleUpdatedReload =>
      'Die Konsole wurde aktualisiert. Lade die Seite neu, um weiterzumachen.';

  @override
  String get consoleReload => 'Neu laden';

  @override
  String get adminNavAgents => 'Agenten';

  @override
  String get adminAgentsTitle => 'Agenten';

  @override
  String get adminAgentsEmpty => 'Keine Agenten.';

  @override
  String get adminAgentsSwitchAll => 'Alle Agenten umstellen';

  @override
  String get adminAgentDisabled => 'Deaktiviert';

  @override
  String adminAgentWarningCount(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count Warnungen',
      one: '1 Warnung',
    );
    return '$_temp0';
  }

  @override
  String get adminAgentBack => 'Zurück zu den Agenten';

  @override
  String get adminAgentStateOnline => 'Online';

  @override
  String get adminAgentStateDegraded => 'Eingeschränkt';

  @override
  String get adminAgentStateOffline => 'Offline';

  @override
  String get adminAgentStateUnknown => 'Noch kein Heartbeat';

  @override
  String adminAgentAdapter(String kind) {
    return 'Adapter $kind';
  }

  @override
  String adminAgentRuntimeVersion(String version) {
    return 'Laufzeit-Version $version';
  }

  @override
  String adminAgentLastHeartbeat(String when) {
    return 'Letzter Heartbeat $when';
  }

  @override
  String get adminAgentConfigured => 'Eingestellter Modus';

  @override
  String get adminAgentReported => 'Gemeldeter Modus';

  @override
  String get adminAgentReportedPerRun =>
      'Folgt dem eingestellten Modus je Lauf';

  @override
  String adminAgentEvidence(String method) {
    return 'Beleg: $method';
  }

  @override
  String adminAgentSupported(String modes) {
    return 'Kann belegen: $modes';
  }

  @override
  String get adminAgentSupportedNone => 'Kann belegen: nicht gemeldet';

  @override
  String adminAgentHolder(String name) {
    return 'Inhaber: $name';
  }

  @override
  String adminAgentWarnMismatch(String reported, String configured) {
    return 'Adapter meldet $reported, eingestellt ist $configured';
  }

  @override
  String get adminAgentWarnUnsupported =>
      'Adapter kann diesen Modus nicht belegen';

  @override
  String get adminAgentWarnEvidenceUnknown =>
      'Anmeldeart der Laufzeit unbekannt';

  @override
  String get adminAgentWarnToolClass =>
      'Adapter meldet eine andere Werkzeugklasse';

  @override
  String adminAgentWarnUnknown(String code) {
    return 'Warnung $code';
  }

  @override
  String get adminAgentWarnFailUntilSwitched =>
      'Aufrufe scheitern, bis der Adapter umgestellt ist.';

  @override
  String get adminAgentWarnFailAll => 'Alle Aufrufe scheitern.';

  @override
  String adminAgentSwitchHint(String mode) {
    return 'Zum Umstellen stellt, wer die Instanz betreibt, die Anmeldung der Laufzeit auf $mode um. Die Warnung erlischt mit dem nächsten Heartbeat, der $mode meldet.';
  }

  @override
  String get adminAgentTargets => 'Bedient';

  @override
  String get adminAgentTargetsNone => 'Kein Bereich und kein Kanal';

  @override
  String adminAgentTargetDefault(String target) {
    return '$target, als Standard';
  }

  @override
  String get adminAgentBackend => 'Backend';

  @override
  String get adminAgentSwitch => 'Backend umstellen';

  @override
  String get adminAgentHolderChange => 'Inhaber ändern';

  @override
  String get adminAgentSwitchExplain =>
      'Umstellen braucht einen Grund und eine Bestätigung. Persönlich geht nur, solange genau eine Person den Agenten auslösen darf (I1).';

  @override
  String adminSwitchTitle(String agent) {
    return '$agent umstellen';
  }

  @override
  String adminSwitchToOrgBody(String agent) {
    return '$agent läuft dann mit dem Schlüssel der Organisation: Die Organisation trägt die Kosten, und alle mit Agenten auslösen dürfen ihn auslösen. Der Inhaber wird entfernt.';
  }

  @override
  String adminSwitchToPersonalBody(String agent) {
    return '$agent läuft dann mit dem persönlichen Abo des Inhabers: Der Inhaber zahlt, und nur er darf ihn auslösen.';
  }

  @override
  String get adminSwitchHolderPick => 'Inhaber wählen';

  @override
  String get adminSwitchConfirm => 'Umstellen';

  @override
  String adminSwitched(String agent, String mode) {
    return '$agent läuft jetzt mit $mode';
  }

  @override
  String stepUpPurposeSwitchAgent(String agent) {
    return 'Backend von $agent umstellen';
  }

  @override
  String get stepUpPurposeSwitchAll => 'Backend mehrerer Agenten umstellen';

  @override
  String stepUpPurposeSetHolder(String agent) {
    return 'Inhaber von $agent ändern';
  }

  @override
  String stepUpPurposeCreateToken(String agent) {
    return 'Token für $agent erzeugen';
  }

  @override
  String adminHolderTitle(String agent) {
    return 'Inhaber von $agent ändern';
  }

  @override
  String adminHolderBody(String agent) {
    return '$agent läuft weiter Persönlich, mit dem Abo des neuen Inhabers. Nur der neue Inhaber darf ihn auslösen.';
  }

  @override
  String adminHolderChanged(String name, String agent) {
    return '$name ist jetzt Inhaber von $agent';
  }

  @override
  String get adminI1Title => 'Persönlich ist gerade nicht möglich';

  @override
  String adminI1Agent(String agent, String holder, String names) {
    return '$agent liefe mit dem Abo von $holder, aber diese dürfen ihn auch auslösen: $names';
  }

  @override
  String adminI1AgentNoHolder(String agent, String names) {
    return '$agent dürfen auch auslösen: $names';
  }

  @override
  String adminI1Hidden(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count weitere, die du nicht sehen kannst',
      one: '1 weitere, die du nicht sehen kannst',
    );
    return '$_temp0';
  }

  @override
  String get adminI1Way =>
      'Entziehe ihnen zuerst ihre Freigaben, Gruppenmitgliedschaften oder Owner-Rechte, oder bleib bei Organisation.';

  @override
  String get adminSwitchAllTitle => 'Mehrere Agenten umstellen';

  @override
  String get adminSwitchAllOrgBody =>
      'Alle Agenten laufen dann mit dem Schlüssel der Organisation; Inhaber werden entfernt. Nutze das bei Zweifeln an einem persönlichen Betrieb.';

  @override
  String get adminSwitchAllPersonalBody =>
      'Die angehakten Agenten laufen dann mit dem persönlichen Abo des Inhabers. Würde einer davon I1 verletzen, ändert sich nichts.';

  @override
  String get adminSwitchAllProposed =>
      'Vorschlag: die Agenten, die die letzte Umstellung mehrerer Agenten auf Organisation geändert hat';

  @override
  String get adminSwitchAllNoProposal =>
      'Keine frühere Umstellung als Vorschlag. Hake die Agenten selbst an.';

  @override
  String adminSwitchAllDone(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count Agenten geändert',
      one: '1 Agent geändert',
      zero: 'Kein Agent geändert',
    );
    return '$_temp0';
  }

  @override
  String get adminTokensTitle => 'API-Tokens';

  @override
  String get adminTokensEmpty => 'Keine Tokens.';

  @override
  String get adminTokenCreate => 'Token erzeugen';

  @override
  String adminTokenCreateTitle(String agent) {
    return 'Token für $agent erzeugen';
  }

  @override
  String adminTokenCreateBody(String agent) {
    return 'Mit diesem Token handelt der Adapter als $agent. Sein Geheimnis wird nur einmal angezeigt.';
  }

  @override
  String get adminTokenName => 'Name';

  @override
  String get adminTokenLastDay => 'Letzter Tag (JJJJ-MM-TT)';

  @override
  String get adminTokenLastDayInvalid => 'Gib einen Tag als JJJJ-MM-TT ein.';

  @override
  String get adminTokenDpop => 'Fingerabdruck des DPoP-Schlüssels des Adapters';

  @override
  String adminTokenValidUntil(String when) {
    return 'gültig bis $when';
  }

  @override
  String adminTokenLastUsed(String when) {
    return 'zuletzt benutzt $when';
  }

  @override
  String get adminTokenBound => 'DPoP-gebunden';

  @override
  String get adminTokenRevokedLabel => 'Widerrufen';

  @override
  String get adminTokenRevoke => 'Widerrufen';

  @override
  String adminTokenRevokeTitle(String name) {
    return 'Token $name widerrufen?';
  }

  @override
  String get adminTokenRevokeBody =>
      'Ein Adapter mit diesem Token verliert sofort seinen Zugang.';

  @override
  String adminTokenRevokedNotice(String name) {
    return 'Das Token $name ist widerrufen.';
  }

  @override
  String adminTokenSecretTitle(String name) {
    return 'Token $name';
  }

  @override
  String get adminTokenSecretOnce =>
      'Dieses Geheimnis wird nur einmal angezeigt. Kopiere es jetzt in den Geheimnisspeicher des Adapters.';

  @override
  String get adminTokenSecretLabel => 'Token-Geheimnis';

  @override
  String adminGuidedExplainGrant(String person, String agent, String holder) {
    return 'Diese Freigabe erlaubt $person, den Agenten $agent auszulösen. $agent läuft mit dem persönlichen Abo von $holder. $agent muss zuerst auf Organisation umgestellt werden; die Kosten trägt dann der Schlüssel der Organisation.';
  }

  @override
  String adminGuidedExplainChange(String person, String agent, String holder) {
    return 'Diese Änderung erlaubt $person, den Agenten $agent auszulösen. $agent läuft mit dem persönlichen Abo von $holder. $agent muss zuerst auf Organisation umgestellt werden; die Kosten trägt dann der Schlüssel der Organisation.';
  }

  @override
  String adminGuidedNotReady(String agent) {
    return 'Der Adapter von $agent meldet noch keinen Schlüssel der Organisation. Aufrufe scheitern, bis er umgestellt ist.';
  }

  @override
  String adminGuidedGrant(String agent) {
    return '$agent auf Organisation umstellen und freigeben';
  }

  @override
  String adminGuidedContinue(String agent) {
    return '$agent auf Organisation umstellen und fortfahren';
  }

  @override
  String get adminGuidedTitle => 'Auf Organisation umstellen';

  @override
  String adminGuidedDone(String agent) {
    return '$agent läuft jetzt mit Organisation, und die Änderung ist gespeichert.';
  }

  @override
  String adminGuidedStillReports(String agent, String mode) {
    return 'Der Adapter von $agent meldet noch $mode.';
  }

  @override
  String get adminGuidedDoneClose => 'Schließen';

  @override
  String get adminPersonViewAs => 'Als diese Person ansehen';

  @override
  String adminViewAsRemaining(String time) {
    return 'noch $time';
  }

  @override
  String get adminViewAsEnd => 'Ansicht beenden';

  @override
  String adminViewAsSidebarOf(String name) {
    return 'Seitenleiste, wie $name sie sieht';
  }

  @override
  String adminViewAsNotVisible(String name, String names) {
    return 'Nicht sichtbar für $name: $names';
  }

  @override
  String get adminViewAsStructureOnly =>
      'Die Konsole zeigt nur Aufbau und Zugriff, nie den Inhalt eines Kanals.';

  @override
  String get adminViewAsStructureLater =>
      'Der Aufbau erscheint beim Start einer Ansicht. Beende diese Ansicht und starte sie neu, um ihn zu sehen.';

  @override
  String get consoleStepUpPasskeyNext =>
      'Als Nächstes fragt dein Browser nach deinem Passkey.';

  @override
  String get consoleStepUpOidcAction =>
      'Diese Änderung braucht eine neue Anmeldung bei deinem Anmeldedienst. Bestätige dort und mache die Änderung danach noch einmal.';

  @override
  String get terminalHeaderConnected => 'Verbunden';

  @override
  String get terminalHeaderConnecting => 'Verbindet';

  @override
  String get terminalHeaderOffline => 'Nicht verbunden';

  @override
  String terminalHeaderFingerprint(String value) {
    return 'Fp $value';
  }

  @override
  String terminalHeaderFingerprintLabel(String fingerprint) {
    return 'Fingerabdruck des Rechners: $fingerprint';
  }

  @override
  String terminalHeaderRemaining(String time) {
    return 'Noch $time';
  }

  @override
  String get terminalLlmLoginNone => 'Keine LLM-Anmeldung';

  @override
  String terminalTabLabel(String profile, int number) {
    return '$profile $number';
  }

  @override
  String get terminalDetach => 'Trennen';

  @override
  String get terminalEnd => 'Beenden';

  @override
  String get terminalReattach => 'Wiederanhängen';

  @override
  String get terminalStateConnecting =>
      'Die verschlüsselte Verbindung zum Rechner wird aufgebaut';

  @override
  String get terminalStateDetached =>
      'Getrennt. Die Terminal-Sitzung läuft auf dem Rechner weiter.';

  @override
  String get terminalStateDetachedLost =>
      'Die Verbindung ist abgebrochen. Die Terminal-Sitzung läuft auf dem Rechner weiter.';

  @override
  String get terminalStateDetachedHostUnavailable =>
      'Der Rechner ist nicht verbunden. Später wiederanhängen.';

  @override
  String get terminalStateDetachedTokenStale =>
      'Das Sitzungs-Token ist abgelaufen oder wurde schon benutzt. Wiederanhängen fragt erneut nach der Bestätigung.';

  @override
  String get terminalStateDetachedReplaced =>
      'Ein anderes Gerät hat sich an diese Terminal-Sitzung angehängt.';

  @override
  String get terminalStateDetachedServer =>
      'Der Server wurde neu gestartet. Die Terminal-Sitzung läuft auf dem Rechner weiter.';

  @override
  String get terminalStateDetachedSlow =>
      'Die Verbindung kam nicht hinterher. Die Terminal-Sitzung läuft auf dem Rechner weiter.';

  @override
  String get terminalStateDetachedAuth =>
      'Die Anmeldung der Verbindung ist gescheitert. Neu anmelden und dann wiederanhängen.';

  @override
  String get terminalStateDetachedRateLimited =>
      'Zu viele Versuche. Kurz warten und dann wiederanhängen.';

  @override
  String terminalStateExited(int code) {
    return 'Der Prozess wurde mit Code $code beendet.';
  }

  @override
  String terminalStateEnded(String reason) {
    return 'Die Terminal-Sitzung ist beendet: $reason';
  }

  @override
  String terminalStateBlockedRevoked(String reason) {
    return 'Zugriff beendet: $reason';
  }

  @override
  String get terminalStateBlockedHandshake =>
      'Die verschlüsselte Verbindung hat ihre Prüfung nicht bestanden. Jemand hat sie möglicherweise manipuliert. Es gibt keinen neuen Versuch.';

  @override
  String get terminalStateBlockedKeyMismatch =>
      'Der Server hat einen anderen Schlüssel des Rechners geschickt als den geprüften. Die Terminal-Sitzung wurde geschlossen.';

  @override
  String get terminalStateBlockedProtocol =>
      'Die Verbindung hat das Terminal-Protokoll verletzt. Es gibt keinen neuen Versuch.';

  @override
  String terminalStateFailed(String problem) {
    return 'Das Terminal konnte nicht geöffnet werden: $problem';
  }

  @override
  String terminalCloseReason(String reason) {
    String _temp0 = intl.Intl.selectLogic(reason, {
      'client_closed': 'von dir beendet',
      'closed_by_owner': 'von einem Eigentümer beendet',
      'process_exited': 'der Prozess wurde beendet',
      'idle_timeout': 'zu lange keine Eingabe',
      'max_duration': 'die Höchstdauer ist erreicht',
      'detach_expired': 'zu lange getrennt',
      'access_revoked': 'der Zugriff wurde entzogen',
      'i6_violation': 'die LLM-Anmeldung des Rechners hat sich geändert',
      'host_disabled': 'der Rechner wurde deaktiviert',
      'host_retired': 'der Rechner wurde stillgelegt',
      'host_rekeyed': 'der Rechner wurde neu gekoppelt',
      'host_restarted': 'der Host-Dienst wurde neu gestartet',
      'user_disabled': 'das Konto wurde deaktiviert',
      'device_revoked': 'dieses Gerät wurde entfernt',
      'session_revoked': 'die Anmeldung wurde beendet',
      'logout': 'du hast dich abgemeldet',
      'open_failed': 'das Öffnen ist gescheitert',
      'host_policy_violation': 'eine Prüfung auf dem Rechner ist gescheitert',
      'other': 'kein Grund angegeben',
    });
    return '$_temp0';
  }

  @override
  String get terminalFooterEncrypted => 'Ende-zu-Ende verschlüsselt';

  @override
  String terminalFooterProfile(String profile) {
    return 'Profil $profile';
  }

  @override
  String get terminalFooterIdle => 'Leerlauf-Zeitlimit aktiv';

  @override
  String get terminalOpenTitle => 'Terminal öffnen';

  @override
  String get terminalOpenHost => 'Rechner';

  @override
  String get terminalOpenProfile => 'Profil';

  @override
  String get terminalOpenWorkdir => 'Arbeitsordner';

  @override
  String get terminalOpenLogin => 'LLM-Anmeldung';

  @override
  String get terminalOpenLogged => 'Jedes Öffnen wird protokolliert.';

  @override
  String get terminalOpenConfirm =>
      'Mit Passkey oder Geräteschlüssel bestätigen';

  @override
  String get terminalCancel => 'Abbrechen';

  @override
  String get terminalKeyChangedTitle =>
      'Der Schlüssel des Rechners hat sich geändert';

  @override
  String get terminalKeyChangedStored => 'Gespeichert';

  @override
  String get terminalKeyChangedNow => 'Jetzt';

  @override
  String get terminalKeyChangedBody =>
      'Ohne neue Kopplung durch einen Eigentümer ist das ein Zeichen für einen Angriff. Verbinde dich nur, wenn du den neuen Fingerabdruck auf dem Rechner geprüft hast.';

  @override
  String get terminalKeyChangedReject => 'Nicht verbinden';

  @override
  String get terminalKeyChangedAccept => 'Neuen Schlüssel annehmen';

  @override
  String get terminalForeignTitle =>
      'Dieser Rechner gehört einer anderen Person';

  @override
  String terminalForeignBody(String host) {
    return '$host ist einer anderen Person zugewiesen. Deine Terminal-Sitzung arbeitet in deinem eigenen Sitzungsbenutzer, aber diese Person richtet den Rechner ein und könnte über eine lokale Rechteausweitung mitlesen. Tippe hier keine Geheimnisse, wenn du dem Rechner nicht vertraust.';
  }

  @override
  String get terminalForeignConfirm => 'Trotzdem verbinden';

  @override
  String get terminalLinkTitle => 'Link öffnen?';

  @override
  String get terminalLinkBody =>
      'Das Programm im Terminal verlinkt auf diese Adresse:';

  @override
  String get terminalLinkOpen => 'Link öffnen';

  @override
  String get terminalLinkUnsupported =>
      'Nur https- und http-Links werden geöffnet. Diese Adresse bleibt zu.';

  @override
  String terminalLinkHover(String target) {
    return 'Link: $target';
  }

  @override
  String get terminalPasteTitle => 'Mit Zeilenumbrüchen einfügen?';

  @override
  String terminalPasteBody(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count Zeilen',
      one: '1 Zeile',
    );
    return 'Der Text hat $_temp0. Jeder Zeilenumbruch kann einen Befehl starten.';
  }

  @override
  String get terminalPasteConfirm => 'Einfügen';

  @override
  String get terminalReadScreen => 'Bildschirm vorlesen';

  @override
  String terminalScreenLabel(String line) {
    return 'Terminal, Zeile am Cursor: $line';
  }

  @override
  String get terminalExtraKeys => 'Sondertasten-Leiste';

  @override
  String get terminalKeyEsc => 'Esc';

  @override
  String get terminalKeyTab => 'Tab';

  @override
  String get terminalKeyCtrl => 'Strg';

  @override
  String get terminalKeyAlt => 'Alt';

  @override
  String get terminalKeyUp => '↑';

  @override
  String get terminalKeyDown => '↓';

  @override
  String get terminalKeyLeft => '←';

  @override
  String get terminalKeyRight => '→';

  @override
  String get terminalKeyPipe => '|';

  @override
  String get terminalKeyTilde => '~';

  @override
  String get terminalKeySlash => '/';

  @override
  String get terminalKeyMinus => '-';

  @override
  String get terminalKeyEscName => 'Escape';

  @override
  String get terminalKeyTabName => 'Tabulator';

  @override
  String get terminalKeyCtrlName => 'Steuerung';

  @override
  String get terminalKeyAltName => 'Alt';

  @override
  String get terminalKeyUpName => 'Pfeil nach oben';

  @override
  String get terminalKeyDownName => 'Pfeil nach unten';

  @override
  String get terminalKeyLeftName => 'Pfeil nach links';

  @override
  String get terminalKeyRightName => 'Pfeil nach rechts';

  @override
  String get terminalKeyPipeName => 'Senkrechter Strich';

  @override
  String get terminalKeyTildeName => 'Tilde';

  @override
  String get terminalKeySlashName => 'Schrägstrich';

  @override
  String get terminalKeyMinusName => 'Minus';

  @override
  String terminalKeyLatched(String key) {
    return '$key, eingerastet';
  }

  @override
  String terminalKeyHeld(String key) {
    return '$key, gehalten';
  }

  @override
  String terminalStepUpOpen(String name) {
    return 'Terminal auf $name öffnen';
  }

  @override
  String terminalStepUpAttach(String name) {
    return 'Terminal auf $name wiederanhängen';
  }

  @override
  String terminalLinkHost(String name) {
    return 'Rechner: $name';
  }

  @override
  String fileStateUploading(int percent) {
    return 'Wird hochgeladen, $percent %';
  }

  @override
  String get fileStateWaiting => 'Wartet auf das Hochladen';

  @override
  String get fileStateChecking => 'Wird geprüft';

  @override
  String get fileStateUploadFailed => 'Hochladen fehlgeschlagen';

  @override
  String get fileStateBlocked => 'Blockiert';

  @override
  String get fileStateCanceled => 'Hochladen abgebrochen';

  @override
  String get fileDeleted => 'Datei gelöscht';

  @override
  String fileSizeBytes(String size) {
    return '$size B';
  }

  @override
  String fileSizeKilobytes(String size) {
    return '$size KB';
  }

  @override
  String fileSizeMegabytes(String size) {
    return '$size MB';
  }

  @override
  String fileSizeGigabytes(String size) {
    return '$size GB';
  }

  @override
  String fileTileLabel(String name, String size, String state) {
    return '$name, $size, $state';
  }

  @override
  String get fileOpen => 'Öffnen';

  @override
  String get fileSaveAs => 'Speichern unter';

  @override
  String fileSaved(String name) {
    return '$name gespeichert';
  }

  @override
  String fileDownloadFailed(String reason) {
    return 'Herunterladen fehlgeschlagen: $reason';
  }

  @override
  String get fileDownloadDamaged =>
      'Der Download war beschädigt und wurde gelöscht. Versuche es erneut.';

  @override
  String fileImageOpen(String name) {
    return 'Bild $name öffnen';
  }

  @override
  String get fileImageClose => 'Schließen';

  @override
  String get fileImageLoading => 'Bild wird geladen';

  @override
  String fileMediaPlay(String name) {
    return '$name abspielen';
  }

  @override
  String get fileMediaPause => 'Anhalten';

  @override
  String get fileMediaFailed => 'Abspielen fehlgeschlagen';

  @override
  String get fileUploadAgain => 'Erneut hochladen';

  @override
  String fileRemove(String name) {
    return '$name entfernen';
  }

  @override
  String get fileFailPartConflict =>
      'Ein Teil wurde zweimal mit anderem Inhalt gesendet';

  @override
  String get fileFailSessionEnded => 'Die Upload-Sitzung ist beendet';

  @override
  String get fileFailQuota => 'Das Speicherkontingent ist aufgebraucht';

  @override
  String get fileFailNotAttachable => 'Diese Datei kann nicht angehängt werden';

  @override
  String get fileFailUnreadable => 'Die Datei kann nicht mehr gelesen werden';

  @override
  String composerFileTooLarge(String name, String limit) {
    return '$name ist größer als die Grenze von $limit';
  }

  @override
  String composerTooManyFiles(int count) {
    return 'Höchstens $count Dateien pro Nachricht';
  }

  @override
  String composerFileUnreadable(String name) {
    return '$name kann nicht gelesen werden';
  }

  @override
  String get composerDropFiles => 'Dateien hier ablegen, um sie anzuhängen';

  @override
  String composerAttachments(int count) {
    return 'Anhänge ($count)';
  }

  @override
  String voiceRecording(String elapsed) {
    return 'Aufnahme $elapsed';
  }

  @override
  String voiceRecorded(String elapsed) {
    return 'Sprachnachricht $elapsed';
  }

  @override
  String get voiceSend => 'Sprachnachricht senden';

  @override
  String get voiceStop => 'Aufnahme beenden';

  @override
  String get voiceCancel => 'Aufnahme verwerfen';

  @override
  String get voiceMicrophoneDenied =>
      'Der Zugriff auf das Mikrofon ist aus. Erlaube ihn in den Systemeinstellungen unter Datenschutz und Sicherheit, Mikrofon.';

  @override
  String voiceLimitReached(String limit) {
    return 'Die Aufnahme endete an der Grenze von $limit';
  }

  @override
  String get voiceFailed => 'Die Aufnahme ist fehlgeschlagen';

  @override
  String voiceSpeed(String speed) {
    return 'Abspielgeschwindigkeit $speed';
  }

  @override
  String voiceSpeedValue(String speed) {
    return '${speed}x';
  }

  @override
  String get transcriptLabel => 'Transkript';

  @override
  String get transcriptFailed => 'Kein Transkript';

  @override
  String get transcriptRequest => 'Transkribieren';

  @override
  String get transcriptUnavailable => 'Transkription ist hier nicht verfügbar';

  @override
  String sessionFiles(int count) {
    return 'Dateien ($count)';
  }

  @override
  String get sessionFilesEmpty => 'Keine Dateien';

  @override
  String fileVideoTitle(String name) {
    return 'Video $name';
  }

  @override
  String get adminNavApps => 'Apps';

  @override
  String get adminNavIncomingWebhooks => 'Eingehende Webhooks';

  @override
  String get adminAppListTitle => 'Apps';

  @override
  String get adminAppInstall => 'App installieren';

  @override
  String get adminAppInstallNoPicker =>
      'Zum Installieren braucht es einen Dateidialog; nutze die Web-Konsole.';

  @override
  String get adminAppListEmpty => 'Keine Apps installiert.';

  @override
  String adminAppVersion(String version) {
    return 'Version $version';
  }

  @override
  String adminAppPublisher(String publisher) {
    return 'von $publisher';
  }

  @override
  String get adminAppBack => 'Zurück zu den Apps';

  @override
  String get adminAppStateActive => 'Aktiv';

  @override
  String get adminAppStateSuspended => 'Angehalten';

  @override
  String get adminAppStateUninstalled => 'Deinstalliert';

  @override
  String get adminAppStateUnknown => 'Unbekannter Zustand';

  @override
  String get adminAppScopeMessagesRead =>
      'Nachrichten und Threads lesen und darin suchen';

  @override
  String get adminAppScopeMessagesWrite =>
      'Nachrichten schreiben, ändern und löschen, reagieren und Threads beginnen';

  @override
  String get adminAppScopeFilesRead => 'Dateien lesen';

  @override
  String get adminAppScopeFilesWrite => 'Dateien hochladen';

  @override
  String get adminAppScopeCommandsWrite =>
      'Befehle anlegen und entfernen und ihre Vorschläge beantworten';

  @override
  String get adminAppScopeComponentsWrite =>
      'Eigene Schaltflächen und Formulare ändern und auf Interaktionen antworten';

  @override
  String get adminAppScopeUsersRead => 'Profile von Personen lesen';

  @override
  String get adminAppScopeEventsSubscribe =>
      'Ereignisse empfangen und verpasste nachholen';

  @override
  String get adminAppScopeCostsReconcile =>
      'Kostenabgleiche lesen und einreichen';

  @override
  String get adminAppScopeUnknown => 'Keine Beschreibung in dieser Version';

  @override
  String get adminAppScopeRequired => 'erforderlich';

  @override
  String get adminAppScopeOptional => 'optional';

  @override
  String get adminAppFileNotText => 'Die Datei ist kein Text (UTF-8).';

  @override
  String get adminAppFileNotJsonOrYaml => 'Die Datei ist weder JSON noch YAML.';

  @override
  String get adminAppFileNotObject => 'Die Datei enthält kein Manifest-Objekt.';

  @override
  String get adminAppUpdate => 'Aktualisieren';

  @override
  String get adminAppSuspend => 'Anhalten';

  @override
  String get adminAppResume => 'Fortsetzen';

  @override
  String get adminAppUninstall => 'Deinstallieren';

  @override
  String adminAppSuspendTitle(String name) {
    return '$name anhalten?';
  }

  @override
  String get adminAppSuspendBody =>
      'Ein Nothalt: nichts wird gelöscht, und das Fortsetzen braucht keine neue Zustimmung.';

  @override
  String adminAppResumeTitle(String name) {
    return '$name fortsetzen?';
  }

  @override
  String get adminAppResumeBody =>
      'Das kommt zurück, wenn die App fortgesetzt wird:';

  @override
  String adminAppUninstallTitle(String name) {
    return '$name deinstallieren?';
  }

  @override
  String get adminAppUninstallBody =>
      'Das lässt sich nicht rückgängig machen; eine neue Installation braucht eine neue Zustimmung.';

  @override
  String get adminAppEffectsPart => 'Teil';

  @override
  String get adminAppEffectsResult => 'Wirkung';

  @override
  String get adminAppEffectRowTokens => 'Tokens';

  @override
  String get adminAppEffectRowDeliveries => 'Zustellungen';

  @override
  String get adminAppEffectRowCommands => 'Befehle';

  @override
  String get adminAppEffectRowComponents => 'Komponenten';

  @override
  String get adminAppEffectRowGrants => 'Freigaben und eingehende Webhooks';

  @override
  String get adminAppEffectRowMessages => 'Nachrichten der App';

  @override
  String get adminAppEffectBlocked => 'Gesperrt, nicht widerrufen';

  @override
  String get adminAppEffectPaused => 'Pausiert, die Position bleibt erhalten';

  @override
  String get adminAppEffectHidden => 'Ausgeblendet';

  @override
  String get adminAppEffectDisabled => 'Deaktiviert';

  @override
  String get adminAppEffectSuspendGrants =>
      'Bleiben; eingehende Webhooks antworten mit 404, bis die App fortgesetzt wird';

  @override
  String get adminAppEffectRemain => 'Bleiben';

  @override
  String get adminAppEffectRevoked => 'Widerrufen';

  @override
  String get adminAppEffectSubscriptionsDisabled =>
      'Abonnements werden deaktiviert';

  @override
  String get adminAppEffectRemoved => 'Entfernt';

  @override
  String get adminAppEffectUninstallMessages =>
      'Bleiben; der Autor bleibt erhalten, die App wird deaktiviert';

  @override
  String get adminAppEffectWorkAgain => 'Funktionieren wieder';

  @override
  String get adminAppEffectDeliveriesContinue =>
      'Gehen ab der gespeicherten Position weiter';

  @override
  String get adminAppEffectShownAgain => 'Wieder sichtbar';

  @override
  String get adminAppEffectEnabledAgain => 'Wieder aktiv';

  @override
  String get adminAppEffectResumeGrants =>
      'Unverändert; eingehende Webhooks nehmen wieder Aufrufe an';

  @override
  String get adminAppEffectUnchanged => 'Unverändert';

  @override
  String adminAppSuspended(String name) {
    return '$name ist angehalten.';
  }

  @override
  String adminAppResumed(String name) {
    return '$name ist wieder aktiv.';
  }

  @override
  String adminAppUninstalled(String name) {
    return '$name ist deinstalliert.';
  }

  @override
  String adminAppUpdated(String name, String version) {
    return '$name hat jetzt Version $version.';
  }

  @override
  String adminAppConsentSaved(String name) {
    return 'Die neue Zustimmung für $name ist gespeichert.';
  }

  @override
  String adminAppPurposeInstall(String name) {
    return 'Die App $name installieren';
  }

  @override
  String adminAppPurposeUpdate(String name) {
    return 'Die App $name aktualisieren';
  }

  @override
  String adminAppPurposeScopes(String name) {
    return 'Die Zustimmung für $name ändern';
  }

  @override
  String adminAppPurposeSuspend(String name) {
    return 'Die App $name anhalten';
  }

  @override
  String adminAppPurposeResume(String name) {
    return 'Die App $name fortsetzen';
  }

  @override
  String adminAppPurposeUninstall(String name) {
    return 'Die App $name deinstallieren';
  }

  @override
  String adminAppPurposeGrant(String name, String target) {
    return 'Die Freigabe für $name auf $target anlegen';
  }

  @override
  String adminAppPurposeToken(String name) {
    return 'Ein Token für $name anlegen';
  }

  @override
  String get adminAppInstallTitle => 'Eine App installieren';

  @override
  String get adminAppInstallPickBody =>
      'Wähle die Manifest-Datei der App (JSON oder YAML). Der Server prüft sie, bevor etwas installiert wird.';

  @override
  String get adminAppPick => 'Datei wählen';

  @override
  String get adminAppChecking => 'Das Manifest wird geprüft';

  @override
  String adminAppFileChosen(String name) {
    return 'Datei: $name';
  }

  @override
  String get adminAppInvalid => 'Der Server hat das Manifest abgelehnt:';

  @override
  String adminAppErrorLine(String pointer, String rule) {
    return '$pointer: Regel „$rule“';
  }

  @override
  String get adminAppManifestRoot => 'das ganze Manifest';

  @override
  String adminAppConsentIntro(String name, String publisher, String version) {
    return '$name von $publisher, Version $version, bittet um Folgendes. Lies es, bevor du zustimmst.';
  }

  @override
  String get adminAppConsentScopes => 'Was die App lesen und schreiben darf';

  @override
  String get adminAppConsentEvents => 'Ereignisse, die die App empfängt';

  @override
  String get adminAppConsentDestinations => 'Wohin Daten gesendet werden';

  @override
  String get adminAppConsentCommands => 'Befehle';

  @override
  String get adminAppConsentComponents => 'Komponenten';

  @override
  String get adminAppConsentGrants => 'Freigaben, um die die App bittet';

  @override
  String get adminAppConsentEgress =>
      'Daten, die die Instanz verlassen, in den Worten des Herausgebers';

  @override
  String get adminAppConsentNoEgress =>
      'Das Manifest nennt keine Daten, die die Instanz verlassen.';

  @override
  String get adminAppConsentNone => 'Keine.';

  @override
  String get adminAppComponentButton => 'Schaltflächen';

  @override
  String get adminAppComponentSelect => 'Auswahllisten';

  @override
  String get adminAppComponentForm => 'Formulare';

  @override
  String adminAppGrantAskChannel(String level) {
    return '$level auf einem Kanal';
  }

  @override
  String adminAppGrantAskSpace(String level) {
    return '$level in einem Bereich';
  }

  @override
  String adminAppGrantAskInstance(String level) {
    return '$level auf der ganzen Instanz';
  }

  @override
  String adminAppGrantAskOther(String level) {
    return '$level auf einem Ziel, das diese Version nicht kennt';
  }

  @override
  String adminAppGrantPurpose(String purpose) {
    return 'Zweck: $purpose';
  }

  @override
  String get adminAppGrantTarget => 'Ziel';

  @override
  String get adminAppGrantNone => 'Nicht anlegen';

  @override
  String get adminAppConsentToken => 'Token der App';

  @override
  String get adminAppTokenBody =>
      'Die App meldet sich mit diesem Token an. Sein Geheimnis wird nur einmal angezeigt.';

  @override
  String get adminAppTokenDpop => 'DPoP-Schlüssel-Thumbprint (optional)';

  @override
  String get adminAppInstallSubmit => 'Installieren';

  @override
  String get adminAppGrantsStepTitle => 'Freigaben und Token';

  @override
  String adminAppGrantsStepBody(String name) {
    return '$name ist installiert. Prüfe, was jede Freigabe ändert, und lege dann die Freigaben und das Token an.';
  }

  @override
  String get adminAppNoGrants =>
      'Keine Freigabe gewählt; nur das Token wird angelegt.';

  @override
  String get adminAppCreateGrants => 'Freigaben und Token anlegen';

  @override
  String adminAppStepInstall(String name) {
    return '$name installieren';
  }

  @override
  String adminAppStepGrant(String target) {
    return 'Freigabe auf $target';
  }

  @override
  String adminAppStepToken(String name) {
    return 'Token $name';
  }

  @override
  String adminAppStepLine(String step, String status) {
    return '$step: $status';
  }

  @override
  String get adminAppStepFinished => 'abgeschlossen';

  @override
  String adminAppStepFailed(String reason) {
    return 'fehlgeschlagen: $reason';
  }

  @override
  String get adminAppStepWaiting => 'wartet';

  @override
  String adminAppTokenSecretTitle(String name) {
    return 'Token von $name';
  }

  @override
  String get adminAppTokenSecretOnce =>
      'Dieses Geheimnis wird nur einmal angezeigt. Kopiere es jetzt in den Geheimnisspeicher der App.';

  @override
  String adminAppKeyId(String kid) {
    return 'Schlüssel-ID der Webhook-Signaturen: $kid';
  }

  @override
  String adminAppHomepage(String url) {
    return 'Website: $url';
  }

  @override
  String get adminAppPageEvents => 'Abonnierte Ereignisse';

  @override
  String get adminAppTokenNew => 'Neues Token';

  @override
  String adminAppTokenCreateTitle(String name) {
    return 'Ein Token für $name anlegen';
  }

  @override
  String get adminAppTokenRevokeBody =>
      'Die App verliert den Zugang mit diesem Token sofort.';

  @override
  String adminAppUpdateTitle(String name) {
    return '$name aktualisieren';
  }

  @override
  String adminAppUpdateBody(String slug) {
    return 'Wähle die neue Manifest-Datei mit demselben Slug ($slug).';
  }

  @override
  String adminAppDiffTitle(String from, String to) {
    return 'Änderungen von Version $from zu $to';
  }

  @override
  String get adminAppDiffScopes => 'Was die App lesen und schreiben darf';

  @override
  String get adminAppDiffEvents => 'Ereignisse';

  @override
  String get adminAppDiffGrants => 'Erbetene Freigaben';

  @override
  String adminAppDiffAdded(String items) {
    return 'Neu: $items';
  }

  @override
  String adminAppDiffRemoved(String items) {
    return 'Entfernt: $items';
  }

  @override
  String get adminAppDiffNone =>
      'Bei Berechtigungen, Ereignissen, Zielen, Freigaben und Befehlen ändert sich nichts.';

  @override
  String get adminAppDiffRemovedNow => 'Entfernte Punkte wirken sofort.';

  @override
  String get adminAppDiffHostsConfirmed =>
      'Neue Ziele bestätigst du mit diesem Update.';

  @override
  String get adminAppDiffGrantsLater =>
      'Ein Update legt nie eine Freigabe an: lege neue unter Freigaben an.';

  @override
  String get adminAppDiffNewConsent =>
      'Neue Berechtigungen und Ereignisse brauchen nach dem Update eine neue Zustimmung; bis dahin behält die App die bisherigen.';

  @override
  String adminAppNewConsentTitle(String name) {
    return 'Neue Zustimmung für $name';
  }

  @override
  String adminAppNewConsentBody(String version) {
    return 'Version $version bittet um mehr. Wähle, was die App nutzen darf; erforderliche Punkte bleiben an.';
  }

  @override
  String get adminAppNewConsentSubmit => 'Zustimmung speichern';

  @override
  String get adminAppNewConsentLater => 'Später';

  @override
  String get adminHookTitle => 'Ausgehende Webhooks';

  @override
  String get adminHookEmpty => 'Keine Abonnements.';

  @override
  String get adminHookCreate => 'Neues Abonnement';

  @override
  String get adminHookStateActive => 'Aktiv';

  @override
  String get adminHookStatePaused => 'Pausiert';

  @override
  String get adminHookStateDisabled => 'Deaktiviert';

  @override
  String get adminHookStateUnknown => 'Unbekannter Zustand';

  @override
  String adminHookEvents(String events) {
    return 'Ereignisse: $events';
  }

  @override
  String adminHookFailures(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count Fehlschläge in Folge',
      one: '1 Fehlschlag in Folge',
      zero: 'Keine Fehlschläge in Folge',
    );
    return '$_temp0';
  }

  @override
  String adminHookSigned(String schemes, String kid) {
    return 'Signiert mit $schemes, Schlüssel $kid';
  }

  @override
  String adminHookDisabledAt(String when) {
    return 'Deaktiviert $when';
  }

  @override
  String get adminHookReasonGap =>
      'Eine Lücke: Ereignisse haben das Ereignisprotokoll verlassen, bevor sie zugestellt wurden.';

  @override
  String get adminHookReasonGone =>
      'Der Empfänger hat mit 410 Gone geantwortet.';

  @override
  String adminHookReasonFailures(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: 'Deaktiviert nach $count Fehlschlägen in Folge.',
      one: 'Deaktiviert nach 1 Fehlschlag in Folge.',
    );
    return '$_temp0';
  }

  @override
  String get adminHookReasonNone => 'Der Server nennt keinen Grund.';

  @override
  String get adminHookPause => 'Pausieren';

  @override
  String get adminHookResume => 'Fortsetzen';

  @override
  String get adminHookDelete => 'Löschen';

  @override
  String get adminHookDeliveries => 'Zustellungen';

  @override
  String adminHookPauseTitle(String url) {
    return 'Abonnement an $url pausieren?';
  }

  @override
  String get adminHookPauseBody =>
      'An diese Adresse geht nichts, bis du fortsetzt. Passende Ereignisse werden weiter vorgemerkt und gehen danach der Reihe nach hinaus.';

  @override
  String adminHookResumeTitle(String url) {
    return 'Abonnement an $url fortsetzen?';
  }

  @override
  String get adminHookResumeBody =>
      'Wartende Zustellungen gehen wieder der Reihe nach an diese Adresse.';

  @override
  String adminHookDeleteTitle(String url) {
    return 'Abonnement an $url löschen?';
  }

  @override
  String get adminHookDeleteBody =>
      'An diese Adresse gehen keine weiteren Ereignisse.';

  @override
  String adminHookPausedNotice(String url) {
    return 'Das Abonnement an $url ist pausiert.';
  }

  @override
  String adminHookResumedNotice(String url) {
    return 'Das Abonnement an $url ist aktiv.';
  }

  @override
  String adminHookDeletedNotice(String url) {
    return 'Das Abonnement an $url ist gelöscht.';
  }

  @override
  String adminHookCreatedNotice(String url) {
    return 'Das Abonnement an $url ist angelegt.';
  }

  @override
  String adminHookFormTitle(String app) {
    return 'Neues Abonnement für $app';
  }

  @override
  String get adminHookFormBody =>
      'Ereignisse der gewählten Typen gehen an diese Adresse, signiert mit v1a. Der Server nimmt nur https-Adressen an, die seine Webhook-Einstellungen erlauben.';

  @override
  String get adminHookUrl => 'Adresse';

  @override
  String get adminHookUrlHttps => 'Die Adresse muss mit https:// beginnen.';

  @override
  String get adminHookEventTypes => 'Ereignistypen';

  @override
  String get adminHookNoEvents =>
      'Die Installation gewährt keine Ereignisse, die sich abonnieren lassen.';

  @override
  String get adminHookHmac => 'Zusätzlich mit HMAC signieren (v1)';

  @override
  String get adminHookSubmit => 'Abonnement anlegen';

  @override
  String adminHookPurposeCreate(String app) {
    return 'Webhook-Abonnement für $app anlegen';
  }

  @override
  String adminHookPurposeChange(String url) {
    return 'Webhook-Abonnement an $url ändern';
  }

  @override
  String adminHookPurposeDelete(String url) {
    return 'Webhook-Abonnement an $url löschen';
  }

  @override
  String get adminHookPurposeRedeliver => 'Ereignis erneut zustellen';

  @override
  String adminHookDeliveriesTitle(String url) {
    return 'Zustellungen an $url';
  }

  @override
  String get adminHookDeliveriesEmpty => 'Noch keine Zustellungen.';

  @override
  String get adminHookReload => 'Neu laden';

  @override
  String get adminHookMore => 'Mehr laden';

  @override
  String get adminHookDeliveryPending => 'Wartet';

  @override
  String get adminHookDeliverySucceeded => 'Zugestellt';

  @override
  String get adminHookDeliveryFailed => 'Fehlgeschlagen';

  @override
  String get adminHookDeliveryDead => 'Aufgegeben';

  @override
  String get adminHookDeliveryUnknown => 'Unbekannter Status';

  @override
  String adminHookAttempts(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count Versuche',
      one: '1 Versuch',
    );
    return '$_temp0';
  }

  @override
  String adminHookNextAttempt(String when) {
    return 'nächster Versuch $when';
  }

  @override
  String adminHookLastStatus(String code) {
    return 'letzter Status $code';
  }

  @override
  String adminHookLastError(String error) {
    return 'letzter Fehler: $error';
  }

  @override
  String adminHookEventSeq(String type, String seq) {
    return '$type, Sequenz $seq';
  }

  @override
  String adminHookEventGone(String seq) {
    return 'Ereignis nicht mehr im Protokoll, Sequenz $seq';
  }

  @override
  String adminHookCreatedAt(String when) {
    return 'angelegt $when';
  }

  @override
  String adminHookDeliveredAt(String when) {
    return 'zugestellt $when';
  }

  @override
  String get adminHookErrorTimeout => 'Zeitüberschreitung';

  @override
  String get adminHookErrorConnectionFailed => 'Verbindung fehlgeschlagen';

  @override
  String get adminHookErrorTlsFailed =>
      'TLS-Aufbau oder Zertifikat fehlgeschlagen';

  @override
  String get adminHookErrorAddressBlocked =>
      'die Adresse führt in ein gesperrtes Netz';

  @override
  String get adminHookErrorHostNotAllowed =>
      'die Zieladresse ist nicht erlaubt';

  @override
  String get adminHookErrorHttpStatus => 'unerwarteter HTTP-Status';

  @override
  String get adminHookErrorThrottled => 'vom Empfänger gebremst';

  @override
  String get adminHookErrorGone => 'der Empfänger hat mit 410 Gone geantwortet';

  @override
  String get adminHookErrorNotAuthorized =>
      'die App darf dieses Ereignis nicht mehr empfangen';

  @override
  String get adminHookErrorNotSubscribed =>
      'das Abonnement umfasst diesen Ereignistyp nicht mehr';

  @override
  String get adminHookErrorEventExpired =>
      'das Ereignis ist nicht mehr im Ereignisprotokoll';

  @override
  String get adminHookRedeliver => 'Erneut zustellen';

  @override
  String get adminHookRedelivered => 'Die Zustellung ist wieder fällig.';

  @override
  String get adminHookRedeliverExpired =>
      'Das Ereignis ist nicht mehr im Ereignisprotokoll; es lässt sich nicht erneut zustellen.';

  @override
  String get adminHookKeysTitle => 'Signaturschlüssel';

  @override
  String get adminHookKeysBody =>
      'Empfänger prüfen die v1a-Signatur jeder Zustellung mit einem dieser öffentlichen Schlüssel.';

  @override
  String get adminHookKeyActive => 'Aktiv';

  @override
  String get adminHookKeyNext => 'Nächster';

  @override
  String get adminHookKeyRetiring => 'Läuft aus';

  @override
  String adminHookKeyUntil(String when) {
    return 'gültig bis $when';
  }

  @override
  String adminSecretHmacTitle(String url) {
    return 'HMAC-Geheimnis für $url';
  }

  @override
  String get adminSecretHmacLabel => 'HMAC-Geheimnis (v1)';

  @override
  String get adminSecretHmacOnce =>
      'Dieses Geheimnis wird nur jetzt angezeigt. Hinterlege es beim Empfänger, der damit die v1-Signatur prüft.';

  @override
  String get adminManifestFileType => 'App-Manifest';

  @override
  String get adminIncomingTitle => 'Eingehende Webhooks';

  @override
  String get adminIncomingChannel => 'Kanal';

  @override
  String get adminIncomingNoChannels => 'Du siehst keinen Kanal.';

  @override
  String get adminIncomingEmpty =>
      'Dieser Kanal hat keine eingehenden Webhooks.';

  @override
  String get adminIncomingCreate => 'Eingehenden Webhook anlegen';

  @override
  String adminIncomingWritesAs(String app) {
    return 'schreibt als $app';
  }

  @override
  String adminIncomingCreatedAt(String when) {
    return 'angelegt $when';
  }

  @override
  String adminIncomingLastUsed(String when) {
    return 'zuletzt benutzt $when';
  }

  @override
  String get adminIncomingNeverUsed => 'noch nie benutzt';

  @override
  String get adminIncomingRevoke => 'Widerrufen';

  @override
  String adminIncomingRevokeTitle(String name) {
    return 'Eingehenden Webhook $name widerrufen?';
  }

  @override
  String get adminIncomingRevokeBody =>
      'Aufrufe seiner Adresse werden ab jetzt mit 404 beantwortet.';

  @override
  String adminIncomingRevokedNotice(String name) {
    return 'Der eingehende Webhook $name ist widerrufen.';
  }

  @override
  String adminIncomingCreatedNotice(String name) {
    return 'Der eingehende Webhook $name ist angelegt.';
  }

  @override
  String adminIncomingDialogTitle(String path) {
    return 'Neuer eingehender Webhook in $path';
  }

  @override
  String get adminIncomingDialogBody =>
      'Ein Dienst schreibt über die Adresse des Webhooks Nachrichten in diesen Kanal. Der Webhook schreibt als App.';

  @override
  String get adminIncomingName => 'Name';

  @override
  String get adminIncomingModeExisting => 'Als bestehende App schreiben';

  @override
  String get adminIncomingModeNewApp => 'Neue Eingangs-App';

  @override
  String get adminIncomingNewAppBody =>
      'Installiert eine minimale App, die nur Nachrichten schreiben darf, mit Schreibzugriff auf genau diesen Kanal.';

  @override
  String get adminIncomingApp => 'App';

  @override
  String get adminIncomingNoApps => 'Es ist keine aktive App installiert.';

  @override
  String get adminIncomingSubmit => 'Anlegen';

  @override
  String get adminIncomingStepInstall => 'Eingangs-App installieren';

  @override
  String adminIncomingStepGrant(String path) {
    return 'Schreibzugriff auf $path gewähren';
  }

  @override
  String get adminIncomingStepWebhook => 'Webhook anlegen';

  @override
  String adminIncomingStepLine(String step, String state) {
    return '$step: $state';
  }

  @override
  String get adminIncomingStepDone => 'erledigt';

  @override
  String get adminIncomingStepWaiting => 'wartet';

  @override
  String get adminIncomingRetry => 'Erneut versuchen';

  @override
  String get adminIncomingUninstall => 'Eingangs-App deinstallieren';

  @override
  String get adminIncomingUninstalled => 'Die Eingangs-App ist deinstalliert.';

  @override
  String adminIncomingPurposeCreate(String path) {
    return 'Eingehenden Webhook in $path anlegen';
  }

  @override
  String adminIncomingPurposeRevoke(String name) {
    return 'Eingehenden Webhook $name widerrufen';
  }

  @override
  String adminIncomingPurposeUninstall(String name) {
    return 'Eingangs-App $name deinstallieren';
  }

  @override
  String adminSecretIncomingTitle(String name) {
    return 'Adresse von $name';
  }

  @override
  String get adminSecretIncomingLabel => 'Webhook-Adresse mit Token';

  @override
  String get adminSecretIncomingOnce =>
      'Diese Adresse enthält das Token und wird nur jetzt angezeigt. Kopiere sie in den sendenden Dienst.';

  @override
  String get adminSecretIncomingBearer =>
      'Sender, die einen Header setzen können, lassen den letzten Abschnitt der Adresse weg und senden ihn als Authorization: Bearer <token>.';

  @override
  String get adminNavHints => 'Rechtehinweise';

  @override
  String get adminNavExpiring => 'Läuft bald ab';

  @override
  String get hintsTitle => 'Rechtehinweise';

  @override
  String get hintsIntro =>
      'Hinweise aus den aktuellen Rechten: was Personen über Agenten erfahren können und welche Freigaben nicht wirken. Ein Hinweis ist keine Sperre.';

  @override
  String get hintsScopeLabel => 'Anzeigen für';

  @override
  String get hintsScopeInstance => 'Ganze Instanz';

  @override
  String get hintsKindLabel => 'Art';

  @override
  String get hintsKindAll => 'Alle Arten';

  @override
  String get hintsKindH1 => 'Lesehorizont (H1)';

  @override
  String get hintsKindGrantToDisabled =>
      'Freigabe an ein deaktiviertes oder gelöschtes Konto';

  @override
  String get hintsKindTriggerBlocked => 'Agenten auslösen ohne Wirkung';

  @override
  String get hintsKindAgentPersonal => 'Agent im Modus Persönlich';

  @override
  String get hintsKindUnknown => 'Anderer Hinweis';

  @override
  String hintsH1Text(String person, String channelPath, String agent) {
    return '$person kann über Agent $agent Inhalte aus $channelPath erfahren.';
  }

  @override
  String hintsGrantToDisabledText(String subject, String target, String level) {
    return '$subject ist deaktiviert oder gelöscht und hat noch eine Freigabe auf $target: $level.';
  }

  @override
  String hintsTriggerBlockedText(String subject, String target) {
    return '$subject hat die Stufe Agenten auslösen auf $target, kann dort aber keinen Agenten auslösen.';
  }

  @override
  String get hintsRuleSr2 =>
      'SR2: Ein Agent dort hat eine Werkzeugklasse, die nur Owner und Personen mit einer eigenen bereiten Entwicklungsumgebung auslösen dürfen.';

  @override
  String get hintsRuleSr3 =>
      'SR3: Ein Kanal dort ist an ein Projekt gebunden; ein Lauf einer Person, die kein Owner ist, braucht eine eigene bereite Entwicklungsumgebung.';

  @override
  String hintsAgentPersonalText(String agent, String holder) {
    return 'Agent $agent nutzt den Modus Persönlich mit dem Abo von $holder.';
  }

  @override
  String hintsAgentPersonalNoHolder(String agent) {
    return 'Agent $agent nutzt den Modus Persönlich und hat keinen Inhaber.';
  }

  @override
  String hintsAgentPersonalSet(String people) {
    return 'I1 verlangt, dass nur der Inhaber ihn auslösen kann. Jetzt können ihn auslösen: $people.';
  }

  @override
  String get hintsAgentPersonalNobody =>
      'I1 verlangt, dass nur der Inhaber ihn auslösen kann. Jetzt kann ihn niemand auslösen.';

  @override
  String get hintsUnknownText =>
      'Diese Version kennt diesen Hinweis noch nicht.';

  @override
  String get hintsEmpty => 'Keine Rechtehinweise.';

  @override
  String get hintsExplain => 'Erklären';

  @override
  String get hintsExtend => 'Verlängern';

  @override
  String get hintsOpenAgent => 'Agent öffnen';

  @override
  String get hintsClose => 'Schließen';

  @override
  String hintsExtendTitle(String name) {
    return 'Freigabe für $name verlängern';
  }

  @override
  String get hintsExtendNote =>
      'Verlängern legt eine neue Freigabe mit neuem Grund und dem eingegebenen letzten Tag an und widerruft danach die alte.';

  @override
  String hintsExtended(String name) {
    return 'Die Freigabe für $name wurde verlängert: Die neue gilt, die alte ist widerrufen.';
  }

  @override
  String hintsExtendRevokeFailed(String name, String reason) {
    return 'Die neue Freigabe für $name gilt, aber die alte konnte nicht widerrufen werden: $reason';
  }

  @override
  String get expiringTitle => 'Läuft bald ab';

  @override
  String get expiringIntro =>
      'Freigaben, Gruppenmitgliedschaften und Token von Agenten und Apps, die bald enden. Verlängern ersetzt eine Freigabe durch eine neue; ein Token erneuerst du, indem du ein neues anlegst und das alte widerrufst.';

  @override
  String get expiringEmpty => 'Nichts läuft bald ab.';

  @override
  String get expiringKindGrant => 'Freigabe';

  @override
  String get expiringKindMembership => 'Gruppenmitgliedschaft';

  @override
  String get expiringKindToken => 'Token';

  @override
  String get expiringKindUnknown => 'Anderer Zugriff';

  @override
  String expiringGrantText(String subject, String target, String level) {
    return '$subject auf $target: $level';
  }

  @override
  String expiringMembershipText(String person, String group) {
    return '$person in der Gruppe $group';
  }

  @override
  String expiringAgentTokenText(String agent) {
    return 'Token des Agenten $agent';
  }

  @override
  String expiringAppTokenText(String app) {
    return 'Token der App $app';
  }

  @override
  String get expiringAppTokenNote =>
      'Für Apps gibt es hier noch keine Token-Seite: Lege ein neues Token für die App an, stelle die App um und widerrufe danach das alte.';

  @override
  String get expiringUnknownText =>
      'Diese Version kennt diese Art von Zugriff noch nicht.';

  @override
  String get expiringOpenGroup => 'Gruppe öffnen';

  @override
  String get expiringRenewToken => 'Token erneuern';

  @override
  String get adminNavInstance => 'Instanz und Branding';

  @override
  String get instanceTitle => 'Instanz und Branding';

  @override
  String get instanceTabSettings => 'Einstellungen';

  @override
  String get instanceTabBranding => 'Branding';

  @override
  String get instanceTabProviders => 'Anmeldeanbieter';

  @override
  String get instanceTabFlags => 'Feature-Flags';

  @override
  String get instanceSave => 'Speichern';

  @override
  String get instanceValueNone => 'kein Wert';

  @override
  String get instanceValueEmpty => 'leer';

  @override
  String get instanceNeverChanged => 'Standardwert, nie geändert';

  @override
  String instanceChangedBy(int version, String name, String when) {
    return 'Version $version, geändert von $name, $when';
  }

  @override
  String instanceChangedAt(int version, String when) {
    return 'Version $version, geändert $when';
  }

  @override
  String instanceChangeFromTo(String from, String to) {
    return 'Bisher: $from\nNeu: $to';
  }

  @override
  String instanceConflict(String current, String input) {
    return 'Eine andere Person mit Inhaberrechten hat diese Einstellung inzwischen geändert. Jetzt gespeichert: $current. Deine Eingabe: $input. Speichere erneut, um den gespeicherten Wert durch deine Eingabe zu ersetzen.';
  }

  @override
  String get instanceFieldRequired => 'Dieses Feld ist erforderlich.';

  @override
  String get instanceFieldTooLong => 'Das ist für den Server zu lang.';

  @override
  String get instanceFieldInvalidText =>
      'Enthält ein unzulässiges Zeichen, zum Beispiel ein Steuer- oder Schreibrichtungszeichen.';

  @override
  String get instanceFieldUnknownColor =>
      'Nur eine der Palettenfarben kann gewählt werden.';

  @override
  String get instanceFieldInvalidEnum =>
      'Dieser Wert ist keiner der erlaubten Werte.';

  @override
  String get instanceFieldInvalidFormat =>
      'Dieser Wert hat nicht die erwartete Form.';

  @override
  String get instanceFieldInvalidType => 'Dieser Wert hat den falschen Typ.';

  @override
  String get instanceFieldNotAllowed =>
      'Dieses Feld ist mit diesen Angaben nicht erlaubt.';

  @override
  String get instanceFieldDuplicate => 'Ein Eintrag kommt doppelt vor.';

  @override
  String get instanceFieldUnknown => 'Der Server kennt dieses Feld nicht.';

  @override
  String instanceFieldOther(String code) {
    return 'Der Server hat diesen Wert abgelehnt ($code).';
  }

  @override
  String get instanceSettingsIntro =>
      'Diese Einstellungen gelten für die ganze Instanz und wirken sofort.';

  @override
  String get instanceKindText => 'Text';

  @override
  String get instanceKindUuid => 'Datei-ID';

  @override
  String instanceKindPalette(String keys) {
    return 'Eine der Palettenfarben: $keys';
  }

  @override
  String instanceKindLocale(String tags) {
    return 'Sprache: $tags';
  }

  @override
  String get instanceSettingChange => 'Ändern';

  @override
  String get instanceSettingOnBranding => 'Wird unter Branding geändert.';

  @override
  String get instanceSettingUnknownKey =>
      'Diese Version hat für diesen Schlüssel keinen Editor; ändere ihn über die API.';

  @override
  String get instanceSettingValue => 'Wert';

  @override
  String instanceSettingConfirmTitle(String key) {
    return '$key ändern?';
  }

  @override
  String instanceSettingSaved(String key, String value) {
    return '$key ist jetzt $value.';
  }

  @override
  String get instanceBrandingName => 'Produktname';

  @override
  String get instanceBrandingNameHelp =>
      'Wird allen in den Apps und der Konsole angezeigt. Der Server prüft die Länge und lehnt Steuer- und Schreibrichtungszeichen ab.';

  @override
  String get instanceAccentLabel => 'Akzent';

  @override
  String get instanceAccentHelp =>
      'Der Akzent stammt nur aus der geprüften Palette; einen freien Farbwert gibt es nicht.';

  @override
  String get instanceAccentSpruce => 'Fichte';

  @override
  String get instanceAccentLoch => 'Loch';

  @override
  String get instanceAccentHeather => 'Heide';

  @override
  String get instanceAccentRowan => 'Eberesche';

  @override
  String get instanceAccentAmber => 'Bernstein';

  @override
  String get instanceAccentGorse => 'Ginster';

  @override
  String get instanceAccentMoss => 'Moos';

  @override
  String get instancePreviewLight => 'Hell';

  @override
  String get instancePreviewDark => 'Dunkel';

  @override
  String get instancePreviewButton => 'Senden';

  @override
  String get instancePreviewChip => 'Ausgewählt';

  @override
  String get instancePreviewLink => 'Ein Link';

  @override
  String get instanceBrandingConfirmTitle => 'Branding ändern?';

  @override
  String instanceBrandingNameChange(String from, String to) {
    return 'Produktname: $from wird zu $to';
  }

  @override
  String instanceBrandingAccentChange(String from, String to) {
    return 'Akzent: $from wird zu $to';
  }

  @override
  String get instanceBrandingEveryone => 'Alle sehen die Änderung sofort.';

  @override
  String get instanceBrandingSaved => 'Das Branding ist gespeichert.';

  @override
  String instanceBrandingConflict(String name, String accent) {
    return 'Eine andere Person mit Inhaberrechten hat das Branding inzwischen geändert: jetzt $name mit dem Akzent $accent. Deine Eingabe bleibt; speichere erneut, um es zu ersetzen.';
  }

  @override
  String get instanceLogoTitle => 'Logo';

  @override
  String get instanceLogoNone => 'Kein Logo gesetzt.';

  @override
  String get instanceLogoPending =>
      'Das Logo wird geprüft und erscheint, sobald es bereit ist.';

  @override
  String get instanceLogoRefresh => 'Erneut prüfen';

  @override
  String get instanceLogoUpload => 'Logo hochladen';

  @override
  String get instanceLogoRemove => 'Logo entfernen';

  @override
  String get instanceLogoConfirmTitle => 'Logo hochladen?';

  @override
  String instanceLogoConfirmBody(String file) {
    return '$file ersetzt das bisherige Logo für alle. Der Server prüft es und entfernt Metadaten, bevor es angezeigt wird.';
  }

  @override
  String get instanceLogoUploaded => 'Das Logo ist hochgeladen.';

  @override
  String get instanceLogoRemoveTitle => 'Logo entfernen?';

  @override
  String get instanceLogoRemoveBody => 'Alle sehen den Produktnamen ohne Logo.';

  @override
  String get instanceLogoRemoved => 'Das Logo ist entfernt.';

  @override
  String get instanceProvidersIntro =>
      'Über diese Anbieter melden sich Personen an. Änderungen wirken sofort.';

  @override
  String get instanceProvidersEmpty => 'Kein Anmeldeanbieter eingerichtet.';

  @override
  String get instanceProviderAdd => 'Anbieter hinzufügen';

  @override
  String get instanceProviderKindOidc => 'OpenID Connect';

  @override
  String get instanceProviderKindHeader => 'Proxy-Header';

  @override
  String get instanceProviderKindLocal => 'Lokale Konten';

  @override
  String get instanceProviderKindUnknown => 'Unbekannter Treiber';

  @override
  String get instanceProviderStateEnabled => 'Aktiv';

  @override
  String get instanceProviderStateDisabled => 'Deaktiviert';

  @override
  String get instanceProviderProvisioningShort => 'Legt Konten an';

  @override
  String instanceProviderHeaderLine(String header) {
    return 'Anmelde-Header: $header';
  }

  @override
  String instanceProviderIssuerLine(String issuer) {
    return 'Aussteller: $issuer';
  }

  @override
  String get instanceProviderChange => 'Ändern';

  @override
  String get instanceProviderEnable => 'Aktivieren';

  @override
  String get instanceProviderDisable => 'Deaktivieren';

  @override
  String get instanceProviderDelete => 'Löschen';

  @override
  String instanceProviderDisableTitle(String name) {
    return '$name deaktivieren?';
  }

  @override
  String instanceProviderDisableBody(String name) {
    return 'Alle Sitzungen, die über $name begonnen haben, enden sofort. Niemand kann sich darüber anmelden, bis er wieder aktiviert ist.';
  }

  @override
  String instanceProviderEnableTitle(String name) {
    return '$name aktivieren?';
  }

  @override
  String instanceProviderEnableBody(String name) {
    return 'Personen können sich wieder über $name anmelden.';
  }

  @override
  String instanceProviderDeleteTitle(String name) {
    return '$name löschen?';
  }

  @override
  String instanceProviderDeleteBody(String name) {
    return 'Alle Sitzungen, die über $name begonnen haben, enden sofort. Der Server lehnt ab, solange Identitäten damit verknüpft sind; deaktiviere ihn dann.';
  }

  @override
  String instanceProviderDisabled(String name) {
    return '$name ist deaktiviert; seine Sitzungen sind beendet.';
  }

  @override
  String instanceProviderEnabled(String name) {
    return '$name ist aktiv.';
  }

  @override
  String instanceProviderDeleted(String name) {
    return '$name ist gelöscht.';
  }

  @override
  String instanceProviderAdded(String name) {
    return '$name ist hinzugefügt.';
  }

  @override
  String instanceProviderSaved(String name) {
    return '$name ist gespeichert.';
  }

  @override
  String get instanceProviderLoopbackRule =>
      'Der Proxy-Header-Anbieter funktioniert nur, solange der Server auf einer Loopback-Adresse lauscht (127.0.0.1 oder ::1); sonst lehnt der Server das Aktivieren ab.';

  @override
  String get instanceProviderLoopbackRefused =>
      'Der Server hat abgelehnt: Der Proxy-Header-Anbieter braucht einen Server, der nur auf einer Loopback-Adresse lauscht.';

  @override
  String get instanceProviderRedirectLabel =>
      'Weiterleitungs-URI für die Registrierung beim OIDC-Anbieter';

  @override
  String get instanceProviderRedirectCopy => 'Kopieren';

  @override
  String get instanceProviderRedirectCopied => 'Kopiert';

  @override
  String instanceProviderAddTitle(String kind) {
    return '$kind-Anbieter hinzufügen';
  }

  @override
  String instanceProviderEditTitle(String name) {
    return '$name ändern';
  }

  @override
  String get instanceProviderName => 'Name';

  @override
  String get instanceProviderNameHelp =>
      'Wird auf der Anmeldeschaltfläche angezeigt.';

  @override
  String get instanceProviderIssuer => 'Aussteller';

  @override
  String get instanceProviderIssuerHelp =>
      'Genaue https-Adresse des Ausstellers, zum Beispiel https://login.example.com';

  @override
  String get instanceProviderClientId => 'Client-ID';

  @override
  String get instanceProviderClientAuth => 'Client-Authentifizierung';

  @override
  String get instanceProviderSecretRef => 'Referenz auf das Client-Secret';

  @override
  String get instanceProviderKeyRef => 'Referenz auf den privaten Schlüssel';

  @override
  String get instanceProviderSecretRefHelp =>
      'Eine Referenz wie file:/srv/secrets/idp/client oder env:BOTHY_IDP_CLIENT, nie das Secret selbst. Sie wird nie wieder angezeigt.';

  @override
  String get instanceProviderSecretKeep =>
      'Leer lassen, um die gespeicherte Referenz zu behalten.';

  @override
  String get instanceProviderKeyAlg => 'Schlüsselalgorithmus';

  @override
  String get instanceProviderKeyId => 'Schlüssel-ID';

  @override
  String get instanceProviderX5t => 'Zertifikat-Fingerprint (x5t#S256)';

  @override
  String get instanceProviderTenants => 'Erlaubte Mandanten-IDs';

  @override
  String instanceProviderTenantsHelp(String tenantid) {
    return 'Durch Kommas getrennt; nötig, wenn der Aussteller $tenantid enthält.';
  }

  @override
  String get instanceProviderAppRedirects => 'Weiterleitungs-URIs der Apps';

  @override
  String get instanceProviderAppRedirectsHelp =>
      'Eine pro Zeile: private Schemata oder beanspruchte https-Adressen der Apps.';

  @override
  String get instanceProviderGroupsClaim => 'Gruppen-Claim';

  @override
  String get instanceProviderGroupsClaimHelp =>
      'Ohne Gruppen-Claim werden keine Gruppen abgeglichen.';

  @override
  String get instanceProviderLoginHeader => 'Anmelde-Header';

  @override
  String get instanceProviderLoginHeaderHelp =>
      'Zum Beispiel Tailscale-User-Login';

  @override
  String get instanceProviderNameHeader => 'Namens-Header (optional)';

  @override
  String get instanceProviderLocalNote =>
      'Lokale Konten melden sich mit Passwort und TOTP oder einem Passkey an; hier ist nichts einzustellen.';

  @override
  String get instanceProviderEnableNow => 'Sofort aktivieren';

  @override
  String get instanceProviderProvisioning =>
      'Konten für unbekannte Personen anlegen';

  @override
  String get instanceProviderProvisioningHelp =>
      'Sonst braucht eine Person für die erste Anmeldung einen Einladungscode.';

  @override
  String instanceProviderCreateSummary(String kind, String name) {
    return 'Ein neuer $kind-Anbieter namens $name wird hinzugefügt.';
  }

  @override
  String get instanceProviderStartsEnabled => 'Er ist sofort aktiv.';

  @override
  String get instanceProviderStartsDisabled => 'Er startet deaktiviert.';

  @override
  String get instanceProviderProvisioningOn =>
      'Unbekannte Personen erhalten bei ihrer ersten Anmeldung ein Konto.';

  @override
  String get instanceProviderProvisioningOff =>
      'Unbekannte Personen brauchen einen Einladungscode.';

  @override
  String instanceProviderNameChange(String from, String to) {
    return 'Name: $from wird zu $to';
  }

  @override
  String instanceProviderMembersChange(String members) {
    return 'Geänderte Angaben: $members';
  }

  @override
  String stepUpPurposeInstanceSetting(String key) {
    return 'Instanzeinstellung $key ändern';
  }

  @override
  String get stepUpPurposeBranding => 'Branding der Instanz ändern';

  @override
  String get stepUpPurposeLogo => 'Logo der Instanz hochladen';

  @override
  String get stepUpPurposeLogoRemove => 'Logo der Instanz entfernen';

  @override
  String stepUpPurposeProviderAdd(String name) {
    return 'Anmeldeanbieter $name hinzufügen';
  }

  @override
  String stepUpPurposeProviderEdit(String name) {
    return 'Anmeldeanbieter $name ändern';
  }

  @override
  String stepUpPurposeProviderDisable(String name) {
    return 'Anmeldeanbieter $name deaktivieren';
  }

  @override
  String stepUpPurposeProviderEnable(String name) {
    return 'Anmeldeanbieter $name aktivieren';
  }

  @override
  String stepUpPurposeProviderDelete(String name) {
    return 'Anmeldeanbieter $name löschen';
  }

  @override
  String stepUpPurposeFlagOn(String key) {
    return 'Feature-Flag $key einschalten';
  }

  @override
  String stepUpPurposeFlagOff(String key) {
    return 'Feature-Flag $key ausschalten';
  }

  @override
  String get instanceFlagsIntro =>
      'Feature-Flags schalten Funktionen für die ganze Instanz. Der Zustand für dich berücksichtigt Freischaltungen für Personen und Gruppen.';

  @override
  String get instanceFlagOn => 'an';

  @override
  String get instanceFlagOff => 'aus';

  @override
  String get instanceFlagKindRelease => 'Release-Schalter';

  @override
  String get instanceFlagKindOption => 'Option';

  @override
  String get instanceFlagKindUnknown => 'Unbekannte Art';

  @override
  String get instanceFlagOrphaned => 'Nicht mehr registriert';

  @override
  String instanceFlagForYou(String state) {
    return 'Für dich: $state';
  }

  @override
  String get instanceFlagForYouUnknown => 'Für dich: nicht ausgewertet';

  @override
  String instanceFlagStored(String state) {
    return 'Gespeichert: $state';
  }

  @override
  String instanceFlagDefault(String state) {
    return 'Standard: $state';
  }

  @override
  String instanceFlagRollout(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: 'Umgekehrt für $count Personen oder Gruppen',
      one: 'Umgekehrt für 1 Person oder Gruppe',
    );
    return '$_temp0';
  }

  @override
  String instanceFlagRolloutStays(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: 'Die Freischaltung für $count Personen oder Gruppen bleibt.',
      one: 'Die Freischaltung für 1 Person oder Gruppe bleibt.',
    );
    return '$_temp0';
  }

  @override
  String instanceFlagOnTitle(String key) {
    return '$key einschalten?';
  }

  @override
  String instanceFlagOffTitle(String key) {
    return '$key ausschalten?';
  }

  @override
  String get instanceFlagEveryone =>
      'Der globale Wert ändert sich sofort für alle.';

  @override
  String get instanceFlagReason => 'Grund (erforderlich)';

  @override
  String get instanceFlagSwitchOn => 'Einschalten';

  @override
  String get instanceFlagSwitchOff => 'Ausschalten';

  @override
  String instanceFlagSwitchedOn(String key) {
    return '$key ist eingeschaltet.';
  }

  @override
  String instanceFlagSwitchedOff(String key) {
    return '$key ist ausgeschaltet.';
  }

  @override
  String instanceFlagConflict(String key, String state) {
    return 'Eine andere Person mit Inhaberrechten hat $key inzwischen geändert; es ist jetzt $state. Nichts wurde geändert; schalte bei Bedarf erneut.';
  }

  @override
  String get adminLogoFileType => 'Bild';

  @override
  String get notificationPlaceholder => 'Neue Nachricht';

  @override
  String notificationTitle(String name, String title) {
    return '$name · $title';
  }

  @override
  String notificationText(String author, String excerpt) {
    return '$author: $excerpt';
  }

  @override
  String notificationBundled(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count neue Nachrichten',
      one: '1 neue Nachricht',
    );
    return '$_temp0';
  }

  @override
  String get notificationSecurityTitle => 'Neues Gerät';

  @override
  String get notificationSecurityText =>
      'Ein neues Gerät wartet auf deine Bestätigung.';

  @override
  String get notificationAdminTitle => 'Verwaltung';

  @override
  String get notificationAdminText => 'Eine Erinnerung der Verwaltung wartet.';

  @override
  String get pushTitle => 'Neue Benachrichtigung';

  @override
  String get pushBodyMention => 'Du wurdest erwähnt';

  @override
  String get pushBodyDm => 'Neue Direktnachricht';

  @override
  String get pushBodyRunDone => 'Ein Lauf ist fertig';

  @override
  String get pushBodyRunFailed => 'Ein Lauf ist fehlgeschlagen';

  @override
  String get pushBodyInputNeeded => 'Ein Agent wartet auf deine Antwort';

  @override
  String get pushMessageNotAvailable => 'Nachricht nicht verfügbar';

  @override
  String get trayOpen => 'Öffnen';

  @override
  String get trayQuit => 'Beenden';

  @override
  String get settingsNotificationsWhileRunning =>
      'Hinweise kommen nur, solange die App läuft.';

  @override
  String get settingsStartAtLogin => 'Bei Anmeldung starten';

  @override
  String get settingsStartAtLoginUnavailable =>
      'Dieses System bietet den Start bei Anmeldung für die App nicht an.';

  @override
  String get importTitle => 'Import aus Discord';

  @override
  String get importAdd => 'Quelle hinzufügen';

  @override
  String get importRefresh => 'Aktualisieren';

  @override
  String get importEmpty =>
      'Noch keine Importquelle. Füge eine mit der Zuordnungsdatei des Import-Werkzeugs hinzu.';

  @override
  String get importStateDraft => 'Entwurf';

  @override
  String get importStateRunning => 'Gestartet';

  @override
  String get importStatePaused => 'Pausiert';

  @override
  String get importStateCompleted => 'Abgeschlossen';

  @override
  String get importStateFailed => 'Fehlgeschlagen';

  @override
  String get importStateCancelled => 'Abgebrochen';

  @override
  String importItemsImported(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count Einträge importiert',
      one: '1 Eintrag importiert',
    );
    return '$_temp0';
  }

  @override
  String get importBack => 'Alle Importquellen';

  @override
  String get importServerCounters => 'Zähler des Servers';

  @override
  String get importToolCounters => 'Zähler des Import-Werkzeugs';

  @override
  String get importNoCounters => 'Noch keine Zähler.';

  @override
  String get importFailedItems => 'Fehlgeschlagene Einträge';

  @override
  String get importNoFailedItems => 'Keine fehlgeschlagenen Einträge.';

  @override
  String get importChannelMapping => 'Kanal-Zuordnung';

  @override
  String get importNoChannels =>
      'Die Zuordnung nennt keinen Kanal zum Importieren.';

  @override
  String importChannelDiscord(String id) {
    return 'Discord-ID $id';
  }

  @override
  String get importChannelUnknownName => 'Name nicht lesbar';

  @override
  String get importPhaseNotImported => 'Nicht importiert';

  @override
  String get importPhaseMirror => 'Spiegel, schreibgeschützt';

  @override
  String get importPhaseOperatorOk => 'Hier geführt, Bediener stimmt';

  @override
  String get importPhaseNoOperator => 'Hier geführt, kein Bediener';

  @override
  String get importPhaseOtherOperator =>
      'Hier geführt, anderer Bediener als geplant';

  @override
  String get importReadOnly => 'Schreibgeschützt';

  @override
  String get importWritable => 'Beschreibbar';

  @override
  String importPlanned(String agent) {
    return 'Geplanter Bediener: $agent';
  }

  @override
  String get importPlannedNone => 'Kein Bediener geplant';

  @override
  String get importChannelUnreadable =>
      'Der Kanal konnte nicht gelesen werden.';

  @override
  String get importLeadHereAction => 'Hier führen';

  @override
  String get importLeadDiscordAction => 'Discord führt';

  @override
  String get importSetOperatorAction => 'Bediener setzen';

  @override
  String get importStart => 'Starten';

  @override
  String get importPause => 'Pausieren';

  @override
  String get importCancel => 'Import abbrechen';

  @override
  String get importComplete => 'Abschließen';

  @override
  String get importCompleteHint =>
      'Abschließen geht erst, wenn jeder importierte Kanal hier geführt wird.';

  @override
  String importStartTitle(String name) {
    return '$name starten?';
  }

  @override
  String get importStartBody =>
      'Das Import-Werkzeug darf danach mit --apply in diese Quelle schreiben. Importiert wird erst, wenn du das Werkzeug aufrufst.';

  @override
  String importPauseTitle(String name) {
    return '$name pausieren?';
  }

  @override
  String get importPauseBody =>
      'Das Import-Werkzeug kann nicht in diese Quelle schreiben, bis du sie wieder startest. Die Führung bleibt, wie sie ist.';

  @override
  String importCancelTitle(String name) {
    return '$name abbrechen?';
  }

  @override
  String get importCancelBody =>
      'Die Quelle endet als abgebrochen und kann nicht wieder gestartet werden. Was importiert ist, bleibt; das Import-Werkzeug kann nicht mehr in diese Quelle schreiben.';

  @override
  String importCompleteTitle(String name) {
    return '$name abschließen?';
  }

  @override
  String get importCompleteBody =>
      'Der Spiegel endet und die Quelle ist abgeschlossen. Die Zuordnung in import_items bleibt, bis die Aufbewahrungsklasse import gelöscht wird. Nach dieser Löschung ist ein wiederholter Import nicht mehr idempotent: er würde alles erneut importieren.';

  @override
  String importLeadHereTitle(String name) {
    return '$name hier führen?';
  }

  @override
  String get importLeadHereOrder =>
      'Halte diese Reihenfolge ein, damit nie zwei Orte gleichzeitig führen:';

  @override
  String get importLeadHereStepDiscord =>
      '1. In Discord hast du den Kanal schreibgeschützt und den Agenten dort entfernt.';

  @override
  String importLeadHereStepSwitch(String command) {
    return '2. $command war erfolgreich.';
  }

  @override
  String get importLeadHereStepLead =>
      '3. Jetzt führt dieser Server: der Kanal hier wird beschreibbar.';

  @override
  String importLeadHereStepOperator(String agent) {
    return '4. Danach setzt du den Bediener $agent.';
  }

  @override
  String get importLeadHereStepNoOperator =>
      '4. Die Zuordnung plant für diesen Kanal keinen Bediener.';

  @override
  String importLeadHereOtherOperator(String agent) {
    return 'Der Kanal hat einen anderen Bediener als den geplanten $agent. Importierte Threads behalten $agent; mit einem anderen Bediener bleiben sie ohne Agent.';
  }

  @override
  String importLeadDiscordTitle(String name) {
    return 'Discord führt in $name?';
  }

  @override
  String importLeadDiscordBody(String command) {
    return 'Der Kanal hier wird für alle schreibgeschützt, auch für Inhaber, und laufende Antworten von Agenten werden abgebrochen. Hier geschriebene Nachrichten bleiben hier; $command zählt sie. Danach kannst du den Bediener entfernen.';
  }

  @override
  String importLeadHereDone(String name) {
    return '$name wird hier geführt.';
  }

  @override
  String importLeadDiscordDone(String name) {
    return 'Discord führt in $name; hier ist er schreibgeschützt.';
  }

  @override
  String importStarted(String name) {
    return '$name ist gestartet.';
  }

  @override
  String importPausedDone(String name) {
    return '$name ist pausiert.';
  }

  @override
  String importCancelledDone(String name) {
    return '$name ist abgebrochen.';
  }

  @override
  String importCompletedDone(String name) {
    return '$name ist abgeschlossen.';
  }

  @override
  String get importConflictState =>
      'Die Quelle ist inzwischen in einem anderen Zustand. Die Seite zeigt den aktuellen Zustand.';

  @override
  String get importConflictLeadNotHere =>
      'Noch wird nicht jeder importierte Kanal hier geführt. Stelle zuerst jeden Kanal um.';

  @override
  String get importConflictNotImported =>
      'Dieser Kanal ist noch nicht importiert.';

  @override
  String get importCreateTitle => 'Importquelle hinzufügen';

  @override
  String get importCreateName => 'Name der Quelle';

  @override
  String get importCreateHint =>
      'Die Zuordnungsdatei ist das YAML des Import-Werkzeugs (bothy import discord init-map schreibt ein Gerüst). Sie enthält kein Geheimnis und wird nicht im Browser gespeichert.';

  @override
  String get importCreatePick => 'Zuordnungsdatei wählen';

  @override
  String get importCreateFileRead => 'Zuordnungsdatei gelesen.';

  @override
  String get importCreateSubmit => 'Hinzufügen';

  @override
  String importFileInvalidYaml(int line, int column) {
    return 'Die Datei ist kein gültiges YAML (Zeile $line, Spalte $column). Auch ein doppelt genannter Schlüssel wird abgelehnt.';
  }

  @override
  String get importFileInvalid => 'Die Datei ist kein gültiges YAML.';

  @override
  String get importFileNotMapping =>
      'Die Datei muss eine YAML-Zuordnung mit Text-Schlüsseln sein.';

  @override
  String get importFileUnreadable => 'Die Datei ist kein UTF-8-Text.';

  @override
  String get importServerErrors => 'Der Server hat die Zuordnung abgelehnt:';

  @override
  String importFieldError(String path, String problem) {
    return '$path: $problem';
  }

  @override
  String get importFieldWholeFile => 'die Datei';

  @override
  String get importFieldRequired => 'fehlt';

  @override
  String get importFieldUnknownMember => 'ist kein Feld der Zuordnungsdatei';

  @override
  String get importFieldInvalid => 'hat einen falschen Typ oder Wert';

  @override
  String get importFieldNotDigits =>
      'muss eine Discord-ID in Anführungszeichen sein, nur Ziffern';

  @override
  String get importFieldDuplicate => 'nennt eine Discord-ID ein zweites Mal';

  @override
  String get importFieldTooLong => 'ist zu lang';

  @override
  String get importFieldUnknownHandle =>
      'nennt keine Person und keinen Agenten mit diesem Handle';

  @override
  String get importFieldNotActive =>
      'nennt einen Agenten oder eine Person, die nicht aktiv ist';

  @override
  String get importFieldUnknownExisting => 'nennt nichts, was hier existiert';

  @override
  String get importFieldUnknownCategory =>
      'nennt keine Kategorie der Liste spaces (Bereiche)';

  @override
  String get importFieldNotEnabled => 'ist noch nicht verfügbar';

  @override
  String importCreateConfirmTitle(String name) {
    return '$name hinzufügen?';
  }

  @override
  String get importCreateConfirmBody =>
      'Der Server speichert die Zuordnung ohne Geheimnisse. Die Quelle beginnt als Entwurf; noch wird nichts importiert.';

  @override
  String get importNextSteps => 'Nächste Schritte:';

  @override
  String importNextCheck(String command) {
    return '1. Prüfe die Zuordnung mit $command; das liest nur.';
  }

  @override
  String get importNextStart => '2. Starte die Quelle hier.';

  @override
  String importNextApply(String command) {
    return '3. Importiere mit $command.';
  }

  @override
  String importOperatorTitle(String name) {
    return 'Bediener von $name';
  }

  @override
  String get importOperatorBody =>
      'Der Kanal wird jetzt hier geführt. Setze den geplanten Bediener, damit die importierten Threads und der Kanal denselben Agenten haben.';

  @override
  String get importOperatorAgent => 'Agent';

  @override
  String importOperatorNotPlanned(String agent, String planned) {
    return '$agent ist nicht der geplante Bediener $planned. Importierte Threads behalten $planned; sie bleiben in diesem Kanal ohne Agent.';
  }

  @override
  String get importOperatorSet => 'Bediener setzen';

  @override
  String get importOperatorSkip => 'Nicht jetzt';

  @override
  String importOperatorDone(String agent, String name) {
    return '$agent ist der Bediener von $name.';
  }

  @override
  String importOperatorRemoveTitle(String name) {
    return 'Bediener von $name entfernen?';
  }

  @override
  String get importOperatorRemoveBody =>
      'Discord führt wieder. Ohne Bediener antwortet hier kein Agent.';

  @override
  String get importOperatorRemove => 'Bediener entfernen';

  @override
  String importOperatorRemoved(String name) {
    return '$name hat jetzt keinen Bediener.';
  }

  @override
  String importFailedItem(String kind, String origin, String error) {
    return '$kind $origin: $error';
  }

  @override
  String stepUpPurposeImportCreate(String name) {
    return 'Importquelle $name hinzufügen';
  }

  @override
  String stepUpPurposeImportStart(String name) {
    return 'Import $name starten';
  }

  @override
  String stepUpPurposeImportPause(String name) {
    return 'Import $name pausieren';
  }

  @override
  String stepUpPurposeImportCancel(String name) {
    return 'Import $name abbrechen';
  }

  @override
  String stepUpPurposeImportComplete(String name) {
    return 'Import $name abschließen';
  }

  @override
  String stepUpPurposeImportLead(String name) {
    return 'Führung von $name ändern';
  }

  @override
  String stepUpPurposeImportOperator(String name) {
    return 'Bediener von $name setzen';
  }

  @override
  String get notifyLevelAll => 'Alle';

  @override
  String get notifyLevelMentions => 'Erwähnungen';

  @override
  String get notifyLevelMute => 'Stumm';

  @override
  String notifyInherits(String level) {
    return 'erbt: $level';
  }

  @override
  String notifyInheritFrom(String level, String source) {
    return 'Erben ($level von $source)';
  }

  @override
  String get notifySourceThread => 'dem Thread';

  @override
  String get notifySourceChannel => 'dem Kanal';

  @override
  String get notifySourceSpace => 'dem Bereich';

  @override
  String get notifySourceChannelDefault => 'dem Standard des Kanals';

  @override
  String get notifySourceDm => 'der Direktnachricht';

  @override
  String get notifySourcePersonalDefault => 'deinem Standard';

  @override
  String get notifySourceInstanceDefault => 'dem Standard der Instanz';

  @override
  String get notifyFollowing => 'Gefolgt';

  @override
  String notifyMutedUntil(String when) {
    return 'stumm bis $when';
  }

  @override
  String get notifyMenuNotifications => 'Benachrichtigungen';

  @override
  String get notifyMenuMuteUntil => 'Stumm bis';

  @override
  String get notifyMuteOneHour => 'Für 1 Stunde';

  @override
  String notifyMuteTomorrow(String time) {
    return 'Morgen $time';
  }

  @override
  String get notifyMuteChooseDate => 'Datum und Uhrzeit wählen';

  @override
  String notifyMenuOf(String name) {
    return 'Menü von $name';
  }

  @override
  String get notifyMenuOpen => 'Mehr';

  @override
  String get channelMenuExport => 'Verlauf exportieren';

  @override
  String exportTitle(String name) {
    return '$name exportieren';
  }

  @override
  String get exportFrom => 'Von';

  @override
  String get exportTo => 'Bis';

  @override
  String get exportRangeInvalid =>
      'Der erste Tag darf nicht nach dem letzten liegen.';

  @override
  String get exportStart => 'Datei wählen und exportieren';

  @override
  String exportProgress(String size) {
    return '$size geschrieben';
  }

  @override
  String exportDone(String size) {
    return 'Export gespeichert: $size.';
  }

  @override
  String get exportCancelled =>
      'Export abgebrochen. Die unvollständige Datei wurde gelöscht.';

  @override
  String get exportBrokeOff =>
      'Der Export ist abgebrochen. Die unvollständige Datei wurde gelöscht.';

  @override
  String get exportCancel => 'Abbrechen';

  @override
  String get exportClose => 'Schließen';

  @override
  String stepUpPurposeChannelExport(String name) {
    return 'Den Verlauf von $name exportieren';
  }

  @override
  String get notifySettingsDefault => 'Standard für jeden neuen Kanal';

  @override
  String get notifySettingsDefaultHint =>
      'Gilt, wo nichts anderes eingestellt ist.';

  @override
  String get notifySettingsRuns => 'Von mir ausgelöster Lauf';

  @override
  String get notifySettingsRunsExplain =>
      'Eine Frage aus deinem Lauf benachrichtigt dich immer, auch bei Stumm. Sein Ende oder Fehler folgt der Stufe.';

  @override
  String get notifySettingsPush => 'Push-Benachrichtigungen';

  @override
  String get notifySettingsPushUnavailable =>
      'Diese Version der App empfängt auf diesem Gerät keine Push-Benachrichtigungen.';

  @override
  String get notifySettingsPreview => 'Inhalt in Benachrichtigungen';

  @override
  String get notifyPreviewFull => 'Text und Namen';

  @override
  String get notifyPreviewNames => 'Nur Namen';

  @override
  String get notifyPreviewNone => 'Nur Neue Nachricht';

  @override
  String get notifySettingsQuietHours => 'Ruhezeiten';

  @override
  String get notifySettingsInputBreaks => 'Fragen durchbrechen Ruhezeiten';

  @override
  String get notifySettingsInputBreaksHint =>
      'Eine Frage aus deinem eigenen Lauf bleibt hörbar.';

  @override
  String get notifySettingsAdmin => 'Erinnerungen der Verwaltung';

  @override
  String get notifySettingsAdminHint =>
      'Zum Beispiel eine Freigabe, die abläuft.';

  @override
  String get notifySettingsTree => 'Pro Bereich, Kanal und Thread';

  @override
  String get notifySettingsTreeEmpty => 'Noch nichts zu zeigen.';

  @override
  String get notifyChoiceInherit => 'Erben';

  @override
  String get quietHoursNone => 'Keine Ruhezeiten.';

  @override
  String get quietHoursAdd => 'Ruhezeit hinzufügen';

  @override
  String get quietHoursEdit => 'Bearbeiten';

  @override
  String get quietHoursDelete => 'Löschen';

  @override
  String get quietHoursAllDay => 'Ganztägig';

  @override
  String quietHoursRange(String start, String end) {
    return '$start bis $end';
  }

  @override
  String get quietHoursOff => 'aus';

  @override
  String get quietHoursDayMon => 'Mo';

  @override
  String get quietHoursDayTue => 'Di';

  @override
  String get quietHoursDayWed => 'Mi';

  @override
  String get quietHoursDayThu => 'Do';

  @override
  String get quietHoursDayFri => 'Fr';

  @override
  String get quietHoursDaySat => 'Sa';

  @override
  String get quietHoursDaySun => 'So';

  @override
  String get quietHoursEditorTitle => 'Ruhezeit';

  @override
  String get quietHoursDays => 'Tage';

  @override
  String get quietHoursStart => 'Beginn';

  @override
  String get quietHoursEnd => 'Ende';

  @override
  String get quietHoursTimeZone => 'Zeitzone';

  @override
  String quietHoursTimeZoneMine(String zone) {
    return 'Meine ($zone)';
  }

  @override
  String get quietHoursEnabled => 'An';

  @override
  String get quietHoursSave => 'Speichern';

  @override
  String get quietHoursCancel => 'Abbrechen';

  @override
  String get quietHoursTimeInvalid => 'Format HH:MM, zum Beispiel 07:30.';

  @override
  String get quietHoursNoDays => 'Wähle mindestens einen Tag.';

  @override
  String get searchTitle => 'Suche';

  @override
  String get searchFieldLabel => 'Suchtext';

  @override
  String get searchFieldHint => 'Nachrichten, Threads und Dateien durchsuchen';

  @override
  String get searchSubmit => 'Suchen';

  @override
  String get searchFilters => 'Filter';

  @override
  String get searchFilterSpace => 'Bereich';

  @override
  String get searchFilterChannel => 'Kanal';

  @override
  String get searchFilterFrom => 'Von';

  @override
  String get searchFilterAll => 'alle';

  @override
  String get searchFilterType => 'Typ';

  @override
  String get searchTypeMessages => 'Nachrichten';

  @override
  String get searchTypeThreads => 'Threads';

  @override
  String get searchTypeFiles => 'Dateien';

  @override
  String get searchFilterTime => 'Zeit';

  @override
  String get searchTimeAny => 'beliebig';

  @override
  String get searchTimeDay => 'letzte 24 Stunden';

  @override
  String get searchTimeWeek => 'letzte 7 Tage';

  @override
  String get searchTimeMonth => 'letzte 30 Tage';

  @override
  String get searchFilterStatus => 'Status';

  @override
  String get searchStatusAll => 'offen und erledigt';

  @override
  String get searchStatusOpen => 'offen';

  @override
  String get searchStatusDone => 'erledigt';

  @override
  String get searchHasFile => 'Nur mit Dateien';

  @override
  String get searchSort => 'Sortierung';

  @override
  String get searchSortRelevance => 'Relevanz';

  @override
  String get searchSortNewest => 'Neueste zuerst';

  @override
  String searchResultCount(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count Treffer, nur aus Kanälen, die du sehen darfst',
      one: '1 Treffer, nur aus Kanälen, die du sehen darfst',
    );
    return '$_temp0';
  }

  @override
  String get searchNoResults => 'Keine Treffer';

  @override
  String get searchLocalOnly => 'Nur auf diesem Gerät gespeicherte Nachrichten';

  @override
  String get searchCandidatesCapped =>
      'Es gibt zu viele Treffer. Schränke die Suche ein, um die besten zu sehen.';

  @override
  String get searchLoadMore => 'Mehr laden';

  @override
  String get searchQueryEmpty => 'Gib mindestens ein Wort ein.';

  @override
  String get searchQueryTooLong => 'Der Suchtext ist zu lang.';

  @override
  String get searchFailed =>
      'Die Suche hat nicht geklappt. Versuche es noch einmal.';

  @override
  String get searchHitThread => 'Thread';

  @override
  String get searchHitFile => 'Datei';

  @override
  String get searchHitTranscript => 'Transkript';

  @override
  String searchHitLabel(String path, String author, String time, String text) {
    return '$path, $author, $time: $text';
  }

  @override
  String get switcherLabel => 'Schnellwechsel';

  @override
  String get switcherHint =>
      'Zu Kanal, Thread, Person, Agent oder Befehl springen';

  @override
  String get switcherFilterAll => 'Alle';

  @override
  String get switcherFilterChannels => 'Kanäle';

  @override
  String get switcherFilterThreads => 'Threads';

  @override
  String get switcherFilterPeople => 'Personen';

  @override
  String get switcherFilterAgents => 'Agenten';

  @override
  String get switcherFilterCommands => 'Befehle';

  @override
  String get switcherTabHint => 'Tab: Filter';

  @override
  String get switcherKindSpace => 'Bereich';

  @override
  String get switcherKindChannel => 'Kanal';

  @override
  String get switcherKindThread => 'Thread';

  @override
  String get switcherKindPerson => 'Person';

  @override
  String get switcherKindAgent => 'Agent';

  @override
  String get switcherKindCommand => 'Befehl';

  @override
  String get switcherNothing => 'Nichts gefunden';

  @override
  String get switcherClose => 'Schließen';

  @override
  String get mentionsTitle => 'Erwähnungen';

  @override
  String get mentionsEmpty => 'Dich hat noch niemand erwähnt.';

  @override
  String get mentionsFailed =>
      'Die Erwähnungen konnten nicht geladen werden. Versuche es noch einmal.';

  @override
  String get mentionsRetry => 'Noch einmal versuchen';

  @override
  String runningTitle(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: 'Läuft gerade ($count)',
      zero: 'Läuft gerade',
    );
    return '$_temp0';
  }

  @override
  String get runningFilterSpace => 'Bereich';

  @override
  String get runningFilterAgent => 'Agent';

  @override
  String get runningFilterAll => 'alle';

  @override
  String get runningMineOnly => 'nur meine';

  @override
  String get runningStop => 'Stoppen';

  @override
  String get runningOpen => 'Öffnen';

  @override
  String get runningEmpty => 'Kein Lauf ist aktiv.';

  @override
  String runningSumOrganization(String amount, String estimate) {
    return 'Organisation: Summe der aktiven Läufe $amount ($estimate).';
  }

  @override
  String get runningPersonalNote => 'Persönliche Läufe: nur zur Information.';

  @override
  String runningContext(String percent) {
    return '$percent Kontext';
  }

  @override
  String runningRowLabel(
    String agent,
    String path,
    String state,
    String duration,
  ) {
    return '$agent, $path, $state, $duration';
  }

  @override
  String get runningFailed =>
      'Die aktiven Läufe konnten nicht geladen werden; die Liste zeigt, was dieses Gerät kennt.';

  @override
  String sidebarRunningNowCount(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: 'Läuft gerade, $count aktive Läufe',
      one: 'Läuft gerade, 1 aktiver Lauf',
    );
    return '$_temp0';
  }

  @override
  String get consoleTerminalPinningNotice =>
      'Im Browser bemerkt diese Prüfung nur einen Schlüsselwechsel, den der Server meldet; gegen den Server selbst schützt sie nicht.';

  @override
  String get consoleTerminalPasskeyTitle => 'Passkey nötig';

  @override
  String get consoleTerminalPasskeySetup =>
      'Ein Terminal öffnet in der Konsole nur mit einem Passkey mit Benutzerprüfung, und dein Konto hat noch keinen. Richte zuerst einen Passkey für dein Konto ein und öffne das Terminal danach erneut. In der Desktop- und der Mobil-App öffnet das Terminal stattdessen mit dem Geräteschlüssel.';

  @override
  String get consoleTerminalPasskeyClose => 'Schließen';

  @override
  String get consoleTerminalOpen => 'Terminal öffnen';

  @override
  String get consoleTerminalBack => 'Zurück zum Rechner';

  @override
  String get componentsDeniedLevel =>
      'Du darfst Agenten in diesem Kanal nicht antworten';

  @override
  String get liveBoxReasonInterrupted => 'Lauf unterbrochen';

  @override
  String get liveBoxSendAgain => 'Erneut senden';

  @override
  String agentWorking(String agent) {
    return '$agent arbeitet';
  }

  @override
  String get commandBuiltinStop => 'Den aktiven Lauf stoppen';

  @override
  String get commandBuiltinStatus => 'Lauf- und Sitzungsdetails zeigen';

  @override
  String get commandBuiltinNew => 'Neue Agentensitzung beginnen';

  @override
  String get commandBuiltinResume => 'Frühere Sitzung fortsetzen';

  @override
  String get commandBuiltinTitle => 'Titel des Threads setzen';

  @override
  String get commandBuiltinClose => 'Thread als erledigt markieren';

  @override
  String get commandStatusTitle => 'Lauf und Sitzung';

  @override
  String get commandStatusNoRun => 'Kein Lauf aktiv';

  @override
  String get commandResumeTitle => 'Sitzung fortsetzen';

  @override
  String get commandResumeEmpty => 'Keine früheren Sitzungen';

  @override
  String get sessionAgentBackend => 'Backend';

  @override
  String get sessionAgentModel => 'Modell';

  @override
  String get sessionAgentSession => 'Sitzung';

  @override
  String get sessionAgentProject => 'Projekt';

  @override
  String get sessionAgentContext => 'Kontext';

  @override
  String get sessionAgentThreadCost => 'Kosten des Threads';

  @override
  String get sessionAgentRuns => 'Läufe';

  @override
  String sessionAgentRunsMore(int count) {
    return '$count+';
  }

  @override
  String sessionAgentEarlier(int count) {
    return 'Frühere Sitzungen ($count)';
  }

  @override
  String commandNotAvailable(String command) {
    return '$command ist hier nicht verfügbar; nichts wurde gesendet';
  }

  @override
  String get shortcutListTitle => 'Tastenkürzel';

  @override
  String get shortcutListClose => 'Schließen';

  @override
  String get shortcutListDisabled => 'Noch nicht verfügbar';

  @override
  String get shortcutQuickSwitcher => 'Schnellwechsel';

  @override
  String get shortcutSearch => 'Suche';

  @override
  String get shortcutSearchInThread => 'Im Thread suchen';

  @override
  String get shortcutNewPost =>
      'Neuer Beitrag oder neuer Chat im aktuellen Forum';

  @override
  String get shortcutFocusInput => 'Zum Eingabefeld';

  @override
  String get shortcutChannel => 'Nächster oder vorheriger Kanal';

  @override
  String get shortcutUnreadChannel =>
      'Nächster oder vorheriger ungelesener Kanal';

  @override
  String get shortcutColumn =>
      'Spalte wechseln (Seitenleiste, Liste, Verlauf, Sitzung)';

  @override
  String get shortcutMarkRead => 'Thread als gelesen markieren';

  @override
  String get shortcutMarkUnread =>
      'Als ungelesen markieren (ab einer Nachricht)';

  @override
  String get shortcutDone => 'Thread als erledigt markieren oder wieder öffnen';

  @override
  String get shortcutStopRun => 'Lauf im aktuellen Thread stoppen';

  @override
  String get shortcutLiveBox => 'Live-Box auf- und zuklappen';

  @override
  String get shortcutSessionColumn => 'Sitzungsspalte ein- und ausblenden';

  @override
  String get shortcutEditLast => 'Letzte eigene Nachricht bearbeiten';

  @override
  String get shortcutSend => 'Senden / neue Zeile';

  @override
  String get shortcutRunningNow => 'Läuft gerade';

  @override
  String get shortcutMentions => 'Erwähnungen';

  @override
  String get shortcutSettings => 'Einstellungen';

  @override
  String get shortcutList => 'Liste der Tastenkürzel';

  @override
  String get shortcutHostsTerminal => 'Rechner und Terminal öffnen';

  @override
  String get shortcutKeyCmd => 'Cmd';

  @override
  String get shortcutKeyCtrl => 'Strg';

  @override
  String get shortcutKeyOption => 'Option';

  @override
  String get shortcutKeyAlt => 'Alt';

  @override
  String get shortcutKeyShift => 'Umschalt';

  @override
  String get shortcutKeyArrow => 'Pfeil';

  @override
  String get shortcutKeyUp => 'oben';

  @override
  String get shortcutKeyDown => 'unten';

  @override
  String get shortcutKeyEscape => 'Esc';

  @override
  String get shortcutKeyTab => 'Tab';

  @override
  String get shortcutKeyEnter => 'Eingabe';

  @override
  String get shortcutKeyClick => 'Klick';

  @override
  String get shortcutKeyThen => 'dann';

  @override
  String get shortcutKeyOr => 'oder';

  @override
  String get shortcutInEmptyField => 'im leeren Feld';

  @override
  String get shortcutReversible => '(umkehrbar)';

  @override
  String get menuGo => 'Gehe zu';

  @override
  String get menuThread => 'Thread';

  @override
  String get menuWindow => 'Fenster';

  @override
  String get menuHelp => 'Hilfe';

  @override
  String get threadSearchTitle => 'In diesem Thread suchen';

  @override
  String get phoneTabSpaces => 'Bereiche';

  @override
  String get phoneTabMentions => 'Erwähnungen';

  @override
  String get phoneTabRunning => 'Läuft';

  @override
  String get phoneTabMe => 'Ich';

  @override
  String get phoneBack => 'Zurück';

  @override
  String phoneBackTo(String name) {
    return 'Zurück zu $name';
  }

  @override
  String get phoneMoreActions => 'Weitere Aktionen';

  @override
  String phoneOfflineWaiting(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: 'Offline. $count Änderungen warten',
      one: 'Offline. 1 Änderung wartet',
      zero: 'Offline',
    );
    return '$_temp0';
  }

  @override
  String get phoneSearchFilters => 'Filter';

  @override
  String get componentsOpenForm => 'Ausfüllen';

  @override
  String get pushChannelMentions => 'Erwähnungen';

  @override
  String get pushChannelMentionsDescription =>
      'Wenn dich jemand erwähnt oder dir direkt schreibt';

  @override
  String get pushChannelQuestions => 'Fragen';

  @override
  String get pushChannelQuestionsDescription =>
      'Wenn ein Agent dich fragt, ein neues Gerät von dir auf Freigabe wartet oder die Verwaltung dich braucht';

  @override
  String get pushChannelMessages => 'Alle Nachrichten';

  @override
  String get pushChannelMessagesDescription =>
      'Neue Nachrichten in jedem Kanal und Thread, dem du mit Alle folgst';

  @override
  String get pushChannelRuns => 'Laufergebnisse';

  @override
  String get pushChannelRunsDescription =>
      'Wenn ein von dir gestarteter Lauf endet oder fehlschlägt';

  @override
  String get adminNavStatistics => 'Statistik';

  @override
  String get statsTitle => 'Statistik';

  @override
  String get statsRangeLabel => 'Zeitraum';

  @override
  String get statsRangeDay => 'Letzte 24 Stunden';

  @override
  String get statsRangeWeek => 'Letzte 7 Tage';

  @override
  String get statsRangeMonth => 'Letzte 30 Tage';

  @override
  String get statsRangeQuarter => 'Letzte 90 Tage';

  @override
  String get statsRangeCustom => 'Eigener Zeitraum';

  @override
  String statsRangeCustomValue(String from, String to) {
    return '$from bis $to';
  }

  @override
  String get statsScopeLabel => 'Bereich';

  @override
  String get statsScopeAll => 'Alle Bereiche';

  @override
  String statsScopeRemoved(String name, String reason) {
    return '$name wird nicht angeboten: $reason';
  }

  @override
  String get statsOpenTelemetry => 'Telemetrie-Dashboard öffnen';

  @override
  String get statsLoading => 'Statistik wird geladen';

  @override
  String get statsEmpty => 'Keine Daten in diesem Zeitraum';

  @override
  String get statsCurrencyLabel => 'Währung';

  @override
  String get statsCurrencyRequired =>
      'Der Zeitraum hat Kosten in mehreren Währungen. Die Kosten zeigen jeweils eine Währung.';

  @override
  String get statsSectionRuns => 'Lauf-Ergebnisse';

  @override
  String get statsSectionDuration => 'Laufdauer je Agent';

  @override
  String get statsDurationNote =>
      'Die Werte des letzten Abschnitts je Agent; die Tabelle nennt jeden Abschnitt.';

  @override
  String get statsSectionCostsAgent => 'Kosten je Agent';

  @override
  String get statsSectionCostsSpace => 'Kosten je Bereich';

  @override
  String get statsSectionFailures => 'Fehlergründe';

  @override
  String get statsSectionTools => 'Tool-Aufrufe je Tool';

  @override
  String get statsSectionErrors => 'Fehlerquoten';

  @override
  String get statsSectionUsage => 'Nutzung je Bereich';

  @override
  String get statsColumnPeriod => 'Abschnitt';

  @override
  String get statsColumnAgent => 'Agent';

  @override
  String get statsColumnSpace => 'Bereich';

  @override
  String get statsColumnRuns => 'Lauf-Anzahl';

  @override
  String get statsColumnMedian => 'Median (p50)';

  @override
  String get statsColumnP95 => 'p95';

  @override
  String get statsColumnMode => 'Modus';

  @override
  String get statsColumnAmount => 'Betrag';

  @override
  String get statsColumnReason => 'Grund';

  @override
  String get statsColumnCount => 'Anzahl';

  @override
  String get statsColumnTool => 'Tool';

  @override
  String get statsColumnCalls => 'Aufrufe';

  @override
  String get statsColumnFailed => 'Fehlgeschlagen';

  @override
  String get statsColumnMetric => 'Kennzahl';

  @override
  String get statsColumnValue => 'Wert';

  @override
  String get statsColumnMessages => 'Nachrichten';

  @override
  String get statsColumnThreadsCreated => 'Neue Threads';

  @override
  String get statsColumnThreadsDone => 'Erledigte Threads';

  @override
  String get statsColumnActivePeople => 'Aktive Personen';

  @override
  String get statsColumnStorage => 'Speicher';

  @override
  String get statsFewerThanMinimum => 'weniger als die Mindestgruppe';

  @override
  String get statsErrorRunFailure => 'Lauf-Fehlerquote';

  @override
  String get statsErrorInvocationDenials => 'Abgelehnte Aufrufe';

  @override
  String get statsErrorWebhookFailure => 'Fehlgeschlagene Webhook-Zustellungen';

  @override
  String statsErrorPushFailure(String provider) {
    return 'Deaktivierte Push-Registrierungen ($provider)';
  }

  @override
  String get statsErrorScanErrors => 'Scanfehler';

  @override
  String get statsErrorScanInfected => 'Infizierte Dateien';

  @override
  String statsChartSummary(String title, String rows) {
    return '$title: $rows';
  }

  @override
  String get liveBoxGraph => 'Graph';

  @override
  String liveBoxOpenGraph(String name) {
    return 'Ablaufgraph von $name öffnen';
  }

  @override
  String get runGraphTitle => 'Ablaufgraph';

  @override
  String get runGraphAsList => 'Als Liste';

  @override
  String get runGraphBackToThread => 'Thread';

  @override
  String get runGraphBackToRunning => 'Läuft gerade';

  @override
  String runGraphStepsSummarized(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count Schritte zusammengefasst',
      one: '1 Schritt zusammengefasst',
    );
    return '$_temp0';
  }

  @override
  String get runGraphStatePending => 'Wartet';

  @override
  String runGraphTreePosition(int level, int position, int count) {
    return 'Ebene $level, $position von $count';
  }

  @override
  String get runGraphExpand => 'Schritte zeigen';

  @override
  String get runGraphCollapse => 'Schritte ausblenden';

  @override
  String runGraphAnnounce(String name, String state) {
    return '$name: $state';
  }

  @override
  String get runGraphUnavailable => 'Der Lauf konnte nicht geladen werden.';

  @override
  String get runGraphLoading => 'Lauf wird geladen';

  @override
  String runGraphTimeAxis(String start, String end) {
    return 'Zeitachse von $start bis $end';
  }

  @override
  String runGraphSubtitle(String tool) {
    return 'über $tool';
  }

  @override
  String runGraphRowLabel(String kind, String name) {
    return '$kind $name';
  }

  @override
  String get runningCards => 'Karten';

  @override
  String get runningList => 'Liste';

  @override
  String get runningGraph => 'Graph';

  @override
  String get runningFilters => 'Filter';

  @override
  String get runningFiltersDone => 'Schließen';

  @override
  String get runningLivePaused => 'Live-Details pausiert';

  @override
  String runningSubagents(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count Unteragenten',
      one: '1 Unteragent',
    );
    return '$_temp0';
  }

  @override
  String runningWorkflows(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count Workflows',
      one: '1 Workflow',
    );
    return '$_temp0';
  }

  @override
  String runningBackgroundJobs(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count Hintergrundjobs',
      one: '1 Hintergrundjob',
    );
    return '$_temp0';
  }

  @override
  String runningCardOpenGraph(String name) {
    return 'Ablaufgraph von $name öffnen';
  }

  @override
  String get emergencyEntry => 'Notfall';

  @override
  String get emergencyEntryHint =>
      'Gerät oder Freigabe widerrufen, Person sperren';

  @override
  String get meLiveEntry => 'Live';

  @override
  String get meLiveEntryHint =>
      'Live-Topologie und die anderen Ansichten der Statistik';

  @override
  String get emergencyTitle => 'Notfall';

  @override
  String get emergencyIntro =>
      'Hier stehen nur die dringenden Schritte. Freigaben anlegen, das Audit-Log und die Statistik gibt es am Desktop oder in der Web-Konsole.';

  @override
  String get emergencySearch => 'Personen suchen';

  @override
  String get emergencyGrantsEmpty => 'Keine Freigaben, die du sehen kannst.';

  @override
  String get emergencyDevicesEmpty => 'Keine Geräte.';

  @override
  String emergencyRevokeDeviceBody(String device, String person) {
    return 'Das Gerät \"$device\" von $person wird sofort gesperrt. Es wird abgemeldet und löscht seine lokalen Daten beim nächsten Kontakt.';
  }

  @override
  String emergencyRevokeGrantBody(String person, String target) {
    return 'Die Freigabe von $person auf $target wird sofort widerrufen. Der Zugriff endet jetzt.';
  }

  @override
  String emergencyDisableBody(String person) {
    return '$person wird sofort gesperrt. Alle Sitzungen von $person enden jetzt, alle Geräte werden widerrufen und jeder Lauf von $person wird abgebrochen.';
  }

  @override
  String get emergencyConfirmFaceId => 'Mit Face ID bestätigen';

  @override
  String get emergencyConfirmDeviceLock =>
      'Mit Fingerabdruck oder Displaysperre bestätigen';

  @override
  String get emergencyCancelled => 'Abgebrochen. Nichts wurde geändert.';

  @override
  String get emergencyLastOwner =>
      'Mach zuerst eine andere Person zum Owner, am Desktop oder in der Web-Konsole.';

  @override
  String get problemTerminalDeniedNotHostAssignee =>
      'Dieser Rechner ist einer anderen Person zugewiesen';

  @override
  String get problemTerminalDeniedServerHostOwnerOnly =>
      'Nur ein Owner kann auf dem Server-Rechner ein Terminal öffnen';

  @override
  String get problemTerminalDeniedI6HolderOnly =>
      'Nur die Person mit dem persönlichen Login kann auf diesem Rechner ein Terminal öffnen';

  @override
  String get problemTerminalDeniedNoSessionUser =>
      'Du hast auf diesem Rechner keinen eigenen Sitzungsbenutzer';

  @override
  String get problemTerminalDeniedSessionLimit =>
      'Die Grenze offener Terminal-Sitzungen ist erreicht';

  @override
  String get problemTerminalDeniedProfileUnknown =>
      'Dieses Profil oder dieser Arbeitsordner ist auf dem Rechner nicht verfügbar';

  @override
  String get hostsGroupTitle => 'Rechner';

  @override
  String hostRowLabel(String name, String state, int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count eigene Sitzungen',
      one: '1 eigene Sitzung',
      zero: 'keine eigene Sitzung',
    );
    return '$name, $state, $_temp0';
  }

  @override
  String get hostsLoadFailed => 'Die Rechner konnten nicht geladen werden';

  @override
  String get terminalNewSession => 'Neue Sitzung';

  @override
  String get terminalNoSessions =>
      'Keine offene Terminal-Sitzung auf diesem Rechner. Starte eine mit Neue Sitzung.';

  @override
  String get terminalChooseHost =>
      'Wähle links in der Gruppe Rechner einen Rechner.';

  @override
  String get terminalNoHosts =>
      'Du siehst noch keinen Rechner. Ein Owner koppelt Rechner und vergibt das Terminal.';

  @override
  String get terminalHostGone => 'Dieser Rechner ist für dich nicht verfügbar.';

  @override
  String get terminalSearchHint => 'Im Verlauf suchen';

  @override
  String terminalSearchMatches(int count) {
    String _temp0 = intl.Intl.pluralLogic(
      count,
      locale: localeName,
      other: '$count passende Zeilen',
      one: '1 passende Zeile',
      zero: 'Keine passende Zeile',
    );
    return '$_temp0';
  }

  @override
  String get terminalSearchClose => 'Suche schließen';

  @override
  String get shortcutListTerminal => 'Im Terminal';

  @override
  String get shortcutTerminalCopyPaste => 'Kopieren / Einfügen';

  @override
  String get shortcutTerminalNewSession => 'Neue Sitzung auf diesem Rechner';

  @override
  String get shortcutTerminalSession => 'Nächste oder vorherige Sitzung';

  @override
  String get shortcutTerminalDetach => 'Sitzung trennen (läuft weiter)';

  @override
  String get shortcutTerminalSearch => 'Im Verlauf suchen';

  @override
  String get shortcutTerminalFontSize => 'Schrift größer / kleiner';

  @override
  String get shortcutKeyPlus => 'Plus';

  @override
  String get shortcutKeyMinus => 'Minus';

  @override
  String get shortcutKeyLeft => 'links';

  @override
  String get shortcutKeyRight => 'rechts';

  @override
  String get voicePhoneHoldHint =>
      'Halten zum Aufnehmen. Nach links wischen verwirft, nach oben wischen sperrt.';

  @override
  String get voicePhoneSwipeHint =>
      'Nach links wischen zum Verwerfen, nach oben zum Sperren';

  @override
  String voicePhoneRemaining(String remaining) {
    return 'Noch $remaining';
  }

  @override
  String voicePhoneElapsed(String elapsed) {
    return 'Aufnahme $elapsed';
  }

  @override
  String voicePhoneLimitNotice(String remaining) {
    return 'Die Aufnahme endet in $remaining';
  }

  @override
  String voicePhoneTooLong(String limit) {
    return 'Die Aufnahme ist länger als die Grenze von $limit und kann nicht gesendet werden';
  }

  @override
  String get voicePhonePlay => 'Aufnahme abspielen';

  @override
  String get voicePhonePause => 'Aufnahme anhalten';

  @override
  String get voicePhoneDeniedIos =>
      'Der Zugriff auf das Mikrofon ist aus. Erlaube ihn in den Einstellungen: Apps, diese App, Mikrofon.';

  @override
  String get voicePhoneDeniedAndroid =>
      'Der Zugriff auf das Mikrofon ist aus. Erlaube ihn in den Systemeinstellungen: Apps, diese App, Berechtigungen, Mikrofon.';

  @override
  String get photoCameraDeniedIos =>
      'Der Zugriff auf die Kamera ist aus. Erlaube ihn in den Einstellungen: Apps, diese App, Kamera.';

  @override
  String get photoCameraDeniedAndroid =>
      'Der Zugriff auf die Kamera ist aus. Erlaube ihn in den Systemeinstellungen: Apps, diese App, Berechtigungen, Kamera.';

  @override
  String get photoLibraryDenied =>
      'Der Zugriff auf Fotos ist aus. Erlaube ihn in den Systemeinstellungen für diese App.';

  @override
  String photoUnsupported(String name) {
    return '$name konnte nicht in JPEG umgewandelt werden';
  }

  @override
  String get photoFailed =>
      'Das Foto konnte nicht aufgenommen oder ausgewählt werden';

  @override
  String get composerAttachCamera => 'Foto aufnehmen';

  @override
  String get composerAttachLibrary => 'Fotomediathek';

  @override
  String get composerAttachFile => 'Datei';

  @override
  String get permissionMicrophoneUsage =>
      'Das Mikrofon nimmt die Sprachnachrichten auf, die du sendest.';

  @override
  String get permissionCameraUsage =>
      'Die Kamera nimmt die Fotos auf, die du sendest.';

  @override
  String get permissionPhotoLibraryUsage =>
      'Fotos, die du in der Fotoauswahl des Systems wählst, werden als Anhänge gesendet; die App liest keine anderen Fotos.';

  @override
  String get settingsSectionSecurity => 'Sicherheit';

  @override
  String get settingsAppLock => 'App-Sperre';

  @override
  String get settingsAppLockHint =>
      'Fragt bei jedem Start und nach einer Zeit ohne Nutzung nach Biometrie oder dem Gerätecode.';

  @override
  String get settingsAppLockRequired =>
      'Ein Owner dieses Servers verlangt die App-Sperre.';

  @override
  String get settingsAppLockNoCredential =>
      'Richte zuerst in den Systemeinstellungen einen Gerätecode ein (PIN, Muster oder Passwort).';

  @override
  String get settingsAppLockFailed =>
      'Die App-Sperre bleibt aus: Die Bestätigung ist nicht gelungen.';

  @override
  String appLockReason(String productName) {
    return '$productName entsperren';
  }

  @override
  String appLockNoCredentialTitle(String productName) {
    return '$productName braucht einen Gerätecode';
  }

  @override
  String get appLockNoCredentialBody =>
      'Die App-Sperre ist an, aber dieses Gerät hat keinen Gerätecode. Öffne die Systemeinstellungen, richte einen Gerätecode ein (PIN, Muster oder Passwort) und kehre dann zur App zurück.';

  @override
  String get appLockCheckAgain => 'Erneut prüfen';

  @override
  String get permissionFaceIdUsage =>
      'Face ID entsperrt die App, wenn die App-Sperre an ist.';

  @override
  String get statusTitle => 'Statusseite';

  @override
  String get statusNavEntry => 'Statusseite';

  @override
  String get adminNavStatus => 'Statusseite';

  @override
  String get statusUnreachable => 'Server nicht erreichbar';

  @override
  String get statusUnreachableHint =>
      'Die Statusseite ist Teil des Servers, deshalb kann kein aktueller Zustand angezeigt werden. Prüfe die Verbindung dieses Geräts; der Eigentümer wird auf einem eigenen Weg alarmiert.';

  @override
  String statusOverall(String state) {
    return 'Gesamtzustand: $state';
  }

  @override
  String get statusStateOperational => 'In Betrieb';

  @override
  String get statusStateDegraded => 'Eingeschränkt';

  @override
  String get statusStatePartialOutage => 'Teilausfall';

  @override
  String get statusStateMajorOutage => 'Ausfall';

  @override
  String get statusStateMaintenance => 'Wartung';

  @override
  String get statusMaintenanceMode =>
      'Die Instanz wird gewartet. Lesen geht; Änderungen warten, bis die Wartung endet.';

  @override
  String get statusComponents => 'Komponenten';

  @override
  String get statusNoComponents => 'Noch keine Komponenten.';

  @override
  String statusComponentSemantics(String name, String state) {
    return '$name: $state';
  }

  @override
  String statusAgentComponent(String name) {
    return '$name (Agent)';
  }

  @override
  String get statusOpenIncidents => 'Offene Störungen';

  @override
  String get statusNoIncidents => 'Keine offenen Störungen.';

  @override
  String get statusMaintenanceWindows => 'Wartungsfenster';

  @override
  String get statusHistory => 'Verlauf';

  @override
  String get statusKindIncident => 'Störung';

  @override
  String get statusKindMaintenance => 'Wartung';

  @override
  String get statusImpactNone => 'Keine Auswirkung';

  @override
  String get statusImpactMinor => 'Geringe Auswirkung';

  @override
  String get statusImpactMajor => 'Große Auswirkung';

  @override
  String get statusImpactCritical => 'Kritische Auswirkung';

  @override
  String get statusIncidentScheduled => 'Geplant';

  @override
  String get statusIncidentInvestigating => 'Wird untersucht';

  @override
  String get statusIncidentIdentified => 'Ursache gefunden';

  @override
  String get statusIncidentMonitoring => 'Wird beobachtet';

  @override
  String get statusIncidentResolved => 'Behoben';

  @override
  String statusStarted(String time) {
    return 'Begonnen $time';
  }

  @override
  String statusResolvedAt(String time) {
    return 'Behoben $time';
  }

  @override
  String statusAffects(String names) {
    return 'Betrifft $names';
  }

  @override
  String get statusAddUpdate => 'Aktualisierung hinzufügen';

  @override
  String get statusAddComponent => 'Komponente hinzufügen';

  @override
  String get statusNewIncident => 'Störung oder Wartung eröffnen';

  @override
  String get statusEdit => 'Bearbeiten';

  @override
  String get statusDelete => 'Löschen';

  @override
  String statusDeleteTitle(String name) {
    return '$name löschen?';
  }

  @override
  String get statusDeleteBody =>
      'Die Komponente verschwindet von der Statusseite. Eine Standardkomponente kommt mit der nächsten Prüfrunde zurück.';

  @override
  String statusEditComponentTitle(String name) {
    return '$name bearbeiten';
  }

  @override
  String get statusFormBody =>
      'Alle Personen sehen die Änderung sofort auf der Statusseite.';

  @override
  String get statusSave => 'Speichern';

  @override
  String get statusPublish => 'Veröffentlichen';

  @override
  String get statusSaved => 'Die Statusseite ist aktualisiert.';

  @override
  String get statusComponentName => 'Name';

  @override
  String get statusComponentDescription => 'Beschreibung';

  @override
  String get statusComponentState => 'Zustand';

  @override
  String get statusHealthSource => 'Automatische Prüfung';

  @override
  String get statusHealthSourceNone => 'Keine, von Hand gesetzt';

  @override
  String get statusPlaceAfter => 'Einordnen nach';

  @override
  String get statusPlaceEnd => 'Am Ende';

  @override
  String get statusPlaceKeep => 'Platz behalten';

  @override
  String get statusNameRequired => 'Gib einen Namen ein.';

  @override
  String statusNameTooLong(int max) {
    return 'Höchstens $max Zeichen.';
  }

  @override
  String get statusIncidentTitle => 'Titel';

  @override
  String get statusIncidentImpact => 'Auswirkung';

  @override
  String get statusIncidentStateLabel => 'Zustand';

  @override
  String get statusIncidentComponents => 'Betroffene Komponenten';

  @override
  String get statusIncidentText => 'Text';

  @override
  String get statusMarkdownHint =>
      'Markdown wie in Nachrichten: Hervorhebung, Listen, Links.';

  @override
  String statusUpdateTitle(String title) {
    return 'Aktualisierung zu $title';
  }

  @override
  String get statusPublicTitle => 'Status dieser Instanz';

  @override
  String get statusPublicOff =>
      'Diese Instanz zeigt ihren Status nur nach der Anmeldung.';

  @override
  String get statusPublicLink => 'Status dieser Instanz';

  @override
  String get statusBack => 'Zurück';
}
