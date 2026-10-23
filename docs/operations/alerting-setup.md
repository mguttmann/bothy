# Setting up alerts

Guide for the owner of an instance (brief M7-20). Background and the routes:
[docs/observability/alerting.md](../observability/alerting.md) sections 1 and 2. The secrets:
[configuration.md](configuration.md) section 5.

Alertmanager (Compose profile `observability`) sends every `page` and `ticket` alert to the alert
channel "Operations" of the instance, every `page` alert in addition to an external on-call
channel, and the always-firing `Watchdog` to an external heartbeat service. If the heartbeat stops
arriving there, that service reports a dead alert path. Which external channel and which heartbeat
service you use is your choice; Bothy needs no account anywhere. Until all three URLs are set up,
Alertmanager does not start and says which file is missing (`docker compose logs alertmanager`).

## 1. Channel, app and incoming webhook

These steps need a session with step-up: the console or the desktop app, signed in as an owner.
An API token cannot step up, so no script can do them for you.

1. Create a space and in it the text channel **Operations**. Give no one else access: only the
   owners and their deputies read it.
2. Install an app for the alerts (for example with the name "Alertmanager") with the scope
   `messages:write`, and grant it `write` on the channel Operations.
3. Create an incoming webhook of that app in the channel. Copy the URL it shows once; it holds
   the token. It looks like `https://chat.example.com/api/v1/hooks/<id>/<token>`.

Messages from the webhook have the origin `app` and never trigger an agent.

## 2. Write the URLs into the secret files

In the folder of the installation (`infra/compose`), run the setup script once per route. It
reads the URL from standard input (hidden when you paste it into a terminal), never prints it and
writes it into `secrets/` with mode `0600`. Its only output is the status and the fingerprint:

```sh
sh alert-setup.sh channel       # paste the URL of the incoming webhook
sh alert-setup.sh external    # for example https://oncall.example.com/hook
sh alert-setup.sh heartbeat   # for example https://heartbeat.example.com/ping
```

```text
created alertmanager_bothy_webhook_url sha256:0123456789ab
```

For `channel` the script keeps the path of the webhook and replaces the public address by the
internal listener `http://api:8081`, because Alertmanager reaches the server only inside the
Compose network (`--internal-base` names another one). Running it again with the same URL
reports `kept` and changes nothing; a new URL reports `updated`. Exit codes: `0` done, `1` the
file cannot be written, `2` the input is not a URL of the expected form (the message never shows
the value).

## 3. Apply

```sh
docker compose run --rm secrets-init
docker compose up -d --force-recreate --no-deps alertmanager
```

`secrets-init` copies the files into the volume `secrets` and prints the same fingerprints
(`adopted` or `updated`), so you can compare them with the output of step 2. Then
`docker compose logs alertmanager` shows the start, and the next `ticket` or `page` alert appears
in the channel Operations; the heartbeat service receives the Watchdog every few minutes.

## 4. Rotate or remove

Revoke the incoming webhook in the console, create a new one and repeat steps 2 and 3 with
`channel`. The URLs of the external services change the same way.

## Test

`make alert-e2e` runs the whole path against a local test stack with stubs for the external
services ([gates.md](../quality/gates.md)). A real external on-call or heartbeat service is not
part of that test.
