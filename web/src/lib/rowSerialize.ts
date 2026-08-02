// Pure serialization helpers for RowGrid row/selection actions.
// No React dependency — independently unit-testable.

/** Resolve which columns to use for keying (WHERE/UPDATE/DELETE). Falls back to all columns when pkColumns is empty. */
function resolveKeyColumns(columns: string[], pkColumns: string[]): string[] {
  return pkColumns && pkColumns.length > 0 ? pkColumns : columns;
}

/** Double-quote an identifier, doubling embedded double-quotes. */
export function quoteIdentifier(name: string): string {
  return `"${String(name).replace(/"/g, '""')}"`;
}

/**
 * Format a JS value as a SQL literal.
 * null/undefined -> NULL (bare), numbers -> bare, strings -> single-quoted
 * ('' doubled), booleans -> 1/0 (SQLite convention), anything else -> best-effort
 * String() wrapped as a string literal.
 */
export function sqlLiteral(value: any): string {
  if (value === null || value === undefined) return 'NULL';
  if (typeof value === 'number') {
    if (Number.isNaN(value)) return 'NULL';
    return String(value);
  }
  if (typeof value === 'boolean') return value ? '1' : '0';
  if (typeof value === 'string') return `'${value.replace(/'/g, "''")}'`;
  return `'${String(value).replace(/'/g, "''")}'`;
}

function csvField(value: any): string {
  const s = value === null || value === undefined ? '' : String(value);
  if (/[",\n\r]/.test(s)) {
    return `"${s.replace(/"/g, '""')}"`;
  }
  return s;
}

export function toCSV(columns: string[], rows: any[][]): string {
  const lines = [columns.map(csvField).join(',')];
  for (const row of rows) {
    lines.push(row.map(csvField).join(','));
  }
  return lines.join('\r\n');
}

function rowToObject(columns: string[], row: any[]): Record<string, any> {
  const obj: Record<string, any> = {};
  columns.forEach((c, i) => {
    obj[c] = row[i];
  });
  return obj;
}

export function toJSON(columns: string[], rows: any[][]): string {
  const arr = rows.map((row) => rowToObject(columns, row));
  return JSON.stringify(arr, null, 2);
}

export function toJSONL(columns: string[], rows: any[][]): string {
  return rows.map((row) => JSON.stringify(rowToObject(columns, row))).join('\n');
}

function xmlEscape(value: any): string {
  const s = value === null || value === undefined ? '' : String(value);
  return s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&apos;');
}

export function toXML(columns: string[], rows: any[][]): string {
  const rowsXml = rows
    .map((row) => {
      const cols = columns
        .map((c, i) => `    <${c}>${xmlEscape(row[i])}</${c}>`)
        .join('\n');
      return `  <row>\n${cols}\n  </row>`;
    })
    .join('\n');
  return `<rows>\n${rowsXml}\n</rows>`;
}

function yamlScalar(value: any): string {
  if (value === null || value === undefined) return 'null';
  if (typeof value === 'number' || typeof value === 'boolean') return String(value);
  const s = String(value);
  if (s === '') return "''";
  const needsQuote = /^[\s]|[\s]$|[:#\-\[\]{}&*!|>'"%@`]|^(true|false|null|~|\d)/i.test(s) || /[:]\s|[\n]/.test(s);
  if (needsQuote) {
    return `'${s.replace(/'/g, "''")}'`;
  }
  return s;
}

export function toYAML(columns: string[], rows: any[][]): string {
  if (rows.length === 0) return '[]';
  return rows
    .map((row) => {
      const lines = columns.map((c, i) => `${i === 0 ? '- ' : '  '}${c}: ${yamlScalar(row[i])}`);
      return lines.join('\n');
    })
    .join('\n');
}

function mdField(value: any): string {
  const s = value === null || value === undefined ? '' : String(value);
  return s.replace(/\|/g, '\\|').replace(/\n/g, ' ');
}

export function toMarkdownTable(columns: string[], rows: any[][]): string {
  const header = `| ${columns.map(mdField).join(' | ')} |`;
  const sep = `| ${columns.map(() => '---').join(' | ')} |`;
  const dataLines = rows.map((row) => `| ${row.map(mdField).join(' | ')} |`);
  return [header, sep, ...dataLines].join('\n');
}

export function toSQLValues(_columns: string[], rows: any[][]): string {
  return rows
    .map((row) => `(${row.map((v) => sqlLiteral(v)).join(', ')})`)
    .join(', ');
}

export function toInsertSQL(tableName: string, columns: string[], rows: any[][]): string {
  const colList = columns.map(quoteIdentifier).join(', ');
  return rows
    .map((row) => {
      const vals = row.map((v) => sqlLiteral(v)).join(', ');
      return `INSERT INTO ${quoteIdentifier(tableName)} (${colList}) VALUES (${vals});`;
    })
    .join('\n');
}

export function toUpdateSQL(
  tableName: string,
  columns: string[],
  rows: any[][],
  pkColumns: string[]
): string {
  const keyCols = resolveKeyColumns(columns, pkColumns);
  const setCols = pkColumns && pkColumns.length > 0 ? columns.filter((c) => !pkColumns.includes(c)) : columns;

  return rows
    .map((row) => {
      const obj = rowToObject(columns, row);
      const setClause = setCols.map((c) => `${quoteIdentifier(c)} = ${sqlLiteral(obj[c])}`).join(', ');
      const whereClause = keyCols
        .map((c) => `${quoteIdentifier(c)} = ${sqlLiteral(obj[c])}`)
        .join(' AND ');
      return `UPDATE ${quoteIdentifier(tableName)} SET ${setClause} WHERE ${whereClause};`;
    })
    .join('\n');
}

export function toDeleteSQL(
  tableName: string,
  columns: string[],
  rows: any[][],
  pkColumns: string[]
): string {
  const keyCols = resolveKeyColumns(columns, pkColumns);
  return rows
    .map((row) => {
      const obj = rowToObject(columns, row);
      const whereClause = keyCols
        .map((c) => `${quoteIdentifier(c)} = ${sqlLiteral(obj[c])}`)
        .join(' AND ');
      return `DELETE FROM ${quoteIdentifier(tableName)} WHERE ${whereClause};`;
    })
    .join('\n');
}

export function toWhereClause(columns: string[], rows: any[][], pkColumns: string[]): string {
  const keyCols = resolveKeyColumns(columns, pkColumns);

  if (rows.length <= 1) {
    const row = rows[0] ?? [];
    const obj = rowToObject(columns, row);
    return keyCols.map((c) => `${quoteIdentifier(c)} = ${sqlLiteral(obj[c])}`).join(' AND ');
  }

  if (keyCols.length === 1) {
    const col = keyCols[0];
    const colIdx = columns.indexOf(col);
    const vals = rows.map((row) => sqlLiteral(row[colIdx])).join(', ');
    return `${quoteIdentifier(col)} IN (${vals})`;
  }

  const groups = rows.map((row) => {
    const obj = rowToObject(columns, row);
    const cond = keyCols.map((c) => `${quoteIdentifier(c)}=${sqlLiteral(obj[c])}`).join(' AND ');
    return `(${cond})`;
  });
  return groups.join(' OR ');
}

export function toSelectSQL(
  tableName: string,
  columns: string[],
  rows: any[][],
  pkColumns: string[]
): string {
  const where = toWhereClause(columns, rows, pkColumns);
  return `SELECT * FROM ${quoteIdentifier(tableName)} WHERE ${where};`;
}
