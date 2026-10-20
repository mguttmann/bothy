# Local SonarQube

| File | Purpose |
|---|---|
| `compose.yaml` | SonarQube Community Build and PostgreSQL; web interface only on `127.0.0.1`, database only in the internal network; images pinned by tag and digest |
| `quality_sync.py` | apply the gate and the profiles from `quality/sonar/` to a SonarQube server (`apply`) or compare them (`check`); token only from `SONAR_TOKEN` |
| `tests/` | tests against a test server that imitates the Web API calls in use |
| `.env` | created by `make sonar-up` with a random database password; git-ignored |

Step-by-step procedure: [docs/quality/sonarqube-local.md](../../docs/quality/sonarqube-local.md).

```sh
make sonar-up             # start and wait for UP
make sonar-config         # apply gate and profiles (token with admin permissions)
make sonar-config-check   # check equality with quality/sonar/
make sonar-main-branch    # SonarQube Cloud: name the main branch of the project main
make sonar-visibility     # SonarQube Cloud: make the project public
make sonar                # analyze (from the first product code on)
make sonar-down           # stop, the data is kept
```
