import AppKit
import Foundation

struct Asset {
    let source: String
    let output: String
    let width: Int
    let height: Int
}

let root = URL(fileURLWithPath: FileManager.default.currentDirectoryPath)
guard FileManager.default.fileExists(atPath: root.appendingPathComponent("Cargo.toml").path) else {
    fputs("run from the Soda OS repository root\n", stderr)
    exit(1)
}

let assets = [
    Asset(source: "assets/branding/source/soda-logo-sidebar.svg", output: "assets/branding/installer/sidebar-logo.png", width: 114, height: 36),
    Asset(source: "assets/branding/source/sidebar-bg.svg", output: "assets/branding/installer/sidebar-bg.png", width: 240, height: 1200),
    Asset(source: "assets/branding/source/topbar-bg.svg", output: "assets/branding/installer/topbar-bg.png", width: 1920, height: 132),
    Asset(source: "assets/branding/source/grub-background.svg", output: "assets/branding/installer/grub-background.png", width: 1024, height: 768),
    Asset(source: "assets/branding/source/soda-symbol.svg", output: "assets/branding/installer/soda-symbol-256.png", width: 256, height: 256),
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

func sha256(_ path: String) throws -> String {
    let process = Process()
    let output = Pipe()
    process.executableURL = URL(fileURLWithPath: "/usr/bin/shasum")
    process.arguments = ["-a", "256", root.appendingPathComponent(path).path]
    process.standardOutput = output
    try process.run()
    process.waitUntilExit()
    guard process.terminationStatus == 0,
          let result = String(data: output.fileHandleForReading.readDataToEndOfFile(), encoding: .utf8)?.split(separator: " ").first
    else {
        throw NSError(domain: "branding", code: 5, userInfo: [NSLocalizedDescriptionKey: "cannot hash \(path)"])
    }
    return String(result)
}

do {
    for asset in assets {
        try render(asset)
    }
    for (source, output) in [
        ("assets/branding/source/soda-logo-horizontal.svg", "cockpit/internal/server/static/soda-logo.svg"),
        ("assets/branding/source/soda-logo-horizontal-dark.svg", "cockpit/internal/server/static/soda-logo-dark.svg"),
        ("assets/branding/source/soda-symbol-small.svg", "cockpit/internal/server/static/favicon.svg"),
        ("assets/branding/web/favicon-32.png", "cockpit/internal/server/static/favicon-32.png"),
        ("assets/branding/web/apple-touch-icon.png", "cockpit/internal/server/static/apple-touch-icon.png"),
        ("assets/branding/installer/sidebar-bg.png", "packaging/anaconda/product/usr/share/anaconda/pixmaps/soda-sidebar-bg.png"),
        ("assets/branding/installer/sidebar-logo.png", "packaging/anaconda/product/usr/share/anaconda/pixmaps/soda-sidebar-logo.png"),
        ("assets/branding/installer/soda-symbol-256.png", "packaging/anaconda/product/usr/share/anaconda/pixmaps/soda-symbol.png"),
        ("assets/branding/installer/topbar-bg.png", "packaging/anaconda/product/usr/share/anaconda/pixmaps/soda-topbar-bg.png"),
    ] {
        try copy(source, output)
    }
    let entries = try assets.map { asset in
        """
        [[asset]]
        source = "\(asset.source)"
        output = "\(asset.output)"
        width = \(asset.width)
        height = \(asset.height)
        sha256 = "\(try sha256(asset.output))"
        """
    }.joined(separator: "\n")
    try ("schema_version = 1\n\n" + entries + "\n").write(
        to: root.appendingPathComponent("packaging/anaconda/branding.toml"),
        atomically: true,
        encoding: .utf8
    )
} catch {
    fputs("branding render failed: \(error.localizedDescription)\n", stderr)
    exit(1)
}
