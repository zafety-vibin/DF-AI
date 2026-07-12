# Plugin build & deploy

WHAT: the C++ DFHack plugin (`dfhack-plugin/`) is the game-side half of DF-AI — it extracts state, executes designations/buildings/orders, and answers queries over the binary TCP protocol.

WHY it builds the way it does: **there is no DFHack SDK.** Release zips and the Steam app ship zero headers or CMake helpers. Plugins compile in-tree inside a DFHack *source checkout* at the exact release tag, because the plugin loader enforces an exact version-string match at load time.

## Layout

- A DFHack source checkout lives as a sibling of this repo (conventionally `../dfhack-build`; run `git -C ../dfhack-build describe --tags` for its current tag — never trust a doc for this).
- Inside it, `plugins/df_ai_protocol` is a **directory junction** to this repo's `dfhack-plugin/`. Editing files here and there is the same thing. If that path is ever a real directory, stop and re-junction — a stale copy silently builds wrong code.
- The plugin is registered via `plugins/CMakeLists.custom.txt` (DFHack's gitignored local-plugin hook), so tag switches never conflict.

## Build

From the checkout root:

```bash
cmake --build build --target df_ai_protocol --config Release
```

- Redirect output to a log file and check the real exit code — never pipe through `tail` (pipeline exit masking has produced false greens twice).
- Verify the artifact mtime is fresh: `build/plugins/df_ai_protocol/Release/df_ai_protocol.plug.dll`. Stale artifacts have fooled us.
- The `generate_headers` step (Perl codegen) crashes flakily (exit -1073741819). Run it standalone once and retry before diagnosing anything.
- Release or RelWithDebInfo only — Debug is not binary-compatible with DF. MSVC 2022 (v143) required.

## Deploy

Copy the `.plug.dll` into `<Steam DF>/hack/plugins/` with DF closed (or the plugin unloaded — the file is locked while loaded). In the DFHack console: `load df_ai_protocol`, then `ai-connect`. The plugin dials the port in `config/orchestrator.yaml` (`listen_port`).

## Upgrading DFHack versions

1. In the checkout: `git fetch --tags && git checkout -f <new-tag> && git submodule update --init --recursive` (the `-f` discards their tracked-file drift; our plugin hook and junction survive).
2. Re-run the build; fix drift. Audit these surfaces against `library/include/` (hand-written modules) and `library/include/df/` (generated): workshop/building/construction type enums (insertions shift values), `modules/Buildings.h` signatures, `modules/World.h` pause API, `df/tiletype.h` shape/material enums, `df/manager_order.h`, announcement types.
3. Rebuild, redeploy, live-verify. Every tag bump requires redeploy — exact version match.

## Threading model (do not regress)

- The socket thread receives messages and enqueues commands/queries; queues drain BOTH in `plugin_onupdate` (main thread, unpaused) AND via `drain_from_socket_thread()` under `CoreSuspender` — because **onupdate never fires while DF is paused**, and the turn protocol lives paused.
- All socket writes go through `socket_send_locked` (send mutex).
- Every dispatch path that touches DF state must sit inside the `executeCommand` try/catch — DFHack `CHECK_*` precondition macros throw, and an uncaught throw off a thread terminates DF with no trace.
- Never delete `df::report` objects (DF pools them; the announcements vector grows monotonically — cursor by max report id).
