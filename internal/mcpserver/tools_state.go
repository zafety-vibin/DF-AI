package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/df-ai/orchestrator/internal/protocol"
	"github.com/df-ai/orchestrator/internal/worldmodel"
)

// maxDwarfList caps the dwarves listing; beyond it the tool summarizes.
const maxDwarfList = 50

// rawJSONCap bounds raw plugin-query payloads (stocks, orders) so one huge
// response cannot blow the MCP output budget (~10k-token cap per response).
const rawJSONCap = 8 * 1024

// capRawJSON truncates an oversized raw payload at a rune boundary and says
// so, instead of silently flooding the response.
func capRawJSON(raw string) string {
	if len(raw) <= rawJSONCap {
		return raw
	}
	cut := rawJSONCap
	for cut > 0 && !utf8.RuneStart(raw[cut]) {
		cut--
	}
	return raw[:cut] + "\n...truncated, refine your query"
}

// buildingListEntry is one entry from the plugin's list_buildings query.
// Shared between the flat `buildings` tool (renderBuildings) and the
// `buildings` look lens (internal/mcpserver/lenses.go) so both parse the
// exact same wire shape once.
type buildingListEntry struct {
	Type        string `json:"type"`
	X           int    `json:"x"`
	Y           int    `json:"y"`
	Z           int    `json:"z"`
	X1          int    `json:"x1"`
	Y1          int    `json:"y1"`
	X2          int    `json:"x2"`
	Y2          int    `json:"y2"`
	Stage       int    `json:"stage"`
	MaxStage    int    `json:"max_stage"`
	Done        bool   `json:"done"`
	BridgeState string `json:"bridge_state,omitempty"` // Bridge only: "raised"/"raising"/"lowering"/"lowered" (queries.cpp handleListBuildings)
}

