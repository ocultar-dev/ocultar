# OCULTAR | Connectors Guide

> **Audience:** DevOps engineers and developers who need to ingest data from external platforms (Slack, SharePoint, etc.) into OCULTAR.

---

## 1. Overview

OCULTAR Connectors are modular ingestion components that fetch or receive data from external sources and feed it into the Zero-Egress Refinery. This ensures that data from your enterprise collaboration tools is sanitized before it reaches any LLM or is stored in your vault.

## 2. Supported Connectors

### 2.1 Slack Workspace
The Slack connector allows you to ingest channel history and listen for message events.

**Configuration:** there is no YAML block for this — `services/refinery/cmd/main.go` auto-starts the connector at boot if `SLACK_TOKEN` is set:

```bash
export SLACK_TOKEN="xoxb-your-slack-bot-token"
export SLACK_WORKSPACE_ID="T12345678"
```

Note: this is the **refinery CLI/server binary's** outbound Slack connector (used to ingest channel history). It is a separate integration from Sombra's inbound `/v1/slack/events` webhook, which verifies incoming Slack events using `SLACK_SIGNING_SECRET` instead — see `CLAUDE.md` and the [Sombra Guide](./SOMBRA_GUIDE.md).

### 2.2 Microsoft SharePoint & Teams
The SharePoint connector ingests documents from Microsoft SharePoint via the Microsoft Graph API using OAuth2 Client Credentials. It authenticates as your Azure AD application, polls the drive for new and changed files using delta queries (incremental sync), extracts their text content, and passes everything through the Zero-Egress Refinery before any downstream use.

**Azure AD setup (one-time):**
1. Register an app in Azure AD → **App registrations → New registration**.
2. Under **Certificates & secrets**, create a client secret. Note the value.
3. Under **API permissions**, add `Sites.Read.All` (Microsoft Graph, Application type) and grant admin consent.
4. Note the **Application (client) ID** and **Directory (tenant) ID**.

**Configuration:** also env-var-driven, auto-started if `MS_CLIENT_ID` is set:

```bash
export MS_TENANT_ID="your-tenant-id"      # Azure AD Directory ID
export MS_CLIENT_ID="your-client-id"      # Azure AD Application ID
export MS_CLIENT_SECRET="your-client-secret"
export MS_SHAREPOINT_SITE_ID="your-site-id"  # SharePoint site ID (required for Fetch)
```

**Supported file formats:**

| Format | Extension | Extraction method |
|---|---|---|
| Plain text | `.txt`, `.csv`, `.json`, `.md`, `.log`, `.xml`, `.html`, `.eml` | Raw content |
| Word documents | `.docx` | ZIP/XML — extracts all `<w:t>` run text |
| Excel workbooks | `.xlsx` | ZIP/XML — extracts shared strings table |
| PowerPoint | `.pptx` | ZIP/XML — extracts all slide `<a:t>` runs |
| PDF | `.pdf` | Byte-scan — extracts literal string operands from text blocks |

> [!NOTE]
> PDF extraction covers standard Latin-character PDFs. Fully-encrypted PDFs and CID-keyed fonts (common in scanned/image-only PDFs) will produce empty output and are skipped with a log warning.

**How it works:**
- On startup, a background goroutine polls every 30 seconds using `GET /sites/{id}/drive/root/delta`. The first run performs a full enumeration; subsequent runs fetch only changed files (delta links are persisted in memory).
- Files whose extension is not in the supported list are silently skipped.
- All extracted text is passed to `refinery.ProcessInterface` before any logging or forwarding — no raw PII ever leaves the connector.

### 2.3 Dynamic Plugins
`Manager.LoadPlugin` (`services/refinery/pkg/connector/manager.go`) can load a connector from a Go plugin (`.so` file) implementing a `NewConnector() Connector` symbol. As of this writing, no binary in the repo calls `LoadPlugin` — it is library capability without a CLI flag or config path wired up to invoke it. Using it today requires calling it yourself from a fork of `services/refinery/cmd/main.go`.

## 3. Configuration

Connectors are configured entirely through environment variables read directly by `services/refinery/cmd/main.go` at startup — there is no `connectors` section in `configs/config.yaml`; the `Settings` struct (`services/refinery/pkg/config/config.go`) has no such field.

### Environment Variables
- `SLACK_TOKEN`: The API token for your Slack bot. Setting this auto-starts the Slack connector.
- `SLACK_WORKSPACE_ID`: Your Slack Workspace ID.
- `MS_CLIENT_ID`, `MS_TENANT_ID`, `MS_CLIENT_SECRET`, `MS_SHAREPOINT_SITE_ID`: Azure AD credentials. Setting `MS_CLIENT_ID` auto-starts the SharePoint connector.

## 4. Zero-Egress Implementation

Every connector follows the **Refinery-First** principle:
1. Data is fetched from the source (e.g., Slack API).
2. Data is immediately passed to `pkg/refinery.ProcessInterface`.
3. Only the **refined** (redacted) data is logged or forwarded.
4. All secrets (API keys, tokens) stay within the secure OCULTAR environment.

## 5. Development

To build a new connector, implement the `Connector` interface in `services/refinery/pkg/connector`:

```go
type Connector interface {
    ID() string
    Type() string
    Init(config map[string]interface{}, eng *refinery.Refinery) error
    Start() error
    Stop() error
    Fetch(ctx context.Context, params map[string]interface{}) ([]byte, error)
}
```

Register your connector in its `init()` function:
```go
func init() {
    connector.Register("my-connector", func() connector.Connector {
        return &MyConnector{}
    })
}
```
