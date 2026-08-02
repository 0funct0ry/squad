// Client-side value transforms for the Data tab's "Transform column" /
// "Transform selected rows" feature. All functions are synchronous except
// "hash" (uses SubtleCrypto). "template" is handled server-side (see
// App.tsx's call to POST /api/transform/template) since it needs Go's
// text/template + internal/seed's generator FuncMap.

export type TransformType =
  | 'regex_replace'
  | 'trim'
  | 'upper'
  | 'lower'
  | 'title'
  | 'null_to_empty'
  | 'empty_to_null'
  | 'date_format'
  | 'base64_encode'
  | 'base64_decode'
  | 'url_encode'
  | 'url_decode'
  | 'hash_sha256'
  | 'hash_md5'
  | 'uuid'
  | 'template';

export interface TransformParams {
  pattern?: string; // regex_replace
  replacement?: string; // regex_replace
  inputFormat?: string; // date_format (informational only — we rely on Date parsing)
  outputFormat?: string; // date_format
  template?: string; // template (rendered server-side)
}

function titleCase(s: string): string {
  return s.replace(/\w\S*/g, (w) => w[0].toUpperCase() + w.slice(1).toLowerCase());
}

function formatDate(d: Date, fmt: string): string {
  const pad = (n: number, len = 2) => String(n).padStart(len, '0');
  return fmt
    .replace(/yyyy/g, String(d.getFullYear()))
    .replace(/MM/g, pad(d.getMonth() + 1))
    .replace(/dd/g, pad(d.getDate()))
    .replace(/HH/g, pad(d.getHours()))
    .replace(/mm/g, pad(d.getMinutes()))
    .replace(/ss/g, pad(d.getSeconds()));
}

async function sha256Hex(input: string): Promise<string> {
  const data = new TextEncoder().encode(input);
  const digest = await crypto.subtle.digest('SHA-256', data);
  return Array.from(new Uint8Array(digest))
    .map((b) => b.toString(16).padStart(2, '0'))
    .join('');
}

// Minimal, dependency-free MD5 (RFC 1321) — non-cryptographic-use utility
// hashing only (dedup keys, cache-busting, etc.), never for security.
function md5Hex(input: string): string {
  function rotl(x: number, c: number) {
    return (x << c) | (x >>> (32 - c));
  }
  function toHexLE(n: number) {
    let hex = '';
    for (let i = 0; i < 4; i++) {
      hex += ((n >> (i * 8)) & 0xff).toString(16).padStart(2, '0');
    }
    return hex;
  }
  const s = [
    7, 12, 17, 22, 7, 12, 17, 22, 7, 12, 17, 22, 7, 12, 17, 22, 5, 9, 14, 20, 5, 9, 14, 20, 5, 9, 14, 20, 5, 9, 14, 20,
    4, 11, 16, 23, 4, 11, 16, 23, 4, 11, 16, 23, 4, 11, 16, 23, 6, 10, 15, 21, 6, 10, 15, 21, 6, 10, 15, 21, 6, 10, 15,
    21,
  ];
  const K = new Array(64);
  for (let i = 0; i < 64; i++) K[i] = Math.floor(Math.abs(Math.sin(i + 1)) * 2 ** 32);

  const bytes = new TextEncoder().encode(input);
  const bitLen = bytes.length * 8;
  const withOne = new Uint8Array(((bytes.length + 8) >> 6) * 64 + 64);
  withOne.set(bytes);
  withOne[bytes.length] = 0x80;
  const view = new DataView(withOne.buffer);
  view.setUint32(withOne.length - 8, bitLen >>> 0, true);
  view.setUint32(withOne.length - 4, Math.floor(bitLen / 2 ** 32), true);

  let a0 = 0x67452301,
    b0 = 0xefcdab89,
    c0 = 0x98badcfe,
    d0 = 0x10325476;

  for (let chunkStart = 0; chunkStart < withOne.length; chunkStart += 64) {
    const M = new Array(16);
    for (let i = 0; i < 16; i++) M[i] = view.getUint32(chunkStart + i * 4, true);

    let A = a0,
      B = b0,
      C = c0,
      D = d0;
    for (let i = 0; i < 64; i++) {
      let F, g;
      if (i < 16) {
        F = (B & C) | (~B & D);
        g = i;
      } else if (i < 32) {
        F = (D & B) | (~D & C);
        g = (5 * i + 1) % 16;
      } else if (i < 48) {
        F = B ^ C ^ D;
        g = (3 * i + 5) % 16;
      } else {
        F = C ^ (B | ~D);
        g = (7 * i) % 16;
      }
      F = (F + A + K[i] + M[g]) | 0;
      A = D;
      D = C;
      C = B;
      B = (B + rotl(F, s[i])) | 0;
    }
    a0 = (a0 + A) | 0;
    b0 = (b0 + B) | 0;
    c0 = (c0 + C) | 0;
    d0 = (d0 + D) | 0;
  }

  return toHexLE(a0) + toHexLE(b0) + toHexLE(c0) + toHexLE(d0);
}

