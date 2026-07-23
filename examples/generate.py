#!/usr/bin/env python3
"""Generate the example SQLite databases used for testing squad.

Usage:
    python3 examples/generate.py            # writes into this script's directory
    python3 examples/generate.py <out_dir>  # writes into <out_dir>

Deterministic (fixed RNG seed): re-running reproduces byte-for-byte-equivalent
data. Only stdlib is required. See README.md in this directory for a description
of each database.
"""
import os
import sys
import sqlite3
import random
import datetime as dt

random.seed(1729)
# Default output is the directory this script lives in (examples/).
OUT = os.path.dirname(os.path.abspath(__file__))
if len(sys.argv) > 1:
    OUT = sys.argv[1]
os.makedirs(OUT, exist_ok=True)


def fresh(name):
    path = os.path.join(OUT, name)
    if os.path.exists(path):
        os.remove(path)
    conn = sqlite3.connect(path)
    conn.execute("PRAGMA foreign_keys=ON")
    return conn, path


def ts(days_ago):
    d = dt.datetime(2026, 7, 24, 9, 0, 0) - dt.timedelta(days=days_ago, minutes=random.randint(0, 1440))
    return d.strftime("%Y-%m-%d %H:%M:%S")


FIRST = ["Ada", "Linus", "Grace", "Alan", "Margaret", "Dennis", "Barbara", "Ken",
         "Radia", "Guido", "Katherine", "Tim", "Anita", "Donald", "Hedy", "Vint"]
LAST = ["Lovelace", "Torvalds", "Hopper", "Turing", "Hamilton", "Ritchie", "Liskov",
        "Thompson", "Perlman", "Rossum", "Johnson", "Berners-Lee", "Borg", "Knuth"]


def rand_name():
    return f"{random.choice(FIRST)} {random.choice(LAST)}"


# ---------------------------------------------------------------------------
# 1. blog.db — classic relational: users/posts/comments/tags(+m2m), view, trigger
# ---------------------------------------------------------------------------
def build_blog():
    conn, path = fresh("blog.db")
    c = conn.cursor()
    c.executescript(
        """
        CREATE TABLE users (
            id          INTEGER PRIMARY KEY AUTOINCREMENT,
            username    TEXT NOT NULL UNIQUE,
            email       TEXT NOT NULL UNIQUE,
            full_name   TEXT,
            is_active   INTEGER NOT NULL DEFAULT 1,
            created_at  TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
        );
        CREATE TABLE posts (
            id          INTEGER PRIMARY KEY AUTOINCREMENT,
            author_id   INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            title       TEXT NOT NULL,
            slug        TEXT NOT NULL UNIQUE,
            body        TEXT,
            status      TEXT NOT NULL DEFAULT 'draft'
                        CHECK (status IN ('draft','published','archived')),
            views       INTEGER NOT NULL DEFAULT 0,
            published_at TEXT,
            created_at  TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
        );
        CREATE TABLE comments (
            id          INTEGER PRIMARY KEY AUTOINCREMENT,
            post_id     INTEGER NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
            user_id     INTEGER REFERENCES users(id) ON DELETE SET NULL,
            body        TEXT NOT NULL,
            created_at  TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
        );
        CREATE TABLE tags (
            id    INTEGER PRIMARY KEY AUTOINCREMENT,
            name  TEXT NOT NULL UNIQUE
        );
        CREATE TABLE post_tags (
            post_id INTEGER NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
            tag_id  INTEGER NOT NULL REFERENCES tags(id)  ON DELETE CASCADE,
            PRIMARY KEY (post_id, tag_id)
        );

        CREATE INDEX idx_posts_author  ON posts(author_id);
        CREATE INDEX idx_posts_status  ON posts(status, published_at);
        CREATE INDEX idx_comments_post ON comments(post_id);

        CREATE VIEW published_posts AS
            SELECT p.id, p.title, u.username AS author, p.views, p.published_at
            FROM posts p JOIN users u ON u.id = p.author_id
            WHERE p.status = 'published';

        -- keep a running comment count denormalized on posts via triggers
        CREATE TABLE post_stats (
            post_id       INTEGER PRIMARY KEY REFERENCES posts(id) ON DELETE CASCADE,
            comment_count INTEGER NOT NULL DEFAULT 0
        );
        CREATE TRIGGER trg_comment_ai AFTER INSERT ON comments
        BEGIN
            INSERT INTO post_stats(post_id, comment_count) VALUES (NEW.post_id, 1)
            ON CONFLICT(post_id) DO UPDATE SET comment_count = comment_count + 1;
        END;
        """
    )

    users = []
    for i in range(40):
        fn = rand_name()
        uname = fn.split()[0].lower() + str(i)
        users.append((uname, f"{uname}@example.com", fn, random.randint(0, 1), ts(random.randint(100, 900))))
    c.executemany(
        "INSERT INTO users(username,email,full_name,is_active,created_at) VALUES (?,?,?,?,?)",
        users,
    )

    tag_names = ["go", "sqlite", "web", "database", "tutorial", "release",
                 "performance", "security", "howto", "opinion"]
    c.executemany("INSERT INTO tags(name) VALUES (?)", [(t,) for t in tag_names])

    statuses = ["draft", "published", "published", "published", "archived"]
    for i in range(1, 121):
        author = random.randint(1, 40)
        status = random.choice(statuses)
        pub = ts(random.randint(1, 90)) if status == "published" else None
        c.execute(
            "INSERT INTO posts(author_id,title,slug,body,status,views,published_at,created_at)"
            " VALUES (?,?,?,?,?,?,?,?)",
            (author, f"Post number {i}", f"post-number-{i}",
             f"This is the body of post {i}. " * random.randint(2, 8),
             status, random.randint(0, 5000), pub, ts(random.randint(1, 120))),
        )
        for tid in random.sample(range(1, 11), random.randint(1, 3)):
            c.execute("INSERT OR IGNORE INTO post_tags(post_id,tag_id) VALUES (?,?)", (i, tid))

    for _ in range(600):
        post = random.randint(1, 120)
        user = random.choice([None] + list(range(1, 41)))
        c.execute(
            "INSERT INTO comments(post_id,user_id,body,created_at) VALUES (?,?,?,?)",
            (post, user, "Nice write-up, thanks!", ts(random.randint(0, 60))),
        )

    conn.commit()
    conn.close()
    return path


