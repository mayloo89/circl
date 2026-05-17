// Generate the full raster branding kit from the SVG masters.
//
// Sources (must exist in public/branding/):
//   mark.svg          — symbol only, 1:1, transparent background
//   logo-dark.svg     — full lockup with bone wordmark, 16:9, transparent
//   logo-light.svg    — full lockup with ink wordmark, 16:9, transparent
//
// Outputs (overwrites in public/branding/):
//   favicon-48.png             — 48×48 PNG for the .ico bundle
//   favicon.ico                — multi-resolution (16+32+48)
//   icon-maskable-192.png      — 192×192 PWA maskable icon (ink bg)
//   icon-maskable-512.png      — 512×512 PWA maskable icon (ink bg)
//   logo-dark-2048.png         — lockup raster master, transparent
//   logo-light-2048.png        — lockup raster master, transparent
//   logo-email-dark.png        — 1200×240 (retina @2x of 600×120), transparent
//   logo-email-light.png       — 1200×240, transparent
//   og-image.png               — 1200×630 Open Graph, logo-dark on ink
//   twitter-card.png           — 1200×675 Twitter/X large card, logo-dark on ink
//   social-cover.png           — 1500×500 Twitter/X header, logo-dark on ink
//
// Re-run any time the SVG masters change:
//   npm run gen:favicons

import { readFile, writeFile, mkdir } from "node:fs/promises"
import { fileURLToPath } from "node:url"
import path from "node:path"

import sharp from "sharp"
import pngToIco from "png-to-ico"

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const brandingDir = path.resolve(__dirname, "..", "public", "branding")

// Maskable icon background — ink (dark near-black), the app's primary surface
// color. Picked over magenta because the mark's lens is itself magenta and
// would disappear against a same-colour background.
const MASKABLE_BG = { r: 0x0a, g: 0x0a, b: 0x0f, alpha: 1 }

// Maskable PWA icons reserve a "safe zone" of the inner ~80% for the mark.
// Launchers can crop the outer ~10% on each side (circle, squircle, etc.),
// so the mark itself sits at 60% of the canvas — well inside any crop.
const MASKABLE_MARK_RATIO = 0.6

async function renderSvgToPng(svgBuffer, size) {
  // density bumps the rasterizer DPI so the SVG re-rasterizes at the target
  // resolution instead of upscaling a low-res rendering.
  return sharp(svgBuffer, { density: 384 })
    .resize(size, size, { fit: "contain", background: { r: 0, g: 0, b: 0, alpha: 0 } })
    .png()
    .toBuffer()
}

// renderSvgToRect renders an SVG into a rectangular target (width × height)
// preserving aspect ratio with `contain`. Used for non-square assets like the
// 16:9 lockups.
async function renderSvgToRect(svgBuffer, width, height) {
  return sharp(svgBuffer, { density: 384 })
    .resize(width, height, { fit: "contain", background: { r: 0, g: 0, b: 0, alpha: 0 } })
    .png()
    .toBuffer()
}

async function generateMaskable(svgBuffer, size, outPath) {
  const inner = Math.round(size * MASKABLE_MARK_RATIO)
  const innerPng = await renderSvgToPng(svgBuffer, inner)

  await sharp({
    create: {
      width: size,
      height: size,
      channels: 4,
      background: MASKABLE_BG,
    },
  })
    .composite([{ input: innerPng, gravity: "center" }])
    .png()
    .toFile(outPath)

  console.log(`  ✓ ${path.basename(outPath)} (${size}×${size}, mark @ ${inner}×${inner})`)
}

async function generateFavicon48(svgBuffer, outPath) {
  await writeFile(outPath, await renderSvgToPng(svgBuffer, 48))
  console.log(`  ✓ favicon-48.png (48×48)`)
}

async function generateFaviconIco(svgBuffer, outPath) {
  // Bundle 16+32+48 into a single .ico — every modern OS picks the best one.
  const sizes = [16, 32, 48]
  const pngs = await Promise.all(sizes.map((s) => renderSvgToPng(svgBuffer, s)))
  const ico = await pngToIco(pngs)
  await writeFile(outPath, ico)
  console.log(`  ✓ favicon.ico (${sizes.join("+")})`)
}

// generateLockupRaster takes a 16:9 SVG lockup and writes a transparent PNG
// at the target width (height is derived to preserve aspect).
async function generateLockupRaster(svgBuffer, width, outPath) {
  const height = Math.round(width * 9 / 16)
  const png = await renderSvgToRect(svgBuffer, width, height)
  await writeFile(outPath, png)
  console.log(`  ✓ ${path.basename(outPath)} (${width}×${height})`)
}

// generateSocialComposite centers a 16:9 SVG lockup over a solid ink
// background at the target canvas size. The lockup is sized to occupy
// `logoWidthRatio` of the canvas width so there's breathing room around it.
async function generateSocialComposite(svgBuffer, canvasW, canvasH, logoWidthRatio, outPath) {
  const logoW = Math.round(canvasW * logoWidthRatio)
  const logoH = Math.round(logoW * 9 / 16)
  const logoPng = await renderSvgToRect(svgBuffer, logoW, logoH)

  await sharp({
    create: {
      width: canvasW,
      height: canvasH,
      channels: 4,
      background: MASKABLE_BG, // ink — same dark surface as the app
    },
  })
    .composite([{ input: logoPng, gravity: "center" }])
    .png()
    .toFile(outPath)

  console.log(`  ✓ ${path.basename(outPath)} (${canvasW}×${canvasH}, logo @ ${logoW}×${logoH})`)
}

async function main() {
  console.log("Generating branding raster kit from SVG masters...")

  await mkdir(brandingDir, { recursive: true })
  const markBuffer = await readFile(path.join(brandingDir, "mark.svg"))
  const logoDarkBuffer = await readFile(path.join(brandingDir, "logo-dark.svg"))
  const logoLightBuffer = await readFile(path.join(brandingDir, "logo-light.svg"))

  // --- mark-derived ---
  await generateFavicon48(markBuffer, path.join(brandingDir, "favicon-48.png"))
  await generateFaviconIco(markBuffer, path.join(brandingDir, "favicon.ico"))
  await generateMaskable(markBuffer, 192, path.join(brandingDir, "icon-maskable-192.png"))
  await generateMaskable(markBuffer, 512, path.join(brandingDir, "icon-maskable-512.png"))

  // --- logo-dark-derived (raster + composites) ---
  await generateLockupRaster(logoDarkBuffer, 2048, path.join(brandingDir, "logo-dark-2048.png"))
  await generateLockupRaster(logoDarkBuffer, 1200, path.join(brandingDir, "logo-email-dark.png"))
  await generateSocialComposite(logoDarkBuffer, 1200, 630, 0.55, path.join(brandingDir, "og-image.png"))
  await generateSocialComposite(logoDarkBuffer, 1200, 675, 0.55, path.join(brandingDir, "twitter-card.png"))
  await generateSocialComposite(logoDarkBuffer, 1500, 500, 0.4, path.join(brandingDir, "social-cover.png"))

  // --- logo-light-derived ---
  await generateLockupRaster(logoLightBuffer, 2048, path.join(brandingDir, "logo-light-2048.png"))
  await generateLockupRaster(logoLightBuffer, 1200, path.join(brandingDir, "logo-email-light.png"))

  console.log("Done.")
}

main().catch((err) => {
  console.error(err)
  process.exit(1)
})
