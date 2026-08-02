import { describe, it, expect } from 'vitest';
import {
  toCSV,
  toJSON,
  toJSONL,
  toXML,
  toYAML,
  toMarkdownTable,
  toSQLValues,
  toInsertSQL,
  toUpdateSQL,
  toDeleteSQL,
  toWhereClause,
  toSelectSQL,
  quoteIdentifier,
  sqlLiteral,
} from './rowSerialize';

const columns = ['id', 'name', 'notes'];
const singleRow = [[1, 'Alice', null]];
const multiRows = [
  [1, 'Alice', 'hello'],
  [2, 'Bob, Jr.', 'has "quotes"'],
  [3, 'Carol\nNewline', null],
];

describe('quoteIdentifier', () => {
  it('quotes plain identifiers', () => {
    expect(quoteIdentifier('col')).toBe('"col"');
  });
  it('doubles embedded double quotes', () => {
    expect(quoteIdentifier('we"ird')).toBe('"we""ird"');
  });
});

describe('sqlLiteral', () => {
  it('handles null/undefined as bare NULL', () => {
    expect(sqlLiteral(null)).toBe('NULL');
    expect(sqlLiteral(undefined)).toBe('NULL');
  });
  it('handles numbers bare', () => {
    expect(sqlLiteral(42)).toBe('42');
    expect(sqlLiteral(3.14)).toBe('3.14');
  });
  it('handles strings quoted with escaping', () => {
    expect(sqlLiteral("O'Brien")).toBe("'O''Brien'");
    expect(sqlLiteral('plain')).toBe("'plain'");
  });
  it('handles booleans as 1/0', () => {
    expect(sqlLiteral(true)).toBe('1');
    expect(sqlLiteral(false)).toBe('0');
  });
});

describe('toCSV', () => {
  it('serializes single row with header', () => {
    const csv = toCSV(columns, singleRow);
    expect(csv).toBe('id,name,notes\r\n1,Alice,');
  });
  it('quotes fields with commas, quotes, newlines', () => {
    const csv = toCSV(columns, multiRows);
    const lines = csv.split('\r\n');
    expect(lines[0]).toBe('id,name,notes');
    expect(lines[2]).toBe('2,"Bob, Jr.","has ""quotes"""');
    expect(lines[3]).toBe('3,"Carol\nNewline",');
  });
});

describe('toJSON', () => {
  it('produces pretty-printed array of objects', () => {
    const json = toJSON(columns, singleRow);
    const parsed = JSON.parse(json);
    expect(parsed).toEqual([{ id: 1, name: 'Alice', notes: null }]);
    expect(json).toContain('\n');
  });
  it('handles multiple rows', () => {
    const parsed = JSON.parse(toJSON(columns, multiRows));
    expect(parsed).toHaveLength(3);
    expect(parsed[1].name).toBe('Bob, Jr.');
  });
});

describe('toJSONL', () => {
  it('produces newline-delimited json, no pretty print', () => {
    const jsonl = toJSONL(columns, multiRows);
    const lines = jsonl.split('\n');
    expect(lines).toHaveLength(3);
    expect(JSON.parse(lines[0])).toEqual({ id: 1, name: 'Alice', notes: 'hello' });
    expect(lines[0]).not.toContain('\n  ');
  });
});

describe('toXML', () => {
  it('escapes special characters', () => {
    const xml = toXML(['name'], [['<a & "b">']]);
    expect(xml).toContain('&lt;a &amp; &quot;b&quot;&gt;');
    expect(xml).toContain('<rows>');
    expect(xml).toContain('<row>');
    expect(xml).toContain('<name>');
  });
  it('handles null as empty', () => {
    const xml = toXML(columns, singleRow);
    expect(xml).toContain('<notes></notes>');
  });
});

describe('toYAML', () => {
  it('produces a list of mappings', () => {
    const yaml = toYAML(columns, singleRow);
    expect(yaml).toContain('- id: 1');
    expect(yaml).toContain('  name: Alice');
    expect(yaml).toContain('  notes: null');
  });
  it('quotes strings needing quoting', () => {
    const yaml = toYAML(['name'], [['has: colon']]);
    expect(yaml).toContain("'has: colon'");
  });
});

