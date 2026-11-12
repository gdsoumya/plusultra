# Brand

`logo.svg` is the source of truth for the mark: a horizontal dumbbell in the accent `#B4F23C` on
`#0B0C0F`, no text. The adaptive layers use it as drawn (inside the centre 66% that Android's icon
masks keep); the standalone icon and favicon scale it up to fill the square.

`splash.svg` is the splash: the same dumbbell with the "PlusUltra" wordmark below it, both in the
accent, on transparent so the splash background colour shows. The wordmark is Roboto Bold (Apache 2.0)
converted to a path with opentype.js, so nothing needs a font to render it.

`./render.sh` regenerates every PNG in `app/assets` from the two SVGs (icon, adaptive
foreground/background/monochrome, splash, favicon, notification icon) through
`npx @resvg/resvg-js-cli`; nothing to install. Run it after editing an SVG and commit the PNGs.
`assets.test.ts` checks their sizes.
