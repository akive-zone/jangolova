# Grimlock agent subsystem

Grimlock is Jangolova's model-powered agent subsystem. It is part of
Jangolova, alongside Cymonkey, Pacman, and the core interaction engine. It is
not a separate product and it is not a target, display, or workload manager.

Grimlock gives people, applications, IDEs, and other agents an agent-oriented
way to use Jangolova. HTTP, MCP, ACP, and future protocols are adapters at
its northbound boundary. Google Agent Development Kit (ADK) for Go supplies
the model-neutral agent, tool, session, streaming, and workflow primitives.

```text
user, application, IDE, or external agent
                  |
          HTTP / MCP / ACP
                  |
              Grimlock
     model + agent + policy + approvals
                  |
      Jangolova interaction capabilities
          |                       |
       Cymonkey                 Pacman
       browsers              Unity / Unreal
                  |
       caller-owned target or Xallet
```

Deterministic callers do not have to use a model. They may continue calling
the interaction-engine API directly. Grimlock is used when the caller wants
Jangolova to run an agent, compose tools, maintain agent state, or expose that
agent through an agent protocol.

## Caller-supplied models

Model ownership is caller-selectable. A deployment can configure a default
model, while an application can supply its own approved model gateway and
opaque credential references for an individual Grimlock session.

The model profile is separate from an interaction target descriptor:

- a model profile says where reasoning happens;
- a target descriptor says where interaction happens;
- a Grimlock session binds one model profile to an approved set of Jangolova
  capabilities and interaction instances.

Initial model profile:

```json
{
  "apiVersion": "agent.model/v1alpha1",
  "profileId": "application-model",
  "protocol": "openai-compatible",
  "endpoint": "https://ai-gateway.example/v1",
  "model": "company-approved-model",
  "credentialRef": "application-ai-credential",
  "tlsRef": "company-ai-trust",
  "metadata": {
    "owner": "application-one"
  }
}
```

`protocol` selects a registered Grimlock model connector; it does not select a
Jangolova interaction engine. The first connector is `openai-compatible`.
Gemini, Vertex, an in-process model, another hosted API, or a company-specific
gateway can add connectors without changing Grimlock's agent/session contract.

The endpoint must be absolute HTTP(S). Non-loopback endpoints require HTTPS.
The model name and endpoint are ordinary configuration, but credentials and
private TLS material are always referenced indirectly. Resolved headers,
tokens, client keys, and private CA contents remain in memory and must never be
included in model profiles, events, errors, traces, prompts, or tool results.

Grimlock uses the same expiring connection-material machinery as interaction
targets internally, including renewal, redaction, CA rotation, and mTLS. This
reuse does not turn the model into a display target: the public descriptors and
ownership contracts remain separate.

## Session contract

A Grimlock session contains:

- a stable session ID;
- a model profile or caller-approved default profile reference;
- agent instructions;
- an allowlist of Jangolova capabilities and interaction instances;
- model-call, token, time, and tool-call budgets;
- approval policy for write, external, sensitive, or destructive effects;
- an event stream carrying model output, tool calls, approvals, evidence,
  failures, and completion state.

The model receives only tools admitted by session policy. Authorization is
checked when capabilities are advertised and again immediately before a tool
executes. A model decision is never authorization by itself.

Grimlock does not automatically replay a write after an ambiguous failure. It
observes the current state and either requests approval or reports the unknown
outcome to the caller.

## Interaction capability tools

Each interaction binding identifies a caller-owned interaction instance, its
bridge caller, an optional capability allowlist, an authorization policy, and
an approval rule. Creating an interaction agent performs capability discovery
before the model connection is opened.

The resulting ADK tools are namespaced by interaction ID. For example,
`browser-one` capability `dom.click` becomes
`jangolova_browser_one_dom_click`. Namespacing lets one Grimlock agent operate
several browser, Unity, Unreal, or other interaction instances without tool
collisions or target-location assumptions. `describe` and cursor-based
`events` are also exposed as interaction-scoped read tools.

There are three separate enforcement points:

1. Capability admission determines which tools the model is allowed to see.
2. Write and external effects require ADK human confirmation by default.
3. The caller's policy authorizes the exact capability and validated input
   again immediately before the bridge call.

Confirmation is not authorization. A confirmed action can still be rejected
by application policy. Tool input is validated server-side against the
engine-advertised JSON Schema before policy or engine execution. The default
policy is observation-only: it advertises and authorizes read capabilities
and omits write and external capabilities.

Tool results carry the interaction ID, capability name, effect, and returned
JSON value. Engine adapters remain responsible for secret-safe errors and
results; model connection secrets never enter tool data.

## Protocol adapters

All northbound protocols map to one Grimlock application service:

| Operation | Meaning |
| --- | --- |
| discover | List model connectors and eligible Jangolova capabilities. |
| create session | Resolve a model profile and create an isolated ADK agent. |
| run / message | Submit user or agent input and stream execution events. |
| approve / reject | Resolve a pending tool confirmation. |
| inspect | Read session state, budgets, health, and evidence references. |
| cancel | Cancel a run without terminating caller-owned targets. |
| close | Release the model connection and Grimlock session. |

