package sqlsafe

import "testing"

func TestCheckRejectsFileRead(t *testing.T) {
	err := Check("SELECT pg_read_file('/etc/passwd')")
	if err == nil {
		t.Fatal("expected denylist rejection")
	}
}

func TestCheckAllowsSelect(t *testing.T) {
	if err := Check("SELECT id FROM orders WHERE customer_id = $1"); err != nil {
		t.Fatal(err)
	}
}

func TestSplit(t *testing.T) {
	stmts, err := Split("SELECT 1; SELECT 'a;b'; SELECT $$x;y$$;")
	if err != nil {
		t.Fatal(err)
	}
	if len(stmts) != 3 {
		t.Fatalf("got %d statements: %#v", len(stmts), stmts)
	}
	if stmts[1] != "SELECT 'a;b'" {
		t.Fatalf("string split incorrectly: %s", stmts[1])
	}
}

func TestSplitUnterminated(t *testing.T) {
	if _, err := Split("SELECT 'oops"); err == nil {
		t.Fatal("expected unterminated string error")
	}
}

func TestIsReadOnly(t *testing.T) {
	if !IsReadOnly("SELECT id FROM orders WHERE customer_id = $1") {
		t.Fatal("select should be read only")
	}
	if IsReadOnly("INSERT INTO orders(customer_id) VALUES ($1)") {
		t.Fatal("insert is not read only")
	}
	if IsReadOnly("WITH w AS (INSERT INTO orders(id) VALUES (1) RETURNING id) SELECT * FROM w") {
		t.Fatal("cte write is not read only")
	}
}

func TestFingerprintStable(t *testing.T) {
	a := Fingerprint("SELECT  1\n")
	b := Fingerprint("SELECT 1")
	if a != b {
		t.Fatalf("fingerprints differ: %s %s", a, b)
	}
}
