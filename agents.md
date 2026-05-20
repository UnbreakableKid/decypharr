# PAR2 repair implementation handoff

This file is the short execution handoff for smaller implementation agents.

Use this together with `plan.md`.

## Mission

Implement a first working PAR2 repair flow for decypharr using an **external PAR2 executable** and **local repaired file serving**.

## Architecture decision

The v1 implementation must use:

- external PAR2 binary invocation
- local disk staging of payload and PAR2 files
- parser retry from repaired local files

The v1 implementation must **not** use:

- native Go parity reconstruction
- broad UI work
- health-check repair integration
- support for every PAR2 binary family at once

## Global constraints for all agents

1. Do not rewrite the whole parser.
2. Do not introduce infinite retry loops.
3. Do not block progress on finding a Go PAR2 library.
4. Prefer config/env support over UI support for v1.
5. Preserve current non-repair behavior where possible.
6. Keep write scopes separate between agents.

## Expected v1 runtime behavior

When a likely repairable archive fails:

1. decypharr identifies the failed payload group
2. decypharr stages payload files and related PAR2 files to a repair workspace
3. decypharr discovers and runs a PAR2 executable
4. decypharr checks whether repair succeeded
5. decypharr retries parsing from repaired local files once
6. decypharr streams repaired local files from disk if retry succeeds

## Agent split

---

## Agent A — config + persistence

### Write scope

- `internal/config/usenet.go`
- `pkg/storage/nzb.go`
- `pkg/usenet/nzb.proto`
- `pkg/usenet/nzb.pb.go`
- `pkg/usenet/storage.go`

### Goal

Add the config and persistence fields required for repair and local-file-backed NZB files.

### Tasks

- add config fields for:
  - PAR2 binary path
  - repair workspace path
  - repair timeout
  - optional keep-artifacts flag
- add `LocalPath` support to persisted NZB files
- ensure `LocalPath` survives save/load through protobuf
- ensure defaults are safe when config fields are empty

### Suggested config fields

- `usenet.par2_binary`
- `usenet.repair_work_path`
- `usenet.repair_timeout`
- `usenet.keep_repair_artifacts`

### Acceptance criteria

- an NZB file can persist an optional `LocalPath`
- config values load correctly from config/env
- existing NZB entries without `LocalPath` still load correctly

### Notes

- do not add UI yet unless absolutely required
- backward compatibility matters here

---

## Agent B — local-file serving

### Write scope

- `pkg/usenet/types/volume.go`
- `pkg/usenet/nzb.go`
- `pkg/usenet/fs/fs.go`

### Goal

Allow decypharr to serve repaired local files instead of NNTP-backed segment streams.

### Tasks

- add local-file support to `Volume`
- propagate local path information from `NZBFile` to `Volume`
- update FS open/read paths so a local repaired file is served directly from disk
- keep prefetch as a no-op for local files
- preserve existing NNTP-backed behavior when no local path is set

### Acceptance criteria

- if `LocalPath` is set, reads come from the local file
- if `LocalPath` is empty, reads still use NNTP-backed segments
- there is no NNTP fetch for local repaired files

### Notes

- this is foundational for parser retry and later streaming
- avoid changing parser logic in this agent

---

## Agent C — staging + external repair tool

### Write scope

- new `pkg/usenet/repair_workspace.go`
- new `pkg/usenet/repair_download.go`
- new `pkg/usenet/repair_tool.go`

### Goal

Create the on-disk repair workspace and invoke an external PAR2 repair executable.

### Tasks

- create deterministic repair workspace paths
- stage payload files to disk from NNTP-backed file definitions
- stage related PAR2 files to disk
- discover PAR2 executable via:
  - configured path first
  - PATH lookup second
- run repair command with timeout and captured output
- return structured repair results:
  - executable used
  - workspace path
  - command args
  - exit code
  - stdout/stderr or combined output
  - duration

### Strong recommendation

Support exactly one tool family cleanly first.

Example starting target:

- `par2 repair <index.par2>`

If later supporting alternates:

- isolate syntax differences in a tiny adapter layer

### Acceptance criteria

- a repair workspace can be created for an NZB/group
- payload and PAR2 files can be fully materialized to disk
- the PAR2 executable can be found or a clear error is returned
- repair command output is captured for logs/debugging

### Notes

- do not decide parser fallback policy here
- this agent should expose repair as a callable service/helper

---

## Agent D — parser fallback integration

### Write scope

- new `pkg/usenet/parser/repair_fallback.go`
- `pkg/usenet/parser/parser.go`
- `pkg/usenet/parser/par2.go`

### Goal

Trigger repair only when appropriate, then retry parsing from repaired local files once.

### Tasks

- define likely repairable parser failure conditions
- locate the failed payload group and related PAR2 groups
- call the repair staging/tool layer
- if repair succeeds, construct repaired local-backed files or groups
- retry parser once using repaired local inputs
- prevent infinite loops with explicit state flags

### Required loop protections

- one repair attempt per failed group per processing pass
- one retry parse only
- clear failure if repair still does not produce readable files

### Acceptance criteria

- repair is attempted only on likely repairable failures
- successful repair leads to one parse retry
- failed repair exits with precise error/log output
- no retry recursion or silent looping

### Notes

- this is the highest-risk agent for accidental complexity
- keep orchestration thin and explicit

---

## Agent E — tests + observability

### Write scope

- test files in the touched packages
- log additions where needed in repair/staging/parser retry paths

### Goal

Make the repair workflow diagnosable and safe to iterate on.

### Tasks

- add unit tests for:
  - local-path persistence
  - local-file volume serving
  - repair workspace path generation
  - repair binary discovery
  - parser retry loop protection
- add regression tests for:
  - payload/PAR2 group separation
  - unknown-file retention for PAR2 deobfuscation paths
- add logs for:
  - workspace creation
  - files staged
  - chosen PAR2 index file
  - executable path
  - command invocation result
  - parse retry result

### Acceptance criteria

- failures are explainable from logs alone
- core non-UI repair pieces have test coverage
- parser fallback behavior is reproducible in tests where possible

### Notes

- this agent should avoid large logic changes unless fixing testability blockers

---

## Integration order

If agents are executed in sequence, use this order:

1. Agent A
2. Agent B
3. Agent C
4. Agent D
5. Agent E

If agents are executed in parallel, safe parallelism is:

- Agent A and Agent C can start in parallel if config naming is agreed first
- Agent B can start after `LocalPath` field naming is fixed
- Agent D should start after Agent B and Agent C interfaces are stable
- Agent E can start after the first code from A/B/C/D lands

## Shared interface expectations

To reduce integration pain, align on these concepts early:

- a repaired file is represented by `LocalPath`
- a repair workspace has a stable path per NZB/group
- repair tool execution returns a structured result, not just `error`
- parser fallback receives enough information to know:
  - what failed
  - what was staged
  - whether repair succeeded
  - where repaired files are located

## Suggested definition of done for each agent

An agent is done when:

- its scoped files build cleanly
- it does not require unrelated changes outside its write set
- it has clear logs or tests for its own failure modes
- another agent can integrate against it without guessing behavior

## Final success criteria

The overall effort succeeds when a broken-but-repairable release can:

- be staged locally
- be repaired by an external PAR2 tool
- be reparsed exactly once
- then be streamed from repaired local files

## Reference

Read `plan.md` for the full long-form rationale, phase breakdown, risks, and implementation details.