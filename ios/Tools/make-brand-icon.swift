#!/usr/bin/env swift
//
// make-brand-icon.swift — render a brand's 1024pt App Store icon.
//
//   swift ios/Tools/make-brand-icon.swift --hex EA580C \
//     --out ios/Brands/RedFox/Assets.xcassets/AppIcon.appiconset/icon-1024.png
//
// Writes an OPAQUE RGB PNG. That is the point of using CoreGraphics rather than
// rsvg-convert: App Store Connect rejects an app icon that carries an alpha channel
// at all, even one that is fully opaque, and most SVG rasterisers emit RGBA
// unconditionally. `.noneSkipLast` here guarantees 3 samples per pixel.
//
// The mark drawn is Red Fox Signals' — signal arcs broadcasting from a node, matching
// frontend/src/brand/redfox.ts so the phone icon and the web logo are the same drawing.
// A new brand adds its own draw function beside `drawRedFoxMark`.

import CoreGraphics
import Foundation
import ImageIO
import UniformTypeIdentifiers

func arg(_ name: String) -> String? {
    guard let i = CommandLine.arguments.firstIndex(of: "--\(name)"),
          i + 1 < CommandLine.arguments.count else { return nil }
    return CommandLine.arguments[i + 1]
}

guard let hex = arg("hex"), let out = arg("out") else {
    FileHandle.standardError.write("usage: make-brand-icon.swift --hex RRGGBB --out path.png\n".data(using: .utf8)!)
    exit(2)
}

func channels(_ hex: String) -> (CGFloat, CGFloat, CGFloat) {
    let h = hex.replacingOccurrences(of: "#", with: "")
    let v = UInt32(h, radix: 16) ?? 0
    return (CGFloat((v >> 16) & 0xFF) / 255, CGFloat((v >> 8) & 0xFF) / 255, CGFloat(v & 0xFF) / 255)
}

let size = 1024
let (r, g, b) = channels(hex)

guard let ctx = CGContext(
    data: nil, width: size, height: size, bitsPerComponent: 8, bytesPerRow: 0,
    space: CGColorSpaceCreateDeviceRGB(),
    bitmapInfo: CGImageAlphaInfo.noneSkipLast.rawValue
) else { fatalError("could not create a bitmap context") }

// Solid brand ground — no rounded corners: iOS applies the mask itself, and a
// pre-rounded icon shows dark corners on the App Store.
ctx.setFillColor(red: r, green: g, blue: b, alpha: 1)
ctx.fill(CGRect(x: 0, y: 0, width: size, height: size))

// Centre on the MARK'S OWN bounding box, not the 24×24 viewBox. The mark does not
// fill its viewBox (it spans x 3.5–20.5, y 7–20.5), so centring the box instead of the
// drawing leaves the glyph visibly low and right of centre on the icon.
let strokeWidth: CGFloat = 1.75
let pad = strokeWidth / 2 // a stroked path extends half a line width past its path bounds
// Node circle (centre 6,18 r 1.6, filled — no stroke pad) plus the outermost arc
// (centre 6,18 radius 15, a quarter turn from (21,18) up to (6,3)).
let markMinX: CGFloat = 6 - 1.6, markMaxX: CGFloat = 21 + pad
let markMinY: CGFloat = 3 - pad, markMaxY: CGFloat = 18 + 1.6
let markW = markMaxX - markMinX
let markH = markMaxY - markMinY

// Fit the longer axis to 62% of the canvas, then centre.
let scale = (CGFloat(size) * 0.62) / max(markW, markH)
let midX = (markMinX + markMaxX) / 2
let midY = (markMinY + markMaxY) / 2
let half = CGFloat(size) / 2
// Flip Y so the SVG coordinates from redfox.ts can be transcribed literally.
ctx.translateBy(x: half - midX * scale, y: half + midY * scale)
ctx.scaleBy(x: scale, y: -scale)

func drawRedFoxMark(_ ctx: CGContext) {
    // The transmitting node — filled, so it reads as solid at small sizes.
    ctx.setFillColor(red: 1, green: 1, blue: 1, alpha: 1)
    ctx.addArc(center: CGPoint(x: 6, y: 18), radius: 1.6,
               startAngle: 0, endAngle: 2 * .pi, clockwise: false)
    ctx.fillPath()

    // Three arcs at increasing radius — the signal going out. Each is a quarter turn
    // from due-east of the node round to due-north of it.
    ctx.setStrokeColor(red: 1, green: 1, blue: 1, alpha: 1)
    ctx.setLineWidth(strokeWidth)
    ctx.setLineCap(.round)
    for radius in [CGFloat(5), 10, 15] {
        ctx.beginPath()
        ctx.addArc(center: CGPoint(x: 6, y: 18), radius: radius,
                   startAngle: 0, endAngle: -.pi / 2, clockwise: true)
        ctx.strokePath()
    }
}

drawRedFoxMark(ctx)

guard let image = ctx.makeImage() else { fatalError("could not render the image") }
let url = URL(fileURLWithPath: out)
guard let dest = CGImageDestinationCreateWithURL(url as CFURL, UTType.png.identifier as CFString, 1, nil) else {
    fatalError("could not open \(out) for writing")
}
CGImageDestinationAddImage(dest, image, nil)
guard CGImageDestinationFinalize(dest) else { fatalError("could not write \(out)") }
print("wrote \(out) — \(size)×\(size), opaque")
