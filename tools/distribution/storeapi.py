# SPDX-License-Identifier: Apache-2.0
"""The store services of the phone apps (brief M7-13, docs/spec/18-release.md 5 and 8).

`PlayPublisher` puts an Android App Bundle on a test track of Google Play through the Google
Play Developer API (an edit: insert, upload the bundle, set the track, commit), signed in with
the publisher's service account (OAuth 2.0 JWT bearer, RS256). `AppStoreConnect` finds the
build that `xcrun altool` uploaded, waits until App Store Connect has processed it and adds it
to a TestFlight group (App Store Connect API, a token signed with the publisher's API key,
ES256).

Both reach the network only through `Http`, the one seam the tests replace; a service answers
in JSON, and a refusal raises StepError with the status and the service's message, never a
token or key. Uploads to a store are the owner's decision (D-W-125): these classes run only
in the release jobs with the secrets of their environments, never in a trial.

References (retrieved 2026-10-27): https://developers.google.com/android-publisher/edits,
https://developers.google.com/identity/protocols/oauth2/service-account,
https://developer.apple.com/documentation/appstoreconnectapi/generating-tokens-for-api-requests.
"""

from __future__ import annotations

import base64
import json
import time
import urllib.error
import urllib.parse
import urllib.request
from collections.abc import Callable, Mapping
from dataclasses import dataclass
from typing import Any

from common import PATIENCE_SECONDS, StepError, wait_for
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import ec, padding, rsa
from cryptography.hazmat.primitives.asymmetric.utils import decode_dss_signature

PLAY_SCOPE = "https://www.googleapis.com/auth/androidpublisher"
PLAY_API = "https://androidpublisher.googleapis.com/androidpublisher/v3/applications"
PLAY_UPLOAD = "https://androidpublisher.googleapis.com/upload/androidpublisher/v3/applications"
ASC_API = "https://api.appstoreconnect.apple.com/v1"
# App Store Connect processes an uploaded build before TestFlight can use it; that takes from a
# few minutes to well over half an hour, so the wait has its own, larger bound.
PROCESSING_PATIENCE_SECONDS = max(PATIENCE_SECONDS, 5400.0)


@dataclass(frozen=True)
class Response:
    """One answer of a service: the status and the body."""

    status: int
    body: bytes


Http = Callable[[str, str, Mapping[str, str], bytes | None], Response]


def urllib_http(method: str, url: str, headers: Mapping[str, str], body: bytes | None) -> Response:
    """`Http` over urllib with a bound of COMMAND_PATIENCE_SECONDS per request.

    Raises:
        StepError: the address is no https address of a store service, or the network fails.
    """
    if not url.startswith(
        (
            "https://androidpublisher.googleapis.com/",
            "https://oauth2.googleapis.com/",
            "https://api.appstoreconnect.apple.com/",
        )
    ):
        raise StepError(f"refusing a request to {url}")
    request = urllib.request.Request(url, data=body, method=method, headers=dict(headers))  # noqa: S310
    try:
        with urllib.request.urlopen(request, timeout=600) as resp:  # noqa: S310  # nosec B310
            return Response(resp.status, resp.read())
    except urllib.error.HTTPError as err:
        return Response(err.code, err.read())
    except OSError as err:
        raise StepError(f"{method} {url}: {err}") from err


def _b64url(data: bytes) -> str:
    return base64.urlsafe_b64encode(data).rstrip(b"=").decode("ascii")


def _jwt(header: dict[str, Any], claims: dict[str, Any], sign: Callable[[bytes], bytes]) -> str:
    head = _b64url(json.dumps(header, separators=(",", ":")).encode())
    body = _b64url(json.dumps(claims, separators=(",", ":")).encode())
    signing_input = f"{head}.{body}".encode("ascii")
    return f"{head}.{body}.{_b64url(sign(signing_input))}"


def _json(response: Response, what: str) -> Any:
    """Return the JSON body of a success, or raise StepError naming `what` and the status."""
    if not 200 <= response.status < 300:
        message = response.body.decode("utf-8", "replace")[:500]
        raise StepError(f"{what} refused with {response.status}: {message}")
    if not response.body:
        return {}
    try:
        return json.loads(response.body)
    except ValueError as err:
        raise StepError(f"{what}: the answer is no JSON") from err


def _field(answer: Any, key: str, what: str) -> Any:
    """Return `answer[key]` of a JSON object, or raise StepError naming `what`.

    A store answers with a JSON object; anything else (a list, a missing key) is no answer the
    step can use, and the job ends with this message instead of a traceback.
    """
    if not isinstance(answer, dict) or key not in answer:
        raise StepError(f"{what}: the answer has no {key}")
    return answer[key]


def _items(answer: Any, what: str) -> list[dict[str, Any]]:
    """Return the `data` list of an App Store Connect answer; every item an object with an id.

    Raises:
        StepError: the answer has no such list.
    """
    data = _field(answer, "data", what)
    if not isinstance(data, list) or not all(
        isinstance(item, dict) and "id" in item for item in data
    ):
        raise StepError(f"{what}: the answer has no list of objects with an id")
    return data


