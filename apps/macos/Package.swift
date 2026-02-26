// swift-tools-version: 6.2
// Package manifest for the OpenAcosmi macOS companion (menu bar app + IPC library).

import PackageDescription

let package = Package(
    name: "OpenAcosmi",
    platforms: [
        .macOS(.v15),
    ],
    products: [
        .library(name: "OpenAcosmiIPC", targets: ["OpenAcosmiIPC"]),
        .library(name: "OpenAcosmiDiscovery", targets: ["OpenAcosmiDiscovery"]),
        .executable(name: "OpenAcosmi", targets: ["OpenAcosmi"]),
        .executable(name: "openacosmi-mac", targets: ["OpenAcosmiMacCLI"]),
    ],
    dependencies: [
        .package(url: "https://github.com/orchetect/MenuBarExtraAccess", exact: "1.2.2"),
        .package(url: "https://github.com/swiftlang/swift-subprocess.git", from: "0.1.0"),
        .package(url: "https://github.com/apple/swift-log.git", from: "1.8.0"),
        .package(url: "https://github.com/sparkle-project/Sparkle", from: "2.8.1"),
        .package(url: "https://github.com/steipete/Peekaboo.git", branch: "main"),
        .package(path: "../shared/OpenAcosmiKit"),
        .package(path: "../../Swabble"),
    ],
    targets: [
        .target(
            name: "OpenAcosmiIPC",
            dependencies: [],
            swiftSettings: [
                .enableUpcomingFeature("StrictConcurrency"),
            ]),
        .target(
            name: "OpenAcosmiDiscovery",
            dependencies: [
                .product(name: "OpenAcosmiKit", package: "OpenAcosmiKit"),
            ],
            path: "Sources/OpenAcosmiDiscovery",
            swiftSettings: [
                .enableUpcomingFeature("StrictConcurrency"),
            ]),
        .executableTarget(
            name: "OpenAcosmi",
            dependencies: [
                "OpenAcosmiIPC",
                "OpenAcosmiDiscovery",
                .product(name: "OpenAcosmiKit", package: "OpenAcosmiKit"),
                .product(name: "OpenAcosmiChatUI", package: "OpenAcosmiKit"),
                .product(name: "OpenAcosmiProtocol", package: "OpenAcosmiKit"),
                .product(name: "SwabbleKit", package: "swabble"),
                .product(name: "MenuBarExtraAccess", package: "MenuBarExtraAccess"),
                .product(name: "Subprocess", package: "swift-subprocess"),
                .product(name: "Logging", package: "swift-log"),
                .product(name: "Sparkle", package: "Sparkle"),
                .product(name: "PeekabooBridge", package: "Peekaboo"),
                .product(name: "PeekabooAutomationKit", package: "Peekaboo"),
            ],
            exclude: [
                "Resources/Info.plist",
            ],
            resources: [
                .copy("Resources/OpenAcosmi.icns"),
                .copy("Resources/DeviceModels"),
            ],
            swiftSettings: [
                .enableUpcomingFeature("StrictConcurrency"),
            ]),
        .executableTarget(
            name: "OpenAcosmiMacCLI",
            dependencies: [
                "OpenAcosmiDiscovery",
                .product(name: "OpenAcosmiKit", package: "OpenAcosmiKit"),
                .product(name: "OpenAcosmiProtocol", package: "OpenAcosmiKit"),
            ],
            path: "Sources/OpenAcosmiMacCLI",
            swiftSettings: [
                .enableUpcomingFeature("StrictConcurrency"),
            ]),
        .testTarget(
            name: "OpenAcosmiIPCTests",
            dependencies: [
                "OpenAcosmiIPC",
                "OpenAcosmi",
                "OpenAcosmiDiscovery",
                .product(name: "OpenAcosmiProtocol", package: "OpenAcosmiKit"),
                .product(name: "SwabbleKit", package: "swabble"),
            ],
            swiftSettings: [
                .enableUpcomingFeature("StrictConcurrency"),
                .enableExperimentalFeature("SwiftTesting"),
            ]),
    ])
