cmake_minimum_required(VERSION 3.10)

# DFHack plugin for DF AI binary protocol
project(df_ai_protocol)

# Find DFHack
if(NOT DEFINED DFHACK_ROOT)
    message(FATAL_ERROR "DFHACK_ROOT must be set to your DFHack installation directory")
endif()

# Include DFHack CMake modules
list(APPEND CMAKE_MODULE_PATH "${DFHACK_ROOT}/CMake")
include(DFHackPlugin)

# Define plugin with source files
DFHACK_PLUGIN(df_ai_protocol
    df_ai_protocol.cpp
    tile_extractor.cpp
    LINK_LIBRARIES clsocket
)

# Set C++17 standard
set_target_properties(df_ai_protocol PROPERTIES
    CXX_STANDARD 17
    CXX_STANDARD_REQUIRED ON
)

# Installation instructions
install(TARGETS df_ai_protocol
    LIBRARY DESTINATION "${DFHACK_ROOT}/hack/plugins"
    RUNTIME DESTINATION "${DFHACK_ROOT}/hack/plugins"
)
