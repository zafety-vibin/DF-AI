# Goals — Fort #2 (fresh embark, y100; state as of spring day 45)

## OVERSEER DIRECTION (2026-07-15): a NEW MAP is coming — Fort #3 will be
## a fresh embark. Open it with the fort-planning skill (level categories,
## dig-ahead, confirmed taste canon) + fort-opening. First planning act:
## look scope=elevation for the whole-map layout pass, then scope=fort for
## routine reviews. Blueprint capture (save_blueprint) is live for pods.
## Fort #2's remaining verification items below still apply to whichever
## save runs next with the new DLL.

## NOW (survival)
- [ ] NEXT SESSION FIRST: deploy the freshly built plugin DLL (DF must be
      CLOSED; artifact at ../dfhack-build/build/plugins/df_ai_protocol/
      Release/df_ai_protocol.plug.dll → Steam DF hack/plugins/), reconnect,
      then live-verify the 2026-07-14 wave in this order: list_crops (DLL
      freshness probe) → brew at the still (drink chain!) → farm plot +
      crops → hatch the shaft top → remove_zone on the junk zones →
      confirm 'd' overlay persistence + repeat-warning step lines.
- [x] Aquifer (z=128, single-layer sand) pierced AND sealed — 8 walls, dry
      shaft bottom verified. UNSUPERVISED — the protocol held.
- [x] Underground shelter: base room + dorm z=130, quarry/workshops z=126
- [x] Wood chain: 22 logs, carpenter workshop built (z=126)
- [ ] FOOD IS THIN: ~12 edible + 8 raw fish, no farming possible (tool gap),
      no gatherable shrubs in revealed area. Fishery next (raw fish x8);
      watch stocks every session.
- [ ] Drinks: 12 left day 45; brewing BLOCKED (see gaps). Fallback: 7/7
      stream far W (water_source zone painted at (4-9,71,142)). Watch for
      "thirsty" unhappiness, not just dehydration.
- [x] no_active_hostiles

## SOON (headroom)
- [ ] Place 3 remaining beds (in stock) + bedroom zones toward
      has_bedroom_zones_7 (currently 2 zones: etur's + tavern lodging)
- [ ] Dining hall: queue tables+chairs at carpenter, designate dining_hall
      zone (type exists in vocabulary, untested)
- [ ] Fishery workshop (annex at z=126 has space once fully dug)
- [ ] Watch lodging: tavern exists w/ 1 guest room; first visitor/migrant
      wave will be the real assign_lodging verification
- [ ] Survive first migrant wave (none yet at day 45)

## EVENTUAL (trajectory)
- [ ] Survive year 1 (zero starvation/dehydration deaths)
- [ ] Defensible single entrance (shaft top at (49-50,47-48,131) is the only
      way in; hatch covers still tool-blocked)
- [ ] Trade depot before first caravan (tool-blocked)

## Tool gaps (engineering queue — logged 2026-07-14, updated same day
## after the ultracode wave; SHIPPED = compile/test-verified, NOT yet
## live-verified — the running DF still has the OLD plugin DLL)
- [x] SHIPPED: reaction-based brewing — queue_job gained `reaction` param
      (e.g. BREW_DRINK_FROM_PLANT) + list_reactions discovery tool.
      LIVE-VERIFY FIRST: needs DLL redeploy (DF closed), then queue one
      brew at the still and confirm drink appears + seeds return.
- [x] SHIPPED: remove_zone (by coordinate; refuses zones that found a
      Location, with a named error). Live-verify on the 2 junk zones in
      Fort #2's tavern: Barracks (51,49)-(52,50), animal_training
      (55,49)-(56,50) z=130.
- [x] SHIPPED: connector-suggestion fixes (diagonal suggestions no longer
      emit bounding-box rectangles; vertical adjacency now seen by the
      "not yet connected" check — both false positives from this session
      should be gone).
- [x] SHIPPED: list_zones "animals" label fix (roster phrasing now only
      for Pen/Pond) + designate_zone unknown-type error lists the full
      snake_case vocabulary.
- [x] SHIPPED: look designation overlay now includes in-flight dig jobs
      (plugin-side job-list merge, both full-state and delta paths).
      Live-verify: designate, step until a miner claims the job, look —
      'd' should persist.
- [x] SHIPPED: repeated-announcement visibility — step reports now carry
      "repeated warnings this step: ... xN" (DF pools repeats in-place;
      the old id cursor never saw them). Ends the silent damp-cancel
      blindness during aquifer work.
- [x] SHIPPED: farm plots — build_farm_plot (extent-shaped, own tool, NOT
      a `build` type) + assign_crop (per-season incl. fallow/all) +
      list_crops. A plot with no crop for the current season grows
      NOTHING, silently. list_crops is the canonical probe that the
      running DLL has the farm handlers. See .claude/skills/df-farming.
- [x] SHIPPED: ConstructHatchCover in the queue_job whitelist — hatch the
      shaft top (46-47,50-51 area) once the DLL is deployed.
- [x] SHIPPED: dwarves verbose census mode + dwarf_detail include_labors
      opt-in (default now a count, not the 80-line list); look radius cap
      23 + scope=overview (whole-map downsample); SURFACE is now a
      per-column fact and survey_site reports a min..max range (ground
      ABOVE spawn elevation is normal and workable).
- [ ] kitchen_permissions tool — the ONLY mitigation for cooking
      destroying the farm seed loop; until it ships, don't build a
      kitchen / don't queue cook jobs while the seed loop establishes.
- [ ] well / floodgate / lever build types + linking — blocks
      aquifer-water-infrastructure (see the PLANNED skill stub).
- [ ] Farm-plot state visibility: buildings can't show per-season crop
      assignments — track in memory until a query exposes it.
- [ ] `jobs` tool is workshop-only; still no global "what is everyone
      doing" query (dwarves verbose covers the census case per-call).
- [ ] stocks category "FOOD" isn't a real category (MEAT/FISH/PLANT/DRINK
      are) — returns empty silently; could suggest valid categories.
- [ ] find_dig_site called surface grass/walkable tiles "fully solid" on
      the fresh embark (pre-first-dig) — wrong solidity claim on initial
      full-state; NOT addressed by this wave, still open.
- [ ] Plugin-side batch dwarf-detail query (one round trip for the whole
      roster) — dwarves verbose fans out up to 50 queries; fine for now,
      wasteful at migrant-wave scale.
