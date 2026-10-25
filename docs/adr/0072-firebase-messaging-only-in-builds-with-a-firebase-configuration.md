---
status: accepted
date: 2026-10-25
decision-makers: Maintainer
---

# Firebase messaging only in builds with a Firebase configuration

## Context and Problem Statement

Android push goes through Firebase Cloud Messaging (13 5.2; ADR 0031; ADR 0056 item 3): the app
needs a `FirebaseMessagingService` and an FCM registration token. The Firebase messaging library
is Apache-2.0, but it depends on Google Play services client libraries
(`play-services-basement`, `play-services-base`, `play-services-tasks`,
`play-services-cloud-messaging`, `play-services-stats`) under Google's own terms, which
`quality/licenses/allowed.txt` does not list; docs/quality/licenses.md allows a proprietary
dependency only with an ADR. An instance without a Firebase project gets no push on Android
(15 10.2), and the product repository holds no instance's Firebase project. The question is
which builds link the Firebase and Play services code.

## Decision Drivers

* The neutral build of the repository and every build without push ship only allowed licenses.
* FCM cannot be reached without Play services on the device and these libraries in the app.
* One code base for the push service, tested in both builds (P-09).
* No Firebase project, API key or token in the repository (rule 8 of the briefs, 02 6).

## Considered Options

* Link Firebase only when the build configuration provides `android/app/google-services.json`.
* Always link Firebase and leave it unused without a configuration.
* No FCM; UnifiedPush or an own channel instead.

## Decision Outcome

Chosen option: link Firebase only with a configuration. `android/app/build.gradle.kts` applies
the Google services plugin, adds `firebase-messaging`, compiles `src/firebase` (the service and
the token source) and enables the service in the manifest only when the file exists, and keeps
the resolved versions of that build in `gradle-firebase.lockfile`; otherwise it compiles
`src/nofirebase`, a disabled stand-in with the same names, and links neither Firebase nor Play
services code. The operator who adds the Firebase project of an instance to a build accepts
Google's terms for that build. Automatic token creation and analytics collection are turned off
in the manifest: the token is fetched only when the app registers after the permission.

### Consequences

* Good, because the repository's build and every build without push contain no library outside
  the license list, and nothing initialises Firebase there.
* Good, because the push logic (receiver, worker, store, DPoP, poster) is the same code in both
  builds and is tested in both.
* Bad, because a build with push contains proprietary Play services libraries; the release
  record of such a build names them (ScanCode, docs/quality/licenses.md 4).
* Bad, because there are two Gradle lock files to keep current.

### Confirmation

`PushManifestTest` checks on a device that the service is enabled exactly in a build with a
Firebase configuration; `tool/run_android_tests.sh` runs the push tests in both builds (the
second with the fake configuration of `android/fixtures/google-services.json`), and the
`apps-android-26` and `apps-android-36` jobs run that script.

## Pros and Cons of the Options

### Firebase only with a configuration

* Good, because the license policy holds wherever push is not configured.
* Bad, because the build has two variants of the source set and the lock file.

### Always link Firebase

* Good, because there is one build.
* Bad, because every build, also one without push, ships libraries outside the license list.

### No FCM

* Good, because no proprietary library is needed.
* Bad, because devices with Play services deliver background messages reliably only through FCM
  (13 5.2), and UnifiedPush is a later step (13 5.4).
