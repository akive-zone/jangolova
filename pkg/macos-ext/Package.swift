// swift-tools-version: 5.10
import PackageDescription

let package = Package(
    name: "JangolovaMacExtension",
    platforms: [.macOS(.v13)],
    products: [
        .library(name: "JangolovaMacCore", targets: ["JangolovaMacCore"]),
        .executable(name: "jangolova-macos-ext", targets: ["JangolovaMacApp"]),
    ],
    dependencies: [
        .package(path: "../macos-cymonkey-helper"),
    ],
    targets: [
        .target(
            name: "JangolovaMacCore",
            dependencies: [
                .product(name: "CymonkeyMacOSRuntime", package: "macos-cymonkey-helper"),
            ]
        ),
        .executableTarget(
            name: "JangolovaMacApp",
            dependencies: ["JangolovaMacCore"]
        ),
        .testTarget(name: "JangolovaMacCoreTests", dependencies: ["JangolovaMacCore"]),
    ]
)
