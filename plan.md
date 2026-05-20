# PAR2 repair implementation plan

## Decision summary for implementation agents

This is the short version to hand to implementation-focused agents.

### Primary decision

Do **not** attempt a native Go PAR2 reconstruction engine for v1.

Implement **external-binary PAR2 repair** with local repaired file serving.

### Why

- `nzbdav` is useful for PAR2 metadata/deobfuscation ideas, but it is **not** a full parity-repair implementation model
- no known production-ready Go PAR2 reconstruction engine has been identified in the current project/dependency set
- decypharr already has enough NNTP reading, disk staging, and local file serving patterns to make an external-tool workflow realistic

### Target v1 behavior

When an NZB payload archive fails due to likely incompleteness/corruption:

1. decypharr stages payload files and relevant PAR2 files to disk
2. decypharr invokes a configured or discovered PAR2 executable
3. if repair succeeds, decypharr retries parsing using repaired local files
4. if retry succeeds, decypharr streams from those repaired local files
5. if repair fails, decypharr exits with a precise error and leaves useful logs

### Hard constraints

- do not block v1 on finding a Go PAR2 library
- do not try to support every PAR2 binary family at once
- do not mix parser bug fixing with repair orchestration unless necessary for inputs
- do not build UI first; config/env support is enough for v1

### Recommended v1 scope

Agents should implement these items in order:

1. add config for repair binary/workspace/timeout
2. add local-file-backed `NZBFile` / `Volume` support
3. add repair workspace creation and cleanup helpers
4. add NNTP-to-disk staging of payload and PAR2 files
5. add PAR2 executable discovery and invocation
6. add parser fallback that runs repair once and retries parse once
7. persist repaired local paths so later streaming uses disk instead of NNTP
8. add tests and logs around each stage

### Explicit non-goals for v1

- native parity math in Go
- automatic health-check repair integration
- broad UI/settings support
- optimization for minimal staging
- support for every PAR2 binary syntax on day one

### Suggested agent split

If multiple smaller agents are used, split work like this:

#### Agent A: config + persistence

Owns:

- `internal/config/usenet.go`
- `pkg/storage/nzb.go`
- `pkg/usenet/nzb.proto`
- `pkg/usenet/nzb.pb.go`
- `pkg/usenet/storage.go`

Deliverable:

- config fields for repair binary/work path/timeout
- persisted `LocalPath` support for repaired files

#### Agent B: local-file serving

Owns:

- `pkg/usenet/types/volume.go`
- `pkg/usenet/nzb.go`
- `pkg/usenet/fs/fs.go`

Deliverable:

- `Volume` and FS can serve local repaired files with no NNTP fetch

#### Agent C: staging + external repair tool

Owns:

- new `pkg/usenet/repair_workspace.go`
- new `pkg/usenet/repair_download.go`
- new `pkg/usenet/repair_tool.go`

Deliverable:

- create repair workspace
- download payload/PAR2 files to disk
- run external PAR2 executable and capture results

#### Agent D: parser fallback integration

Owns:

- new `pkg/usenet/parser/repair_fallback.go`
- `pkg/usenet/parser/parser.go`
- `pkg/usenet/parser/par2.go`

Deliverable:

- one repair attempt on likely repairable parser failure
- one retry parse using repaired local files
- no infinite loops

#### Agent E: tests + observability

Owns:

- tests for grouping, staging, tool discovery, local serving, and retry behavior
- log coverage around staging, binary execution, and retry parse outcome

Deliverable:

- reproducible tests and debug output that make failures diagnosable

### Definition of success for agents

The implementation is on the right track if a broken-but-repairable release can:

- be staged locally
- be passed to a PAR2 tool
- produce repaired files
- be reparsed once
- then be streamed from local repaired files

## Goal

Add a practical, working PAR2 repair pipeline to decypharr for Usenet releases that:

- parse correctly but contain missing or damaged articles
- require PAR2 reconstruction before the payload archive becomes readable
- may still be streamable after repair by serving repaired local files instead of raw NNTP segments

This plan is intentionally scoped toward a first working version, not a perfect end-state design.

## Important reality check

Before implementing repair, it is important to be clear about the current situation:

1. `nzbdav` does **not** implement full parity reconstruction in the same way SABnzbd or JDownloader do.
   - It uses PAR2 primarily for deobfuscation metadata.
   - Its "repair" flow is mainly health checking and Arr re-search/removal.
2. decypharr currently has useful PAR2 **metadata** support, but not true PAR2 **reconstruction**.
3. A true repair solution is much easier and much safer to ship first by using an **external PAR2 binary** than by writing a native Go parity engine from scratch.

Because of that, the recommended path is:

- **Phase 1:** external-tool based repair fallback
- **Phase 2:** integrate repaired local files into streaming/parser flow
- **Phase 3:** improve detection, cleanup, and repair orchestration
- **Phase 4:** only consider native repair if external-tool workflow proves insufficient

---

## Recommended first architecture

### Chosen approach

Implement a **disk-based repair fallback** using an external PAR2 tool.

The high-level flow should be:

1. Parse NZB as normal.
2. If archive parsing fails or a repair path is explicitly needed:
   - stage relevant Usenet files and PAR2 files to a repair work directory
   - invoke a PAR2 repair executable
   - if repair succeeds, use the repaired local files instead of raw NNTP-backed segments
3. Continue parsing or streaming from the repaired local files.

### Why this is the right first implementation

- much smaller than writing a native PAR2 engine
- compatible with how real repair software already works
- easier to debug with actual release folders on disk
- decouples parity repair from the NNTP streaming reader internals
- gives a path to support more difficult releases without destabilizing the core reader

### Why not build native PAR2 repair first

A native implementation would require all of these pieces:

- packet parsing
- recovery set handling
- block map creation
- missing block detection
- parity matrix math / reconstruction
- file rewrite / verification
- edge case handling for split files, duplicate packets, damaged parity, etc.

That is a large standalone project.

---

## External dependency strategy

### Requirement

dec ypharr should support a configured or auto-detected PAR2 executable.

Candidate executables:

- `par2`
- `par2j`
- `par2repair`
- optionally a user-specified absolute path

### Current status on this machine

At the time of writing:

- `7z` is available
- no PAR2 repair executable was found in `PATH`

That means the first working repair implementation should:

- support config-driven executable path
- support PATH auto-detection
- fail gracefully with a clear message if no repair tool is available

### Additional gathered notes about tools and libraries

These are the practical observations gathered while investigating the feature.

#### 1. `nzbdav` reference findings

From the local `nzbdav` checkout:

- its `Par2Recovery` code is focused on **reading PAR2 packets** like `FileDesc`
- it uses PAR2 for **deobfuscation metadata**, not full parity reconstruction
- its `HealthCheckService` repair flow is mainly:
  - detect missing articles
  - mark unhealthy
  - delete / trigger Arr re-search

So `nzbdav` is useful inspiration for:

- PAR2 index selection
- packet parsing
- deobfuscation heuristics
- health-check orchestration

But it is **not** a drop-in model for true local parity reconstruction.

#### 2. Existing decypharr codebase patterns that help

The current codebase already has useful patterns for an external-tool repair implementation:

- `exec.LookPath(...)` is already used elsewhere in the project
- `exec.CommandContext(...)` is already used elsewhere in the project
- decypharr already has NNTP-backed readers that can materialize files to disk
- decypharr already has a file-based NZB metadata store that can persist updated file state

This lowers the implementation cost of an external PAR2-tool workflow.

#### 3. External Go library situation

I did **not** identify a known, already-vetted Go library in the current project or dependency set that provides a complete, production-ready PAR2 **repair/reconstruction** engine.

That is the key reason the plan recommends an external binary first.

There may be small PAR2 parsers or packet readers available in the wider ecosystem, but that would still leave the hard part unimplemented:

- recovery block math
- file reconstruction
- verification
- compatibility edge cases

So even if a Go PAR2 packet parser were adopted later, it would not meaningfully reduce the first-version effort unless it also shipped a proven reconstruction engine.

#### 4. Practical recommendation about Go libraries

For v1:

- do **not** spend time hunting for a Go PAR2 repair library unless a clearly mature and maintained one is already known and tested
- assume external binary invocation is the shortest path to a working result

For later evaluation:

- if a Go library is found, evaluate it only if it supports full repair, not just packet parsing
- require real-world validation on broken Usenet releases before trusting it in the main parser flow

