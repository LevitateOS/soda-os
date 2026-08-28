import AppKit
import Foundation

struct Asset {
    let source: String
    let output: String
    let width: Int
    let height: Int
}

let root = URL(fileURLWithPath: FileManager.default.currentDirectoryPath)
guard FileManager.default.fileExists(atPath: root.appendingPathComponent("go.mod").path) else {
    fputs("run from the Soda OS repository root\n", stderr)
    exit(1)
}

let assets = [
    Asset(source: "assets/branding/source/soda-symbol-small.svg", output: "assets/branding/icons/hicolor/16x16/apps/soda-os.png", width: 16, height: 16),
    Asset(source: "assets/branding/source/soda-symbol-small.svg", output: "assets/branding/icons/hicolor/24x24/apps/soda-os.png", width: 24, height: 24),
    Asset(source: "assets/branding/source/soda-symbol-small.svg", output: "assets/branding/icons/hicolor/32x32/apps/soda-os.png", width: 32, height: 32),
    Asset(source: "assets/branding/source/soda-symbol.svg", output: "assets/branding/icons/hicolor/48x48/apps/soda-os.png", width: 48, height: 48),
    Asset(source: "assets/branding/source/soda-symbol.svg", output: "assets/branding/icons/hicolor/64x64/apps/soda-os.png", width: 64, height: 64),
    Asset(source: "assets/branding/source/soda-symbol.svg", output: "assets/branding/icons/hicolor/128x128/apps/soda-os.png", width: 128, height: 128),
    Asset(source: "assets/branding/source/soda-symbol.svg", output: "assets/branding/icons/hicolor/256x256/apps/soda-os.png", width: 256, height: 256),
    Asset(source: "assets/branding/source/soda-symbol.svg", output: "assets/branding/icons/hicolor/512x512/apps/soda-os.png", width: 512, height: 512),
    Asset(source: "assets/branding/source/soda-symbol-small.svg", output: "assets/branding/web/favicon-16.png", width: 16, height: 16),
    Asset(source: "assets/branding/source/soda-symbol-small.svg", output: "assets/branding/web/favicon-32.png", width: 32, height: 32),
    Asset(source: "assets/branding/source/soda-symbol.svg", output: "assets/branding/web/apple-touch-icon.png", width: 180, height: 180),
]

func render(_ asset: Asset) throws {
    let source = root.appendingPathComponent(asset.source)
    let output = root.appendingPathComponent(asset.output)
    guard let image = NSImage(contentsOf: source) else {
        throw NSError(domain: "branding", code: 1, userInfo: [NSLocalizedDescriptionKey: "cannot read \(asset.source)"])
    }
    guard let tiff = image.tiffRepresentation else {
        throw NSError(domain: "branding", code: 2, userInfo: [NSLocalizedDescriptionKey: "cannot rasterize \(asset.source)"])
    }
    try FileManager.default.createDirectory(at: output.deletingLastPathComponent(), withIntermediateDirectories: true)
    let intermediate = FileManager.default.temporaryDirectory.appendingPathComponent("soda-branding-\(UUID().uuidString).tiff")
    defer { try? FileManager.default.removeItem(at: intermediate) }
    try tiff.write(to: intermediate, options: .atomic)
    let process = Process()
    process.executableURL = URL(fileURLWithPath: "/usr/bin/env")
    process.arguments = [
        "magick", "\(intermediate.path)[0]", "-resize", "\(asset.width)x\(asset.height)",
        "-background", "none", "-gravity", "center", "-extent", "\(asset.width)x\(asset.height)",
        "-depth", "8", "-strip", "-define", "png:exclude-chunk=date,time", output.path,
    ]
    try process.run()
    process.waitUntilExit()
    guard process.terminationStatus == 0 else {
        throw NSError(domain: "branding", code: 3, userInfo: [NSLocalizedDescriptionKey: "cannot encode \(asset.output)"])
    }
}

func copy(_ source: String, _ destination: String) throws {
    let from = root.appendingPathComponent(source)
    let to = root.appendingPathComponent(destination)
    try FileManager.default.createDirectory(at: to.deletingLastPathComponent(), withIntermediateDirectories: true)
    try? FileManager.default.removeItem(at: to)
    try FileManager.default.copyItem(at: from, to: to)
}

do {
    for asset in assets {
        try render(asset)
    }
    for (source, output) in [
        ("assets/branding/source/soda-logo-horizontal.svg", "cockpit/internal/web/static/soda-logo.svg"),
        ("assets/branding/source/soda-logo-horizontal-dark.svg", "cockpit/internal/web/static/soda-logo-dark.svg"),
        ("assets/branding/source/soda-symbol-small.svg", "cockpit/internal/web/static/favicon.svg"),
        ("assets/branding/web/favicon-32.png", "cockpit/internal/web/static/favicon-32.png"),
        ("assets/branding/web/apple-touch-icon.png", "cockpit/internal/web/static/apple-touch-icon.png"),
    ] {
        try copy(source, output)
    }
} catch {
    fputs("branding render failed: \(error.localizedDescription)\n", stderr)
    exit(1)
}
