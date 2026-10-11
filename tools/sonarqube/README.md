# Lokaler SonarQube

| Datei | Zweck |
|---|---|
| `compose.yaml` | SonarQube Community Build und PostgreSQL; Web-Oberfläche nur auf `127.0.0.1`, Datenbank nur im internen Netz; Images per Tag und Digest gepinnt |
| `quality_sync.py` | Gate und Profile aus `quality/sonar/` in einen SonarQube-Server einspielen (`apply`) oder vergleichen (`check`); Token nur aus `SONAR_TOKEN` |
| `tests/` | Tests gegen einen Test-Server, der die genutzten Web-API-Aufrufe nachbildet |
| `.env` | wird von `make sonar-up` mit einem zufälligen Datenbank-Passwort angelegt; git-ignoriert |

Ablauf Schritt für Schritt: [docs/quality/sonarqube-local.md](../../docs/quality/sonarqube-local.md).

```sh
make sonar-up             # starten und auf UP warten
make sonar-config         # Gate und Profile einspielen (Token mit Admin-Rechten)
make sonar-config-check   # Gleichheit mit quality/sonar/ prüfen
make sonar                # analysieren (ab erstem Produkt-Code)
make sonar-down           # stoppen, Daten bleiben
```