HTTP is the native application API. MCP exposes sessions and bounded
Jangolova operations as tools/resources. ACP carries interactive client
Protocol adapters must not duplicate model, policy, or tool-routing logic.

## Native HTTP API

`jangolova serve-grimlock` exposes the first northbound adapter. It is
authenticated with `Authorization: Bearer <JANGOLOVA_GRIMLOCK_TOKEN>`; only
`GET /healthz` is unauthenticated.

The application contract is `agent.grimlock/v1alpha1`:

| Route | Purpose |
| --- | --- |
| `GET /v1/connectors` | Discover registered model connector protocols. |
| `POST /v1/sessions` | Create a session from a caller-supplied model profile and one or more target bindings. |
| `GET /v1/sessions/{id}` | Inspect status and pending approvals. |
| `DELETE /v1/sessions/{id}` | Release model/adapter resources without stopping caller-owned targets. |
| `POST /v1/sessions/{id}/run` | Submit text; set `stream: true` for Server-Sent Events. |
| `GET /v1/sessions/{id}/events?after=N` | Read retained ADK execution events by cursor. |
| `POST /v1/sessions/{id}/confirmations/{approvalId}` | Approve or reject a pending write/external capability. |
| `POST /v1/sessions/{id}/cancel` | Cancel the active run while preserving the session and target. |

Create requests embed the existing provider-neutral target descriptor. An
engine adapter is selected explicitly with `agent.engine.adapter`, or with
`auto` from the target protocols and required capabilities. `allowWrites` is
false by default, so a binding advertises observation-only capabilities. A
binding that opts into writes still requires ADK confirmation for write and
external effects and is authorized again immediately before execution.

Example shape (the endpoint and opaque references belong to the caller):

```json
{
  "apiVersion": "agent.grimlock/v1alpha1",
  "userId": "application-one",
  "agent": {
    "sessionId": "browser-task",
    "model": {
      "apiVersion": "agent.model/v1alpha1",
      "profileId": "application-model",
      "protocol": "openai-compatible",
      "endpoint": "https://ai-gateway.example/v1",
      "model": "company-approved-model",
      "credentialRef": "application-ai-credential"
    }
  },
  "bindings": [{
    "interactionId": "browser",
    "engine": {"adapter": "auto", "requiredCapabilities": ["target.cdp"]},
    "target": {
      "apiVersion": "interaction.target/v1alpha1",
      "targetId": "browser-target",
      "kind": "browser",
      "endpoints": [{"name": "cdp", "protocol": "cdp", "url": "https://browser.example"}]
    }
  }]
}
```

The service keeps a bounded in-memory event history for the process lifetime.
MCP and ACP adapters are layered over this same service and must not create a
second agent runtime.

Set `JANGOLOVA_GRIMLOCK_SESSION_STORE` or pass `--session-store DIR` to any
Grimlock server to persist session summaries and retained events. Explicitly
deleting a session removes its record. Normal process shutdown preserves the
record for restart inspection. Model connections and interaction attachments
are deliberately not serialized; a reloaded session reports
`session_reconnect_required` until the caller creates a new attached session.

## MCP adapter

`jangolova serve-grimlock-mcp` exposes the same service as MCP tools and
resources. The default transport is line-delimited JSON-RPC over stdio; use
`--bind host:port` for authenticated single-request Streamable HTTP at `/mcp`.

For local use, `jangolova gl --mcp` is an equivalent short form. The existing
long command remains the stable script-friendly form.

The MCP tool surface is intentionally small and session-oriented:

- `grimlock_connectors`
- `grimlock_session_create`
- `grimlock_session_run`
- `grimlock_session_events`
- `grimlock_session_confirm`
- `grimlock_session_cancel`
- `grimlock_session_close`

`grimlock://connectors`, `grimlock://sessions/{sessionId}`, and the matching
event resource template expose read-only state. MCP calls never create a
second model runtime; they dispatch to the same session, approval, and target
cleanup paths as HTTP.

## ACP adapter

`jangolova serve-grimlock-acp` implements ACP v1 over stdio for editors and
other interactive clients. It supports `initialize`, `session/new`,
`session/load`, `session/prompt`, `session/cancel`, `session/close`, and
`session/update` notifications. Prompt content is text-only in this first
slice.

Because ACP's standard `session/new` request does not define Jangolova target
attachments, the caller supplies them through the `agent`, `model`, and
`bindings` extension fields (or the same fields under `_meta`). The caller's
model gateway, credential references, and target descriptors remain opaque and
caller-owned. A session is not created if those fields do not pass the same
validation used by HTTP.

`jangolova gl --acp` is the equivalent short form for the ACP adapter, while
`jangolova gl` starts the authenticated HTTP agent API.

## Delivery sequence

1. Model profile validation, opaque material resolution, and connector
   registry.
2. ADK agent factory using a caller-selected `model.LLM` implementation.
3. Jangolova capability-to-ADK-tool adapter with effect-aware approvals.
4. HTTP Grimlock session/run API and streaming events.
5. MCP transport over the same service.
6. ACP adapter over the same service.
7. Persistent session stores, quotas, tracing, and multi-agent workflows.

The current implementation completes steps 1 through 6. MCP and ACP adapt this
service rather than owning parallel runtimes.
