package skill

// NewStarterLibrary returns a library seeded with operational recipes for
// early-fortress survival and growth. These are the skills a new DF
// player typically learns in their first dozen forts: get inside, get
// food, get bedrooms, get a workshop chain, defend the entrance.
//
// Each is intentionally under-specified. The LLM picks dimensions,
// materials, locations, and quality based on context (terrain, dwarf
// count, biome, danger level).
func NewStarterLibrary() *Library {
	lib := NewLibrary()

	lib.Register(carveInitialShelter())
	lib.Register(digSafeStairwell())
	lib.Register(buildBedroom())
	lib.Register(setupWorkshopChain())
	lib.Register(setupStockpile())
	lib.Register(produceFurniture())
	lib.Register(establishFoodSupply())
	lib.Register(createDiningHall())
	lib.Register(secureEntrance())
	lib.Register(secureWaterAccess())
	lib.Register(handleAquiferLayer())

	return lib
}

func carveInitialShelter() Skill {
	return Skill{
		Name:          "carve_initial_shelter",
		Summary:       "Get the embark crew underground — the priority before anything else. Dwarves on the surface are exposed to weather, ambush, and starvation. The standard opening is stair-down → carve below.",
		Prerequisites: []string{"a tile near the embark wagon on solid surface (grass, soil, sand)"},
		Provides:      []string{"has_modified_anything", "has_shelter"},
		Steps: []Step{
			{
				ID:          "dig_entry_stair",
				Action:      "dig",
				Description: "Dig a downward stair shaft from the surface into solid ground. Use `dig stairs from (x, y, z_surface) to (x, y, z_surface - 2)` — a 2x2 column scales well until you exceed hundreds of dwarves, 3 Z deep, near the embark wagon.",
				Tips: []string{
					"Pick a tile your dwarves can already reach. Surface tiles next to the wagon are safe.",
					"The system fills in UpStair / UpDownStair / DownStair across the Z range.",
				},
			},
			{
				ID:          "carve_initial_room",
				Action:      "dig",
				Description: "At the BOTTOM of the stair shaft, carve a horizontal room large enough to absorb the embark crew.",
				DependsOn:   []string{"dig_entry_stair"},
				Tips: []string{
					"The bottom of the stair is the room's entrance. Carve outward from there.",
					"Single Z-level. Don't channel through the floor or ceiling.",
				},
			},
		},
	}
}

func digSafeStairwell() Skill {
	return Skill{
		Name:    "dig_safe_stairwell",
		Summary: "Connect Z-levels with a stair shaft so dwarves can move vertically.",
		Steps: []Step{
			{
				ID:          "scout_target_z",
				Action:      "wait",
				Description: "Use region_detail to verify the target Z-levels are not hidden, not flooded, and have walkable terrain at top and bottom.",
				Tips: []string{
					"The plugin rejects stair shafts that pass through undiscovered tiles. If it fails, dig down one Z at a time using sequential commands.",
				},
			},
			{
				ID:          "carve_shaft",
				Action:      "dig",
				Description: "Dig stairs as a SINGLE-COLUMN command spanning the full Z-range. Use `dig stairs from (x, y, z_top) to (x, y, z_bottom)`.",
				DependsOn:   []string{"scout_target_z"},
				Tips: []string{
					"Single column means x1==x2 AND y1==y2. Multi-column shafts must be issued as separate commands per (x, y).",
					"The system fills in UpStair / UpDownStair / DownStair automatically across the range.",
				},
			},
		},
		Tips: []string{
			"For wider stairwells, repeat the single-column command in a 2×2 or 3×3 grid of (x, y) coordinates.",
			"Stairs at corners of rooms are space-efficient. Center stairwells eat usable floor space.",
		},
	}
}

