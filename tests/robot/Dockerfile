# API-tier Robot Framework runner for the KeepSave e2e harness.
# Slim: only the deps needed for the RequestsLibrary (API) suites. The browser
# tier (tests/03_embed_widget.robot) needs the Playwright browsers — use the
# marketsquare/robotframework-browser base image + `rfbrowser init` for that.
FROM python:3.12-slim

WORKDIR /suite
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY . .

# Default: run only the API-tagged suites (no browser needed). Compose DNS name
# `api` is provided via KEEPSAVE_URL in docker-compose.e2e.yml.
CMD ["robot", "--include", "api", "--outputdir", "results", "tests"]