# ---------------------------------------------------------------------------
# 2. ecommerce.db — products/categories/orders/order_items, composite, indexes
# ---------------------------------------------------------------------------
def build_ecommerce():
    conn, path = fresh("ecommerce.db")
    c = conn.cursor()
    c.executescript(
        """
        CREATE TABLE categories (
            id        INTEGER PRIMARY KEY,
            name      TEXT NOT NULL UNIQUE,
            parent_id INTEGER REFERENCES categories(id)
        );
        CREATE TABLE products (
            id          INTEGER PRIMARY KEY AUTOINCREMENT,
            sku         TEXT NOT NULL UNIQUE,
            name        TEXT NOT NULL,
            category_id INTEGER REFERENCES categories(id),
            price       REAL NOT NULL CHECK (price >= 0),
            stock       INTEGER NOT NULL DEFAULT 0,
            active      INTEGER NOT NULL DEFAULT 1,
            created_at  TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
        );
        CREATE TABLE customers (
            id      INTEGER PRIMARY KEY AUTOINCREMENT,
            email   TEXT NOT NULL UNIQUE,
            name    TEXT NOT NULL,
            country TEXT
        );
        CREATE TABLE orders (
            id           INTEGER PRIMARY KEY AUTOINCREMENT,
            customer_id  INTEGER NOT NULL REFERENCES customers(id),
            status       TEXT NOT NULL DEFAULT 'pending'
                         CHECK (status IN ('pending','paid','shipped','cancelled')),
            total        REAL NOT NULL DEFAULT 0,
            placed_at    TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
        );
        CREATE TABLE order_items (
            order_id   INTEGER NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
            product_id INTEGER NOT NULL REFERENCES products(id),
            quantity   INTEGER NOT NULL CHECK (quantity > 0),
            unit_price REAL NOT NULL,
            PRIMARY KEY (order_id, product_id)
        );
        CREATE INDEX idx_products_cat   ON products(category_id);
        CREATE INDEX idx_orders_cust    ON orders(customer_id);
        CREATE INDEX idx_orders_status  ON orders(status);
        CREATE VIEW order_totals AS
            SELECT o.id AS order_id, c.name AS customer,
                   SUM(oi.quantity * oi.unit_price) AS computed_total
            FROM orders o
            JOIN customers c   ON c.id = o.customer_id
            JOIN order_items oi ON oi.order_id = o.id
            GROUP BY o.id;
        """
    )

    cats = [("Electronics", None), ("Books", None), ("Home", None),
            ("Phones", 1), ("Laptops", 1), ("Fiction", 2), ("Kitchen", 3)]
    for name, parent in cats:
        c.execute("INSERT INTO categories(name,parent_id) VALUES (?,?)", (name, parent))

    adjectives = ["Pro", "Max", "Mini", "Ultra", "Lite", "Plus", "Air"]
    nouns = ["Widget", "Gadget", "Phone", "Laptop", "Blender", "Novel", "Lamp"]
    for i in range(1, 201):
        c.execute(
            "INSERT INTO products(sku,name,category_id,price,stock,active,created_at)"
            " VALUES (?,?,?,?,?,?,?)",
            (f"SKU-{i:05d}",
             f"{random.choice(nouns)} {random.choice(adjectives)} {i}",
             random.randint(1, 7),
             round(random.uniform(4.99, 1999.0), 2),
             random.randint(0, 500),
             random.randint(0, 1),
             ts(random.randint(1, 400))),
        )

    for i in range(1, 81):
        fn = rand_name()
        c.execute(
            "INSERT INTO customers(email,name,country) VALUES (?,?,?)",
            (f"cust{i}@example.com", fn,
             random.choice(["US", "IN", "DE", "GB", "JP", "BR", None])),
        )

    for oid in range(1, 301):
        cust = random.randint(1, 80)
        status = random.choice(["pending", "paid", "paid", "shipped", "cancelled"])
        c.execute(
            "INSERT INTO orders(customer_id,status,placed_at) VALUES (?,?,?)",
            (cust, status, ts(random.randint(0, 180))),
        )
        total = 0.0
        for pid in random.sample(range(1, 201), random.randint(1, 5)):
            qty = random.randint(1, 4)
            price = round(random.uniform(4.99, 1999.0), 2)
            c.execute(
                "INSERT OR IGNORE INTO order_items(order_id,product_id,quantity,unit_price)"
                " VALUES (?,?,?,?)",
                (oid, pid, qty, price),
            )
            total += qty * price
        c.execute("UPDATE orders SET total=? WHERE id=?", (round(total, 2), oid))

    conn.commit()
    conn.close()
    return path


