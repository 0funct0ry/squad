// Package testfixtures generates the small SQLite databases used by squad's
// test suite (blog/library/types_zoo schemas with sample rows). It exists
// solely so tests are self-contained and never depend on Python or on
// checked-in .db files; it is a Go port of the schema/data shapes originally
// prototyped in examples/generate.py, which remains as a standalone script
// for manual experimentation and is not used by any test.
package testfixtures

import (
	"database/sql"
	"fmt"
	"math/rand"
	"time"

	_ "modernc.org/sqlite"
)

func open(path string) (*sql.DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := conn.Exec("PRAGMA foreign_keys=ON"); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

var firstNames = []string{"Ada", "Linus", "Grace", "Alan", "Margaret", "Dennis", "Barbara", "Ken",
	"Radia", "Guido", "Katherine", "Tim", "Anita", "Donald", "Hedy", "Vint"}
var lastNames = []string{"Lovelace", "Torvalds", "Hopper", "Turing", "Hamilton", "Ritchie", "Liskov",
	"Thompson", "Perlman", "Rossum", "Johnson", "Berners-Lee", "Borg", "Knuth"}

func randName(r *rand.Rand) string {
	return firstNames[r.Intn(len(firstNames))] + " " + lastNames[r.Intn(len(lastNames))]
}

// ts returns a timestamp daysAgo days before a fixed reference instant, with
// a random sub-day jitter, matching examples/generate.py's ts() helper.
func ts(r *rand.Rand, daysAgo int) string {
	ref := time.Date(2026, 7, 24, 9, 0, 0, 0, time.UTC)
	d := ref.Add(-time.Duration(daysAgo)*24*time.Hour - time.Duration(r.Intn(1441))*time.Minute)
	return d.Format("2006-01-02 15:04:05")
}

// BuildBlog creates the blog.db fixture (users/posts/comments/tags/m2m, a
// view, and a trigger) at path.
func BuildBlog(path string) error {
	conn, err := open(path)
	if err != nil {
		return err
	}
	defer conn.Close()

	r := rand.New(rand.NewSource(1729))

	if _, err := conn.Exec(`
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

		CREATE TABLE post_stats (
			post_id       INTEGER PRIMARY KEY REFERENCES posts(id) ON DELETE CASCADE,
			comment_count INTEGER NOT NULL DEFAULT 0
		);
		CREATE TRIGGER trg_comment_ai AFTER INSERT ON comments
		BEGIN
			INSERT INTO post_stats(post_id, comment_count) VALUES (NEW.post_id, 1)
			ON CONFLICT(post_id) DO UPDATE SET comment_count = comment_count + 1;
		END;
	`); err != nil {
		return fmt.Errorf("create blog schema: %w", err)
	}

	unameOf := func(fn string, i int) string {
		var first string
		fmt.Sscanf(fn, "%s", &first)
		return fmt.Sprintf("%s%d", toLower(first), i)
	}
	for i := 0; i < 40; i++ {
		fn := randName(r)
		uname := unameOf(fn, i)
		if _, err := conn.Exec(
			"INSERT INTO users(username,email,full_name,is_active,created_at) VALUES (?,?,?,?,?)",
			uname, uname+"@example.com", fn, r.Intn(2), ts(r, 100+r.Intn(801)),
		); err != nil {
			return fmt.Errorf("insert user: %w", err)
		}
	}

	tagNames := []string{"go", "sqlite", "web", "database", "tutorial", "release",
		"performance", "security", "howto", "opinion"}
	for _, name := range tagNames {
		if _, err := conn.Exec("INSERT INTO tags(name) VALUES (?)", name); err != nil {
			return fmt.Errorf("insert tag: %w", err)
		}
	}

	statuses := []string{"draft", "published", "published", "published", "archived"}
	for i := 1; i <= 120; i++ {
		author := 1 + r.Intn(40)
		status := statuses[r.Intn(len(statuses))]
		var pub any
		if status == "published" {
			pub = ts(r, 1+r.Intn(90))
		}
		body := ""
		reps := 2 + r.Intn(7)
		for j := 0; j < reps; j++ {
			body += fmt.Sprintf("This is the body of post %d. ", i)
		}
		if _, err := conn.Exec(
			`INSERT INTO posts(author_id,title,slug,body,status,views,published_at,created_at)
			 VALUES (?,?,?,?,?,?,?,?)`,
			author, fmt.Sprintf("Post number %d", i), fmt.Sprintf("post-number-%d", i),
			body, status, r.Intn(5001), pub, ts(r, 1+r.Intn(120)),
		); err != nil {
			return fmt.Errorf("insert post: %w", err)
		}
		for _, tid := range sample(r, 1, 10, 1+r.Intn(3)) {
			if _, err := conn.Exec("INSERT OR IGNORE INTO post_tags(post_id,tag_id) VALUES (?,?)", i, tid); err != nil {
				return fmt.Errorf("insert post_tag: %w", err)
			}
		}
	}

	for i := 0; i < 600; i++ {
		post := 1 + r.Intn(120)
		var userID any
		if r.Intn(41) > 0 { // 40/41 chance of a user, matching choice([None]+range(1,41))
			userID = 1 + r.Intn(40)
		}
		if _, err := conn.Exec(
			"INSERT INTO comments(post_id,user_id,body,created_at) VALUES (?,?,?,?)",
			post, userID, "Nice write-up, thanks!", ts(r, r.Intn(61)),
		); err != nil {
			return fmt.Errorf("insert comment: %w", err)
		}
	}

	return nil
}

// BuildLibrary creates the library.db fixture (authors/books/members/loans,
// many-to-many, a view, and a stock-check trigger) at path.
func BuildLibrary(path string) error {
	conn, err := open(path)
	if err != nil {
		return err
	}
	defer conn.Close()

	r := rand.New(rand.NewSource(1729))

	if _, err := conn.Exec(`
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

		CREATE TRIGGER trg_loan_stock BEFORE INSERT ON loans
		WHEN (SELECT copies FROM books WHERE id = NEW.book_id) <= 0
		BEGIN
			SELECT RAISE(ABORT, 'no copies available');
		END;
	`); err != nil {
		return fmt.Errorf("create library schema: %w", err)
	}

	for i := 0; i < 50; i++ {
		if _, err := conn.Exec("INSERT INTO authors(name,birth_year) VALUES (?,?)",
			randName(r), 1900+r.Intn(96)); err != nil {
			return fmt.Errorf("insert author: %w", err)
		}
	}

	titles := []string{"The Pragmatic", "Deep", "Concurrent", "Distributed", "Elegant",
		"Practical", "Modern", "Foundations of", "Advanced", "Introduction to"}
	subjects := []string{"Systems", "Databases", "Algorithms", "Networks", "Compilers",
		"Security", "Go", "Rust", "SQLite", "Design"}

	haveCopies := make([]int, 0, 120)
	for i := 1; i <= 120; i++ {
		copies := r.Intn(7)
		if _, err := conn.Exec(
			"INSERT INTO books(isbn,title,year,copies) VALUES (?,?,?,?)",
			fmt.Sprintf("978-0-%06d-%d", r.Intn(900000)+100000, i%10),
			titles[r.Intn(len(titles))]+" "+subjects[r.Intn(len(subjects))],
			1980+r.Intn(47), copies,
		); err != nil {
			return fmt.Errorf("insert book: %w", err)
		}
		if copies > 0 {
			haveCopies = append(haveCopies, i)
		}
		for _, aid := range sample(r, 1, 50, 1+r.Intn(2)) {
			if _, err := conn.Exec("INSERT OR IGNORE INTO book_authors(book_id,author_id) VALUES (?,?)", i, aid); err != nil {
				return fmt.Errorf("insert book_author: %w", err)
			}
		}
	}

	for i := 1; i <= 90; i++ {
		if _, err := conn.Exec("INSERT INTO members(name,email,joined_at) VALUES (?,?,?)",
			randName(r), fmt.Sprintf("member%d@example.com", i), ts(r, 10+r.Intn(791))); err != nil {
			return fmt.Errorf("insert member: %w", err)
		}
	}

	if len(haveCopies) > 0 {
		ref := time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC)
		for i := 0; i < 220; i++ {
			book := haveCopies[r.Intn(len(haveCopies))]
			member := 1 + r.Intn(90)
			loaned := ref.Add(-time.Duration(1+r.Intn(200)) * 24 * time.Hour)
			due := loaned.Add(21 * 24 * time.Hour)
			var returned any
			if r.Float64() < 0.7 {
				returned = loaned.Add(time.Duration(1+r.Intn(30)) * 24 * time.Hour).Format("2006-01-02 15:04:05")
			}
			if _, err := conn.Exec(
				"INSERT INTO loans(book_id,member_id,loaned_at,due_at,returned_at) VALUES (?,?,?,?,?)",
				book, member, loaned.Format("2006-01-02 15:04:05"), due.Format("2006-01-02 15:04:05"), returned,
			); err != nil {
				return fmt.Errorf("insert loan: %w", err)
			}
		}
	}

	return nil
}

