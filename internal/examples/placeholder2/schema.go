// Package placeholder2 is scaffolding-only content for M6f: it exists purely
// to exercise the examples registry, embed, flag gate, and GUI/CLI surfaces
// end-to-end. It is expected to be replaced/supplemented once the real 40
// data-model schemas land as internal/examples/<slug>/schema.go files.
package placeholder2

// Schema is a minimal inventory data model: products and warehouse stock.
const Schema = `CREATE TABLE products (
    id INTEGER PRIMARY KEY,
    sku TEXT UNIQUE NOT NULL,
    name TEXT NOT NULL,
    price REAL NOT NULL
);

CREATE TABLE warehouses (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    city TEXT NOT NULL
);

CREATE TABLE stock (
    id INTEGER PRIMARY KEY,
    product_id INTEGER NOT NULL REFERENCES products(id),
    warehouse_id INTEGER NOT NULL REFERENCES warehouses(id),
    quantity INTEGER NOT NULL DEFAULT 0
);
`