# ---------------------------------------------------------------------------
# 3. library.db — books/authors/members/loans, many-to-many, view, trigger
# ---------------------------------------------------------------------------
def build_library():
    conn, path = fresh("library.db")
    c = conn.cursor()
    c.executescript(
        """
        CREATE TABLE authors (
            id         INTEGER PRIMARY KEY AUTOINCREMENT,
            name       TEXT NOT NULL,
            birth_year INTEGER
        );
        CREATE TABLE books (
            id        INTEGER PRIMARY KEY AUTOINCREMENT,
            isbn      TEXT NOT NULL UNIQUE,
            title     TEXT NOT NULL,
            year      INTEGER,
            copies    INTEGER NOT NULL DEFAULT 1 CHECK (copies >= 0)
        );
        CREATE TABLE book_authors (
            book_id   INTEGER NOT NULL REFERENCES books(id)   ON DELETE CASCADE,
            author_id INTEGER NOT NULL REFERENCES authors(id) ON DELETE CASCADE,
            PRIMARY KEY (book_id, author_id)
        );
        CREATE TABLE members (
            id         INTEGER PRIMARY KEY AUTOINCREMENT,
            name       TEXT NOT NULL,
            email      TEXT UNIQUE,
            joined_at  TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
        );
        CREATE TABLE loans (
            id        INTEGER PRIMARY KEY AUTOINCREMENT,
            book_id   INTEGER NOT NULL REFERENCES books(id),
            member_id INTEGER NOT NULL REFERENCES members(id),
            loaned_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
            due_at    TEXT NOT NULL,
            returned_at TEXT
        );
        CREATE INDEX idx_loans_member ON loans(member_id);
        CREATE INDEX idx_loans_open   ON loans(returned_at) WHERE returned_at IS NULL;

        CREATE VIEW open_loans AS
            SELECT l.id, b.title, m.name AS member, l.due_at
            FROM loans l
            JOIN books b   ON b.id = l.book_id
            JOIN members m ON m.id = l.member_id
            WHERE l.returned_at IS NULL;

        -- prevent loaning a book with no copies (raises on violation)
        CREATE TRIGGER trg_loan_stock BEFORE INSERT ON loans
        WHEN (SELECT copies FROM books WHERE id = NEW.book_id) <= 0
        BEGIN
            SELECT RAISE(ABORT, 'no copies available');
        END;
        """
    )

    for _ in range(50):
        c.execute("INSERT INTO authors(name,birth_year) VALUES (?,?)",
                  (rand_name(), random.randint(1900, 1995)))
    titles = ["The Pragmatic", "Deep", "Concurrent", "Distributed", "Elegant",
              "Practical", "Modern", "Foundations of", "Advanced", "Introduction to"]
    subjects = ["Systems", "Databases", "Algorithms", "Networks", "Compilers",
                "Security", "Go", "Rust", "SQLite", "Design"]
    for i in range(1, 121):
        c.execute(
            "INSERT INTO books(isbn,title,year,copies) VALUES (?,?,?,?)",
            (f"978-0-{random.randint(100000,999999)}-{i%10}",
             f"{random.choice(titles)} {random.choice(subjects)}",
             random.randint(1980, 2026), random.randint(0, 6)),
        )
        for aid in random.sample(range(1, 51), random.randint(1, 2)):
            c.execute("INSERT OR IGNORE INTO book_authors(book_id,author_id) VALUES (?,?)", (i, aid))

    for i in range(1, 91):
        c.execute("INSERT INTO members(name,email,joined_at) VALUES (?,?,?)",
                  (rand_name(), f"member{i}@example.com", ts(random.randint(10, 800))))

    # only loan books that currently have copies (trigger enforces this)
    have_copies = [r[0] for r in c.execute("SELECT id FROM books WHERE copies > 0").fetchall()]
    for _ in range(220):
        book = random.choice(have_copies)
        member = random.randint(1, 90)
        loaned = dt.datetime(2026, 7, 24) - dt.timedelta(days=random.randint(1, 200))
        due = loaned + dt.timedelta(days=21)
        returned = None
        if random.random() < 0.7:
            returned = (loaned + dt.timedelta(days=random.randint(1, 30))).strftime("%Y-%m-%d %H:%M:%S")
        c.execute(
            "INSERT INTO loans(book_id,member_id,loaned_at,due_at,returned_at) VALUES (?,?,?,?,?)",
            (book, member, loaned.strftime("%Y-%m-%d %H:%M:%S"),
             due.strftime("%Y-%m-%d %H:%M:%S"), returned),
        )

    conn.commit()
    conn.close()
    return path


