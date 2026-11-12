import { storeFrom, type ListFilter, type Row, type Store, type Table } from "./store";

const matches = (r: Row, f: ListFilter) =>
  (!f.dateFrom || (r.date !== undefined && r.date >= f.dateFrom)) &&
  (!f.dateTo || (r.date !== undefined && r.date <= f.dateTo)) &&
  (!f.programId || r.programId === f.programId) &&
  (f.includeDeleted || !r.deletedAt) &&
  (!f.dirty || !!r.dirty);

// Missing values sort first, as NULLs do in SQLite.
function compare<V extends string | number>(a?: V, b?: V): number {
  if (a === b) return 0;
  if (a === undefined) return -1;
  if (b === undefined) return 1;
  return a < b ? -1 : 1;
}

const order = (a: Row, b: Row) =>
  compare(a.date, b.date) ||
  compare(a.week, b.week) ||
  compare(a.day, b.day) ||
  compare(a.id, b.id);

// Rows are kept as JSON so callers never share objects with the store, as with SQLite.
export function createMemoryStore(): Store {
  let data = new Map<Table, Map<string, string>>();
  const rows = (table: Table) => {
    if (!data.has(table)) data.set(table, new Map());
    return data.get(table)!;
  };
  return storeFrom({
    get(table, id) {
      const json = rows(table).get(id);
      return json === undefined ? undefined : JSON.parse(json);
    },
    put(table, row) {
      rows(table).set(row.id, JSON.stringify(row));
    },
    remove(table, id) {
      rows(table).delete(id);
    },
    clear: () => data.clear(),
    list: (table, filter = {}) =>
      [...rows(table).values()]
        .map((json) => JSON.parse(json))
        .filter((r) => matches(r, filter))
        .sort(order),
    transaction(fn) {
      const snapshot = new Map([...data].map(([table, m]) => [table, new Map(m)]));
      try {
        return fn();
      } catch (e) {
        data = snapshot;
        throw e;
      }
    },
  });
}
