package cofaas

import (
	"os"
	"path"
	"testing"

	"github.com/sergi/go-diff/diffmatchpatch"
	opt "github.com/moznion/go-optional"
)

func call1test(f func(string) (string, error)) func(string, opt.Option[string]) (string, error) {
	return func(a1 string, a2 opt.Option[string]) (string, error) {
		return f(a1)
	}
}

func getTestInput(file string) string {
	return path.Join("testdata", file)
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

func writeGoldenFile(goldenFile string, contents string) error {
	return os.WriteFile(getGoldenFileName(goldenFile), []byte(contents), 0644)
}

func getComparison(a string, b string) string {
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(b, a, false)
	return dmp.DiffPrettyText(diffs)
}

func compareGoldenFile (t *testing.T, goldenFile1 string, extraInput opt.Option[string], transformer func(string, opt.Option[string]) (string, error), doUpdate bool, verbose bool) {

	fn := getTestInput(goldenFile1)
	fn2 := opt.Map(extraInput, getTestInput)
	expected, err := readGoldenFile(goldenFile1)
	if err != nil {
		t.Error(err)
	}

	actual, err := transformer(fn, fn2)
	if err != nil {
		t.Error(err)
	}

	if doUpdate {
		if err := writeGoldenFile(goldenFile1, actual); err != nil {
			t.Error(err)
		}
	} else {
		if actual != expected {
			if verbose {
				t.Errorf("Golden file mismatch\n\n%s", getComparison(actual, expected))
			} else {
				t.Error("Golden file mismatch")
			}
		}
	}

}