# ---------------------------------------------------------------------------
# 4. analytics.db — event stream, BLOBs, larger tables, WITHOUT ROWID, TEXT PK
# ---------------------------------------------------------------------------
def build_analytics():
    conn, path = fresh("analytics.db")
    c = conn.cursor()
    c.executescript(
        """
        CREATE TABLE sessions (
            session_id TEXT PRIMARY KEY,       -- uuid-ish text pk
            user_id    INTEGER,
            device     TEXT,
            country    TEXT,
            started_at TEXT NOT NULL,
            duration_s INTEGER
        ) WITHOUT ROWID;
        CREATE TABLE events (
            id          INTEGER PRIMARY KEY AUTOINCREMENT,
            session_id  TEXT NOT NULL REFERENCES sessions(session_id),
            name        TEXT NOT NULL,
            value       REAL,
            props       TEXT,                  -- JSON stored as text
            payload     BLOB,                  -- raw bytes
            occurred_at TEXT NOT NULL
        );
        CREATE INDEX idx_events_session ON events(session_id);
        CREATE INDEX idx_events_name    ON events(name, occurred_at);

        CREATE TABLE daily_rollup (
            day        TEXT NOT NULL,
            event_name TEXT NOT NULL,
            hits       INTEGER NOT NULL,
            PRIMARY KEY (day, event_name)
        ) WITHOUT ROWID;

        CREATE VIEW event_counts AS
            SELECT name, COUNT(*) AS n, ROUND(AVG(value),3) AS avg_value
            FROM events GROUP BY name;
        """
    )

    devices = ["ios", "android", "web", "desktop"]
    countries = ["US", "IN", "DE", "GB", "JP", "BR", "FR", "CA"]
    ev_names = ["page_view", "click", "signup", "purchase", "error", "scroll", "search"]

    session_ids = []
    for i in range(500):
        sid = f"{random.getrandbits(32):08x}-{random.getrandbits(16):04x}-{i:04x}"
        session_ids.append(sid)
        c.execute(
            "INSERT INTO sessions(session_id,user_id,device,country,started_at,duration_s)"
            " VALUES (?,?,?,?,?,?)",
            (sid, random.choice([None] + list(range(1, 200))),
             random.choice(devices), random.choice(countries),
             ts(random.randint(0, 60)), random.randint(5, 3600)),
        )

    rollup = {}
    for _ in range(5000):
        sid = random.choice(session_ids)
        name = random.choice(ev_names)
        occurred = ts(random.randint(0, 60))
        day = occurred[:10]
        rollup[(day, name)] = rollup.get((day, name), 0) + 1
        val = round(random.uniform(0, 100), 2) if name in ("purchase", "scroll") else None
        props = f'{{"idx":{random.randint(0,9)},"ok":{str(random.random()<0.9).lower()}}}'
        payload = bytes(random.getrandbits(8) for _ in range(random.randint(0, 16)))
        c.execute(
            "INSERT INTO events(session_id,name,value,props,payload,occurred_at)"
            " VALUES (?,?,?,?,?,?)",
            (sid, name, val, props, payload, occurred),
        )

    for (day, name), hits in rollup.items():
        c.execute("INSERT INTO daily_rollup(day,event_name,hits) VALUES (?,?,?)",
                  (day, name, hits))

    conn.commit()
    conn.close()
    return path


