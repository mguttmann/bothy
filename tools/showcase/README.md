# Showcase media

The screenshots and the short video of the README and the docs landing page live in
`docs/assets/showcase/` (M7-28). Nothing there is made or edited by hand: capture drivers
produce every file from the demo instance (M7-27), strip its metadata and record it in a
manifest, and a check in `make check` holds the folder, the manifest and the metadata rule
together.

| File | Purpose |
|---|---|
| `media.py` | Finds and strips the metadata of PNG and MP4 files. |
| `manifest.py` | Reads, validates and writes the manifest; drops a platform; the check. |
| `showcase_cli.py` | The command line of the manifest and the check; `make docs` (part of `make check`) runs `check`. |
| `capture.py` | `make showcase`: starts the demo, captures every platform, strips and files the results. |
| `console_capture.py` | The web console in headless Chromium, logged in through `bothy-smoke seed`. |
| `grafana_capture.py` | The dashboard of the panel plugin in a local Grafana fed with the demo's metrics. |
| `tests/` | Tests; every PNG and MP4 they use is built in code (`tests/showcase_fixtures.py`). |

The command line is `showcase_cli.py`, not `showcase.py`: pytest imports the tests in
importlib mode, which makes this folder the package `showcase` and would hide a module of
the same name.

## Manifest

`docs/assets/showcase/manifest.json` lists every file of the folder:

```json
{
  "assets": [
    {
      "commit": "<40 lower-case hex digits>",
      "demo_seed": 7,
      "file": "android/threads-light.png",
      "kind": "image",
      "platform": "android",
      "script": "<repository path of the capture script>",
      "theme": "light",
      "view": "threads"
    }
  ],
  "version": 1
}
```

| Key | Value |
|---|---|
| `file` | Path relative to `docs/assets/showcase`: `<platform>/<name>.png` or `.mp4`, the name in `[a-z0-9-]+`. |
| `kind` | `image` (PNG) or `video` (MP4). |
| `platform` | The surface in `[a-z0-9-]+` (`android`, `ios`, `macos`, `windows`, `console`, `grafana`, ...); also the folder of the file. |
| `view` | Short name of the view (`threads`, `approval`, `flow`). |
| `theme` | `light` or `dark`. |
| `script` | Repository path of the capture script; it must exist. |
| `commit` | The commit the capture ran on. |
| `demo_seed` | The seed of the demo instance the capture showed. |

The file is written sorted (entries by `file`, keys by name) with two-space indentation and a
trailing newline, so a fresh capture changes only the values that changed. Besides the media,
the folder may hold only `manifest.json` and a `README.md` at its top.

## Metadata rule

A showcase file keeps exactly what a decoder needs to show it. Everything that can say who
made it, when, or on which machine is metadata and fails the check.

| Format | Kept as codec data | Metadata |
|---|---|---|
| PNG | `IHDR`, `PLTE`, `tRNS`, `IDAT`, `IEND`, and the colour decoding chunks `sRGB`, `gAMA`, `cHRM` | Every other chunk (`tEXt`, `zTXt`, `iTXt`, `eXIf`, `tIME`, `iCCP`, `pHYs`, private chunks) and bytes after `IEND` |
| MP4 | The box tree and the sample data | `udta`, `meta`, `ilst`, every `uuid` box (XMP and other vendor data), `free` or `skip` with a payload that is not zero, and the creation and modification times of `mvhd`, `tkhd` and `mdhd` |

Any other format (JPEG with EXIF, GIF, WebP) is refused. Stripping a PNG rebuilds it from the
kept chunks with their CRCs; the pixel data is unchanged. Stripping an MP4 moves no byte,
because `stco` and `co64` hold absolute offsets into `mdat`: each metadata box becomes a
`free` box of the same size with a zeroed payload, and the times become zero. The MP4 walk
descends into `moov`, `trak`, `mdia`, `minf`, `stbl`, `edts`, `dinf`, `udta` and `meta`
(after its version and flags), and understands 64-bit sizes and a size of 0 (to the end of
the parent); the sample data and fragmented files (`moof`) are not inspected, because the
capture drivers write a plain MP4. A truncated or malformed file is a finding.

## Command line

From the repository root:

```sh
uv run --project tools --locked --quiet python tools/showcase/showcase_cli.py <command>
```

| Command | Effect |
|---|---|
| `strip <file>...` | Removes the metadata of each file in place, PNG or MP4 by content. |
| `record --file F --kind K --platform P --view V --theme T --script S --commit C --seed N` | Adds or replaces the manifest entry of `F` (relative to `docs/assets/showcase`). |
| `drop-platform <platform>` | Removes the platform's entries and every file of its folder, before its fresh capture. |
| `check` | Prints one line per finding: a file not in the manifest, an entry without its file, a bad name or folder, a wrong format, metadata, a missing capture script, a schema error. |

`--root <path>` (before the command) names the repository root; it defaults to the repository
the script is in. Exit codes: 0 done or clean; 1 findings, or a file `strip` cannot read or
strip; 2 usage, including values that break the manifest schema.

`capture.py` does the same through the functions of `manifest.py` and `media.py`: it
captures into a temp folder, strips every file, refuses a file that still carries metadata or
is larger than `repo.max_file_kb`, and only then replaces the platform's folder and entries.

## Capture

```sh
make image                                   # the server image of the demo
make showcase PLATFORM=android,console,grafana [SEED=1]
```

`tools/showcase/capture.py` starts `infra/compose/demo.sh` (the demo instance of M7-27 with the
seed), waits for the three lines the script prints when the demo is ready (address, CA,
enrollment code; a test holds the parser equal to the script), captures every platform in
turn and stops the demo, which removes its stack. The first platform signs in with the code the
demo printed; every further one gets a fresh code of Alex from `bothy setup
issue-enrollment-code` in the demo's `api` container. The run refuses a working tree with
changes outside `docs/assets/showcase`, because the manifest names the commit;
`--allow-dirty` is for developing a driver only.

| Platform | How | Files |
|---|---|---|
| `android` | `flutter drive` of `apps/client/e2e_test/showcase/showcase_test.dart` on an emulator (`SHOWCASE_ANDROID_SERIAL`, or the AVD `SHOWCASE_AVD`, default the first of `emulator -list-avds`, started headless and stopped again); the test captures each view inside the app through Flutter's image API and hands the PNGs to `test_driver/showcase_driver.dart`; then it plays the flow of the video while `adb shell screenrecord` records the screen, the status bar in the system's demo mode (fixed clock, no notifications) | `spaces`, `thread-live`, `approval`, `live`, `hosts`, `terminal` per theme, `flow` (MP4) |
| `ios`, `macos` | The same test on a simulator (`SHOWCASE_DEVICE_IOS`, its UDID) or the desktop of a Mac with Xcode. Without Docker there, set `SHOWCASE_REMOTE_SSH` (an SSH destination) and `SHOWCASE_REMOTE_DIR` (a build folder the run replaces; `SHOWCASE_REMOTE_FLUTTER` names Flutter there): the run ships the working tree, forwards the demo's port with an SSH reverse tunnel and copies the images back | as `android`, without the video |
| `windows` | The same test on a Windows host that runs the demo next to the app (D-W-128) | as `macos` |
| `console` | The web console of the demo in headless Chromium (Playwright of the tools environment; `uv run --project tools playwright install chromium` once): Alex gets a password and TOTP from `bothy-smoke seed` (built from `services/server/Dockerfile` target `smoke`, run in the namespace of the demo's `api`), logs in, confirms the step-up, and each view is captured in the light and the dark colour scheme the browser emulates | `live`, `tv-mode`, `statistics`, `hosts`, `agents-costs`, `people`, `audit` per theme |
| `grafana` | A second collector reads the demo's metrics file, a Prometheus with the stack's configuration stores them and a Grafana with the stack's provisioning and the plugin's release build shows the dashboard `bothy-live` in kiosk mode; all three run in a network of their own, Grafana on a free port of `127.0.0.1`, and are removed after the capture | `live-dashboard` per theme |

Nothing of the build host can appear: the apps are captured inside the app (no window frame,
no desktop), the browser pages as their viewport, and the video shows only the emulator's screen
with the demo's made-up data. The images then pass `make denylist` (file names and embedded text
chunks) and the check above.