func buildBedroom() Skill {
	return Skill{
		Name:          "build_bedroom",
		Summary:       "Excavate and furnish a personal bedroom for one dwarf. The same recipe scales from peasant huts to royal suites — the difference is dimensions, material quality, and decorative furniture.",
		Prerequisites: []string{"a bed exists in stockpile (or order one via produce_furniture)", "a wall to dig into"},
		Provides:      []string{"has_bedroom_zones"},
		Steps: []Step{
			{
				ID:          "carve_room",
				Action:      "dig",
				Description: "Carve an enclosed rectangular room.",
				Tips: []string{
					"Smallest functional bedroom is 2×3. Peasant rooms 3×3 to 4×4. Noble rooms 5×5+. Royal rooms 7×7+ with separate sleeping/dining/office areas.",
					"Single Z-level. Don't channel through to a level below or you'll need to fill it.",
				},
			},
			{
				ID:          "place_door",
				Action:      "build",
				Description: "Place a door at the room entrance to give privacy and restrict access.",
				DependsOn:   []string{"carve_room"},
				Tips: []string{
					"Doors require a wood, stone, metal, or glass door item from a stockpile. Order via produce_furniture if none exist.",
					"For nobles: prefer high-value materials (gold, silver) for the door material.",
				},
			},
			{
				ID:          "place_bed",
				Action:      "build",
				Description: "Place a bed inside the room.",
				DependsOn:   []string{"carve_room"},
				Tips: []string{
					"Beds are made from wood by a Carpenter. Always order at least one bed per dwarf early.",
					"Quality beds (well-crafted, masterwork) raise the room value and dwarf happiness.",
				},
			},
			{
				ID:          "place_storage",
				Action:      "build",
				Description: "Add a cabinet (clothes) and coffer (valuables). Optional but standard.",
				DependsOn:   []string{"carve_room"},
				Optional:    true,
				Tips: []string{
					"Without storage, dwarves drop owned items in the dining hall — clutter problem.",
					"Cabinets and coffers can be wood, stone, or metal. Match material to the dwarf's status.",
				},
			},
			{
				ID:          "designate_zone",
				Action:      "zone",
				Description: "Designate the room area as a Bedroom zone. Once a dwarf claims a bed, the room becomes theirs.",
				DependsOn:   []string{"carve_room", "place_bed"},
				Tips: []string{
					"The zone should cover the whole room interior — every tile, including the bed.",
					"Zone size affects room VALUE. Bigger room = higher value if furniture is decorative.",
				},
			},
		},
		Tips: []string{
			"A 'good' bedroom for a noble has high room VALUE. Sources: large dimensions, masterwork bed, decorated cabinet, smoothed/engraved walls (later skills).",
			"For peasants, prioritize speed and density over quality. A 3×3 stone-floor room with a wooden bed is plenty.",
			"Don't build many bedrooms before you have many beds — placement queues will stall and clutter the plan.",
		},
	}
}

func setupWorkshopChain() Skill {
	return Skill{
		Name:    "setup_workshop_chain",
		Summary: "Place a workshop and the stockpiles that feed and consume from it, so the work order chain actually flows.",
		Steps: []Step{
			{
				ID:          "carve_workshop_room",
				Action:      "dig",
				Description: "Carve a room large enough for a 3×3 workshop plus walking space — minimum 5×5.",
				Tips: []string{
					"Workshops are 3×3 with the (x,y,z) coord being the CENTER tile. Surrounding 8 tiles must be clear floor.",
				},
			},
			{
				ID:          "place_workshop",
				Action:      "build",
				Description: "Place the workshop building (carpenter, mason, still, kitchen, etc.) at the room center.",
				DependsOn:   []string{"carve_workshop_room"},
				Tips: []string{
					"Workshops require a build material (a log for carpenter, a stone for mason, etc.). Make sure one is available.",
					"Prefer carpenter first — produces beds, barrels, and most basic furniture from logs.",
				},
			},
			{
				ID:          "carve_input_stockpile",
				Action:      "dig",
				Description: "Carve an adjacent area for input materials (logs for carpenter, stones for mason).",
				DependsOn:   []string{"carve_workshop_room"},
				Optional:    true,
				Tips: []string{
					"Adjacent stockpiles drastically speed up workshop throughput.",
				},
			},
		},
		Tips: []string{
			"Workshops without nearby material stockpiles cause dwarves to walk across the fort for each job.",
			"Cluster related workshops: carpenter near logs, mason near stones, smelter near ores.",
		},
	}
}

