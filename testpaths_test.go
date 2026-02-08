package rap

import "path/filepath"

// testDataPath builds stable path under repository testdata directory.
func testDataPath(parts ...string) string {
	all := make([]string, 0, len(parts)+1)
	all = append(all, "testdata")
	all = append(all, parts...)

	return filepath.Join(all...)
}
