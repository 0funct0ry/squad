// Generates public/og.png (1200x630) from an inline SVG using the design
// tokens' colors. Run with: node scripts/gen-og-image.mjs
import sharp from 'sharp';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const out = path.join(__dirname, '..', 'public', 'og.png');

const svg = `
<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="630" viewBox="0 0 1200 630">
  <rect width="1200" height="630" fill="#0a0a0b"/>
  <rect x="0" y="0" width="1200" height="630" fill="url(#grad)" opacity="0.5"/>
  <defs>
    <radialGradient id="grad" cx="20%" cy="10%" r="80%">
      <stop offset="0%" stop-color="#123842"/>
      <stop offset="100%" stop-color="#0a0a0b"/>
    </radialGradient>
  </defs>
  <rect x="90" y="90" width="64" height="64" rx="14" fill="#0a0a0b" stroke="#232326" stroke-width="2"/>
  <path d="M108 116c0-6.6 7.2-12 16-12h24c8.8 0 16 5.4 16 12s-7.2 12-16 12h-24c-8.8 0-16 5.4-16 12s7.2 12 16 12h24c8.8 0 16-5.4 16-12"
        stroke="#22d3ee" stroke-width="6" stroke-linecap="round" fill="none"/>
  <text x="90" y="260" font-family="Helvetica, Arial, sans-serif" font-size="88" font-weight="700" fill="#f5f5f4">squad</text>
  <text x="90" y="330" font-family="Helvetica, Arial, sans-serif" font-size="34" font-weight="400" fill="#a1a1aa">A single-binary, web-based SQLite client.</text>
  <text x="90" y="380" font-family="Helvetica, Arial, sans-serif" font-size="28" font-weight="400" fill="#5a5a60">Read-only by default. Zero install. One binary.</text>
  <text x="90" y="560" font-family="ui-monospace, monospace" font-size="26" font-weight="400" fill="#22d3ee">go install github.com/0funct0ry/squad@latest</text>
</svg>`;

await sharp(Buffer.from(svg)).png().toFile(out);
console.log('wrote', out);
