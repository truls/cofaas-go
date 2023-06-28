package cofaas

import (
	"os"
	"path"
	"testing"
	"flag"

	"github.com/sergi/go-diff/diffmatchpatch"
)

var (
	update = flag.Bool("update", false, "update the golden files of this test")
)

func getTestInput(file string) string {
	return path.Join("testdata", file);
}

func getGoldenFileName(file string) string {
	return getTestInput(file) + ".golden"
}

func readGoldenFile(file string) (string, error) {
	contents, err := os.ReadFile(getGoldenFileName(file))
	if err != nil {
		return "", err
	}

	return string(contents), err
}

func writeGoldenFile(file string) error {
	return os.WriteFile(file, []byte(file), 0644)
}

func getComparison(a string, b string) string {
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(b, a, false)
	return dmp.DiffPrettyText(diffs)
}

func testRewriter(t *testing.T, f string) {
	fn := getTestInput(f)
	r, err := GetRewriter(fn)
	if err != nil {
		t.Error(err)
	}

	res, err := r.Rewrite(fn)
	if err != nil {
		t.Error(err)
	}
	expected, err := readGoldenFile(f)

	resfmt, err := res.Format()
	if err != nil {
		t.Error(err)
	}

	if *update {
		if err := res.Write(getGoldenFileName(f)); err != nil {
			t.Error(err)
		}
	} else {
		if resfmt != expected {
			t.Errorf("Golden file mismatch\n\n%s", getComparison(resfmt, expected))
		}
	}
}

func TestRewriteModule(t *testing.T) {
	testRewriter(t, "go.mod")
}

func TestRewriteFile(t *testing.T) {
	testRewriter(t, "producer.go")
}


func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}
