// Shared column-filter type for the Data tab's filter builder (Phase 2 of
// M10b2). Mirrors internal/db.Filter's JSON shape so it serializes directly
// into the `filters` query param the backend parses.

export type FilterOperator =
  | 'eq'
  | 'neq'
  | 'contains'
  | 'starts_with'
  | 'ends_with'
  | 'gt'
  | 'lt'
  | 'between'
  | 'is_null'
  | 'is_not_null'
  | 'in'
  | 'not_in';

export interface ColumnFilter {
  column: string;
  operator: FilterOperator;
  value?: string | number;
  value2?: string | number;
  values?: (string | number)[];
}

export const OPERATOR_LABELS: Record<FilterOperator, string> = {
  eq: 'Equals',
  neq: 'Not equals',
  contains: 'Contains',
  starts_with: 'Starts with',
  ends_with: 'Ends with',
  gt: 'Greater than',
  lt: 'Less than',
  between: 'Between',
  is_null: 'Is NULL',
  is_not_null: 'Is not NULL',
  in: 'In (comma-separated)',
  not_in: 'Not in (comma-separated)',
};

export function operatorNeedsNoValue(op: FilterOperator): boolean {
  return op === 'is_null' || op === 'is_not_null';
}

export function operatorNeedsTwoValues(op: FilterOperator): boolean {
  return op === 'between';
}

export function operatorNeedsList(op: FilterOperator): boolean {
  return op === 'in' || op === 'not_in';
}

export function describeFilter(f: ColumnFilter): string {
  if (operatorNeedsNoValue(f.operator)) {
    return `${f.column} ${OPERATOR_LABELS[f.operator]}`;
  }
  if (operatorNeedsTwoValues(f.operator)) {
    return `${f.column} between ${f.value} and ${f.value2}`;
  }
  if (operatorNeedsList(f.operator)) {
    return `${f.column} ${f.operator === 'in' ? 'in' : 'not in'} (${(f.values || []).join(', ')})`;
  }
  return `${f.column} ${OPERATOR_LABELS[f.operator].toLowerCase()} "${f.value}"`;
}