### Recommendation

Add config like:

- `usenet.par2_binary`
- `usenet.repair_work_path`
- optionally `usenet.keep_repair_artifacts`
- optionally `usenet.par2_binary_args`
- optionally `usenet.repair_timeout`

No UI is required in the first pass if environment/config file support is easier.

### Tool compatibility notes to document in code/docs

The repair layer should explicitly document which tools are supported and tested.

Suggested policy:

- first-class support for one tool family only in v1
- detect others, but mark them untested unless an adapter exists

For example:

- supported: one chosen PAR2 executable syntax
- maybe supported later: alternate executable names with adapter-specific arguments
- unsupported in v1: tools requiring substantially different command semantics

---

## Implementation phases

# Phase 0: stabilize parsing prerequisites

This phase is not the repair feature itself, but it is required so repair has correct inputs.

### Step 0.1
Ensure PAR2 files and payload files never collapse into one mixed logical group.

### Step 0.2
Ensure unknown files are retained long enough for PAR2 deobfuscation to rename/promote them.

### Step 0.3
Ensure the smallest/main PAR2 index file is preferred for `FileDesc` extraction.

### Step 0.4
Add explicit debug logging for:

- number of payload groups
- number of PAR2 groups
- chosen PAR2 index file
- whether no payload groups remained after grouping

### Acceptance criteria

A release like the JDownloader example should produce:

- one payload archive group
- one or more separate PAR2 groups
- a useful log showing the split

---

# Phase 1: create a repair staging pipeline

This phase builds a temporary on-disk repair folder from NNTP-backed files.

## Objective

Be able to materialize all files needed for repair into a local work directory.

## Step 1.1: add repair workspace helpers

Create a repair workspace manager under something like:

- `pkg/usenet/repair_workspace.go`

Responsibilities:

- create a deterministic repair directory for a given NZB/group
- sanitize filenames
- expose paths for payload files, par2 files, logs, and repaired outputs
- optionally clean up old workspaces

Suggested path structure:

- `<main_path>/usenet/repair/<nzb-id>/<group-name>/`

## Step 1.2: add file materialization logic

Implement logic that downloads a full logical NZB file to disk.

Potential location:

- `pkg/usenet/repair_download.go`

Use existing segment/reader infrastructure wherever possible.

For each file being staged:

- build a reader from its segments
- copy to a target file in the repair workspace
- preserve the expected filename from the NZB/par2 deobfuscation pass

This should support:

- payload archive files
- PAR2 index files
- PAR2 recovery volume files

## Step 1.3: select what gets staged

Initial rule:

- stage **all files** from the failed payload group
- stage **all related PAR2 files** from the same NZB base name/release

Do not try to be too clever in v1. Over-staging is acceptable.

## Step 1.4: verify staged files exist and are non-empty

Before invoking repair:

- verify all staged files exist
- verify the chosen main `.par2` file exists
- log byte sizes

### Acceptance criteria

Given a failed group, decypharr can write a repair workspace containing:

- all payload parts
- all relevant PAR2 files
- a clear log of what was staged

---

# Phase 2: invoke the external PAR2 repair tool

## Objective

Call a PAR2 executable against the staged repair workspace.

## Step 2.1: add executable discovery

Potential file:

- `pkg/usenet/repair_tool.go`

Implement:

- explicit configured path wins
- fallback to `exec.LookPath` for known executable names
- return a structured error if none found

## Step 2.2: add command execution wrapper

Implement a wrapper that:

- uses `exec.CommandContext`
- runs in the repair workspace directory
- captures stdout/stderr to a log file and in-memory string
- returns exit code, output, duration

## Step 2.3: define supported invocation syntax

For the first pass, support one known syntax cleanly.

Recommended first target:

- `par2 repair <index.par2>`

If supporting shorthand too:

- `par2 r <index.par2>`

If you want to support `par2j`, isolate syntax differences behind a small adapter layer.

The implementation should not assume every detected executable accepts the same flags. Build a tiny adapter interface, for example:

- discover executable
- identify tool family
- build command arguments
- parse output / success conditions

That keeps the first implementation narrow while leaving room for later compatibility expansion.

## Step 2.4: detect repair success

Do not rely only on exit code.

Success criteria should include:

