// swift-tools-version: 5.9
import PackageDescription

let package = Package(
    name: "M",
    platforms: [
        .iOS(.v17),
        .macOS(.v14)
    ],
    products: [
        .library(
            name: "M",
            targets: ["M"]
        ),
    ],
    targets: [
        .target(
            name: "M",
            path: "Sources"
        ),
        .testTarget(
            name: "MTests",
            dependencies: ["M"],
            path: "Tests"
        ),
    ]
)
