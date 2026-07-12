// DFHack Plugin — Manager Work Orders
//
// Implements WORK_ORDER commands by adding entries to the fortress's
// manager order queue. The manager dispatches each order to whichever
// workshop can handle it, fetching reagents from stockpiles automatically.
//
// IMPORTANT: The manager_order struct and df::global::world->manager_orders
// field have shifted across DF 53.x versions. This implementation targets
// DFHack 53.12 conventions. When upgrading, verify against the SDK headers:
//
//   df/manager_order.h          — manager_order struct fields
//   df/job_type.h               — JobType enum (ConstructBed etc.)
//   df/world.h                  — manager_orders.all field name & type
//   modules/Job.h               — any helper for allocating manager_order
//
// On a fresh DFHack version, the most likely issue is the JobType enum
// value names (e.g., ConstructBed vs MakeBed) or the manager_order
// allocation pattern (some versions use Job::createManagerOrder helper,
// others require direct struct construction and push_back).

#include "Core.h"
#include "Console.h"
#include "modules/Job.h"

#include "df/job_type.h"
#include "df/manager_order.h"
#include "df/world.h"

#include "protocol.h"

#include <vector>
#include <string>
#include <cstdio>

using namespace DFHack;

// Map protocol order type → df::job_type. Returns -1 if unrecognized.
//
// VERIFY: Each JobType name against df/job_type.h in DFHack 53.12. Some
// names may differ (e.g., ConstructTable vs MakeTable). Item-production
// jobs typically include ConstructBed/Table/Chair/Door/Cabinet/Coffer
// (Box) plus BrewDrink, PrepareMeal, MakeCrafts.
static int protocolToJobType(uint8_t orderType) {
    switch (orderType) {
        case ORDER_TYPE_MAKE_BED:     return df::job_type::ConstructBed;
        case ORDER_TYPE_MAKE_TABLE:   return df::job_type::ConstructTable;
        case ORDER_TYPE_MAKE_CHAIR:   return df::job_type::ConstructThrone;   // "Throne" in code = chair
        case ORDER_TYPE_MAKE_DOOR:    return df::job_type::ConstructDoor;
        case ORDER_TYPE_MAKE_BARREL:  return df::job_type::MakeBarrel;
        case ORDER_TYPE_MAKE_BUCKET:  return df::job_type::MakeBucket;
        case ORDER_TYPE_MAKE_CABINET: return df::job_type::ConstructCabinet;
        case ORDER_TYPE_MAKE_COFFER:  return df::job_type::ConstructChest;     // 53.12 renamed Box → Chest
        case ORDER_TYPE_BREW_DRINK:   return -1;                                // BrewDrink not in 53.12 job_type; needs reagent-based reaction lookup
        case ORDER_TYPE_PREPARE_MEAL: return df::job_type::PrepareMeal;        // VERIFY: in some versions takes a meal-size param
        case ORDER_TYPE_MAKE_BLOCKS:  return df::job_type::ConstructBlocks;
        case ORDER_TYPE_MAKE_CRAFTS:  return df::job_type::MakeCrafts;
        default: return -1;
    }
}

bool applyWorkOrder(uint8_t orderType, uint16_t quantity, std::string &error)
{
    int jobType = protocolToJobType(orderType);
    if (jobType < 0) {
        char buf[64];
        snprintf(buf, sizeof(buf), "Unknown work order type: 0x%02X", orderType);
        error = buf;
        return false;
    }

    if (!df::global::world) {
        error = "world is null";
        return false;
    }

    // Build a manager_order entry. The simple-and-safe path is to
    // allocate, populate, and push onto world->manager_orders.all.
    df::manager_order* order = new df::manager_order();
    if (!order) {
        error = "manager_order allocation failed";
        return false;
    }

    // VERIFY: the field names below against df/manager_order.h. Common
    // shape: job_type, item_type, item_subtype, mat_type, mat_index,
    // amount_total, amount_left, status. Some versions split status into
    // a flags struct.
    order->job_type = (df::job_type) jobType;
    order->item_type = df::item_type::NONE;       // let manager pick item subtype
    order->item_subtype = -1;
    order->mat_type = -1;                          // any material (manager picks)
    order->mat_index = -1;
    order->amount_total = quantity;
    order->amount_left = quantity;
    // DFHack 53.12: manager_order::status is a bitfield union, not a flag.
    // Set the validated bit so the manager dispatches the order.
    order->status.bits.validated = 1;

    // VERIFY: the world->manager_orders field name. Some versions
    // (manager_orders), some (manager_order_count), some store a struct
    // with an .all vector. If the build complains, check world.h.
    df::global::world->manager_orders.all.push_back(order);

    return true;
}