- zero exit code
- repaired payload files now exist and are readable
- optionally archive test/list now succeeds

### Acceptance criteria

dec ypharr can:

- find a repair executable
- run it in the workspace
- capture output
- report success/failure clearly

---

# Phase 3: feed repaired files back into decypharr

## Objective

Allow parser and streamer to use repaired local files instead of NNTP segments.

## Step 3.1: add a local-file backing field to logical NZB files

Add a field like:

- `LocalPath string`

in `pkg/storage/nzb.go`

and persist it in the NZB protobuf model.

This allows a logical file to become:

- normal NNTP-backed file, or
- repaired local-disk-backed file

## Step 3.2: add local path support to Usenet volumes/filesystem

Update the Usenet FS layer so that if a `Volume` has `LocalPath` set:

- reads come from the local file
- no NNTP fetch is attempted
- prefetch becomes a no-op

Likely files:

- `pkg/usenet/types/volume.go`
- `pkg/usenet/nzb.go`
- `pkg/usenet/fs/fs.go`

## Step 3.3: update parser fallback to emit local-backed repaired files

When repair succeeds, create `storage.NZBFile` values that point at repaired local files.

Two possible patterns:

### Option A: serve repaired archive parts directly

- if repair only reconstructed `.rar`/`.7z` parts, point logical files to those local repaired archives
- continue archive parsing from local disk

### Option B: parse repaired archive immediately and emit extracted logical files

- run the normal parser again against the repaired local archive set
- emit final extracted files with `LocalPath` set

Recommended first implementation:

- **Option A first**, because it reuses current parser flow and keeps repair separate from extraction

## Step 3.4: save updated NZB metadata after successful repair

If an NZB now has local-backed files, call storage save again so later stream requests can access them.

### Acceptance criteria

After repair:

- decypharr can stream from repaired local files
- no NNTP fetch is used for those repaired files
- the NZB entry remains usable after processing completes

---

# Phase 4: trigger repair from parser failures

## Objective

Use repair automatically when it is actually needed.

## Step 4.1: define repair-trigger conditions

Initial triggers should be conservative.

Good candidates:

- archive parser fails with known corruption/incomplete symptoms
- NNTP fetches show definitive missing articles for payload segments
- archive test/list still fails after PAR2 deobfuscation and format fallback

Avoid triggering repair for:

- auth errors
- connection timeouts only
- unsupported compression formats
- clearly unrelated parser bugs

## Step 4.2: add parser-side repair fallback orchestration

Potential file:

- `pkg/usenet/parser/repair_fallback.go`

This orchestration should:

1. collect failed payload group
2. locate related PAR2 groups
3. stage files
4. run repair tool
5. if repair succeeds, retry parser using repaired local inputs

## Step 4.3: add loop protection

Prevent infinite fallback loops with flags such as:

- `repairAttempted`
- `par2Attempted`

at group or processing scope.

### Acceptance criteria

When a repairable archive fails during parse:

- decypharr stages files
- runs repair once
- retries parsing once
- either succeeds or exits with a precise error

---

# Phase 5: add post-processing and cleanup

## Objective

Make the feature safe to run repeatedly.

## Step 5.1: artifact retention policy

Config options:

- keep artifacts on failure only
- keep nothing by default
- optionally keep everything for debugging

## Step 5.2: workspace cleanup

Delete repair workspace on:

- successful parse if retention disabled
- NZB delete
- periodic cleanup for stale folders

## Step 5.3: logging and observability

Log these explicitly:

- repair workspace path
- payload files staged
- par2 files staged
- chosen repair index file
- repair binary path
- command line used
- exit code
- whether retry parse succeeded

### Acceptance criteria

The repair system is debuggable without needing to instrument the code again.

---

# Phase 6: integrate with health-check style repair later

This is a later extension, not required for v1.

## Objective

Allow existing entries that later become unhealthy to be repaired locally instead of only being marked broken.

## Step 6.1
Add a path in the repair sweep for Usenet entries:

- if specific files become missing/unhealthy
- and if PAR2 files are available in NZB metadata or can be restaged
- try local repair before escalating to Arr replacement

## Step 6.2
If repair succeeds:

- update the logical file to local-backed repaired file
- mark health check successful

## Step 6.3
If repair fails:

- continue current behavior: unhealthy, re-search, or delete flow

This phase should come only after parser-time repair is stable.

---

## Concrete code touch points

These are the most likely files to change.

### Config

- `internal/config/usenet.go`
- optionally config UI later

### Storage / persistence

- `pkg/storage/nzb.go`
- `pkg/usenet/nzb.proto`
- `pkg/usenet/nzb.pb.go`
- `pkg/usenet/storage.go`

### Usenet local-file serving

- `pkg/usenet/types/volume.go`
- `pkg/usenet/nzb.go`
- `pkg/usenet/fs/fs.go`

### Repair orchestration

- new: `pkg/usenet/repair_workspace.go`
- new: `pkg/usenet/repair_tool.go`
- new: `pkg/usenet/repair_download.go`
- new: `pkg/usenet/parser/repair_fallback.go`
- maybe new later: `pkg/usenet/par2_adapter.go`
- maybe new later: `pkg/usenet/par2_output.go`

### Parser integration

- `pkg/usenet/parser/parser.go`
- `pkg/usenet/parser/par2.go`
- maybe `pkg/usenet/parser/7z.go`
- maybe `pkg/usenet/parser/rar.go`
- maybe `pkg/usenet/parser/zip.go`

---

## Suggested implementation order

This is the exact order I would implement it in.

1. Fix grouping and unknown-file retention so payload groups are correct.
2. Add `LocalPath` support to logical files and Usenet FS.
3. Add repair workspace creation.
4. Add file materialization from NNTP to repair workspace.
5. Add PAR2 executable discovery and invocation.
6. Add parser repair fallback orchestration.
7. Retry parsing from repaired local files.
8. Add cleanup policy and logs.
9. Add targeted tests.
10. Only then consider repair-sweep integration.

---

## Testing plan

### Unit tests

Add tests for:

- PAR2 and payload grouping separation
- repair binary discovery logic
- workspace path generation
- local-path volume serving in FS
- success/failure parsing of repair command output

### Integration-style tests

Use fixture-style scenarios for:

1. obfuscated archive + index par2 + recovery volumes
2. missing one archive part but enough PAR2 recovery available
3. no repair executable available
4. repair executable present but repair fails
5. repair succeeds and parser retry succeeds

### Manual real-world tests

Use at least three NZB types:

- obfuscated multipart RAR release
- obfuscated multipart 7z release
- release with known missing article that JDownloader can repair

---

## Risks and mitigations

### Risk: no PAR2 binary installed

Mitigation:

- make it explicit in logs and config validation
- allow configured binary path
- fail gracefully

### Risk: repair workspace uses too much disk

Mitigation:

- workspace per NZB/group
- cleanup policy
- optional max size checks

### Risk: local repaired files diverge from metadata

Mitigation:

- persist `LocalPath`
- re-save NZB metadata after successful repair
- log every substitution clearly

### Risk: parser fallback loops forever

Mitigation:

- one repair attempt per group per processing pass
- explicit repair-attempt flags

### Risk: syntax differences between PAR2 binaries

Mitigation:

- support one binary family first
- abstract binary adapter logic
- document supported tools
- keep command building and output parsing separate from parser fallback logic

### Risk: hidden assumption that a Go library will save time later

Mitigation:

- treat Go-library adoption as a separate evaluation task
- require that any candidate library solve actual reconstruction, not only packet parsing
- do not block v1 on third-party Go library research

---

## Non-goals for first version

Do **not** include these in the first pass:

- native Go parity reconstruction engine
- UI for every repair option
- automatic repair during periodic health check
- optimized partial-file staging
- support for every PAR2 tool variant on day one

---

## Definition of done for v1

v1 is done when all of the following are true:

1. A repairable Usenet release can be staged to disk.
2. decypharr can invoke a PAR2 repair executable successfully.
3. Repaired output can be used by decypharr for parsing/streaming.
4. Failures are logged clearly.
5. The system does not loop forever or silently discard payload groups.

---

## Final recommendation

The best next implementation is **not** to chase more PAR2 metadata tricks first.

The best next implementation is:

- external-binary repair fallback
- local repaired file serving
- parser retry from repaired files

That gives the highest chance of matching the real behavior users expect from JDownloader/SABnzbd while keeping the implementation realistic for decypharr.