def _attribute(item: dict[str, Any], key: str) -> Any:
    attributes = item.get("attributes")
    return attributes.get(key) if isinstance(attributes, dict) else None


# ---- Google Play ----------------------------------------------------------------------------


class PlayPublisher:
    """The Google Play Developer API for one app, signed in with a service account.

    Args:
        service_account: the JSON key of the service account (`client_email`, `private_key`,
            `token_uri`), which the publisher grants release rights in the Play Console.
        package_name: the application ID (`android.package_name`).
        http: the network seam.
        clock: seconds since the epoch, for the token's lifetime.
    """

    def __init__(
        self,
        service_account: str,
        package_name: str,
        http: Http = urllib_http,
        clock: Callable[[], float] = time.time,
    ) -> None:
        """Create the client and read the service account key.

        Raises:
            StepError: the key is no service account key with an RSA private key.
        """
        try:
            account = json.loads(service_account)
            self._email = str(account["client_email"])
            self._token_uri = str(account.get("token_uri") or "https://oauth2.googleapis.com/token")
            key = serialization.load_pem_private_key(account["private_key"].encode(), None)
        except (ValueError, KeyError, TypeError, AttributeError) as err:
            raise StepError("PLAY_SERVICE_ACCOUNT_JSON is no service account key") from err
        if not isinstance(key, rsa.RSAPrivateKey):
            raise StepError("PLAY_SERVICE_ACCOUNT_JSON holds no RSA key")
        self._key = key
        self._package = package_name
        self._http = http
        self._clock = clock
        self._token: str | None = None

    def _access_token(self) -> str:
        if self._token is None:
            now = int(self._clock())
            assertion = _jwt(
                {"alg": "RS256", "typ": "JWT"},
                {
                    "iss": self._email,
                    "scope": PLAY_SCOPE,
                    "aud": self._token_uri,
                    "iat": now,
                    "exp": now + 3600,
                },
                lambda data: self._key.sign(data, padding.PKCS1v15(), hashes.SHA256()),
            )
            form = urllib.parse.urlencode(
                {
                    "grant_type": "urn:ietf:params:oauth:grant-type:jwt-bearer",
                    "assertion": assertion,
                }
            ).encode()
            answer = _json(
                self._http(
                    "POST",
                    self._token_uri,
                    {"Content-Type": "application/x-www-form-urlencoded"},
                    form,
                ),
                "the Google sign-in of the service account",
            )
            self._token = str(
                _field(answer, "access_token", "the Google sign-in of the service account")
            )
        return self._token

    def _call(
        self, method: str, url: str, what: str, body: bytes | None = None, json_body: Any = None
    ) -> Any:
        headers = {"Authorization": f"Bearer {self._access_token()}"}
        if json_body is not None:
            body = json.dumps(json_body).encode()
            headers["Content-Type"] = "application/json"
        elif body is not None:
            headers["Content-Type"] = "application/octet-stream"
        return _json(self._http(method, url, headers, body), what)

    def _edit(self) -> str:
        answer = self._call("POST", f"{PLAY_API}/{self._package}/edits", "a Play edit")
        return str(_field(answer, "id", "a Play edit"))

    def _set_track(self, edit: str, track: str, version_code: int, name: str) -> None:
        self._call(
            "PUT",
            f"{PLAY_API}/{self._package}/edits/{edit}/tracks/{track}",
            f"the Play track {track}",
            json_body={
                "track": track,
                "releases": [
                    {"name": name, "versionCodes": [str(version_code)], "status": "completed"}
                ],
            },
        )

    def _commit(self, edit: str) -> None:
        self._call("POST", f"{PLAY_API}/{self._package}/edits/{edit}:commit", "the Play edit")

    def upload(self, bundle: bytes, *, version_code: int, name: str, track: str) -> None:
        """Upload `bundle` and release it on `track` in one edit.

        Raises:
            StepError: Play refuses a step, or the uploaded bundle has another version code.
        """
        edit = self._edit()
        uploaded = self._call(
            "POST",
            f"{PLAY_UPLOAD}/{self._package}/edits/{edit}/bundles?uploadType=media",
            "the bundle upload",
            body=bundle,
        )
        read = _field(uploaded, "versionCode", "the bundle upload")
        if str(read) != str(version_code):
            raise StepError(f"Play read version code {read}, the build has {version_code}")
        self._set_track(edit, track, version_code, name)
        self._commit(edit)

    def promote(self, *, version_code: int, name: str, track: str) -> None:
        """Release the uploaded build `version_code` on `track` too (18 5 rule 1: no rebuild).

        Raises:
            StepError: Play refuses a step.
        """
        edit = self._edit()
        self._set_track(edit, track, version_code, name)
        self._commit(edit)


