# Modul hetzner

Bothy auf Hetzner Cloud.

## Eingaben

Gemeinsame Eingaben siehe [variables.tf](variables.tf). Zusätzlich: `location`, `server_type`.

## Geplante Ressourcen

| Teil | Inhalt |
|---|---|
| Netz | privates Netz, Firewall: öffentlich nur der Rand (HTTPS beim Reverse-Proxy); SSH nur mit Schlüsseln aus `admin_ssh_public_keys` und nur aus Netzen, die der Betreiber freigibt |
| Host | Cloud-Server `server_type` in `location`, Volume für Docker-Daten getrennt vom Systemdatenträger |
| Dateien | `object_storage_mode = s3`: Bucket in Hetzner Object Storage (S3-kompatibel, Versionierung und Object Lock vorhanden; https://docs.hetzner.com/storage/object-storage, abgerufen 2026-10-10) [belegt]. Zugangsschlüssel legt die Cloud Console an, nicht die API [belegt, gleiche Quelle]; darum liefert der Owner sie als Geheimnis |
| Off-site | zweiter Bucket mit Object Lock an einem anderen Standort |
| Datenbank | Container auf dem Host (kein verwalteter PostgreSQL-Dienst angenommen) |
| Einrichtung | Aufruf von `generic-vm` |

## Hinweise

1. Object Storage gibt es laut Hetzner nur an europäischen Standorten [belegt, gleiche Quelle].
2. Laut einer Anleitung der Community unterstützt Hetzner Object Storage nicht die neueste
   Signaturversion von S3, die manche Bibliotheken als Standard nutzen [vermutet, nur über
   Suchergebnis]. Der S3-Treiber (Kapitel 12) testet gegen Hetzner, bevor das Modul `s3` anbietet.
