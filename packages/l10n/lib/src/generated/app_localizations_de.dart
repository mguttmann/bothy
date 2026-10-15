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
}
