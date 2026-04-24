package schemas

import (
	"embed"
	"io/fs"
	"sort"
	"strings"
)

//go:embed *.json
var files embed.FS

// For returns the JSON Schema document for the given RTM method
// name (e.g. "rtm.tasks.getList"). Returns nil if the name is
// not known.
func For(rtmMethod string) []byte {
	b, err := files.ReadFile(rtmMethod + ".json")
	if err != nil {
		return nil
	}
	return b
}

// Methods returns the sorted list of RTM method names the
// embedded schema set covers.
func Methods() []string {
	entries, err := fs.ReadDir(files, ".")
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		out = append(out, strings.TrimSuffix(name, ".json"))
	}
	sort.Strings(out)
	return out
}