# ---- App Store Connect ----------------------------------------------------------------------


class AppStoreConnect:
    """The App Store Connect API with the publisher's API key (team key, App Manager role).

    Args:
        key: the private key (`.p8`, PEM) of the API key.
        key_id: its key ID.
        issuer: the issuer ID of the team.
        http: the network seam.
        clock: seconds since the epoch, for the token's lifetime (at most 20 minutes).
        sleep: waits between the polls of a processing build.
    """

    def __init__(
        self,
        key: str,
        key_id: str,
        issuer: str,
        http: Http = urllib_http,
        clock: Callable[[], float] = time.time,
        monotonic: Callable[[], float] = time.monotonic,
        sleep: Callable[[float], None] = time.sleep,
    ) -> None:
        """Create the client and read the API key.

        Raises:
            StepError: the key is no PEM private key on an elliptic curve.
        """
        try:
            loaded = serialization.load_pem_private_key(key.encode(), None)
        except (ValueError, TypeError) as err:
            raise StepError("the App Store Connect key is no PEM private key") from err
        if not isinstance(loaded, ec.EllipticCurvePrivateKey):
            raise StepError("the App Store Connect key is no EC key")
        self._key = loaded
        self._key_id = key_id
        self._issuer = issuer
        self._http = http
        self._clock = clock
        self._monotonic = monotonic
        self._sleep = sleep

    def _token(self) -> str:
        now = int(self._clock())

        def sign(data: bytes) -> bytes:
            # JOSE wants the raw r and s (RFC 7518 3.4), cryptography returns DER.
            r, s = decode_dss_signature(self._key.sign(data, ec.ECDSA(hashes.SHA256())))
            return r.to_bytes(32, "big") + s.to_bytes(32, "big")

        return _jwt(
            {"alg": "ES256", "kid": self._key_id, "typ": "JWT"},
            {"iss": self._issuer, "iat": now, "exp": now + 1200, "aud": "appstoreconnect-v1"},
            sign,
        )

    def _call(self, method: str, path: str, what: str, json_body: Any = None) -> Any:
        headers = {"Authorization": f"Bearer {self._token()}"}
        body = None
        if json_body is not None:
            body = json.dumps(json_body).encode()
            headers["Content-Type"] = "application/json"
        return _json(self._http(method, f"{ASC_API}{path}", headers, body), what)

    def app_id(self, bundle_id: str) -> str:
        """Return the ID of the app with `bundle_id`.

        Raises:
            StepError: no app has the bundle identifier (the publisher creates the app record
                in App Store Connect first).
        """
        query = urllib.parse.urlencode({"filter[bundleId]": bundle_id})
        found = _items(self._call("GET", f"/apps?{query}", "the app lookup"), "the app lookup")
        for app in found:
            if _attribute(app, "bundleId") == bundle_id:
                return str(app["id"])
        raise StepError(
            f"App Store Connect has no app {bundle_id}; create its record first "
            "(docs/operations/release.md 4)"
        )

    def processed_build(self, app_id: str, build_number: int) -> str:
        """Wait until the build `build_number` of the app is processed; return its ID.

        Raises:
            StepError: the build is refused (`INVALID`, `FAILED`) or not processed within
                PROCESSING_PATIENCE_SECONDS.
        """
        query = urllib.parse.urlencode(
            {"filter[app]": app_id, "filter[version]": str(build_number)}
        )
        found: dict[str, str] = {}

        def processed() -> bool:
            answer = self._call("GET", f"/builds?{query}", "the build lookup")
            builds = _items(answer, "the build lookup")
            if not builds:
                return False
            state = _attribute(builds[0], "processingState")
            if state in ("INVALID", "FAILED"):
                raise StepError(f"App Store Connect refused build {build_number}: {state}")
            found["id"] = str(builds[0]["id"])
            return bool(state == "VALID")

        wait_for(
            f"App Store Connect processes build {build_number}",
            processed,
            patience=PROCESSING_PATIENCE_SECONDS,
            interval=30.0,
            clock=self._monotonic,
            sleep=self._sleep,
        )
        return found["id"]

    def add_to_group(self, app_id: str, build_id: str, group: str) -> None:
        """Add the build to the TestFlight group named `group` (18 5: `beta`, `stable`).

        Raises:
            StepError: the app has no such group (the publisher creates `beta` and `stable`
                in App Store Connect), or the service refuses.
        """
        groups = self._call("GET", f"/apps/{app_id}/betaGroups", "the TestFlight groups")
        for g in _items(groups, "the TestFlight groups"):
            if _attribute(g, "name") == group:
                self._call(
                    "POST",
                    f"/betaGroups/{g['id']}/relationships/builds",
                    f"adding the build to the TestFlight group {group}",
                    json_body={"data": [{"type": "builds", "id": build_id}]},
                )
                return
        raise StepError(
            f"the app has no TestFlight group {group}; create the groups beta and stable "
            "(docs/operations/release.md 4)"
        )
