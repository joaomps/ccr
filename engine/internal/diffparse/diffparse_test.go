package diffparse

import "testing"

const twoHunk = `diff --git a/svc/a.go b/svc/a.go
index 111..222 100644
--- a/svc/a.go
+++ b/svc/a.go
@@ -10,3 +10,4 @@ func A() {
 	ctx := context.Background()
+	x := foo()
 	bar()
+	return x.Y()
@@ -40,2 +41,2 @@ func B() {
-	old()
+	new1()
 	keep()
`

func TestParseTwoHunks(t *testing.T) {
	files, err := Parse(twoHunk)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("want 1 file, got %d", len(files))
	}
	f := files[0]
	if f.NewPath != "svc/a.go" {
		t.Errorf("NewPath=%q", f.NewPath)
	}
	want := []struct {
		line int
		code string
	}{
		{11, "\tx := foo()"},
		{13, "\treturn x.Y()"},
		{41, "\tnew1()"},
	}
	if len(f.ReviewLines) != len(want) {
		t.Fatalf("review lines=%+v", f.ReviewLines)
	}
	for i, w := range want {
		if f.ReviewLines[i].Line != w.line || f.ReviewLines[i].Code != w.code {
			t.Errorf("line %d: got {%d,%q} want {%d,%q}", i, f.ReviewLines[i].Line, f.ReviewLines[i].Code, w.line, w.code)
		}
	}
	if len(f.HunkStarts) != 2 || f.HunkStarts[0] != 10 || f.HunkStarts[1] != 41 {
		t.Errorf("HunkStarts=%v", f.HunkStarts)
	}
}

func TestParseNewFile(t *testing.T) {
	d := `diff --git a/new.go b/new.go
new file mode 100644
index 000..111
--- /dev/null
+++ b/new.go
@@ -0,0 +1,2 @@
+package main
+func main() {}
`
	files, err := Parse(d)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].NewPath != "new.go" {
		t.Fatalf("got %+v", files)
	}
	if len(files[0].ReviewLines) != 2 || files[0].ReviewLines[0].Line != 1 || files[0].ReviewLines[1].Line != 2 {
		t.Errorf("review lines=%+v", files[0].ReviewLines)
	}
}

func TestParseDeletedAndBinarySkipped(t *testing.T) {
	d := `diff --git a/gone.go b/gone.go
deleted file mode 100644
--- a/gone.go
+++ /dev/null
@@ -1,2 +0,0 @@
-package main
-func main() {}
diff --git a/img.png b/img.png
index 111..222 100644
Binary files a/img.png and b/img.png differ
`
	files, err := Parse(d)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatalf("expected deleted + binary skipped, got %+v", files)
	}
}
