// swift-tools-version: 6.2

import PackageDescription

let package = Package(
    name: "OpenAcosmiKit",
    platforms: [
        .iOS(.v18),
        .macOS(.v15),
    ],
    products: [
        .library(name: "OpenAcosmiProtocol", targets: ["OpenAcosmiProtocol"]),
        .library(name: "OpenAcosmiKit", targets: ["OpenAcosmiKit"]),
        .library(name: "OpenAcosmiChatUI", targets: ["OpenAcosmiChatUI"]),
    ],
    dependencies: [
        .package(url: "https://github.com/steipete/ElevenLabsKit", exact: "0.1.0"),
        .package(url: "https://github.com/gonzalezreal/textual", exact: "0.3.1"),
    ],
    targets: [
        .target(
            name: "OpenAcosmiProtocol",
            path: "Sources/OpenAcosmiProtocol",
            swiftSettings: [
                .enableUpcomingFeature("StrictConcurrency"),
            ]),
        .target(
            name: "OpenAcosmiKit",
            dependencies: [
                "OpenAcosmiProtocol",
                .product(name: "ElevenLabsKit", package: "ElevenLabsKit"),
            ],
            path: "Sources/OpenAcosmiKit",
            resources: [
                .process("Resources"),
            ],
            swiftSettings: [
                .enableUpcomingFeature("StrictConcurrency"),
            ]),
        .target(
            name: "OpenAcosmiChatUI",
            dependencies: [
                "OpenAcosmiKit",
                .product(
                    name: "Textual",
                    package: "textual",
                    condition: .when(platforms: [.macOS, .iOS])),
            ],
            path: "Sources/OpenAcosmiChatUI",
            swiftSettings: [
                .enableUpcomingFeature("StrictConcurrency"),
            ]),
        .testTarget(
            name: "OpenAcosmiKitTests",
            dependencies: ["OpenAcosmiKit", "OpenAcosmiChatUI"],
            path: "Tests/OpenAcosmiKitTests",
            swiftSettings: [
                .enableUpcomingFeature("StrictConcurrency"),
                .enableExperimentalFeature("SwiftTesting"),
            ]),
    ])
