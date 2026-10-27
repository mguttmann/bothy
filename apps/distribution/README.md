# App distribution

The publisher values of the apps and the templates of their update files
([18-release.md](../../docs/spec/18-release.md) 8, [ADR 0037](../../docs/adr/0037-app-distribution-and-auto-update-channels.md),
R-RL-05). The apps are white label: every organization that ships them builds and signs them with
its own accounts, and this folder holds what it has to fill in.

| File | Content |
|---|---|
| `distribution.yaml` | the template: every value of a publisher, with placeholders |
| `distribution.schema.json` | its JSON Schema (draft 2020-12) |
| `templates/appcast.xml`, `templates/appcast-item.xml` | the Sparkle appcast of the macOS app and one entry of it |
| `templates/channel.appinstaller` | the App Installer file of one channel of the Windows app |
| `templates/AppxManifest.xml` | the MSIX manifest of the Windows app |

The release steps that read them are in [tools/distribution](../../tools/README.md)
(`appdist.py`, `apple.py`, `windows.py`, `ios.py`, `android.py`), started by the targets of
[apps/client/Makefile](../client/README.md#release-builds).

## Filling in distribution.yaml

Copy the template into the ops repository of the instance and replace every value. A release
stops on any value the template still holds (`PLACEHOLDERS` in `tools/distribution/appdist.py`)
and names the key; so does a missing or malformed value, checked against the schema. Check a copy
with:

```sh
uv run --project tools --locked python tools/distribution/appdist.py validate --distribution <file> --release
```

| Key | Meaning |
|---|---|
| `publisher.name` | the publisher as users see it (the MSIX publisher display name) |
| `bundle_id` | the bundle identifier on Apple platforms; the release build writes it into the build branding, so the App Group, the keychain groups and the Windows toast activator (D-M4-30-2) follow it |
| `apple.team_id` | the team of the Developer ID certificate; the release checks the signature against it |
| `apple.apns_topic` | the APNs topic of the instance's push (R-Push), as a rule the bundle identifier |
| `apple.sparkle.feed_url` | the appcast the macOS app checks (`SUFeedURL`), one for both channels |
| `apple.sparkle.download_url` | the address below which the archives of the appcast are published (ends with `/`) |
| `apple.sparkle.public_ed_key` | the public EdDSA key of archives and appcast (`SUPublicEDKey`) |
| `apple.sparkle.phased_rollout_interval` | seconds between the groups of a phased rollout; 0 offers an update to everyone at once |
| `windows.package_name` | the name of the MSIX package identity |
| `windows.publisher` | the subject of the signing certificate; it must equal the certificate's subject exactly |
| `windows.update_url` | the address below which the MSIX files and `stable.appinstaller` and `beta.appinstaller` are published (ends with `/`) |
| `windows.hours_between_update_checks` | how often App Installer checks at the start of the app (0 to 255) |
| `windows.timestamp_url` | optional RFC 3161 timestamp server of the certificate authority |
| `ios.update_url` | the page the iOS app opens as its update path: the public TestFlight link until the App Store listing exists, then the App Store page (M7-13) |
| `android.package_name` | the application ID of the Android app (M7-13) |
| `android.update_url` | the page the Android app opens as its update path, as a rule its Play page (M7-13) |
| `android.stable_track` | the closed test track of the channel stable in the Play Console; never `internal` (beta) or `production` (M7-13) |
| `firebase.project_id` | the Firebase project of the instance's push (M7-13) |
| `crash_reports.target` | optional target of crash reports; empty sends none |

Secrets never go into this file. The release jobs read the file from the environment variable
`DISTRIBUTION_YAML`, the build branding of the instance from `BRANDING_YAML`, and the signing material from the secrets of the environments
`release-apple`, `release-windows` and `release-android` ([release.md](../../docs/operations/release.md) 4).
The update path of each build (`update_url` of the build branding) comes from this file: iOS
`ios.update_url`, Android `android.update_url`, macOS `apple.sparkle.download_url` (after
Sparkle's own check), Windows `windows.update_url` with `stable.appinstaller`.

## Keys

Create the EdDSA key pair of Sparkle once and keep the private half only as the secret
`SPARKLE_ED_PRIVATE_KEY`: Sparkle's `generate_keys` stores it in the login keychain of the person
who runs it and `generate_keys -x <file>` exports it; any other way to make an ed25519 key works
too, the secret is the base64 of the 32-byte seed. The public half (base64 of 32 bytes) is
`apple.sparkle.public_ed_key`. A release checks that both belong together.

## Channels and promotion

A release publishes into the channel beta; the same build moves to stable without a rebuild
(18 5 rule 1):

- macOS: the appcast entry of a new build carries `sparkle:channel` beta. Promotion removes the
  element and signs the appcast again:
  `appdist.py promote-appcast --appcast <file> --build <build number> --ed-key-file - --distribution <file>`.
  A release's own appcast holds only its entry;
  `appdist.py appcast-merge --appcast <published> --add <release appcast> ...` adds it to the
  published one.
- Windows: `beta.appinstaller` points at the new MSIX; promotion writes `stable.appinstaller`
  for the same MSIX: `appdist.py promote-appinstaller --beta <file> --stable <file> --distribution <file>`.
- iOS: a release adds the build to the TestFlight group `beta`; promotion adds the same build to
  `stable`: `ios.py promote --build <build number> --distribution <file>` (with the
  `APPLE_NOTARY_*` key in the environment).
- Android: a release puts the bundle on the internal track; promotion releases the same version
  code on `android.stable_track`: `android.py promote --version-code <n> --distribution <file>`
  (with `PLAY_SERVICE_ACCOUNT_JSON` in the environment).

The build number is `X * 1000000 + Y * 1000 + Z` of the version `X.Y.Z`; app versions carry no
pre-release. Apps are corrected forward only (R-RL-07): a published build is never replaced, a
fix is a higher version.

Hosting the appcast, the archives and the `.appinstaller` files is the publisher's update host;
the release jobs only produce the files.
