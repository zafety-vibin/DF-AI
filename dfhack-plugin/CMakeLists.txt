cmake_minimum_required(VERSION 3.10)

# DFHack plugin for DF AI binary protocol.
#
# Dual-mode CMake:
#
#   IN-TREE   When this CMakeLists is invoked as part of a DFHack source
#             build (e.g., via dfhack/plugins/CMakeLists.txt's
#             add_subdirectory(df_ai_protocol)), the parent project has
#             already defined DFHACK_PLUGIN / dfhack_plugin and set up
#             include paths and link libraries. We just call the macro.
#
#   OUT-OF-TREE   When invoked standalone (cmake on this directory with
#                 DFHACK_ROOT pointed at a DFHack install), we load the
#                 install's CMake helpers and call DFHACK_PLUGIN.

# Detect parent-project mode: DFHack defines the plugin macro before
# calling add_subdirectory on plugin folders.
if(NOT COMMAND DFHACK_PLUGIN)
    project(df_ai_protocol)

    if(NOT DEFINED DFHACK_ROOT)
        message(FATAL_ERROR
            "DFHACK_ROOT must be set when building this plugin out-of-tree.\n"
            "Either set DFHACK_ROOT to your DFHack install (so this CMake\n"
            "file can find DFHackPlugin.cmake), or add this directory as a\n"
            "subdirectory of a DFHack source build.")
    endif()

    list(APPEND CMAKE_MODULE_PATH "${DFHACK_ROOT}/CMake")
    include(DFHackPlugin)
endif()

# Define plugin with source files. DFHACK_PLUGIN is supplied either by the
# parent DFHack project (in-tree) or by DFHackPlugin.cmake (out-of-tree).
DFHACK_PLUGIN(df_ai_protocol
    df_ai_protocol.cpp
    tile_extractor.cpp
    tile_updates.cpp
    entities.cpp
    designations.cpp
    plants.cpp
    buildings.cpp
    zones.cpp
    locations.cpp
    work_orders.cpp
    queries.cpp
    announcements.cpp
    LINK_LIBRARIES clsocket
)

# C++20 — DFHack 53.x internal headers (MiscUtils.h, BitArray.h, Units.h)
# use std::invocable, std::ranges, std::views, and concepts. Plugins MUST
# compile against C++20 or the DFHack headers themselves fail to compile.
set_target_properties(df_ai_protocol PROPERTIES
    CXX_STANDARD 20
    CXX_STANDARD_REQUIRED ON
)

# Install rule only applies out-of-tree; in-tree DFHack builds handle
# installation through their own packaging step.
if(NOT COMMAND DFHACK_PLUGIN_PARENT_PROJECT AND DEFINED DFHACK_ROOT)
    install(TARGETS df_ai_protocol
        LIBRARY DESTINATION "${DFHACK_ROOT}/hack/plugins"
        RUNTIME DESTINATION "${DFHACK_ROOT}/hack/plugins"
    )
endif()