// BuildTypesZoo creates the types_zoo.db fixture (all column affinities,
// NULLs, generated columns, quoted/reserved identifiers, and a partial
// unique index) at path.
func BuildTypesZoo(path string) error {
	conn, err := open(path)
	if err != nil {
		return err
	}
	defer conn.Close()

	r := rand.New(rand.NewSource(1729))

	if _, err := conn.Exec(`
		CREATE TABLE affinities (
			id       INTEGER PRIMARY KEY,
			c_int    INTEGER,
			c_real   REAL,
			c_text   TEXT,
			c_blob   BLOB,
			c_numeric NUMERIC,
			c_bool   INTEGER,
			c_null   TEXT
		);
		CREATE TABLE measurements (
			id       INTEGER PRIMARY KEY,
			label    TEXT NOT NULL,
			width_cm REAL NOT NULL,
			height_cm REAL NOT NULL,
			area_cm2 REAL GENERATED ALWAYS AS (width_cm * height_cm) STORED,
			ratio    REAL GENERATED ALWAYS AS (width_cm / height_cm) VIRTUAL
		);
		CREATE TABLE "weird names" (
			"select"      INTEGER PRIMARY KEY,
			"space col"   TEXT,
			"from"        TEXT,
			"MixedCase"   INTEGER
		);
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
	`); err != nil {
		return fmt.Errorf("create types_zoo schema: %w", err)
	}

	texts := []any{nil, "hello", "unicode: café ☕ 日本語", "with 'quotes'"}
	nums := []any{nil, int64(42), 3.14, "123"}
	for i := 1; i <= 25; i++ {
		var cInt any
		if r.Intn(2) == 1 {
			cInt = -1000 + r.Intn(2001)
		}
		var cReal any
		if r.Intn(2) == 1 {
			cReal = round4(-1e3 + r.Float64()*(1e6+1e3))
		}
		cText := texts[r.Intn(len(texts))]
		var cBlob any
		if r.Intn(2) == 1 {
			n := r.Intn(13)
			b := make([]byte, n)
			for j := range b {
				b[j] = byte(j)
			}
			cBlob = b
		}
		cNumeric := nums[r.Intn(len(nums))]
		if _, err := conn.Exec(
			"INSERT INTO affinities(id,c_int,c_real,c_text,c_blob,c_numeric,c_bool,c_null) VALUES (?,?,?,?,?,?,?,?)",
			i, cInt, cReal, cText, cBlob, cNumeric, r.Intn(2), nil,
		); err != nil {
			return fmt.Errorf("insert affinity: %w", err)
		}
	}

	labels := []string{"panel", "screen", "tile", "sheet", "card"}
	for i := 1; i <= 20; i++ {
		if _, err := conn.Exec(
			"INSERT INTO measurements(id,label,width_cm,height_cm) VALUES (?,?,?,?)",
			i, labels[r.Intn(len(labels))], round1(1+r.Float64()*29), round1(1+r.Float64()*29),
		); err != nil {
			return fmt.Errorf("insert measurement: %w", err)
		}
	}

	choices := []string{"a", "b", "c"}
	for i := 1; i <= 10; i++ {
		if _, err := conn.Exec(
			`INSERT INTO "weird names"("select","space col","from","MixedCase") VALUES (?,?,?,?)`,
			i, fmt.Sprintf("value %d", i), choices[r.Intn(len(choices))], r.Intn(101),
		); err != nil {
			return fmt.Errorf("insert weird names row: %w", err)
		}
	}

	tags := []any{"friend", "work", "family", nil}
	for i := 1; i <= 30; i++ {
		if _, err := conn.Exec(
			"INSERT INTO contacts(id,email,phone,tag) VALUES (?,?,?,?)",
			i, fmt.Sprintf("person%d@Example.com", i), fmt.Sprintf("+1-555-%d", 1000+r.Intn(9000)), tags[r.Intn(len(tags))],
		); err != nil {
			return fmt.Errorf("insert contact: %w", err)
		}
	}

	return nil
}

// sample draws n distinct integers from [lo, hi] without replacement.
func sample(r *rand.Rand, lo, hi, n int) []int {
	pool := make([]int, 0, hi-lo+1)
	for v := lo; v <= hi; v++ {
		pool = append(pool, v)
	}
	r.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
	if n > len(pool) {
		n = len(pool)
	}
	return pool[:n]
}

func round1(v float64) float64 {
	return float64(int(v*10+0.5)) / 10
}

func round4(v float64) float64 {
	return float64(int(v*10000+0.5)) / 10000
}

func toLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}