func setupStockpile() Skill {
	return Skill{
		Name:    "setup_stockpile",
		Summary: "Designate a stockpile so dwarves haul items into it. Stockpiles are the connective tissue of a fort — workshops produce, stockpiles store, hauling routes form between them.",
		Steps: []Step{
			{
				ID:          "carve_stockpile_area",
				Action:      "dig",
				Description: "Carve a clear floor area sized to expected throughput. Even a quick fort wants 6×6+ for general items.",
				Tips: []string{
					"Stockpile tiles must be walkable floor (not wall, not open air). Dig out the area first if underground.",
					"Larger isn't always better — long hauling distances slow dwarves. Many small stockpiles near workshops beats one giant one.",
				},
			},
			{
				ID:          "designate_stockpile",
				Action:      "stockpile",
				Description: "Designate the floor area as a stockpile. Use \"stockpile from ... to ...\" for an everything stockpile, or \"stockpile <category> from ...\" for a focused one.",
				DependsOn:   []string{"carve_stockpile_area"},
				Tips: []string{
					"Early-fort default: ONE everything stockpile near the entrance to gather embark goods. Specialize later.",
					"Category options: food, furniture, stone, wood, weapons, armor, ammo, leather, cloth, gems, finished_goods, bars_blocks, animals, refuse, coins, corpses, sheet.",
					"First time a category-specific stockpile is placed, you may need to manually accept-all in the DF UI tab once.",
				},
			},
		},
		Tips: []string{
			"Stockpiles next to workshops are the BIGGEST performance multiplier in a fort — workshop throughput scales with how close materials are.",
			"Pattern: workshop_chain → carpenter workshop adjacent to a wood stockpile and a finished_goods stockpile. Logs flow in, beds flow out.",
			"For early embarks, just one 'everything' stockpile near the entrance is enough — keeps starting wagon goods organized.",
		},
	}
}

func produceFurniture() Skill {
	return Skill{
		Name:          "produce_furniture",
		Summary:       "Use the manager's work order queue to produce beds, doors, tables, etc. so build_bedroom and friends have items to place.",
		Prerequisites: []string{"a relevant workshop is built and reachable", "raw materials in stockpile (logs for wood, stones for stone, etc.)"},
		Steps: []Step{
			{
				ID:          "queue_orders",
				Action:      "order",
				Description: "Issue manager work orders for the items you'll need (e.g., `order 7 bed` for housing the embark crew + first migrant wave).",
				Tips: []string{
					"Order items in slight surplus — dwarves can claim only existing items.",
					"For a fresh fort: 1 carpenter workshop + `order 10 bed`, `order 7 door`, `order 5 barrel`, `order 5 bucket` covers most early needs.",
				},
			},
		},
		Tips: []string{
			"Manager orders REQUIRE a manager dwarf with a manager office. Without one, orders queue but never dispatch.",
			"You can also queue work orders directly at a workshop, but the manager queue is more reliable for repeatable production.",
		},
	}
}

func establishFoodSupply() Skill {
	return Skill{
		Name:     "establish_food_supply",
		Summary:  "Get a sustainable food source running before the embark stockpile runs out (~1-2 seasons).",
		Provides: []string{"has_food_supply"},
		Steps: []Step{
			{
				ID:          "scout_arable_z",
				Action:      "wait",
				Description: "Identify a Z-level with soil tiles. Underground soil is needed for most farm plots; above-ground soil works too if dwarves can walk safely.",
				Tips: []string{
					"Use region_detail to find soil. Surface soil is fastest to access; underground soil layers are 2-5 Z below the surface.",
				},
			},
			{
				ID:          "carve_farm_room",
				Action:      "dig",
				Description: "Carve a room over the soil tiles. Typical first farm: 5×5 (25 tiles).",
				DependsOn:   []string{"scout_arable_z"},
				Tips: []string{
					"Underground farms need to be on a soil layer (loam, clay, sand, silt). Stone-floor rooms can't grow crops.",
				},
			},
			{
				ID:          "designate_farm",
				Action:      "zone",
				Description: "Designate the room as a Farm zone (NOTE: farm zone designation may not yet be supported in this plugin build — check tool result).",
				DependsOn:   []string{"carve_farm_room"},
			},
			{
				ID:          "place_still",
				Action:      "build",
				Description: "Build a Still nearby for brewing drinks (dwarves prefer alcohol over water).",
				Optional:    true,
				Tips: []string{
					"Brewing is the cheapest way to keep dwarves happy. A still + farm + plant gathering = self-sustaining.",
				},
			},
		},
		Tips: []string{
			"Embark food stockpile lasts roughly 60-100 days for 7 dwarves. Get production going before day 30.",
			"Plant gathering (gather command) on the surface gives a quick burst of food until farms produce.",
			"Fishing zones near water are an alternative if soil is scarce.",
		},
	}
}

