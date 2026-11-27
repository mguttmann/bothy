# Linux test backend of the computer helper

Linux has no computer helper (docs/spec/27-computer-use.md section 1 item 5). For tests only,
`backend.py` stands in for it: it connects to the endpoint the host service opens on Linux with
`host_service.computer_use` (`/run/bothy-host-computer/computer.sock`), where the host service
admits exactly the user of `host_service.computer_test_peer`, speaks the channel protocol of 27 9.4
and drives an X server with python-xlib and the XTEST extension. The Compose end-to-end test of
CU-06 runs it in Xvfb next to `fixture_app.py`. Rulings: `docs/decisions.d/CU-03.md` D-CU-03-10.

## Files

| File | Content |
|---|---|
| `backend.py` | the backend: `hello`, `state`, `stop`; `capture` (root window or region, scaled to `max_edge`, JPEG or PNG with Pillow, only in memory), `do` (pointer, buttons, scroll, keys and Unicode text with XTEST), `position`, `release`, `hide`, `disable` |
| `fixture_app.py` | a Tk window with known controls (buttons `Alpha` and `Beta`, a text field, a red square) that writes every click and text change as a JSON line |
| `tests/test_codec.py` | the codec against `services/host/testdata/computer/channel-vectors.json` |
| `tests/test_xvfb.py` | the backend drives the fixture app in Xvfb through the channel; the test plays the host service |

## Signals

| Signal | Effect |
|---|---|
| `SIGUSR1` | simulated lock: `state` with `locked: true`, every key released, input answers `screen_locked` |
| `SIGUSR2` | unlock: `state` with `locked: false` |
| `SIGHUP` | the stop button of the indicator: `stop` with `indicator_stop` for the shown session |

## Run

Dependencies: Python 3, Xvfb, python-xlib, Pillow and Tk (Debian: `xvfb python3-xlib python3-pil
python3-tk`). The Xvfb test is skipped, naming the missing part, where one of them is absent.

```sh
cd services/host/test/computer_backend
python3 -m unittest discover -s tests -t .
python3 backend.py --endpoint /run/bothy-host-computer/computer.sock --generation 1
```