# ---------------------------------------------------------------------------
# 5. types_zoo.db — edge cases: all affinities, NULLs, generated cols, quoted ids
# ---------------------------------------------------------------------------
def build_types_zoo():
    conn, path = fresh("types_zoo.db")
    c = conn.cursor()
    c.executescript(
        """
        CREATE TABLE affinities (
            id       INTEGER PRIMARY KEY,
            c_int    INTEGER,
            c_real   REAL,
            c_text   TEXT,
            c_blob   BLOB,
            c_numeric NUMERIC,
            c_bool   INTEGER,          -- 0/1
            c_null   TEXT              -- always NULL
        );
        -- generated columns (stored + virtual)
        CREATE TABLE measurements (
            id       INTEGER PRIMARY KEY,
            label    TEXT NOT NULL,
            width_cm REAL NOT NULL,
            height_cm REAL NOT NULL,
            area_cm2 REAL GENERATED ALWAYS AS (width_cm * height_cm) STORED,
            ratio    REAL GENERATED ALWAYS AS (width_cm / height_cm) VIRTUAL
        );
        -- quoted / unusual identifiers and reserved-ish words
        CREATE TABLE "weird names" (
            "select"      INTEGER PRIMARY KEY,
            "space col"   TEXT,
            "from"        TEXT,
            "MixedCase"   INTEGER
        );
        -- composite unique + partial index + collation
        CREATE TABLE contacts (
            id     INTEGER PRIMARY KEY,
            email  TEXT COLLATE NOCASE,
            phone  TEXT,
            tag    TEXT
        );
        CREATE UNIQUE INDEX idx_contacts_email ON contacts(email);
        CREATE INDEX idx_contacts_tagged ON contacts(tag) WHERE tag IS NOT NULL;

        CREATE VIEW big_boxes AS
            SELECT id, label, area_cm2 FROM measurements WHERE area_cm2 > 100;
        """
    )

    # affinities: mix of types including NULLs and a real BLOB
    for i in range(1, 26):
        c.execute(
            "INSERT INTO affinities(id,c_int,c_real,c_text,c_blob,c_numeric,c_bool,c_null)"
            " VALUES (?,?,?,?,?,?,?,?)",
            (i,
             random.choice([None, random.randint(-1000, 1000)]),
             random.choice([None, round(random.uniform(-1e3, 1e6), 4)]),
             random.choice([None, "hello", "unicode: café ☕ 日本語", "with 'quotes'"]),
             random.choice([None, bytes(range(random.randint(0, 12)))]),
             random.choice([None, 42, 3.14, "123"]),
             random.randint(0, 1),
             None),
        )

    labels = ["panel", "screen", "tile", "sheet", "card"]
    for i in range(1, 21):
        c.execute(
            "INSERT INTO measurements(id,label,width_cm,height_cm) VALUES (?,?,?,?)",
            (i, random.choice(labels), round(random.uniform(1, 30), 1),
             round(random.uniform(1, 30), 1)),
        )

    for i in range(1, 11):
        c.execute('INSERT INTO "weird names"("select","space col","from","MixedCase") VALUES (?,?,?,?)',
                  (i, f"value {i}", random.choice(["a", "b", "c"]), random.randint(0, 100)))

    tags = ["friend", "work", "family", None]
    for i in range(1, 31):
        c.execute("INSERT INTO contacts(id,email,phone,tag) VALUES (?,?,?,?)",
                  (i, f"person{i}@Example.com", f"+1-555-{random.randint(1000,9999)}",
                   random.choice(tags)))

    conn.commit()
    conn.close()
    return path


if __name__ == "__main__":
    for fn in (build_blog, build_ecommerce, build_library, build_analytics, build_types_zoo):
        p = fn()
        print("wrote", p)