func createDiningHall() Skill {
	return Skill{
		Name:     "create_dining_hall",
		Summary:  "A communal eating space doubles as a meeting area — central to fort happiness and morale.",
		Provides: []string{"has_dining_hall"},
		Steps: []Step{
			{
				ID:          "expand_initial_room",
				Action:      "dig",
				Description: "Reuse or expand the initial shelter into a larger room (10×10+ for a small fort, 20×20 for a mature one).",
				Tips: []string{
					"Dining halls scale with population. Plan for room to grow as migrants arrive.",
				},
			},
			{
				ID:          "place_tables_chairs",
				Action:      "build",
				Description: "Place pairs of tables and chairs around the room. Each pair seats one dwarf.",
				DependsOn:   []string{"expand_initial_room"},
				Tips: []string{
					"Order tables and chairs from the carpenter or mason first.",
					"Quality matters — masterwork tables/chairs raise room value substantially.",
				},
			},
			{
				ID:          "designate_zone",
				Action:      "zone",
				Description: "Designate the room as a Dining Hall + Meeting Hall zone.",
				DependsOn:   []string{"place_tables_chairs"},
				Tips: []string{
					"Combine dining + meeting on the same zone — saves a designation and creates a natural social hub.",
				},
			},
		},
		Tips: []string{
			"A nice dining hall is the cheapest happiness multiplier in DF.",
			"Smoothed and engraved walls (skills coming later) raise dining hall quality dramatically.",
		},
	}
}

func secureEntrance() Skill {
	return Skill{
		Name:    "secure_entrance",
		Summary: "Reduce the fort's surface exposure to a single chokepoint that can be sealed against goblins, undead, and wild beasts.",
		Steps: []Step{
			{
				ID:          "narrow_to_chokepoint",
				Action:      "build",
				Description: "Wall off all surface entrances except one tunnel.",
				Tips: []string{
					"Constructed walls are placed with `build wall at (x, y, z)`. Each wall needs blocks or a boulder.",
				},
			},
			{
				ID:          "place_door",
				Action:      "build",
				Description: "Place a door at the chokepoint.",
				DependsOn:   []string{"narrow_to_chokepoint"},
				Tips: []string{
					"Doors can be locked from the inside, blocking pathing for hostiles.",
				},
			},
			{
				ID:          "place_drawbridge",
				Action:      "build",
				Description: "Build a drawbridge at the chokepoint, linked to a lever.",
				DependsOn:   []string{"narrow_to_chokepoint"},
				Optional:    true,
				Tips: []string{
					"Drawbridges fully seal the entrance when raised. The most secure option, but requires mechanic + lever skill.",
				},
			},
		},
		Tips: []string{
			"Dwarves can't path through closed doors or raised drawbridges — but neither can hostiles. Don't seal trapped friends inside.",
			"A second emergency exit (deeper underground) is wise — sieges that last seasons can starve a sealed fort.",
		},
	}
}

