# Betrieb mit Tailscale

Status: Entwurf M0. Tailscale ist eine dokumentierte Option, kein Zwang (B9). Regeln:
[16-betrieb.md](../spec/16-betrieb.md) 4.3. Identitätsanbieter `trusted_header`: Kapitel 08.

## 1. Wann diese Option

Die Instanz ist nur im Tailnet erreichbar; Tailscale liefert TLS und die Identität der Menschen.
Es gibt keinen öffentlichen Hostnamen und keinen eigenen Reverse-Proxy.

Folgen, die vorher klar sein müssen:

1. Passkeys in nativen Apple-Apps brauchen eine öffentlich erreichbare Associated Domain; eine
   reine Tailnet-Domain reicht im Normalbetrieb nicht (Baseline R-Passkeys, Beleg
   https://developer.apple.com/documentation/xcode/supporting-associated-domains, abgerufen
   2026-10-10) [belegt]. Step-up und Gerätebindung laufen darum über den Geräteschlüssel.
2. Die Notification Service Extension holt den Inhalt eines Push über die normale API. Ist das VPN
   auf dem Gerät gerade nicht verbunden, bleibt der neutrale Platzhalter stehen (R-Push).
3. Getaggte Geräte und Funnel liefern keine Identitäts-Kopfzeilen
   (https://tailscale.com/kb/1312/serve, abgerufen 2026-10-10) [belegt]. Agenten-Adapter melden
   sich darum mit ihrem API-Token an, nicht über Kopfzeilen.
4. Gerätenamen mit HTTPS-Zertifikat erscheinen im öffentlichen Certificate-Transparency-Log
   (https://tailscale.com/kb/1153/enabling-https, abgerufen 2026-10-10) [belegt]. `TS_HOSTNAME`
   neutral wählen.
5. Der Tarif des Tailnets (Zahl der Menschen, gewerbliche Nutzung) ist eine Entscheidung des
   Owners (https://tailscale.com/pricing, abgerufen 2026-10-10) [belegt].

## 2. Aufbau im Compose-Stack

Profil `tailscale` statt `direct`:

1. Der Dienst `tailscale` (Image `tailscale/tailscale`) meldet sich mit `TS_AUTHKEY` einmal an und
   hält seinen Zustand im Volume `tailscale-state` (`TS_AUTH_ONCE=true`).
2. `TS_SERVE_CONFIG` (in `compose.yaml` als `configs`-Eintrag) leitet HTTPS 443 an
   `http://127.0.0.1:8080` weiter und verbietet Funnel.
3. `api-tailscale` teilt den Netz-Namespace des Sidecars (`network_mode: service:tailscale`) und
   lauscht öffentlich nur auf `127.0.0.1:8080`. Kein anderer Container und kein anderer Host
   erreicht diesen Listener.
4. Der Sidecar läuft mit Kernel-Netz (`TS_USERSPACE=false`, Gerät `/dev/net/tun`, Capability
   `NET_ADMIN`), wie im Beispiel von Tailscale. Dann ist Punkt 3 eine Eigenschaft des Kernels:
   Pakete, die über `tailscale0` ankommen, erreichen keinen Socket, der nur an `127.0.0.1`
   gebunden ist. Ob der Userspace-Modus eingehende Verbindungen an Loopback-Ports weiterreicht,
   ist nicht dokumentiert (https://tailscale.com/kb/1112/userspace-networking, abgerufen
   2026-10-10, beschreibt nur ausgehenden Verkehr) [belegt]; darum kein Userspace-Modus.
   Ob die Docker-VM auf dem Mac `/dev/net/tun` bereitstellt, prüft der Portabilitätstest
   [vermutet].
5. Der interne Listener (`:8081`) ist für Dienste im Stack unter dem Namen `api` erreichbar (Alias
   am Sidecar). Er vertraut keinen Kopfzeilen (R-OP-01); über das Tailnet sperrt ihn die
   Richtlinie aus Abschnitt 3.

Belege für die Variablen und das Muster (abgerufen 2026-10-10):
https://tailscale.com/docs/features/containers/docker/docker-params und das Beispiel
https://github.com/tailscale-dev/docker-guide-code-examples (`07-ts-actual-server`) [belegt].

Kommandozeile ohne Container (z. B. Host-Installation ohne Compose):

```
tailscale serve --bg --https=443 127.0.0.1:8080
tailscale serve status
tailscale serve --https=443 off
```

Flags laut https://tailscale.com/docs/reference/tailscale-cli/serve (abgerufen 2026-10-10)
[belegt]; die Kombination in der ersten Zeile ist zusammengesetzt, nicht wörtlich belegt
[abgeleitet]. Ohne Container muss der Server selbst nur auf `127.0.0.1` lauschen, und kein anderer
Prozess des Hosts darf Kopfzeilen an diesen Port schicken können.

## 3. Tailnet-Richtlinie (Pflicht)

1. Das Bothy-Gerät bekommt einen Tag (z. B. `tag:bothy`), damit es nicht an einem Menschen hängt.
2. Grants erlauben auf `tag:bothy` nur `tcp:443` für die Gruppen, die Bothy nutzen. Alle anderen
   Ports sind verboten.
3. Grund: Zweite Sperre neben dem Kernel-Netz. Erreicht ein Tailnet-Gerät nur 443, landet jede
   Anfrage bei `tailscale serve`, das eingehende Identitäts-Kopfzeilen entfernt und selbst setzt.
   Der Portabilitätstest prüft, dass außer 443 nichts antwortet.
4. Funnel bleibt aus (`AllowFunnel: false`).

## 4. Identität

| Kopfzeile | Verwendung |
|---|---|
| `Tailscale-User-Login` | Subjekt in `user_identities` des Anbieters `trusted_header` |
| `Tailscale-User-Name` | Anzeigename bei der Anlage, nur wenn `allow_provisioning` |
| `Tailscale-User-Profile-Pic` | ignoriert (der Server lädt keine fremden URLs) |
| `Tailscale-App-Capabilities` | nicht ausgewertet; Rechte vergibt nur `authorize` (B7) |

Ohne `Tailscale-User-Login` (getaggtes Gerät, Funnel) ist die Anfrage unangemeldet. Nicht-ASCII
kann RFC-2047-kodiert sein; der Server dekodiert vor dem Vergleich.

WebSocket über `tailscale serve`: nicht in der Dokumentation belegt [vermutet]. Der
Portabilitätstest mit Profil `tailscale` prüft das Gateway ausdrücklich.

## 5. Betrieb

| Aufgabe | Weg |
|---|---|
| Zertifikat | `tailscaled` erneuert es bei `serve` selbst; ein Fehler zeigt sich als Einbruch im SLO `bothy-sync/reconnect` |
| Gerät verloren, neu anmelden | Volume `tailscale-state` löschen, neuen `TS_AUTHKEY` setzen, altes Gerät im Tailnet entfernen |
| Update | `BOTHY_TAILSCALE_VERSION` anheben, `docker compose up -d tailscale` |
| Fehlersuche | `docker compose exec tailscale tailscale status`, `tailscale serve status` |