describe('toMarkdownTable', () => {
  it('builds header, separator, and rows, escaping pipes', () => {
    const md = toMarkdownTable(['a', 'b'], [['x|y', 'z']]);
    const lines = md.split('\n');
    expect(lines[0]).toBe('| a | b |');
    expect(lines[1]).toBe('| --- | --- |');
    expect(lines[2]).toBe('| x\\|y | z |');
  });
});

describe('toSQLValues', () => {
  it('produces comma-separated tuples', () => {
    const vals = toSQLValues(['id', 'name'], [[1, 'Alice'], [2, null]]);
    expect(vals).toBe("(1, 'Alice'), (2, NULL)");
  });
});

describe('toInsertSQL', () => {
  it('produces one INSERT per row', () => {
    const sql = toInsertSQL('users', ['id', 'name'], [[1, 'Alice'], [2, "O'Brien"]]);
    const lines = sql.split('\n');
    expect(lines).toHaveLength(2);
    expect(lines[0]).toBe('INSERT INTO "users" ("id", "name") VALUES (1, \'Alice\');');
    expect(lines[1]).toBe('INSERT INTO "users" ("id", "name") VALUES (2, \'O\'\'Brien\');');
  });
});

describe('toUpdateSQL', () => {
  it('excludes pk columns from SET when single pk given', () => {
    const sql = toUpdateSQL('users', ['id', 'name'], [[1, 'Alice']], ['id']);
    expect(sql).toBe('UPDATE "users" SET "name" = \'Alice\' WHERE "id" = 1;');
  });
  it('handles composite pk', () => {
    const sql = toUpdateSQL(
      'orders',
      ['order_id', 'line_no', 'qty'],
      [[1, 2, 5]],
      ['order_id', 'line_no']
    );
    expect(sql).toBe(
      'UPDATE "orders" SET "qty" = 5 WHERE "order_id" = 1 AND "line_no" = 2;'
    );
  });
  it('falls back to all columns for SET and WHERE when pkColumns empty', () => {
    const sql = toUpdateSQL('t', ['a', 'b'], [[1, 2]], []);
    expect(sql).toBe('UPDATE "t" SET "a" = 1, "b" = 2 WHERE "a" = 1 AND "b" = 2;');
  });
});

describe('toDeleteSQL', () => {
  it('keys on pk when given', () => {
    const sql = toDeleteSQL('users', ['id', 'name'], [[1, 'Alice']], ['id']);
    expect(sql).toBe('DELETE FROM "users" WHERE "id" = 1;');
  });
  it('falls back to all columns when pkColumns empty', () => {
    const sql = toDeleteSQL('t', ['a', 'b'], [[1, 2]], []);
    expect(sql).toBe('DELETE FROM "t" WHERE "a" = 1 AND "b" = 2;');
  });
  it('handles NULL values', () => {
    const sql = toDeleteSQL('t', ['a'], [[null]], []);
    expect(sql).toBe('DELETE FROM "t" WHERE "a" = NULL;');
  });
});

describe('toWhereClause', () => {
  it('single row with single pk', () => {
    const where = toWhereClause(['id', 'name'], [[1, 'Alice']], ['id']);
    expect(where).toBe('"id" = 1');
  });
  it('single row composite pk', () => {
    const where = toWhereClause(['a', 'b'], [[1, 2]], ['a', 'b']);
    expect(where).toBe('"a" = 1 AND "b" = 2');
  });
  it('multi-row single pk uses IN', () => {
    const where = toWhereClause(['id', 'name'], [[1, 'A'], [2, 'B'], [3, 'C']], ['id']);
    expect(where).toBe('"id" IN (1, 2, 3)');
  });
  it('multi-row composite pk uses OR groups', () => {
    const where = toWhereClause(['a', 'b'], [[1, 2], [3, 4]], ['a', 'b']);
    expect(where).toBe('("a"=1 AND "b"=2) OR ("a"=3 AND "b"=4)');
  });
  it('falls back to all columns when pkColumns empty', () => {
    const where = toWhereClause(['a', 'b'], [[1, 2]], []);
    expect(where).toBe('"a" = 1 AND "b" = 2');
  });
});

describe('toSelectSQL', () => {
  it('wraps WHERE clause with SELECT * FROM', () => {
    const sql = toSelectSQL('users', ['id'], [[1]], ['id']);
    expect(sql).toBe('SELECT * FROM "users" WHERE "id" = 1;');
  });
});