function genUUID(): string {
  if (typeof crypto.randomUUID === 'function') return crypto.randomUUID();
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (ch) => {
    const r = (Math.random() * 16) | 0;
    const v = ch === 'x' ? r : (r & 0x3) | 0x8;
    return v.toString(16);
  });
}

export const TRANSFORM_LABELS: Record<TransformType, string> = {
  regex_replace: 'Regex replace',
  trim: 'Trim whitespace',
  upper: 'UPPERCASE',
  lower: 'lowercase',
  title: 'Title Case',
  null_to_empty: 'NULL → empty string',
  empty_to_null: 'Empty string → NULL',
  date_format: 'Reformat date',
  base64_encode: 'Base64 encode',
  base64_decode: 'Base64 decode',
  url_encode: 'URL encode',
  url_decode: 'URL decode',
  hash_sha256: 'Hash (SHA-256)',
  hash_md5: 'Hash (MD5, non-cryptographic use)',
  uuid: 'Generate UUID (per row)',
  template: 'Apply custom Go template',
};

/** True for transforms that need server round-trip (template) or async browser APIs (hash). */
export function isAsyncTransform(type: TransformType): boolean {
  return type === 'hash_sha256' || type === 'hash_md5' || type === 'template';
}

/**
 * Apply a client-side transform to a single value. Returns the new value, or
 * throws/returns the original string unchanged on inputs the transform
 * doesn't apply to (e.g. base64_decode on invalid base64). "template" always
 * throws — callers must resolve it via the /api/transform/template endpoint.
 */
export async function applyTransform(
  type: TransformType,
  value: any,
  params: TransformParams
): Promise<string | null> {
  if (type === 'uuid') return genUUID();

  if (value === null || value === undefined) {
    if (type === 'null_to_empty') return '';
    return value ?? null;
  }
  const str = String(value);

  switch (type) {
    case 'regex_replace': {
      try {
        const re = new RegExp(params.pattern || '', 'g');
        return str.replace(re, params.replacement ?? '');
      } catch {
        return str;
      }
    }
    case 'trim':
      return str.trim();
    case 'upper':
      return str.toUpperCase();
    case 'lower':
      return str.toLowerCase();
    case 'title':
      return titleCase(str);
    case 'null_to_empty':
      return str;
    case 'empty_to_null':
      return str === '' ? null : str;
    case 'date_format': {
      const d = new Date(str);
      if (isNaN(d.getTime())) return str;
      return formatDate(d, params.outputFormat || 'yyyy-MM-dd');
    }
    case 'base64_encode':
      try {
        return btoa(unescape(encodeURIComponent(str)));
      } catch {
        return str;
      }
    case 'base64_decode':
      try {
        return decodeURIComponent(escape(atob(str)));
      } catch {
        return str;
      }
    case 'url_encode':
      return encodeURIComponent(str);
    case 'url_decode':
      try {
        return decodeURIComponent(str);
      } catch {
        return str;
      }
    case 'hash_sha256':
      return sha256Hex(str);
    case 'hash_md5':
      return md5Hex(str);
    case 'template':
      throw new Error('template transforms must be resolved via /api/transform/template');
    default:
      return str;
  }
}
