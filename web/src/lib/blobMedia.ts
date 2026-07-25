// Shared helpers for sniffing/rendering/downloading BLOB media values.
//
// Two encodings show up for the "same" bytes elsewhere in the app:
//  - table row values arrive as hex-encoded strings (see App.tsx blob cell rendering)
//  - the seed generator sample endpoint JSON-marshals []byte as base64 (Go's
//    encoding/json default for byte slices)
//
// Everything below normalizes to a plain Uint8Array first, then works from
// there, so the sniffing/data-URI/download logic is shared regardless of the
// original encoding.

export type BlobMediaType = 'png' | 'svg' | 'wav' | 'unknown';

/** Decode a hex-encoded string (e.g. "89504e47...") into raw bytes. */
export function hexToBytes(hex: string): Uint8Array {
  const clean = hex.length % 2 === 0 ? hex : hex.slice(0, -1);
  const out = new Uint8Array(clean.length / 2);
  for (let i = 0; i < out.length; i++) {
    out[i] = parseInt(clean.substring(i * 2, i * 2 + 2), 16) || 0;
  }
  return out;
}

/** Decode a standard base64 string into raw bytes. */
export function base64ToBytes(b64: string): Uint8Array {
  const binary = atob(b64);
  const out = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) {
    out[i] = binary.charCodeAt(i);
  }
  return out;
}

/**
 * Convert raw bytes to a base64 string. Iterates bytes directly (via
 * String.fromCharCode on each byte) rather than the unescape/encodeURIComponent
 * trick, which mangles multi-byte sequences — this is binary-safe.
 */
export function bytesToBase64(bytes: Uint8Array): string {
  let binary = '';
  const chunkSize = 0x8000; // avoid call-stack blowups on large arrays
  for (let i = 0; i < bytes.length; i += chunkSize) {
    const chunk = bytes.subarray(i, i + chunkSize);
    binary += String.fromCharCode(...chunk);
  }
  return btoa(binary);
}

/** Convert a hex-encoded string directly to base64 (hex -> bytes -> base64). */
export function hexToBase64(hex: string): string {
  return bytesToBase64(hexToBytes(hex));
}

function looksLikeSvgText(bytes: Uint8Array): boolean {
  // Only need to inspect a small leading window; SVG/XML prolog + whitespace
  // is short-lived before the actual "<svg" tag appears.
  const head = bytes.subarray(0, Math.min(bytes.length, 512));
  let text: string;
  try {
    text = new TextDecoder('utf-8', { fatal: false }).decode(head);
  } catch {
    return false;
  }
  const trimmed = text.trimStart().toLowerCase();
  if (trimmed.startsWith('<svg')) return true;
  // Allow an XML prolog / doctype before the actual <svg ...> tag.
  if (trimmed.startsWith('<?xml') || trimmed.startsWith('<!doctype')) {
    return /<svg[\s>]/.test(trimmed);
  }
  return false;
}

/** Sniff the media type of raw bytes by magic number / leading text. */
export function sniffBytes(bytes: Uint8Array): BlobMediaType {
  if (
    bytes.length >= 4 &&
    bytes[0] === 0x89 &&
    bytes[1] === 0x50 &&
    bytes[2] === 0x4e &&
    bytes[3] === 0x47
  ) {
    return 'png';
  }
  if (
    bytes.length >= 12 &&
    bytes[0] === 0x52 && // R
    bytes[1] === 0x49 && // I
    bytes[2] === 0x46 && // F
    bytes[3] === 0x46 && // F
    bytes[8] === 0x57 && // W
    bytes[9] === 0x41 && // A
    bytes[10] === 0x56 && // V
    bytes[11] === 0x45 // E
  ) {
    return 'wav';
  }
  if (looksLikeSvgText(bytes)) return 'svg';
  return 'unknown';
}

/** Sniff a hex-encoded string's media type. */
export function sniffHex(hex: string): BlobMediaType {
  return sniffBytes(hexToBytes(hex));
}

/** Sniff a base64-encoded string's media type. */
export function sniffBase64(b64: string): BlobMediaType {
  try {
    return sniffBytes(base64ToBytes(b64));
  } catch {
    return 'unknown';
  }
}

const MIME_BY_TYPE: Record<BlobMediaType, string> = {
  png: 'image/png',
  svg: 'image/svg+xml',
  wav: 'audio/wav',
  unknown: 'application/octet-stream',
};

const EXT_BY_TYPE: Record<BlobMediaType, string> = {
  png: 'png',
  svg: 'svg',
  wav: 'wav',
  unknown: 'bin',
};

export function extensionForType(type: BlobMediaType): string {
  return EXT_BY_TYPE[type];
}

/** Build a data: URI from raw bytes and a sniffed/known type. */
export function dataUriFromBytes(bytes: Uint8Array, type: BlobMediaType): string {
  return `data:${MIME_BY_TYPE[type]};base64,${bytesToBase64(bytes)}`;
}

/** Build a data: URI from a hex-encoded string. */
export function dataUriFromHex(hex: string, type: BlobMediaType): string {
  return dataUriFromBytes(hexToBytes(hex), type);
}

/** Build a data: URI from a base64-encoded string. */
export function dataUriFromBase64(b64: string, type: BlobMediaType): string {
  return `data:${MIME_BY_TYPE[type]};base64,${b64}`;
}

/** Trigger a browser download of raw bytes as a file. */
export function downloadBytes(bytes: Uint8Array, filenameBase: string, type: BlobMediaType): void {
  const blob = new Blob([bytes.slice().buffer], { type: MIME_BY_TYPE[type] });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = `${filenameBase}.${extensionForType(type)}`;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}

/** Trigger a browser download given a hex-encoded string. */
export function downloadHex(hex: string, filenameBase: string, type: BlobMediaType): void {
  downloadBytes(hexToBytes(hex), filenameBase, type);
}
