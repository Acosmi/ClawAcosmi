cmake_minimum_required(VERSION 3.16)
project(acosmi-cpp VERSION 0.1.0 LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_POSITION_INDEPENDENT_CODE ON)

# --- 视觉处理库 ---
add_library(acosmi_vision STATIC
    cpp/vision/src/vision_bridge.cpp
)
target_include_directories(acosmi_vision PUBLIC
    cpp/vision/include
)

# 可选: OpenCV 集成 (Phase 8)
# find_package(OpenCV QUIET)
# if(OpenCV_FOUND)
#     target_link_libraries(acosmi_vision PUBLIC ${OpenCV_LIBS})
#     target_compile_definitions(acosmi_vision PUBLIC ACOSMI_HAS_OPENCV)
# endif()

# --- 推理引擎库 (Phase 8) ---
# add_library(acosmi_inference STATIC
#     cpp/inference/src/inference_bridge.cpp
# )
# target_include_directories(acosmi_inference PUBLIC
#     cpp/inference/include
# )

# --- 安装规则 ---
install(TARGETS acosmi_vision
    ARCHIVE DESTINATION lib
)
install(DIRECTORY cpp/vision/include/
    DESTINATION include
)

message(STATUS "Acosmi C++ 库配置完成")
message(STATUS "  视觉处理: ON")
message(STATUS "  推理引擎: OFF (Phase 8)")