// handleAquiferLayer describes the standard light-aquifer pattern for
// fortresses that don't want to engineer around heavy aquifers. The skill
// is only correct for LIGHT aquifers — heavy aquifers need pumps and
// double-slit drainage, which is out of scope. The agent should bail to
// "dig elsewhere" if it sees signs of a heavy aquifer (continuous water
// across an entire layer that doesn't slow down after smoothing).
func handleAquiferLayer() Skill {
	return Skill{
		Name:    "handle_aquifer_layer",
		Summary: "Pass through a light aquifer layer without flooding the fort. Two strategies: smooth stone aquifer walls (for stone layers) or replace dirt/soil aquifer walls with constructed walls. The cheapest path is usually 'dig straight through and seal' rather than 'build a fortress on the aquifer level'.",
		Prerequisites: []string{
			"a vertical shaft (or planned shaft) that intersects the aquifer layer",
			"a way to reach the aquifer Z from above (downstair already dug)",
		},
		Steps: []Step{
			{
				ID:          "identify_aquifer_z",
				Action:      "wait",
				Description: "Use region_detail on the suspected aquifer Z to confirm the layer is wet (water tiles weeping in from walls). Check the layer above and below — if dry rock above and dry rock below, you have a single bounded aquifer layer. If multiple wet layers stacked, abort: this is likely a heavy aquifer and the smooth-and-seal pattern won't work.",
				Tips: []string{
					"Light aquifers in DF 53.x typically span 1-3 Z-levels in soil or sedimentary stone (sandstone, conglomerate, dolomite, limestone, chalk).",
					"If you see a `2+_aquifer` flag-equivalent in the snapshot or full water flow across the entire layer, suspect heavy. Skip this skill.",
				},
			},
			{
				ID:          "dig_through_layer",
				Action:      "dig",
				Description: "Continue the stair shaft through the aquifer layer to clean rock below. Do NOT carve a horizontal room ON the aquifer Z — every wall you expose will leak. The pattern is: pierce the layer with the shaft, leak a little water during digging, then seal.",
				DependsOn:   []string{"identify_aquifer_z"},
				Tips: []string{
					"Expect minor water during digging. Dwarves will push through if there's a path back up out of the wet zone.",
					"The exposed walls of the shaft on the aquifer Z(s) are the only tiles you need to seal afterward — keep the shaft narrow (single column or 2x2) to minimize sealing work.",
				},
			},
			{
				ID:          "seal_stone_walls",
				Action:      "smooth",
				Description: "On stone aquifer Z-levels: smooth every exposed wall tile of the shaft. Smoothed natural stone stops weeping. Use `smooth from (x1, y1, z) to (x2, y2, z)` covering the perimeter of the shaft on each affected Z.",
				DependsOn:   []string{"dig_through_layer"},
				Optional:    true,
				Tips: []string{
					"Smooth only works on natural stone walls/floors — DF will skip soil, sand, and constructed tiles automatically.",
					"You only need to smooth the wall tiles bordering the shaft, not the inside of the shaft itself. A 2x2 shaft has 8 wall tiles per Z.",
					"If smoothing doesn't stop the weep within ~50 ticks, the tile is probably soil — switch to constructed walls (the next step).",
				},
			},
			{
				ID:          "seal_soil_walls",
				Action:      "build",
				Description: "On dirt/soil/sand aquifer Z-levels (or on stone tiles where smoothing didn't take): replace each exposed wall tile with a constructed wall. Use `build wall at (x, y, z)` for each leaking tile. Constructed walls are watertight regardless of the original material.",
				DependsOn:   []string{"dig_through_layer"},
				Optional:    true,
				Tips: []string{
					"Constructed walls require blocks or boulders. Order blocks first (`order 20 blocks`) and have a stockpile near the shaft.",
					"Don't build constructed walls preemptively — only on tiles that are actually leaking. Saves materials and dwarf labor.",
				},
			},
		},
		Tips: []string{
			"DESIGN PRINCIPLE: do not build the fortress ON the aquifer Z-level. Pierce through it; build above (surface-adjacent) or below (deep stone). Building inside an aquifer means every exposed tile is a leak risk forever.",
			"If the aquifer is high up (close to surface), drilling straight down is the cheapest path. If it's low (deep), consider whether the fortress fits between surface and aquifer; sometimes you don't need to pierce it at all.",
			"After the seal, channel a single tile down at the bottom of the shaft to verify no water is pooling. If it's dry, the seal is good.",
			"This skill assumes light aquifer. Heavy aquifers (continuous water across the whole map at a Z, no dry pockets) require pumps, double-slit drainage, and a multi-season engineering project — out of scope.",
		},
	}
}

func secureWaterAccess() Skill {
	return Skill{
		Name:    "secure_water_access",
		Summary: "Wells and water access for hospital + dwarf hydration during sieges or magma flooding.",
		Steps: []Step{
			{
				ID:          "find_water_source",
				Action:      "wait",
				Description: "Locate an underground river, pond, or aquifer near the fort using region_detail and hazard counts.",
				Tips: []string{
					"Aquifers can be tapped via well — but breaching one without preparation floods levels.",
					"Surface ponds work but freeze in winter biomes.",
				},
			},
			{
				ID:          "channel_to_water",
				Action:      "dig",
				Description: "Channel down to the water source from a level above so dwarves don't drown digging.",
				DependsOn:   []string{"find_water_source"},
				Tips: []string{
					"Channel = remove the floor of the tile above. Always channel from above, never directly into water.",
				},
			},
			{
				ID:          "build_well",
				Action:      "build",
				Description: "Build a well over the water tile.",
				DependsOn:   []string{"channel_to_water"},
				Optional:    true,
				Tips: []string{
					"Wells require a mechanism + rope/chain + bucket + block.",
					"Without a well, hospitals can't clean wounds — infection risk.",
				},
			},
		},
		Tips: []string{
			"Don't tap aquifers with the build command directly — open them with carefully placed channels.",
			"Wells are quality-of-life early; survival-critical only when sieges last seasons.",
		},
	}
}