// renderBuildings renders the list_buildings query response, one line per
// building. Unfinished buildings are the interesting case: a planned
// building is invisible in every map view, so a plan that silently died
// (missing materials, unreachable site) only shows up here — construction
// stage is rendered loudly.
func renderBuildings(raw []byte) string {
	var resp struct {
		Buildings []buildingListEntry `json:"buildings"`
		Truncated bool                `json:"truncated"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Sprintf("unparseable list_buildings response: %v\nraw: %s", err, capRawJSON(string(raw)))
	}
	if len(resp.Buildings) == 0 {
		return "No buildings."
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "%d buildings:\n", len(resp.Buildings))
	for _, bl := range resp.Buildings {
		state := ""
		if bl.BridgeState != "" {
			state = fmt.Sprintf(" [%s]", bl.BridgeState)
		}
		if bl.Done {
			fmt.Fprintf(&sb, "- %s at (%d,%d,%d) — built%s\n", bl.Type, bl.X, bl.Y, bl.Z, state)
		} else {
			fmt.Fprintf(&sb, "- %s at (%d,%d,%d) — UNDER CONSTRUCTION (stage %d/%d)%s\n",
				bl.Type, bl.X, bl.Y, bl.Z, bl.Stage, bl.MaxStage, state)
		}
	}
	if resp.Truncated {
		sb.WriteString("... list truncated at the plugin's cap — pass z to narrow it\n")
	}
	return sb.String()
}

// stockItem is one (item_type, material) entry from the plugin's
// stockpile_inventory query. Count is free/available stock only; InUse is
// the same item/material already incorporated into a building or
// construction (a built bed, an installed door, a boulder mortared into a
// wall) — still a live item in DF's eyes but not a spare a caller can
// place. The two are tallied separately upstream in queries.cpp
// handleStockpileInventory so callers can't mistake "fort has 3 beds
// total" for "3 beds are free to assign."
type stockItem struct {
	ItemType string `json:"item_type"`
	Material string `json:"material"`
	Count    int    `json:"count"`
	InUse    int    `json:"in_use,omitempty"`
	Economic bool   `json:"economic,omitempty"`
}

// stockQuality is one (item_type, quality tier) tally from the plugin's
// stockpile_inventory query — additive sibling of "items" above, counting
// free and in-use items alike (a built bed's craftsmanship is as real as a
// spare one's). The plugin only emits an entry when it exists, and only for
// item types with at least one above-Ordinary item, so an all-Ordinary
// stockpile (boulders, seeds, ...) doesn't pad the response with it.
type stockQuality struct {
	ItemType string `json:"item_type"`
	Quality  string `json:"quality"`
	Count    int    `json:"count"`
}

// stockSubtype is one (item_type, material, subtype) tally from the
// plugin's stockpile_inventory query — additive sibling of "quality" above,
// WEAPON and TOOL only. "WEAPON: iron x3" alone can't distinguish a pick
// from a battle axe; this breaks that same free+in-use-together aggregate
// down by the raws-defined subtype name (queries.cpp's ItemTypeInfo decode,
// the same one the mandates/moods tools already use). Rendered only in the
// category-filtered detailed view — see renderStocks — so the default
// aggregated view stays compact.
type stockSubtype struct {
	ItemType    string `json:"item_type"`
	Material    string `json:"material"`
	SubtypeName string `json:"subtype_name"`
	Count       int    `json:"count"`
}

// stocksCategoryAliases maps a friendly filter word to the actual
// ENUM_KEY_STR(item_type) name it should search for, when the two don't
// share a substring. Mechanisms are the one confirmed case in this fort's
// experience: DF stores them under item_type TRAPPARTS (its raws/UI name
// is "mechanism"), so category="mechan" finds nothing without this alias.
// Keep this a small lookup, not a general synonym system — add an entry
// only when a real live session hits the same kind of naming mismatch.
var stocksCategoryAliases = map[string]string{
	"MECHANISM":  "TRAPPARTS",
	"MECHANISMS": "TRAPPARTS",
}

// stocksQueryArgs builds the stockpile_inventory query args. The plugin
// (queries.cpp handleStockpileInventory) matches category as a
// case-sensitive substring of ENUM_KEY_STR(item_type), which is always
// uppercase ("BOULDER", "WEAPON") — uppercasing here is what makes the
// tool's documented lowercase examples match at all.
func stocksQueryArgs(category string) string {
	if category == "" {
		return "{}"
	}
	upper := strings.ToUpper(category)
	if alias, ok := stocksCategoryAliases[upper]; ok {
		upper = alias
	}
	argBytes, _ := json.Marshal(map[string]string{"category": upper})
	return string(argBytes)
}

// renderStocks renders the stockpile_inventory response. The plugin keys
// every item by exact (item_type, material) with no cap, which already
// exceeds the MCP payload budget at a 7-dwarf fort (see
// docs/archive/2026-07-12-token-scaling-research.md) — the raw pass-through
// this replaced silently truncated mid-JSON. Default (detailed=false):
// aggregate to one line per item_type with top-3 materials by count.
// detailed=true (set when the caller passed a category filter, which the
// plugin already narrows before this ever sees it): one line per entry.
func renderStocks(raw []byte, detailed bool, minCount int) string {
	var resp struct {
		Items    []stockItem    `json:"items"`
		Quality  []stockQuality `json:"quality"`
		Subtypes []stockSubtype `json:"subtypes"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Sprintf("unparseable stockpile_inventory response: %v\nraw: %s", err, capRawJSON(string(raw)))
	}
	items := resp.Items

	// qualityByType groups the additive quality tally by item type,
	// preserving the plugin's Ordinary→Artifact emission order within each
	// type, so both render paths below can look up "does this type have a
	// quality story worth printing" in O(1).
	qualityByType := make(map[string][]stockQuality)
	for _, q := range resp.Quality {
		qualityByType[q.ItemType] = append(qualityByType[q.ItemType], q)
	}
	formatQuality := func(qs []stockQuality) string {
		parts := make([]string, 0, len(qs))
		for _, q := range qs {
			parts = append(parts, fmt.Sprintf("%d %s", q.Count, q.Quality))
		}
		return strings.Join(parts, ", ")
	}

	// subtypeByType groups the additive WEAPON/TOOL subtype tally by item
	// type — same grouping shape as qualityByType, used only in the detailed
	// (category-filtered) render below so the default aggregated view stays
	// compact.
	subtypeByType := make(map[string][]stockSubtype)
	for _, s := range resp.Subtypes {
		subtypeByType[s.ItemType] = append(subtypeByType[s.ItemType], s)
	}
	formatSubtypes := func(ss []stockSubtype) string {
		sort.Slice(ss, func(i, j int) bool { return ss[i].Count > ss[j].Count })
		parts := make([]string, 0, len(ss))
		for _, s := range ss {
			parts = append(parts, fmt.Sprintf("%d %s %s", s.Count, s.Material, s.SubtypeName))
		}
		return strings.Join(parts, ", ")
	}
	if minCount > 0 {
		filtered := items[:0]
		for _, it := range items {
			// min_count only suppresses noise on the free-stock signal; an
			// entry with genuine built-in presence (InUse > 0) must still
			// surface even when its free Count is below the threshold —
			// otherwise a fully-built-in item type (e.g. "3 built-in
			// tables, 0 free") silently vanishes and a caller can no
			// longer tell the fort has any at all.
			if it.Count >= minCount || it.InUse > 0 {
				filtered = append(filtered, it)
			}
		}
		items = filtered
	}
	if len(items) == 0 {
		return "No stock items (or all below min_count)."
	}

	if detailed {
		var sb strings.Builder
		fmt.Fprintf(&sb, "%d item/material entries:\n", len(items))
		seenQualityType := make(map[string]bool)
		for _, it := range items {
			econ := ""
			if it.Economic {
				econ = " [economic]"
			}
			inUse := ""
			if it.InUse > 0 {
				inUse = fmt.Sprintf(" (+%d built-in)", it.InUse)
			}
			fmt.Fprintf(&sb, "- %s: %s x%d%s%s\n", it.ItemType, it.Material, it.Count, econ, inUse)
		}
		// Quality and subtype are each tallied per item_type, not per
		// material, so both print once per type below the material entries
		// rather than per line.
		for _, it := range items {
			if seenQualityType[it.ItemType] {
				continue
			}
			seenQualityType[it.ItemType] = true
			if qs, ok := qualityByType[it.ItemType]; ok {
				fmt.Fprintf(&sb, "  quality (%s): %s\n", it.ItemType, formatQuality(qs))
			}
			if ss, ok := subtypeByType[it.ItemType]; ok {
				fmt.Fprintf(&sb, "  subtypes (%s): %s\n", it.ItemType, formatSubtypes(ss))
			}
		}
		return sb.String()
	}

	type typeAgg struct {
		total     int
		inUse     int
		economic  int
		materials []stockItem
	}
	order := make([]string, 0)
	byType := make(map[string]*typeAgg)
	for _, it := range items {
		agg, ok := byType[it.ItemType]
		if !ok {
			agg = &typeAgg{}
			byType[it.ItemType] = agg
			order = append(order, it.ItemType)
		}
		agg.total += it.Count
		agg.inUse += it.InUse
		if it.Economic {
			agg.economic += it.Count
		}
		agg.materials = append(agg.materials, it)
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "%d item types, %d item/material entries total (pass category to see full per-material detail for one type):\n",
		len(order), len(items))
	for _, t := range order {
		agg := byType[t]
		sort.Slice(agg.materials, func(i, j int) bool { return agg.materials[i].Count > agg.materials[j].Count })
		topN := agg.materials
		if len(topN) > 3 {
			topN = topN[:3]
		}
		tops := make([]string, 0, len(topN))
		for _, m := range topN {
			tops = append(tops, fmt.Sprintf("%s %d", m.Material, m.Count))
		}
		econNote := ""
		if agg.economic > 0 {
			econNote = fmt.Sprintf(" (%d economic)", agg.economic)
		}
		inUseNote := ""
		if agg.inUse > 0 {
			inUseNote = fmt.Sprintf(" (+%d built-in)", agg.inUse)
		}
		qualityNote := ""
		if qs, ok := qualityByType[t]; ok {
			qualityNote = fmt.Sprintf(" [%s]", formatQuality(qs))
		}
		fmt.Fprintf(&sb, "- %s: %d total across %d material%s%s%s%s; top: %s\n",
			t, agg.total, len(agg.materials), plural(len(agg.materials)), econNote, inUseNote, qualityNote, strings.Join(tops, ", "))
	}
	return sb.String()
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// managerOrder is one entry from the plugin's manager_orders query.
type managerOrder struct {
	ID               int    `json:"id"`
	JobType          string `json:"job_type"`
	AmountTotal      int    `json:"amount_total"`
	AmountLeft       int    `json:"amount_left"`
	Material         string `json:"material"`          // MaterialInfo::toString(); "" when mat_type/mat_index don't decode
	MaterialCategory string `json:"material_category"` // bitfield_to_string(); "" when no bit set
	Frequency        string `json:"frequency"`         // df::workquota_frequency_type key; "OneTime" for a plain one-off order
	Validated        bool   `json:"validated"`
	Active           bool   `json:"active"`
}

// renderManagerOrders renders the manager_orders response compactly. Does
// NOT fetch list_orders (the DF job-type enum catalog, ~240 static entries,
// ~2000 tokens of unchanging noise every call) — the common orderable
// vocabulary is already in the order/queue_job tool schemas, and anything
// outside it is discoverable on demand via the separate job_types tool
// (see registerStateTools below) rather than paid on every orders call.
func renderManagerOrders(raw []byte) string {
	var resp struct {
		Orders []managerOrder `json:"orders"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Sprintf("unparseable manager_orders response: %v\nraw: %s", err, capRawJSON(string(raw)))
	}
	if len(resp.Orders) == 0 {
		return "No manager orders queued."
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "%d manager orders:\n", len(resp.Orders))
	for _, o := range resp.Orders {
		status := "queued, not yet dispatched"
		if o.Active {
			status = "ACTIVE"
		} else if !o.Validated {
			status = "invalid"
		}
		// mat: only printed when the order actually carries a material
		// selector — most job types (PrepareMeal, ConstructBlocks' generic
		// default) carry neither and this segment is silently empty.
		mat := ""
		if o.Material != "" {
			mat = " material=" + o.Material
		} else if o.MaterialCategory != "" {
			mat = " material=" + o.MaterialCategory
		}
		freq := ""
		if o.Frequency != "" && o.Frequency != "OneTime" {
			freq = " freq=" + o.Frequency
		}
		fmt.Fprintf(&sb, "- id=%d %s x%d (%d left)%s%s — %s\n", o.ID, o.JobType, o.AmountTotal, o.AmountLeft, mat, freq, status)
	}
	return sb.String()
}

// mandateIssuer is the noble who issued a mandate, when the plugin could
// resolve the unit pointer (list_mandates' "issued_by", null otherwise).
type mandateIssuer struct {
	ID        int    `json:"id"`
	FirstName string `json:"first_name"`
}

// mandatePunishment mirrors df::punishmentst as read by the plugin verbatim
// — NOT computed or inferred. Whether DF has populated these fields at
// mandate-issue time or only once broken is unconfirmed (that logic lives
// in DF's closed-source engine); an all-zero/all-false punishment on an
// otherwise real mandate reflects DF's own state, not a decode bug.
type mandatePunishment struct {
	Hammerstrikes     int  `json:"hammerstrikes"`
	PrisonMonths      int  `json:"prison_months"`
	Beating           bool `json:"beating"`
	Exiled            bool `json:"exiled"`
	DeathSentence     bool `json:"death_sentence"`
	NoPrisonAvailable bool `json:"no_prison_available"`
}

// mandateEntry is one entry from the plugin's list_mandates query
// (df::global::world->mandates.all — see queries.cpp handleListMandates for
// field provenance). Index is the entry's position in that vector for THIS
// call only — mandates carry no stable ID of their own, so it is not safe
// to remember across calls.
type mandateEntry struct {
	Index           int               `json:"index"`
	Mode            string            `json:"mode"` // "Export" | "Make" | "Guild"
	ItemType        string            `json:"item_type"`
	ItemSubtype     int               `json:"item_subtype"`
	ItemSubtypeName string            `json:"item_subtype_name"` // DFHack ItemTypeInfo::toString(); "" when the type has no meaningful subtype
	Material        string            `json:"material"`
	AmountTotal     int               `json:"amount_total"`
	AmountRemaining int               `json:"amount_remaining"`
	TimeoutCounter  int               `json:"timeout_counter"`
	TimeoutLimit    int               `json:"timeout_limit"`
	TicksRemaining  int64             `json:"ticks_remaining"`
	IssuedBy        *mandateIssuer    `json:"issued_by"`
	Punishment      mandatePunishment `json:"punishment"`
	PunishMultiple  bool              `json:"punish_multiple"`
	TotalExempt     bool              `json:"total_exempt"`
}

// renderMandates renders the list_mandates response. Export and Make are
// the item-quota mandates a fort actually needs to act on (ban an export,
// keep a production target fed); Guild ("pay job" dues) is a distinct
// mandate flavor confirmed only by DF's own enum naming (PAYJOB) — its
// item/material/quantity fields may not carry the same meaning, so this
// prints them uniformly rather than assuming per-mode semantics that
// aren't confirmed.
func renderMandates(raw []byte) string {
	var resp struct {
		Mandates []mandateEntry `json:"mandates"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Sprintf("unparseable list_mandates response: %v\nraw: %s", err, capRawJSON(string(raw)))
	}
	if len(resp.Mandates) == 0 {
		return "No active mandates."
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "%d active mandate%s:\n", len(resp.Mandates), plural(len(resp.Mandates)))
	for _, m := range resp.Mandates {
		item := m.ItemType
		if m.Material != "" {
			item = m.Material + " " + item
		}
		if m.ItemSubtype >= 0 {
			if m.ItemSubtypeName != "" {
				item = fmt.Sprintf("%s (%s, subtype %d)", item, m.ItemSubtypeName, m.ItemSubtype)
			} else {
				item = fmt.Sprintf("%s (subtype %d)", item, m.ItemSubtype)
			}
		}
		issuer := "issuer unresolved"
		if m.IssuedBy != nil {
			issuer = fmt.Sprintf("issued by %s (id %d)", m.IssuedBy.FirstName, m.IssuedBy.ID)
		}

		var punish []string
		if m.Punishment.Beating {
			punish = append(punish, "beating")
		}
		if m.Punishment.Exiled {
			punish = append(punish, "exile")
		}
		if m.Punishment.DeathSentence {
			punish = append(punish, "DEATH SENTENCE")
		}
		if m.Punishment.NoPrisonAvailable {
			punish = append(punish, "would-be-imprisoned (no cell available)")
		}
		if m.Punishment.PrisonMonths > 0 {
			punish = append(punish, fmt.Sprintf("%d month%s prison", m.Punishment.PrisonMonths, plural(m.Punishment.PrisonMonths)))
		}
		if m.Punishment.Hammerstrikes > 0 {
			punish = append(punish, fmt.Sprintf("%d hammerstrike%s", m.Punishment.Hammerstrikes, plural(m.Punishment.Hammerstrikes)))
		}
		if m.PunishMultiple {
			punish = append(punish, "applies to up to 10 units, not just 1")
		}
		punishStr := "none recorded (may mean not-yet-decided by DF, not \"no punishment\")"
		if len(punish) > 0 {
			punishStr = strings.Join(punish, ", ")
		}

		exempt := ""
		if m.TotalExempt {
			exempt = " [exempt from total]"
		}

		fmt.Fprintf(&sb, "- [%d] %s mandate: %s — %d/%d remaining%s — %d ticks remaining — %s — punishment: %s\n",
			m.Index, m.Mode, item, m.AmountRemaining, m.AmountTotal, exempt, m.TicksRemaining, issuer, punishStr)
	}
	return sb.String()
}

// workDetailUnit is one entry of a work_details response's assigned_units
// list — a dwarf currently a member of that detail. FirstName is the same
// crude first-name-only identification the mandates/moods handlers use;
// it renders as "" (via JSON null) when the plugin couldn't resolve the
// unit pointer.
type workDetailUnit struct {
	ID        int    `json:"id"`
	FirstName string `json:"first_name"`
}

// workDetailEntry is one entry from the plugin's work_details query
// (df::global::plotinfo->labor_info.work_details — see queries.cpp
// handleListWorkDetails for field provenance). Index is the entry's
// position in that vector for THIS call only — work_details carries no
// stable ID of its own, so it is not safe to remember across calls where
// a detail might have been created or deleted.
type workDetailEntry struct {
	Index             int              `json:"index"`
	Name              string           `json:"name"`
	Icon              string           `json:"icon"`
	Mode              string           `json:"mode"` // Default | EverybodyDoesThis | NobodyDoesThis | OnlySelectedDoesThis
	NoModify          bool             `json:"no_modify"`
	CannotBeEverybody bool             `json:"cannot_be_everybody"`
	AllowedLabors     []string         `json:"allowed_labors"`
	AssignedUnits     []workDetailUnit `json:"assigned_units"`
}

// renderWorkDetails renders the work_details response — DF's own
// work-details/labor-group mechanism (df::work_detail), the authoritative
// store behind a dwarf's derived status.labors cache that set_labor
// writes (see set_labor's own doc comment in tools_action.go).
func renderWorkDetails(raw []byte) string {
	var resp struct {
		WorkDetails []workDetailEntry `json:"work_details"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Sprintf("unparseable work_details response: %v\nraw: %s", err, capRawJSON(string(raw)))
	}
	if len(resp.WorkDetails) == 0 {
		return "No work details defined."
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "%d work detail%s:\n", len(resp.WorkDetails), plural(len(resp.WorkDetails)))
	for _, wd := range resp.WorkDetails {
		var flags []string
		if wd.NoModify {
			flags = append(flags, "no_modify")
		}
		if wd.CannotBeEverybody {
			flags = append(flags, "cannot_be_everybody")
		}
		flagStr := ""
		if len(flags) > 0 {
			flagStr = " [" + strings.Join(flags, ",") + "]"
		}
		fmt.Fprintf(&sb, "- index=%d %q (icon %s, mode %s)%s\n", wd.Index, wd.Name, wd.Icon, wd.Mode, flagStr)
		if len(wd.AllowedLabors) > 0 {
			fmt.Fprintf(&sb, "    labors: %s\n", strings.Join(wd.AllowedLabors, ", "))
		}
		if len(wd.AssignedUnits) > 0 {
			names := make([]string, 0, len(wd.AssignedUnits))
			for _, u := range wd.AssignedUnits {
				name := u.FirstName
				if name == "" {
					name = fmt.Sprintf("dwarf#%d", u.ID)
				}
				names = append(names, fmt.Sprintf("%s (id %d)", name, u.ID))
			}
			fmt.Fprintf(&sb, "    assigned: %s\n", strings.Join(names, ", "))
		}
	}
	return sb.String()
}

// resolveWorkDetailIndex resolves a work-detail selector to its current
// vector index. If detailIndex is non-nil it's used directly; otherwise
// name is matched case-insensitively against a fresh work_details query
// (the first match wins). Re-querying by name on every call rather than
// caching is deliberate: work_details carries no stable ID of its own, so
// an index can shift if a detail is ever created or removed between calls.
func resolveWorkDetailIndex(ctx context.Context, b *Bridge, detailIndex *int, name string) (int, error) {
	if detailIndex != nil {
		return *detailIndex, nil
	}
	if name == "" {
		return 0, fmt.Errorf("must provide detail_index or detail_name")
	}
	raw, err := b.Query(ctx, "work_details", "{}")
	if err != nil {
		return 0, fmt.Errorf("work_details query failed: %w", err)
	}
	var resp struct {
		WorkDetails []workDetailEntry `json:"work_details"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return 0, fmt.Errorf("unparseable work_details response: %w", err)
	}
	for _, wd := range resp.WorkDetails {
		if strings.EqualFold(wd.Name, name) {
			return wd.Index, nil
		}
	}
	return 0, fmt.Errorf("no work detail named %q", name)
}

// moodTimeoutStart is unit->job.mood_timeout's documented starting value
// (df/unit.h's own doc comment: "counts down from 50000, insanity upon
// reaching zero"). Used only to render "X/50000 remaining" the way the
// human overseer's own example phrased it — the raw counter itself is
// never rescaled or reinterpreted. Whether it ticks continuously or only
// while the dwarf can't act on the mood is NOT confirmed by anything in
// this checkout (see queries.cpp handleListMoods) — treat the number as a
// countdown, not a clock.
const moodTimeoutStart = 50000

// moodJobWorkshopHint maps the eleven StrangeMood* job_type keys to the
// specific workshop each one targets — DF's own fixed job_type-to-workshop
// association (mirrors plugins/showmood.cpp's switch statement), not a
// judgment call. Brooding/Fell don't target a workshop at all (macabre/fell
// moods are a breakdown, not a claim-a-shop mood).
var moodJobWorkshopHint = map[string]string{
	"StrangeMoodCrafter":    "Craftsdwarf's Workshop",
	"StrangeMoodJeweller":   "Jeweler's Workshop",
	"StrangeMoodForge":      "Metalsmith's Forge",
	"StrangeMoodMagmaForge": "Magma Forge",
	"StrangeMoodCarpenter":  "Carpenter's Workshop",
	"StrangeMoodMason":      "Stoneworker's Workshop",
	"StrangeMoodBowyer":     "Bowyer's Workshop",
	"StrangeMoodTanner":     "Leather Works",
	"StrangeMoodWeaver":     "Clothier's Shop",
	"StrangeMoodGlassmaker": "Glass Furnace",
	"StrangeMoodMechanics":  "Mechanic's Workshop",
	"StrangeMoodBrooding":   "(macabre mood — no workshop claim)",
	"StrangeMoodFell":       "(fell mood — no workshop claim)",
}

// moodItemEntry is one demanded material from a strange mood's
// job->job_items (queries.cpp handleListMoods) — got-vs-needed counts and
// the wildcard flag bits DFHack itself computes (any silk/plant/yarn, any
// inorganic, inorganic wildcard, wafers), so a caller can tell "any rough
// gem" apart from "specifically gold." QuantityNeeded/QuantityGot already
// have showmood.cpp's BAR=150/CLOTH=10000 unit-divisor quirk applied on the
// plugin side.
type moodItemEntry struct {
	Index               int    `json:"index"`
	ItemType            string `json:"item_type"`
	ItemSubtype         int    `json:"item_subtype"`
	ItemSubtypeName     string `json:"item_subtype_name"`
	MatType             int    `json:"mat_type"`
	MatIndex            int    `json:"mat_index"`
	Material            string `json:"material"`
	IsAnyInorganic      bool   `json:"is_any_inorganic"`
	IsInorganicWildcard bool   `json:"is_inorganic_wildcard"`
	IsWafers            bool   `json:"is_wafers"`
	IsAnySilk           bool   `json:"is_any_silk"`
	IsAnyPlantFiber     bool   `json:"is_any_plant_fiber"`
	IsAnyYarn           bool   `json:"is_any_yarn"`
	IsMurderedCorpse    bool   `json:"is_murdered_corpse"`
	QuantityNeeded      int    `json:"quantity_needed"`
	QuantityGot         int    `json:"quantity_got"`
}

// moodClaimedBuilding is the workshop a strange-mood dwarf has claimed, via
// the job's BUILDING_HOLDER general_ref — nil until claimed.
type moodClaimedBuilding struct {
	ID   int    `json:"id"`
	Type string `json:"type"`
	Name string `json:"name"`
}

// moodEntry is one active strange-mood job from the plugin's moods query
// (queries.cpp handleListMoods, ported from this checkout's own
// plugins/showmood.cpp). Unit/mood/moodstage/mood_skill/mood_timeout are
// nil only in the rare case showmood.cpp itself flags — a strange-mood job
// momentarily missing its UNIT_WORKER general_ref.
type moodEntry struct {
	JobID           int                  `json:"job_id"`
	JobType         string               `json:"job_type"` // "StrangeMoodCrafter" etc. — see moodJobWorkshopHint
	UnitID          *int                 `json:"unit_id"`
	FirstName       *string              `json:"first_name"`
	Mood            *string              `json:"mood"`      // df::mood_type key, e.g. "Fey", "Possessed", "Macabre"
	Moodstage       *string              `json:"moodstage"` // "INITIAL" | "WORKING"
	MoodSkill       *string              `json:"mood_skill"`
	MoodTimeout     *int                 `json:"mood_timeout"` // raw countdown, see moodTimeoutStart
	ClaimedBuilding *moodClaimedBuilding `json:"claimed_building"`
	NeededItems     []moodItemEntry      `json:"needed_items"`
}

// moodItemLabel renders one needed-material line's item description from
// its raw material/wildcard fields — the same per-item-type distinctions
// plugins/showmood.cpp prints (specific material vs. "any silk"/"any
// metal"/"any <inorganic>"), not an invented heuristic.
func moodItemLabel(it moodItemEntry) string {
	label := it.ItemType
	switch {
	case it.Material != "":
		label = it.Material + " " + label
	case it.IsAnySilk:
		label = "any silk " + label
	case it.IsAnyPlantFiber:
		label = "any plant fiber " + label
	case it.IsAnyYarn:
		label = "any yarn " + label
	case it.IsInorganicWildcard:
		label = "any metal " + label
	case it.IsAnyInorganic:
		label = "any " + label
	}
	if it.IsWafers {
		label += " (wafers)"
	}
	if it.IsMurderedCorpse {
		label += " (murdered)"
	}
	if it.ItemSubtypeName != "" {
		label = fmt.Sprintf("%s (%s)", label, it.ItemSubtypeName)
	}
	return label
}

// renderMoods renders the moods response. Reports state only, per the
// human overseer's explicit design note for this tool: no "what should I
// do" field and no suggested action — build/queue_job/order already exist
// as the levers once a caller sees a claimed workshop and a shortfall here.
func renderMoods(raw []byte) string {
	var resp struct {
		Moods []moodEntry `json:"moods"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Sprintf("unparseable moods response: %v\nraw: %s", err, capRawJSON(string(raw)))
	}
	if len(resp.Moods) == 0 {
		return "No strange moods currently active."
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "%d active strange mood%s:\n", len(resp.Moods), plural(len(resp.Moods)))
	for _, m := range resp.Moods {
		who := "(unit unresolved)"
		if m.UnitID != nil {
			name := fmt.Sprintf("dwarf#%d", *m.UnitID)
			if m.FirstName != nil && *m.FirstName != "" {
				name = *m.FirstName
			}
			who = fmt.Sprintf("%s (id=%d)", name, *m.UnitID)
		}
		moodLabel := "?"
		if m.Mood != nil {
			moodLabel = *m.Mood
		}
		stage := ""
		if m.Moodstage != nil {
			stage = fmt.Sprintf(" [%s]", *m.Moodstage)
		}
		skill := ""
		if m.MoodSkill != nil && *m.MoodSkill != "" {
			skill = fmt.Sprintf(" — skill: %s", *m.MoodSkill)
		}
		timeout := "unknown"
		if m.MoodTimeout != nil {
			timeout = fmt.Sprintf("%d/%d", *m.MoodTimeout, moodTimeoutStart)
		}
		want := m.JobType
		if hint, ok := moodJobWorkshopHint[m.JobType]; ok {
			want = hint
		}
		claim := "not yet claimed a workshop"
		if m.ClaimedBuilding != nil {
			claim = fmt.Sprintf("claimed %s (id=%d)", m.ClaimedBuilding.Name, m.ClaimedBuilding.ID)
		}

		fmt.Fprintf(&sb, "- %s: %s%s%s — wants: %s — %s — mood_timeout: %s ticks remaining\n",
			who, moodLabel, stage, skill, want, claim, timeout)

		if len(m.NeededItems) == 0 {
			sb.WriteString("  (no item requirements listed)\n")
			continue
		}
		for _, it := range m.NeededItems {
			satisfied := ""
			if it.QuantityGot >= it.QuantityNeeded {
				satisfied = " (satisfied)"
			}
			fmt.Fprintf(&sb, "  needs: %s — got %d of %d%s\n",
				moodItemLabel(it), it.QuantityGot, it.QuantityNeeded, satisfied)
		}
	}
	return sb.String()
}

// nobleDemandEntry is one active df::unit_demand entry from the plugin's
// noble_demands query — the literal "wants a cabinet in his office, with a
// deadline" data, decoded the same way list_mandates' analogous mandate
// fields are (see queries.cpp handleNobleDemands). This is a completely
// separate DF mechanism from mandates (Export/Make/Guild violations).
type nobleDemandEntry struct {
	Place           string `json:"place"` // "Office" | "Bedroom" | "DiningRoom" | "Tomb"
	ItemType        string `json:"item_type"`
	ItemSubtype     int    `json:"item_subtype"`
	ItemSubtypeName string `json:"item_subtype_name"`
	Material        string `json:"material"`
	TimeoutCounter  int    `json:"timeout_counter"`
	TimeoutLimit    int    `json:"timeout_limit"`
	TicksRemaining  int64  `json:"ticks_remaining"`
}

// noblePositionEntry is one df::entity_position a noble holds, from the
// plugin's noble_demands query. required_office/required_bedroom/
// required_dining/required_tomb are room-VALUE minimums (not tile counts);
// required_boxes/required_cabinets/required_racks/required_stands are
// furniture-item-count minimums. Responsibilities lists only the entries DF
// has actually enabled for this position.
type noblePositionEntry struct {
	Code             string   `json:"code"`
	Name             string   `json:"name"`
	Precedence       int      `json:"precedence"`
	Responsibilities []string `json:"responsibilities"`
	RequiredOffice   int      `json:"required_office"`
	RequiredBedroom  int      `json:"required_bedroom"`
	RequiredDining   int      `json:"required_dining"`
	RequiredTomb     int      `json:"required_tomb"`
	RequiredBoxes    int      `json:"required_boxes"`
	RequiredCabinets int      `json:"required_cabinets"`
	RequiredRacks    int      `json:"required_racks"`
	RequiredStands   int      `json:"required_stands"`
}

// nobleEntry is one unit holding at least one noble position, from the
// plugin's noble_demands query (queries.cpp handleNobleDemands). Positions
// and Demands are reported side by side, unresolved against each other — no
// satisfied/violated judgment is computed anywhere in this path.
type nobleEntry struct {
	UnitID    int                  `json:"unit_id"`
	FirstName string               `json:"first_name"`
	Positions []noblePositionEntry `json:"positions"`
	Demands   []nobleDemandEntry   `json:"demands"`
}

// renderNobleDemands renders the noble_demands response. Zero-value
// required_* fields are omitted from the rendered position line (0 means no
// requirement) to keep the common case compact — the raw JSON always
// carries them; this is presentation-only compression, same rationale as
// renderStocks' conditional economic/in_use notes.
func renderNobleDemands(raw []byte) string {
	var resp struct {
		Nobles []nobleEntry `json:"nobles"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Sprintf("unparseable noble_demands response: %v\nraw: %s", err, capRawJSON(string(raw)))
	}
	if len(resp.Nobles) == 0 {
		return "No noble positions currently held."
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "%d noble%s:\n", len(resp.Nobles), plural(len(resp.Nobles)))
	for _, n := range resp.Nobles {
		name := n.FirstName
		if name == "" {
			name = fmt.Sprintf("dwarf#%d", n.UnitID)
		}
		fmt.Fprintf(&sb, "- %s (id=%d)\n", name, n.UnitID)
		for _, p := range n.Positions {
			var reqs []string
			if p.RequiredOffice > 0 {
				reqs = append(reqs, fmt.Sprintf("office>=%d", p.RequiredOffice))
			}
			if p.RequiredBedroom > 0 {
				reqs = append(reqs, fmt.Sprintf("bedroom>=%d", p.RequiredBedroom))
			}
			if p.RequiredDining > 0 {
				reqs = append(reqs, fmt.Sprintf("dining>=%d", p.RequiredDining))
			}
			if p.RequiredTomb > 0 {
				reqs = append(reqs, fmt.Sprintf("tomb>=%d", p.RequiredTomb))
			}
			if p.RequiredBoxes > 0 {
				reqs = append(reqs, fmt.Sprintf("boxes>=%d", p.RequiredBoxes))
			}
			if p.RequiredCabinets > 0 {
				reqs = append(reqs, fmt.Sprintf("cabinets>=%d", p.RequiredCabinets))
			}
			if p.RequiredRacks > 0 {
				reqs = append(reqs, fmt.Sprintf("racks>=%d", p.RequiredRacks))
			}
			if p.RequiredStands > 0 {
				reqs = append(reqs, fmt.Sprintf("stands>=%d", p.RequiredStands))
			}
			reqStr := "no room/furniture requirements"
			if len(reqs) > 0 {
				reqStr = strings.Join(reqs, ", ")
			}
			fmt.Fprintf(&sb, "  position: %s (precedence %d) — %s\n", p.Name, p.Precedence, reqStr)
			if len(p.Responsibilities) > 0 {
				fmt.Fprintf(&sb, "    responsibilities: %s\n", strings.Join(p.Responsibilities, ", "))
			}
		}
		if len(n.Demands) == 0 {
			sb.WriteString("  no active demands\n")
			continue
		}
		for _, d := range n.Demands {
			item := d.ItemType
			if d.Material != "" {
				item = d.Material + " " + item
			}
			if d.ItemSubtypeName != "" {
				item = fmt.Sprintf("%s (%s)", item, d.ItemSubtypeName)
			}
			fmt.Fprintf(&sb, "  DEMANDS: %s in %s — %d ticks remaining\n", item, d.Place, d.TicksRemaining)
		}
	}
	return sb.String()
}

// fortWealthBreakdown mirrors df::entity_activity_statistics::T_wealth as
// read by the plugin (queries.cpp handleFortWealth) — DF's own int32
// wealth-tracking categories, reported verbatim.
type fortWealthBreakdown struct {
	Total        int `json:"total"`
	Weapons      int `json:"weapons"`
	Armor        int `json:"armor"`
	Furniture    int `json:"furniture"`
	Other        int `json:"other"`
	Architecture int `json:"architecture"`
	Displayed    int `json:"displayed"`
	Held         int `json:"held"`
	Imported     int `json:"imported"`
	Offered      int `json:"offered"`
	Exported     int `json:"exported"`
}

// renderFortWealth renders the fort_wealth query response. fortress_age is
// reported verbatim, not rescaled — df.plotinfo.xml's own comment on the
// field ("?; +1 per 10; used in first 2 migrant waves etc") means even
// DFHack's structure-definition maintainers weren't fully certain of its
// tick semantics, unlike list_mandates' timeout_counter/timeout_limit pair.
func renderFortWealth(raw []byte) string {
	var resp struct {
		Wealth      fortWealthBreakdown `json:"wealth"`
		FortressAge int                 `json:"fortress_age"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Sprintf("unparseable fort_wealth response: %v\nraw: %s", err, capRawJSON(string(raw)))
	}
	w := resp.Wealth
	var sb strings.Builder
	fmt.Fprintf(&sb, "Total wealth: %d (weapons %d, armor %d, furniture %d, architecture %d, other %d)\n",
		w.Total, w.Weapons, w.Armor, w.Furniture, w.Architecture, w.Other)
	fmt.Fprintf(&sb, "displayed %d, held %d, imported %d, offered %d, exported %d\n",
		w.Displayed, w.Held, w.Imported, w.Offered, w.Exported)
	fmt.Fprintf(&sb, "fortress_age: %d (DF's own embark-age counter; exact tick-per-unit factor unconfirmed, relevant to early migrant-wave timing)\n", resp.FortressAge)
	return sb.String()
}

// caravanEntry is one active caravan from the plugin's caravan_status query
// (queries.cpp handleCaravanStatus, df::global::plotinfo->caravans). Distinct
// from a pendingEventEntry: a caravanEntry has already arrived on the map (or
// is en route with a depot assigned), while pending events are still just a
// countdown in df::global::timed_events with nothing spawned yet.
type caravanEntry struct {
	Civ                  string  `json:"civ"`
	EntityID             int     `json:"entity_id"`
	TradeState           string  `json:"trade_state"`
	TimeRemainingTicks   int64   `json:"time_remaining_ticks"`
	DaysRemaining        float64 `json:"days_remaining"`
	Tribute              bool    `json:"tribute"`
	Casualty             bool    `json:"casualty"`
	Hardship             bool    `json:"hardship"`
	Seized               bool    `json:"seized"`
	Offended             bool    `json:"offended"`
	GreatlyOffended      bool    `json:"greatly_offended"`
	ImportValue          int     `json:"import_value"`
	ExportValueTotal     int     `json:"export_value_total"`
	ExportValuePersonal  int     `json:"export_value_personal"`
	OfferValue           int     `json:"offer_value"`
	Mood                 int     `json:"mood"`
	HaggleFailCount      int     `json:"haggle_fail_count"`
	LiaisonMeetingActive bool    `json:"liaison_meeting_active"`
}

// pendingEventEntry is one scheduled-but-not-yet-arrived caravan/diplomat
// event from df::global::timed_events — the only place a countdown exists
// before a caravan shows up in plotinfo->caravans at all (queries.cpp
// handleCaravanStatus).
type pendingEventEntry struct {
	Type                 string `json:"type"`
	Civ                  string `json:"civ"`
	Season               string `json:"season"`
	SeasonTicksRemaining int    `json:"season_ticks_remaining"`
}

// caravanDepotStatus is caravan_status's depot-readiness summary — whether
// any TRADE_DEPOT building exists at all, and if so its build/access state.
// Distinct from depotGoodsDepotInfo (depot_goods' depot section), which
// assumes a depot already exists and never reports Exists=false.
type caravanDepotStatus struct {
	Exists          bool `json:"exists"`
	X               int  `json:"x"`
	Y               int  `json:"y"`
	Z               int  `json:"z"`
	Built           bool `json:"built"`
	Accessible      bool `json:"accessible"`
	TraderRequested bool `json:"trader_requested"`
	AnyoneCanTrade  bool `json:"anyone_can_trade"`
}

// caravanFlagSummary joins a caravan's active plot_merchant_flag bits into a
// bracketed suffix, e.g. " [seized,offended]" — omitted entirely when none
// are set, matching renderNobleDemands' conditional-requirement compression.
func caravanFlagSummary(c caravanEntry) string {
	var flags []string
	if c.Tribute {
		flags = append(flags, "tribute")
	}
	if c.Casualty {
		flags = append(flags, "casualty")
	}
	if c.Hardship {
		flags = append(flags, "hardship")
	}
	if c.Seized {
		flags = append(flags, "seized")
	}
	if c.Offended {
		flags = append(flags, "offended")
	}
	if c.GreatlyOffended {
		flags = append(flags, "greatly_offended")
	}
	if len(flags) == 0 {
		return ""
	}
	return " [" + strings.Join(flags, ",") + "]"
}

// renderCaravanStatus renders the caravan_status response: active caravans
// (trade state, departure countdown, session value/mood, liaison-meeting
// flag), scheduled-but-not-arrived caravan/diplomat events, and depot
// readiness. Executing the actual trade exchange is deliberately out of
// scope anywhere in this tool surface — see bring_goods_to_depot's
// description for the honest boundary.
func renderCaravanStatus(raw []byte) string {
	var resp struct {
		Caravans      []caravanEntry      `json:"caravans"`
		PendingEvents []pendingEventEntry `json:"pending_events"`
		Depot         caravanDepotStatus  `json:"depot"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Sprintf("unparseable caravan_status response: %v\nraw: %s", err, capRawJSON(string(raw)))
	}

	var sb strings.Builder
	if len(resp.Caravans) == 0 {
		sb.WriteString("No caravan currently on the map.\n")
	} else {
		fmt.Fprintf(&sb, "%d caravan%s on the map:\n", len(resp.Caravans), plural(len(resp.Caravans)))
		for _, c := range resp.Caravans {
			civ := c.Civ
			if civ == "" {
				civ = fmt.Sprintf("entity#%d", c.EntityID)
			}
			fmt.Fprintf(&sb, "- %s: %s, %.1f days remaining (%d ticks)%s\n",
				civ, c.TradeState, c.DaysRemaining, c.TimeRemainingTicks, caravanFlagSummary(c))
			fmt.Fprintf(&sb, "  value: import=%d export_total=%d export_personal=%d offer=%d mood=%d haggle_fails=%d\n",
				c.ImportValue, c.ExportValueTotal, c.ExportValuePersonal, c.OfferValue, c.Mood, c.HaggleFailCount)
			if c.LiaisonMeetingActive {
				sb.WriteString("  liaison meeting active\n")
			}
		}
	}

	if len(resp.PendingEvents) > 0 {
		fmt.Fprintf(&sb, "%d scheduled event%s not yet arrived:\n", len(resp.PendingEvents), plural(len(resp.PendingEvents)))
		for _, e := range resp.PendingEvents {
			civ := e.Civ
			if civ == "" {
				civ = "unknown civ"
			}
			fmt.Fprintf(&sb, "- %s: %s, %s season, %d ticks remaining in season\n", e.Type, civ, e.Season, e.SeasonTicksRemaining)
		}
	}

	if resp.Depot.Exists {
		fmt.Fprintf(&sb, "Depot at (%d,%d,%d): built=%v accessible=%v trader_requested=%v anyone_can_trade=%v\n",
			resp.Depot.X, resp.Depot.Y, resp.Depot.Z, resp.Depot.Built, resp.Depot.Accessible,
			resp.Depot.TraderRequested, resp.Depot.AnyoneCanTrade)
	} else {
		sb.WriteString("No trade depot built yet.\n")
	}

	return strings.TrimRight(sb.String(), "\n")
}

// depotGoodsAggEntry is one (item_type, material) aggregate bucket from the
// plugin's depot_goods query (queries.cpp handleDepotGoods) — aggregated
// like stocks/stockpile_inventory, not by individual item ID (no tool today
// surfaces item identity, and this is a read, not an ID-based follow-up).
type depotGoodsAggEntry struct {
	ItemType  string `json:"item_type"`
	Material  string `json:"material"`
	Count     int    `json:"count"`
	Value     int64  `json:"value"`
	Requested bool   `json:"requested"`
}

// depotGoodsDepotInfo is depot_goods' depot identity/build-state summary.
// Unlike caravanDepotStatus, this always describes a depot that was found
// (the plugin errors out first if none exists / none matches the given
// coordinates), so there is no Exists field.
type depotGoodsDepotInfo struct {
	X     int  `json:"x"`
	Y     int  `json:"y"`
	Z     int  `json:"z"`
	Built bool `json:"built"`
}

// renderDepotGoodsAgg renders one labeled aggregate section (staged or
// pending) of a depot_goods response.
func renderDepotGoodsAgg(label string, entries []depotGoodsAggEntry) string {
	var sb strings.Builder
	if len(entries) == 0 {
		fmt.Fprintf(&sb, "  %s: none\n", label)
		return sb.String()
	}
	fmt.Fprintf(&sb, "  %s:\n", label)
	for _, e := range entries {
		requested := ""
		if e.Requested {
			requested = " (requested by liaison)"
		}
		fmt.Fprintf(&sb, "    %s %s x%d (value %d)%s\n", e.Material, e.ItemType, e.Count, e.Value, requested)
	}
	return sb.String()
}

// renderDepotGoods renders the depot_goods response: what's physically
// staged at the depot (use_mode TEMP contained_items) versus what's already
// marked and being hauled there but hasn't arrived (pending BringItemToDepot
// jobs) — the two states a model needs to distinguish before deciding
// whether to mark more goods with bring_goods_to_depot.
func renderDepotGoods(raw []byte) string {
	var resp struct {
		Depot   depotGoodsDepotInfo  `json:"depot"`
		Staged  []depotGoodsAggEntry `json:"staged"`
		Pending []depotGoodsAggEntry `json:"pending"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Sprintf("unparseable depot_goods response: %v\nraw: %s", err, capRawJSON(string(raw)))
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "Depot at (%d,%d,%d) (built=%v):\n", resp.Depot.X, resp.Depot.Y, resp.Depot.Z, resp.Depot.Built)
	sb.WriteString(renderDepotGoodsAgg("staged", resp.Staged))
	sb.WriteString(renderDepotGoodsAgg("pending (hauling, not yet arrived)", resp.Pending))
	return strings.TrimRight(sb.String(), "\n")
}

// wellbeingCitizen is one entry from the plugin's wellbeing query
// (queries.cpp handleWellbeing) — one pass over every living citizen
// (Units::isCitizen), unlike dwarf_detail's per-dwarf query. All four
// soul-derived fields are nil together when the unit has no current_soul
// (a defensive case, not expected for a normal citizen).
type wellbeingCitizen struct {
	ID                int      `json:"id"`
	FirstName         string   `json:"first_name"`
	Stress            *int     `json:"stress"`
	StressCategory    *int     `json:"stress_category"`
	CurrentFocus      *int     `json:"current_focus"`
	UndistractedFocus *int     `json:"undistracted_focus"`
	FocusRatio        *float64 `json:"focus_ratio"`
}

func intOrZero(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

// renderWellbeing renders the wellbeing response: raw stress/focus numbers
// per citizen, no "needs attention" verdict — stress_category is DF's own
// 0-6 banding (Units::getStressCategory), not a judgment this code computes.
// For per-need detail on any one citizen, see dwarf_detail's psyche section.
func renderWellbeing(raw []byte) string {
	var resp struct {
		Citizens []wellbeingCitizen `json:"citizens"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Sprintf("unparseable wellbeing response: %v\nraw: %s", err, capRawJSON(string(raw)))
	}
	if len(resp.Citizens) == 0 {
		return "No living citizens found."
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "%d citizen%s:\n", len(resp.Citizens), plural(len(resp.Citizens)))
	for _, c := range resp.Citizens {
		name := c.FirstName
		if name == "" {
			name = fmt.Sprintf("dwarf#%d", c.ID)
		}
		if c.Stress == nil {
			fmt.Fprintf(&sb, "- %s (id=%d): no soul data\n", name, c.ID)
			continue
		}
		ratio := "n/a"
		if c.FocusRatio != nil {
			ratio = fmt.Sprintf("%.2f", *c.FocusRatio)
		}
		fmt.Fprintf(&sb, "- %s (id=%d): stress=%d (category %d/6), focus=%d/%d (ratio %s)\n",
			name, c.ID, *c.Stress, *c.StressCategory, intOrZero(c.CurrentFocus), intOrZero(c.UndistractedFocus), ratio)
	}
	return sb.String()
}

// jobTypeEntry is one entry from the plugin's list_orders query — a
// DFHack df::job_type enum key name plus a heuristic category tag.
type jobTypeEntry struct {
	Name     string `json:"name"`
	Category string `json:"category"`
}

// maxJobTypesList caps the job_types tool's output. Unfiltered, list_orders
// returns DF's full job_type enum (~240 entries) — this is exactly the
// static noise the orders/order/queue_job tools deliberately stopped
// paying on every call (see renderManagerOrders above); job_types exists
// so the model pays for that dump only when it actually needs to look up
// an unfamiliar name, and the cap keeps a forgotten filter from blowing
// the response budget anyway.
const maxJobTypesList = 80

// renderJobTypes renders the list_orders response for the job_types
// discovery tool. Every name shown here round-trips into queue_job's
// name-based path (protocol.OrderTypeByName) verbatim — this is the
// catalog half of generalized item construction, work_orders.cpp
// resolveJobTypeByName is the lookup half.
func renderJobTypes(raw []byte, filtered bool) string {
	var resp struct {
		Orders []jobTypeEntry `json:"orders"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Sprintf("unparseable list_orders response: %v\nraw: %s", err, capRawJSON(string(raw)))
	}
	if len(resp.Orders) == 0 {
		return "No job types matched that filter."
	}
	entries := resp.Orders
	truncated := false
	if len(entries) > maxJobTypesList {
		entries = entries[:maxJobTypesList]
		truncated = true
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "%d job types", len(resp.Orders))
	if truncated {
		fmt.Fprintf(&sb, " (showing first %d — pass filter to narrow)", maxJobTypesList)
	}
	sb.WriteString(":\n")
	for _, o := range entries {
		fmt.Fprintf(&sb, "- %s [%s]\n", o.Name, o.Category)
	}
	if !filtered && !truncated {
		sb.WriteString("(pass filter next time to narrow this list)\n")
	}
	return sb.String()
}

// maxBuildingTypesList caps the building_types tool's output — generous
// relative to job_types' cap since this reads a small, hand-curated Go
// table (buildingTypeCatalog, tools_action.go), not a ~240-entry live DF
// enum dump; it grows one entry at a time as the plugin learns to place new
// building types.
const maxBuildingTypesList = 100

// renderBuildingTypes renders the building_types discovery tool's output.
// Unlike renderJobTypes/renderReactions above (which unmarshal a plugin
// query response), this reads buildingTypeCatalog directly — footprint and
// prerequisite facts aren't exposed by any DFHack enum scan, so there is no
// live query to round-trip for this tool. Every Name shown here
// round-trips into build's type param verbatim.
func renderBuildingTypes(filter string) string {
	lf := strings.ToLower(filter)
	var matched []buildingTypeEntry
	for _, e := range buildingTypeCatalog {
		if lf == "" || strings.Contains(strings.ToLower(e.Name), lf) || strings.Contains(strings.ToLower(e.Category), lf) {
			matched = append(matched, e)
		}
	}
	if len(matched) == 0 {
		return "No building types matched that filter."
	}
	total := len(matched)
	truncated := false
	if len(matched) > maxBuildingTypesList {
		matched = matched[:maxBuildingTypesList]
		truncated = true
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "%d building types", total)
	if truncated {
		fmt.Fprintf(&sb, " (showing first %d — pass filter to narrow)", maxBuildingTypesList)
	}
	sb.WriteString(":\n")
	for _, e := range matched {
		fmt.Fprintf(&sb, "- %s [%s] footprint=%s — %s\n", e.Name, e.Category, e.Footprint, e.Requires)
	}
	if filter == "" && !truncated {
		sb.WriteString("(pass filter next time to narrow this list)\n")
	}
	return sb.String()
}

// reactionReagentEntry is one reagent summary from the plugin's
// list_reactions query. ItemType is omitted when the reagent isn't an
// item-typed reagent (queue_job's reaction path currently rejects those —
// see work_orders.cpp applyQueueReactionJob).
type reactionReagentEntry struct {
	Code     string `json:"code"`
	Quantity int    `json:"quantity"`
	ItemType string `json:"item_type,omitempty"`
}

// reactionEntry is one entry from the plugin's list_reactions query — a
// df::reaction's code, display name, the workshop(s) it can run at, and a
// summary of its reagents.
type reactionEntry struct {
	Code      string                 `json:"code"`
	Name      string                 `json:"name"`
	Buildings []string               `json:"buildings"`
	Reagents  []reactionReagentEntry `json:"reagents"`
}

// maxReactionsList caps the list_reactions tool's output, same rationale
// as maxJobTypesList below — this is the pay-on-demand discovery tool for
// queue_job's reaction path, not something called on every turn.
const maxReactionsList = 60

// renderReactions renders the list_reactions response for the discovery
// tool. Every code shown here round-trips into queue_job's reaction param
// (protocol.OrderTypeCustomReaction) verbatim — this is the catalog half
// of reaction-based job queueing, work_orders.cpp
// applyQueueReactionJob is the lookup/build half.
func renderReactions(raw []byte, filtered bool) string {
	var resp struct {
		Reactions []reactionEntry `json:"reactions"`
		Truncated bool            `json:"truncated"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Sprintf("unparseable list_reactions response: %v\nraw: %s", err, capRawJSON(string(raw)))
	}
	if len(resp.Reactions) == 0 {
		return "No reactions matched that filter."
	}
	entries := resp.Reactions
	truncated := resp.Truncated
	if len(entries) > maxReactionsList {
		entries = entries[:maxReactionsList]
		truncated = true
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "%d reactions", len(resp.Reactions))
	if truncated {
		fmt.Fprintf(&sb, " (showing first %d — pass filter to narrow)", len(entries))
	}
	sb.WriteString(":\n")
	for _, r := range entries {
		fmt.Fprintf(&sb, "- %s (%s) @ %s", r.Code, r.Name, strings.Join(r.Buildings, "/"))
		if len(r.Reagents) > 0 {
			var parts []string
			for _, rg := range r.Reagents {
				part := rg.Code
				if rg.ItemType != "" {
					part += "=" + rg.ItemType
				}
				if rg.Quantity != 1 {
					part += fmt.Sprintf("x%d", rg.Quantity)
				}
				parts = append(parts, part)
			}
			fmt.Fprintf(&sb, " [needs: %s]", strings.Join(parts, ", "))
		}
		sb.WriteString("\n")
	}
	if !filtered && !truncated {
		sb.WriteString("(pass filter next time to narrow this list)\n")
	}
	return sb.String()
}

// cropEntry is one entry from the plugin's list_crops query — a plant
// raw's token/display name (either round-trips into assign_crop's crop
// param, resolved case-insensitively), whether it's a subterranean crop,
// and how many seeds are on hand right now.
type cropEntry struct {
	Token       string `json:"token"`
	Name        string `json:"name"`
	Underground bool   `json:"underground"`
	SeedsOnHand int    `json:"seeds_on_hand"`
}

// maxCropsList caps the list_crops tool's output, same rationale as
// maxReactionsList above.
const maxCropsList = 60

// renderCrops renders the list_crops response for the discovery tool.
func renderCrops(raw []byte, filtered bool) string {
	var resp struct {
		Crops     []cropEntry `json:"crops"`
		Truncated bool        `json:"truncated"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Sprintf("unparseable list_crops response: %v\nraw: %s", err, capRawJSON(string(raw)))
	}
	if len(resp.Crops) == 0 {
		return "No crops matched that filter."
	}
	entries := resp.Crops
	truncated := resp.Truncated
	if len(entries) > maxCropsList {
		entries = entries[:maxCropsList]
		truncated = true
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "%d crops", len(resp.Crops))
	if truncated {
		fmt.Fprintf(&sb, " (showing first %d — pass filter to narrow)", len(entries))
	}
	sb.WriteString(":\n")
	for _, c := range entries {
		where := "surface"
		if c.Underground {
			where = "underground"
		}
		fmt.Fprintf(&sb, "- %s (%s) [%s] seeds on hand: %d\n", c.Token, c.Name, where, c.SeedsOnHand)
	}
	if !filtered && !truncated {
		sb.WriteString("(pass filter next time to narrow this list)\n")
	}
	return sb.String()
}

// renderDwarfList renders the id/position roster, capped at maxDwarfList.
// A dead dwarf's df::unit typically lingers in the plugin's source list
// reporting a frozen last-known position (see EntityInfo.Dead's doc
// comment) -- the [DEAD] marker is the only truthful signal distinguishing
// that from a living, currently-idle dwarf standing still.
func renderDwarfList(dwarves []protocol.EntityInfo) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%d dwarves:\n", len(dwarves))
	for i, d := range dwarves {
		if i >= maxDwarfList {
			fmt.Fprintf(&sb, "... and %d more (use dwarf_detail by id)\n", len(dwarves)-maxDwarfList)
			break
		}
		if d.Dead {
			fmt.Fprintf(&sb, "- id=%d last-known @(%d,%d,%d) [DEAD]\n", d.ID, d.X, d.Y, d.Z)
			continue
		}
		fmt.Fprintf(&sb, "- id=%d @(%d,%d,%d)\n", d.ID, d.X, d.Y, d.Z)
	}
	return sb.String()
}

// dwarfSkillEntry is one skill summary from the plugin's dwarf_detail
// query's top_skills array.
type dwarfSkillEntry struct {
	Skill      string `json:"skill"`
	Level      int    `json:"level"`
	Experience int    `json:"experience"`
}

// dwarfNeedEntry is one entry from dwarf_detail's psyche.needs (queries.cpp
// handleDwarfDetail, personality_needst) -- need_type name, focus_level
// (climbs to 400 when satisfied), and need_level (how fast focus_level
// decays once negative).
type dwarfNeedEntry struct {
	NeedType   string `json:"need_type"`
	FocusLevel int    `json:"focus_level"`
	NeedLevel  int    `json:"need_level"`
}

// dwarfEmotionEntry is one entry from dwarf_detail's psyche.emotions
// (queries.cpp handleDwarfDetail, personality_moodst) -- the server already
// sorted recent-first and capped the array; ThoughtCaption is DF's own
// unit_thought_type caption text (empty when that thought has none, e.g.
// "None"). Divider is DF's own emotion_type 'divider' attribute, raw and
// signed (negative marks a eustress/positive emotion, positive a distress/
// negative one, per queries.cpp's comment) -- not relabeled into a
// positive/negative verdict string. Subthought is the union-typed
// circumstance_id field, reported raw and undecoded (its meaning depends on
// which Thought it belongs to).
type dwarfEmotionEntry struct {
	Emotion        string `json:"emotion"`
	Strength       int    `json:"strength"`
	Thought        string `json:"thought"`
	ThoughtCaption string `json:"thought_caption"`
	Subthought     int    `json:"subthought"`
	Divider        int    `json:"divider"`
}

// dwarfFacetEntry is one entry from dwarf_detail's psyche.top_facets --
// only the extremes by |deviation from 50| are ever emitted, not the full
// 50-facet array.
type dwarfFacetEntry struct {
	Facet string `json:"facet"`
	Value int    `json:"value"`
}

// dwarfValueEntry is one entry from dwarf_detail's psyche.values
// (personality_valuest) -- the full list, not capped.
type dwarfValueEntry struct {
	ValueType string `json:"value_type"`
	Strength  int    `json:"strength"`
}

// dwarfDreamEntry is one entry from dwarf_detail's psyche.dreams
// (personality_goalst) -- the full list, not capped.
type dwarfDreamEntry struct {
	GoalType     string `json:"goal_type"`
	Accomplished bool   `json:"accomplished"`
}

// dwarfPsyche is the plugin's dwarf_detail "psyche" section (queries.cpp
// handleDwarfDetail) -- everything read off
// unit->status.current_soul->personality for the one dwarf. Nil when the
// unit has no current_soul (the plugin's own null guard — see its comment).
// EmotionsTotal is the un-capped count; Emotions itself is capped at
// EmotionsCap entries so a long-lived dwarf's full emotional history can't
// balloon a single response.
type dwarfPsyche struct {
	Stress         int                 `json:"stress"`
	StressCategory int                 `json:"stress_category"`
	Needs          []dwarfNeedEntry    `json:"needs"`
	EmotionsTotal  int                 `json:"emotions_total"`
	EmotionsCap    int                 `json:"emotions_cap"`
	Emotions       []dwarfEmotionEntry `json:"emotions"`
	TopFacets      []dwarfFacetEntry   `json:"top_facets"`
	Values         []dwarfValueEntry   `json:"values"`
	Dreams         []dwarfDreamEntry   `json:"dreams"`
}

// dwarfDetailResp is the full wire shape of the plugin's dwarf_detail
// response (queries.cpp handleDwarfDetail). Shared between renderDwarfDetail
// (the single-dwarf tool) and the dwarves tool's verbose fan-out so both
// parse the exact same shape once.
type dwarfDetailResp struct {
	ID       int `json:"id"`
	Position struct {
		X int `json:"x"`
		Y int `json:"y"`
		Z int `json:"z"`
	} `json:"position"`
	FirstName string `json:"first_name"`
	Dead      bool   `json:"dead"`
	DeathYear *int   `json:"death_year"` // nil when dead unit has no recorded death incident
	DeathTime *int   `json:"death_time"`
	Buried    bool   `json:"buried"`
	BurialPos *struct {
		X int `json:"x"`
		Y int `json:"y"`
		Z int `json:"z"`
	} `json:"burial_position"` // only present when Buried -- the coffin's location, not the stale unit position
	TopSkills  []dwarfSkillEntry `json:"top_skills"`
	CurrentJob *string           `json:"current_job"`
	Mood       string            `json:"mood"` // df::mood_type key ("Fey", "Possessed", ...) or "None"
	Labors     []string          `json:"labors"`
	Psyche     *dwarfPsyche      `json:"psyche"`
}

// parseDwarfDetail unmarshals one dwarf_detail query response.
func parseDwarfDetail(raw []byte) (dwarfDetailResp, error) {
	var d dwarfDetailResp
	err := json.Unmarshal(raw, &d)
	return d, err
}

func dwarfDisplayName(d dwarfDetailResp) string {
	if d.FirstName == "" {
		return fmt.Sprintf("dwarf#%d", d.ID)
	}
	return d.FirstName
}

func dwarfCurrentJobLabel(d dwarfDetailResp) string {
	if d.CurrentJob == nil || *d.CurrentJob == "" {
		return "idle"
	}
	return *d.CurrentJob
}

func dwarfTopSkillLabel(d dwarfDetailResp) string {
	if len(d.TopSkills) == 0 {
		return "no skills"
	}
	top := d.TopSkills[0]
	return fmt.Sprintf("%s Lvl%d", top.Skill, top.Level)
}

// renderPsyche renders dwarf_detail's psyche section. When full is false
// (the default), only the always-cheap summary line is shown — full is
// gated the same way includeLabors gates the labors list, since a citizen's
// needs/emotions/facets/values/dreams together can run well past what a
// per-turn call should cost.
func renderPsyche(sb *strings.Builder, p *dwarfPsyche, full bool) {
	if p == nil {
		sb.WriteString("psyche: no soul data (unit has no current_soul)\n")
		return
	}
	fmt.Fprintf(sb, "psyche: stress=%d (category %d/6) | needs=%d | emotions=%d of %d total | facets=%d | values=%d | dreams=%d\n",
		p.Stress, p.StressCategory, len(p.Needs), len(p.Emotions), p.EmotionsTotal, len(p.TopFacets), len(p.Values), len(p.Dreams))
	if !full {
		sb.WriteString("(pass include_psyche=true for the full needs/emotions/facets/values/dreams breakdown)\n")
		return
	}
	if len(p.Needs) == 0 {
		sb.WriteString("needs: none\n")
	} else {
		sb.WriteString("needs:\n")
		for _, n := range p.Needs {
			fmt.Fprintf(sb, "- %s: focus_level=%d need_level=%d\n", n.NeedType, n.FocusLevel, n.NeedLevel)
		}
	}
	if len(p.Emotions) == 0 {
		sb.WriteString("emotions: none\n")
	} else {
		fmt.Fprintf(sb, "emotions (%d of %d total, recent-first, capped at %d):\n", len(p.Emotions), p.EmotionsTotal, p.EmotionsCap)
		for _, e := range p.Emotions {
			caption := e.ThoughtCaption
			if caption == "" {
				caption = e.Thought
			}
			fmt.Fprintf(sb, "- %s (strength %d) — %s [thought=%s subthought=%d divider=%d]\n",
				e.Emotion, e.Strength, caption, e.Thought, e.Subthought, e.Divider)
		}
	}
	if len(p.TopFacets) == 0 {
		sb.WriteString("top facets: none\n")
	} else {
		sb.WriteString("top facets (|deviation| from neutral 50):\n")
		for _, f := range p.TopFacets {
			fmt.Fprintf(sb, "- %s: %d\n", f.Facet, f.Value)
		}
	}
	if len(p.Values) == 0 {
		sb.WriteString("values: none\n")
	} else {
		sb.WriteString("values:\n")
		for _, v := range p.Values {
			fmt.Fprintf(sb, "- %s: %d\n", v.ValueType, v.Strength)
		}
	}
	if len(p.Dreams) == 0 {
		sb.WriteString("dreams: none\n")
	} else {
		sb.WriteString("dreams:\n")
		for _, dr := range p.Dreams {
			accomplished := "not yet accomplished"
			if dr.Accomplished {
				accomplished = "accomplished"
			}
			fmt.Fprintf(sb, "- %s: %s\n", dr.GoalType, accomplished)
		}
	}
}

// renderDwarfDetail renders the dwarf_detail response as compact text
// lines instead of passing the raw JSON through verbatim (the plugin's
// enabled-only labors list still runs 15-25 lines on its own). Labors and
// the full psyche breakdown are spelled out only when their include_*
// param is set — otherwise just a summary, with a pointer at how to get
// the rest.
func renderDwarfDetail(raw []byte, includeLabors bool, includePsyche bool) string {
	d, err := parseDwarfDetail(raw)
	if err != nil {
		return fmt.Sprintf("unparseable dwarf_detail response: %v\nraw: %s", err, capRawJSON(string(raw)))
	}
	var sb strings.Builder
	if d.Dead {
		// unit->pos is never re-synced once dead (DFHack's Units::getPosition
		// has no dead-unit special case) -- label it "last-known" rather
		// than implying it's where the corpse (or the burying dwarf) is now.
		fmt.Fprintf(&sb, "%s (id=%d) last-known @(%d,%d,%d)\n", dwarfDisplayName(d), d.ID, d.Position.X, d.Position.Y, d.Position.Z)
		// Surfaced first and loudly: a dead unit's record otherwise reads
		// exactly like a living, currently-idle dwarf frozen at its
		// last position -- this is the only truthful signal it's a corpse.
		sb.WriteString("status: DEAD")
		if d.DeathYear != nil && d.DeathTime != nil {
			fmt.Fprintf(&sb, " (died year %d, tick %d)", *d.DeathYear, *d.DeathTime)
		}
		sb.WriteString("\n")
		if d.Buried && d.BurialPos != nil {
			fmt.Fprintf(&sb, "buried: yes, coffin @(%d,%d,%d)\n", d.BurialPos.X, d.BurialPos.Y, d.BurialPos.Z)
		} else {
			sb.WriteString("buried: no\n")
		}
	} else {
		fmt.Fprintf(&sb, "%s (id=%d) @(%d,%d,%d)\n", dwarfDisplayName(d), d.ID, d.Position.X, d.Position.Y, d.Position.Z)
	}
	fmt.Fprintf(&sb, "current job: %s\n", dwarfCurrentJobLabel(d))
	if d.Mood == "None" || d.Mood == "" {
		sb.WriteString("mood: none\n")
	} else {
		fmt.Fprintf(&sb, "mood: %s (active strange mood)\n", d.Mood)
	}
	if len(d.TopSkills) == 0 {
		sb.WriteString("top skills: none\n")
	} else {
		sb.WriteString("top skills:\n")
		for _, s := range d.TopSkills {
			fmt.Fprintf(&sb, "- %s Lvl%d (xp %d)\n", s.Skill, s.Level, s.Experience)
		}
	}
	if includeLabors {
		if len(d.Labors) == 0 {
			sb.WriteString("labors: none enabled\n")
		} else {
			fmt.Fprintf(&sb, "labors (%d enabled): %s\n", len(d.Labors), strings.Join(d.Labors, ", "))
		}
	} else {
		fmt.Fprintf(&sb, "labors: %d enabled (pass include_labors=true to list)\n", len(d.Labors))
	}
	renderPsyche(&sb, d.Psyche, includePsyche)
	return sb.String()
}

// dwarfSummaryLine renders one dwarf_detail response as a single census
// line: name | current job | top skill Lvl | labor count. A dead dwarf is
// tagged [DEAD] up front rather than silently rendered as an idle citizen.
func dwarfSummaryLine(d dwarfDetailResp) string {
	deadTag := ""
	if d.Dead {
		deadTag = " [DEAD]"
	}
	return fmt.Sprintf("- %s (id=%d)%s: %s | top: %s | labors=%d",
		dwarfDisplayName(d), d.ID, deadTag, dwarfCurrentJobLabel(d), dwarfTopSkillLabel(d), len(d.Labors))
}

// renderDwarvesVerbose fans out one dwarf_detail query per dwarf (capped
// at maxDwarfList) and renders one summary line each. This is the
// expensive path — see the dwarves tool's verbose param description.
func renderDwarvesVerbose(ctx context.Context, b *Bridge, dwarves []protocol.EntityInfo) string {
	if len(dwarves) == 0 {
		return "0 dwarves."
	}
	n := len(dwarves)
	capped := n
	if capped > maxDwarfList {
		capped = maxDwarfList
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "%d dwarves (verbose — one dwarf_detail query per dwarf):\n", capped)
	for i := 0; i < capped; i++ {
		d := dwarves[i]
		raw, err := b.Query(ctx, "dwarf_detail", fmt.Sprintf(`{"id":%d}`, d.ID))
		if err != nil {
			fmt.Fprintf(&sb, "- id=%d: query failed: %v\n", d.ID, err)
			continue
		}
		detail, perr := parseDwarfDetail(raw)
		if perr != nil {
			fmt.Fprintf(&sb, "- id=%d: unparseable detail: %v\n", d.ID, perr)
			continue
		}
		sb.WriteString(dwarfSummaryLine(detail))
		sb.WriteString("\n")
	}
	if n > capped {
		fmt.Fprintf(&sb, "... and %d more not queried (verbose caps at %d — use dwarf_detail by id for the rest)\n", n-capped, maxDwarfList)
	}
	return sb.String()
}

// renderAlerts renders the alerts tool's text: one line per active alert,
// oldest-omitted-first per the store's ordering, with the position
// annotated as stale-since-first-occurrence once RepeatCount>0. DF's report
// struct dedups purely on text and bumps repeat_count in place on the
// SAME report object rather than allocating a new one at the new
// coordinates (announcements.cpp's file comment) — so a repeated alert's
// X/Y/Z is where it FIRST happened, not where the latest recurrence did.
// DF's own on-screen convention displays repeat_count=N as "xN+1"
// (df.announcement.xml: "100 => displays: x101") — matched here so the
// number means the same thing a player would see in DF's own log.
func renderAlerts(alerts []worldmodel.Alert, activeCount int) string {
	if len(alerts) == 0 {
		return "No active alerts."
	}
	var sb strings.Builder
	for _, a := range alerts {
		fmt.Fprintf(&sb, "- [%d] sev=%d %s", a.ID, a.Severity, a.Text)
		if a.HasPosition() {
			fmt.Fprintf(&sb, " @(%d,%d,%d)", a.X, a.Y, a.Z)
			if a.RepeatCount > 0 {
				fmt.Fprintf(&sb, " [first occurrence; repeated x%d]", a.RepeatCount+1)
			}
		}
		sb.WriteString("\n")
	}
	if activeCount > len(alerts) {
		fmt.Fprintf(&sb, "... and %d more (dismiss some to see them)\n", activeCount-len(alerts))
	}
	return sb.String()
}

func registerStateTools(srv *mcp.Server, b *Bridge) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "status",
		Description: "Connection and simulation status: is the DFHack plugin connected, is DF paused, current tick. Call this first if anything seems wrong.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
		if b == nil || !b.Connected() {
			return TextResult("NOT CONNECTED: start DF, then run `ai-connect` in the DFHack console. The MCP server listens on the port in config/orchestrator.yaml."), nil, nil
		}
		return withDash(b, ctx, "Connected. The ground-truth header above is live game state."), nil, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "alerts",
		Description: "DF's own announcement stream (job cancellations with reasons, sieges, moods, migrants). READ THIS when work isn't progressing — DF usually says exactly why. Once you've read and acted on one, call dismiss_alerts so it stops repeating in every future call — nothing here auto-clears.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
		snap := b.Snapshot()
		return withDash(b, ctx, renderAlerts(snap.ActiveAlerts, snap.ActiveAlertCount)), nil, nil
	})

	type dismissAlertsIn struct {
		IDs []int `json:"ids,omitempty" jsonschema:"specific alert ids to dismiss (the [N] prefix from the alerts tool)"`
		All bool  `json:"all,omitempty" jsonschema:"dismiss every currently active alert instead of listing ids"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "dismiss_alerts",
		Description: "Mark alerts as seen/handled so they stop repeating in alerts and the dashboard's alert count. Dismissed alerts are NOT deleted from DF's own history — this only shrinks the model's 'still need to look at this' set. Call after reading and acting on an alert.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in dismissAlertsIn) (*mcp.CallToolResult, any, error) {
		if b == nil || b.WM == nil || b.WM.Observed.Alerts == nil {
			return withDash(b, ctx, "no alert store available"), nil, nil
		}
		store := b.WM.Observed.Alerts
		if in.All {
			n := store.DismissAll()
			return withDash(b, ctx, fmt.Sprintf("dismissed all %d active alerts", n)), nil, nil
		}
		if len(in.IDs) == 0 {
			return withDash(b, ctx, "no ids given and all=false — nothing to dismiss"), nil, nil
		}
		dismissed := 0
		var notFound []int
		for _, id := range in.IDs {
			if store.Dismiss(uint32(id)) {
				dismissed++
			} else {
				notFound = append(notFound, id)
			}
		}
		msg := fmt.Sprintf("dismissed %d/%d alerts", dismissed, len(in.IDs))
		if len(notFound) > 0 {
			msg += fmt.Sprintf(" (not found or already dismissed: %v)", notFound)
		}
		return withDash(b, ctx, msg), nil, nil
	})

	type dwarvesIn struct {
		Verbose bool `json:"verbose,omitempty" jsonschema:"when true, render one line per dwarf (name | current job | top skill | labor count) by querying dwarf_detail for every dwarf — verbose issues one query per dwarf (capped at 50), so use it for censuses, not every turn"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "dwarves",
		Description: "List all dwarves with id, position, and a [DEAD] tag for anyone DF itself flags as dead (killed/ghostly) — a dead dwarf's unit lingers here at its last position, so check the tag rather than assuming stillness means idle. Use dwarf_detail for skills/mood/job of one dwarf, or verbose=true here for a one-line-per-dwarf census (costs one extra query per dwarf — see verbose's schema note).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in dwarvesIn) (*mcp.CallToolResult, any, error) {
		dwarves := b.Snapshot().Entities.Dwarves
		if !in.Verbose {
			return withDash(b, ctx, renderDwarfList(dwarves)), nil, nil
		}
		return withDash(b, ctx, renderDwarvesVerbose(ctx, b, dwarves)), nil, nil
	})

	type dwarfDetailIn struct {
		ID            int  `json:"id" jsonschema:"the dwarf's id from the dwarves tool"`
		IncludeLabors bool `json:"include_labors,omitempty" jsonschema:"true to list every currently-enabled labor by name; default just shows the count (pass true when you actually need to check/change labors)"`
		IncludePsyche bool `json:"include_psyche,omitempty" jsonschema:"true to list full needs/emotions/facets/values/dreams detail; default just shows stress category and item counts (pass true when actually reasoning about this dwarf's mental state)"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "dwarf_detail",
		Description: "One dwarf's full record: dead/alive status, skills, mood, current job, labor count, and a psyche summary (stress category, needs/emotions/facets/values/dreams counts). A dead dwarf's unit stays queryable at its last position reporting mundane idle-looking fields — always check status before trusting the rest. Pass include_labors=true for enabled labor names and/or include_psyche=true for the full psyche breakdown (both omitted by default — boilerplate most calls don't need).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in dwarfDetailIn) (*mcp.CallToolResult, any, error) {
		raw, err := b.Query(ctx, "dwarf_detail", fmt.Sprintf(`{"id":%d}`, in.ID))
		if err != nil {
			return withDash(b, ctx, "query failed: "+err.Error()), nil, nil
		}
		return withDash(b, ctx, renderDwarfDetail(raw, in.IncludeLabors, in.IncludePsyche)), nil, nil
	})

	type stocksIn struct {
		Category string `json:"category,omitempty" jsonschema:"optional case-insensitive substring filter on item type (e.g. 'boulder', 'wood') — narrows the query AND switches the response to full per-material detail for that type"`
		MinCount int    `json:"min_count,omitempty" jsonschema:"optional: hide item/material entries whose free count is below this count; an entry with any built-in (in_use) presence always surfaces regardless of this threshold, since min_count filters free-stock noise, not fort-existence"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "stocks",
		Description: "Stockpile inventory by item type and material — what the fort actually has. count is free/available stock only (not yet built or installed); in_use is the same item/material already incorporated into a building or construction (a built bed, an installed door, a boulder mortared into a wall) — a positive count is what's actually available to assign or build with. Default view is aggregated (one line per item type, top materials); pass category to narrow the query and see full per-material detail for one type. Item types with any above-Ordinary craftsmanship also show a quality-tier breakdown. category=\"mechanism\" is aliased to the game's actual TRAPPARTS item type.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in stocksIn) (*mcp.CallToolResult, any, error) {
		raw, err := b.Query(ctx, "stockpile_inventory", stocksQueryArgs(in.Category))
		if err != nil {
			return withDash(b, ctx, "query failed: "+err.Error()), nil, nil
		}
		return withDash(b, ctx, renderStocks(raw, in.Category != "", in.MinCount)), nil, nil
	})

	type buildingsIn struct {
		Z *int `json:"z,omitempty" jsonschema:"optional: only list buildings on this z-level"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "buildings",
		Description: "List every placed building with position and construction progress. Planned buildings are INVISIBLE in map views until built — this is the only way to see whether a build order is actually being worked or died silently. Optional z filters to one level.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in buildingsIn) (*mcp.CallToolResult, any, error) {
		args := "{}"
		if in.Z != nil {
			args = fmt.Sprintf(`{"z":%d}`, *in.Z)
		}
		raw, err := b.Query(ctx, "list_buildings", args)
		if err != nil {
			return withDash(b, ctx, "query failed: "+err.Error()), nil, nil
		}
		return withDash(b, ctx, renderBuildings(raw)), nil, nil
	})

	// queries.cpp handleWorkshopJobs rejects x<0 || y<0, so the workshop
	// tile is required (no plugin-side default).
	type jobsIn struct {
		X int `json:"x" jsonschema:"workshop tile x"`
		Y int `json:"y" jsonschema:"workshop tile y"`
		Z int `json:"z" jsonschema:"workshop tile z"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "jobs",
		Description: "Jobs queued at one workshop, targeted by its tile (find workshops via building queries).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in jobsIn) (*mcp.CallToolResult, any, error) {
		args := fmt.Sprintf(`{"x":%d,"y":%d,"z":%d}`, in.X, in.Y, in.Z)
		raw, err := b.Query(ctx, "workshop_jobs", args)
		if err != nil {
			return withDash(b, ctx, "query failed: "+err.Error()), nil, nil
		}
		return withDash(b, ctx, string(raw)), nil, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "orders",
		Description: "Manager work orders — what's queued and its status. The orderable item vocabulary lives in the order/queue_job tool schemas, not here.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
		raw, err := b.Query(ctx, "manager_orders", "{}")
		if err != nil {
			return withDash(b, ctx, "query failed: "+err.Error()), nil, nil
		}
		return withDash(b, ctx, renderManagerOrders(raw)), nil, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "mandates",
		Description: "Active noble mandates — item/material/quantity, deadline (in ticks), issuing noble, and the punishment DF has on file for breaking it. Export-ban and production-quota mandates carry real item/material/amount data; Guild (pay-job) mandates are a distinct flavor confirmed only by DF's own naming and may not carry the same meaning in those fields.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
		raw, err := b.Query(ctx, "list_mandates", "{}")
		if err != nil {
			return withDash(b, ctx, "query failed: "+err.Error()), nil, nil
		}
		return withDash(b, ctx, renderMandates(raw)), nil, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "work_details",
		Description: "DF's own work-details/labor-group mechanism (df::work_detail) — the authoritative store behind each dwarf's derived labors that set_labor writes. Lists every detail (built-in and custom): index, name, icon, mode, no_modify/cannot_be_everybody flags, allowed labors, and current member dwarves. Use with assign_work_detail / set_work_detail_mode / create_work_detail.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
		raw, err := b.Query(ctx, "work_details", "{}")
		if err != nil {
			return withDash(b, ctx, "query failed: "+err.Error()), nil, nil
		}
		return withDash(b, ctx, renderWorkDetails(raw)), nil, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "moods",
		Description: "Active strange moods — which dwarf, current mood/moodstage, claimed workshop (or none yet), needed materials with got-vs-needed counts and wildcard flags (any silk, any metal, etc.), and the raw mood_timeout countdown DF tracks toward insanity (starts at 50000). State only — no suggested action; build/queue_job/order are the existing levers.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
		raw, err := b.Query(ctx, "moods", "{}")
		if err != nil {
			return withDash(b, ctx, "query failed: "+err.Error()), nil, nil
		}
		return withDash(b, ctx, renderMoods(raw)), nil, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "noble_demands",
		Description: "Every dwarf holding a noble/administrative position: the position's room-value minimums (office/bedroom/dining/tomb) and furniture-count minimums (boxes/cabinets/racks/stands), plus any of their currently active room/item demands with a deadline (in ticks). Completely separate from mandates (Export/Make/Guild violations) — this is DF's own 'noble wants a cabinet in his office' mechanism. Requirements and demands are reported side by side; no satisfied/violated judgment is computed.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
		raw, err := b.Query(ctx, "noble_demands", "{}")
		if err != nil {
			return withDash(b, ctx, "query failed: "+err.Error()), nil, nil
		}
		return withDash(b, ctx, renderNobleDemands(raw)), nil, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "caravan_status",
		Description: "Active caravans (civ, trade state, days until forced departure, tribute/casualty/hardship/seized/offended flags, trade-session value/mood so far, whether the liaison is actively meeting), scheduled-but-not-yet-arrived caravan/diplomat events, and trade depot readiness (built, accessible, trader_requested, anyone_can_trade). Executing the actual trade exchange stays a human-in-the-client action — see bring_goods_to_depot for what a model can do at a caravan, and depot_goods for what's currently staged.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
		raw, err := b.Query(ctx, "caravan_status", "{}")
		if err != nil {
			return withDash(b, ctx, "query failed: "+err.Error()), nil, nil
		}
		return withDash(b, ctx, renderCaravanStatus(raw)), nil, nil
	})

	type depotGoodsIn struct {
		X *int `json:"x,omitempty" jsonschema:"optional depot tile x; omit (with y/z) to auto-target the first trade depot"`
		Y *int `json:"y,omitempty" jsonschema:"optional depot tile y"`
		Z *int `json:"z,omitempty" jsonschema:"optional depot tile z"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "depot_goods",
		Description: "What's physically staged at the trade depot versus what's already marked for trade and being hauled there but hasn't arrived yet, aggregated by item type + material (not individual item IDs). Auto-targets the first trade depot when x/y/z are omitted.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in depotGoodsIn) (*mcp.CallToolResult, any, error) {
		args := "{}"
		if in.X != nil && in.Y != nil {
			z := 0
			if in.Z != nil {
				z = *in.Z
			}
			args = fmt.Sprintf(`{"x":%d,"y":%d,"z":%d}`, *in.X, *in.Y, z)
		}
		raw, err := b.Query(ctx, "depot_goods", args)
		if err != nil {
			return withDash(b, ctx, "query failed: "+err.Error()), nil, nil
		}
		return withDash(b, ctx, renderDepotGoods(raw)), nil, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "fort_wealth",
		Description: "Overall fort wealth by category (weapons/armor/furniture/architecture/other, plus displayed/held/imported/offered/exported) and fortress_age, DF's own embark-age counter.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
		raw, err := b.Query(ctx, "fort_wealth", "{}")
		if err != nil {
			return withDash(b, ctx, "query failed: "+err.Error()), nil, nil
		}
		return withDash(b, ctx, renderFortWealth(raw)), nil, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "wellbeing",
		Description: "Every living citizen's stress (raw value + DF's own 0-6 category) and focus ratio (current_focus/undistracted_focus) in one pass — the roster-scale counterpart to dwarf_detail's psyche section, which costs one query per dwarf and doesn't scale to a large fort. Raw numbers only, no 'needs attention' verdict.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
		raw, err := b.Query(ctx, "wellbeing", "{}")
		if err != nil {
			return withDash(b, ctx, "query failed: "+err.Error()), nil, nil
		}
		return withDash(b, ctx, renderWellbeing(raw)), nil, nil
	})

	type jobTypesIn struct {
		Filter string `json:"filter,omitempty" jsonschema:"optional case-insensitive substring filter on the job type name (e.g. 'hatch', 'construct') — narrows the ~240-entry DF job_type enum instead of dumping all of it"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "job_types",
		Description: "Look up DFHack job_type enum names for queue_job's name-based path — names not in queue_job's short vocabulary (bed/table/.../blocks) can still be queued by passing the exact name found here as queue_job's item, though queue_job still rejects a few workshop-compatible job types outright (meal, crafts) pending a verified material filter — see queue_job's item description. Call this ONLY when you need to discover a name you don't already know; it is a separate tool from order/queue_job/orders precisely so it isn't paid on every call. Pass filter to narrow.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in jobTypesIn) (*mcp.CallToolResult, any, error) {
		args := "{}"
		if in.Filter != "" {
			argBytes, _ := json.Marshal(map[string]string{"filter": in.Filter})
			args = string(argBytes)
		}
		raw, err := b.Query(ctx, "list_orders", args)
		if err != nil {
			return withDash(b, ctx, "query failed: "+err.Error()), nil, nil
		}
		return withDash(b, ctx, renderJobTypes(raw, in.Filter != "")), nil, nil
	})

	type buildingTypesIn struct {
		Filter string `json:"filter,omitempty" jsonschema:"optional case-insensitive substring filter against the building type's name or category (e.g. 'workshop', 'bed') — narrows the catalog instead of dumping all of it"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "building_types",
		Description: "Look up build's type vocabulary: per-entry category, footprint, and required materials/prerequisites. The curated names here are exactly build's short vocabulary; any other DFHack building_type/workshop_type/furnace_type/trap_type enum name can also be passed to build's type param verbatim, though resolving does not guarantee this plugin can place it yet (build's ACK is truthful about that either way). Call this ONLY when you need a name's specifics; it is a separate tool from build precisely so it isn't paid on every call. Pass filter to narrow.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in buildingTypesIn) (*mcp.CallToolResult, any, error) {
		return withDash(b, ctx, renderBuildingTypes(in.Filter)), nil, nil
	})

	type listReactionsIn struct {
		Filter string `json:"filter,omitempty" jsonschema:"optional case-insensitive substring filter against the reaction code or display name (e.g. 'brew', 'plant') — narrows the fortress-mode reaction catalog instead of dumping all of it"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_reactions",
		Description: "Look up raw-defined df::reaction codes for queue_job's reaction param — this is the ONLY way to queue reactions like brewing that have no job_type mapping at all (queue_job's item vocabulary can't reach them). Pass the exact code found here (e.g. BREW_DRINK_FROM_PLANT) as queue_job's reaction argument. Call this ONLY when you need to discover a code you don't already know; it is a separate tool from queue_job precisely so it isn't paid on every call. Pass filter to narrow.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in listReactionsIn) (*mcp.CallToolResult, any, error) {
		args := "{}"
		if in.Filter != "" {
			argBytes, _ := json.Marshal(map[string]string{"filter": in.Filter})
			args = string(argBytes)
		}
		raw, err := b.Query(ctx, "list_reactions", args)
		if err != nil {
			return withDash(b, ctx, "query failed: "+err.Error()), nil, nil
		}
		return withDash(b, ctx, renderReactions(raw, in.Filter != "")), nil, nil
	})

	type listCropsIn struct {
		Filter string `json:"filter,omitempty" jsonschema:"optional case-insensitive substring filter against the crop's raw token or display name (e.g. 'helmet', 'sweet_pod') — narrows the plantable-crop catalog instead of dumping all of it"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_crops",
		Description: "Look up plantable crops (plants with seeds) for build_farm_plot/assign_crop — each entry's token or name round-trips into assign_crop's crop param verbatim, resolved case-insensitively. Reports underground vs surface eligibility and current seeds on hand. Call this before assign_crop if you don't already know a valid crop name. Pass filter to narrow.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in listCropsIn) (*mcp.CallToolResult, any, error) {
		args := "{}"
		if in.Filter != "" {
			argBytes, _ := json.Marshal(map[string]string{"filter": in.Filter})
			args = string(argBytes)
		}
		raw, err := b.Query(ctx, "list_crops", args)
		if err != nil {
			return withDash(b, ctx, "query failed: "+err.Error()), nil, nil
		}
		return withDash(b, ctx, renderCrops(raw, in.Filter != "")), nil, nil
	})
}
