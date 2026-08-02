import { describe, expect, it } from 'vitest';
import { applyTransform } from './transforms';

describe('applyTransform', () => {
  it('trims whitespace', async () => {
    expect(await applyTransform('trim', '  hi  ', {})).toBe('hi');
  });

  it('uppercases and lowercases', async () => {
    expect(await applyTransform('upper', 'hello', {})).toBe('HELLO');
    expect(await applyTransform('lower', 'HELLO', {})).toBe('hello');
  });

  it('title-cases', async () => {
    expect(await applyTransform('title', 'hello world', {})).toBe('Hello World');
  });

  it('regex replaces', async () => {
    expect(await applyTransform('regex_replace', 'a1b2c3', { pattern: '[0-9]', replacement: '' })).toBe('abc');
  });

  it('converts NULL to empty string', async () => {
    expect(await applyTransform('null_to_empty', null, {})).toBe('');
  });

  it('converts empty string to NULL', async () => {
    expect(await applyTransform('empty_to_null', '', {})).toBeNull();
    expect(await applyTransform('empty_to_null', 'x', {})).toBe('x');
  });

  it('leaves NULL untouched for other transforms', async () => {
    expect(await applyTransform('upper', null, {})).toBeNull();
  });

  it('reformats dates', async () => {
    expect(await applyTransform('date_format', '2024-01-15', { outputFormat: 'dd/MM/yyyy' })).toBe('15/01/2024');
  });

  it('leaves un-parseable dates unchanged', async () => {
    expect(await applyTransform('date_format', 'not-a-date', { outputFormat: 'yyyy' })).toBe('not-a-date');
  });

  it('base64 encodes and decodes round-trip', async () => {
    const encoded = await applyTransform('base64_encode', 'hello world', {});
    expect(await applyTransform('base64_decode', encoded, {})).toBe('hello world');
  });

  it('url encodes and decodes round-trip', async () => {
    const encoded = await applyTransform('url_encode', 'a b&c', {});
    expect(encoded).toBe('a%20b%26c');
    expect(await applyTransform('url_decode', encoded, {})).toBe('a b&c');
  });

  it('hashes with sha256 deterministically', async () => {
    const h1 = await applyTransform('hash_sha256', 'hello', {});
    const h2 = await applyTransform('hash_sha256', 'hello', {});
    expect(h1).toBe(h2);
    expect(h1).toMatch(/^[0-9a-f]{64}$/);
  });

  it('hashes with md5 deterministically and matches known vector', async () => {
    // MD5("") == d41d8cd98f00b204e9800998ecf8427e
    expect(await applyTransform('hash_md5', '', {})).toBe('d41d8cd98f00b204e9800998ecf8427e');
    // MD5("abc") == 900150983cd24fb0d6963f7d28e17f72
    expect(await applyTransform('hash_md5', 'abc', {})).toBe('900150983cd24fb0d6963f7d28e17f72');
  });

  it('generates distinct UUIDs per call', async () => {
    const a = await applyTransform('uuid', null, {});
    const b = await applyTransform('uuid', null, {});
    expect(a).not.toBe(b);
    expect(a).toMatch(/^[0-9a-f-]{36}$/);
  });

  it('throws for template (must be resolved server-side)', async () => {
    await expect(applyTransform('template', 'x', { template: '{{.Value}}' })).rejects.toThrow();
  });
});
