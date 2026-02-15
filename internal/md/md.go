package md

import (
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/text"
	meta "go.abhg.dev/goldmark/frontmatter"
)

// parser is a reusable goldmark parser with frontmatter support.
var parser = goldmark.New(
	goldmark.WithExtensions(
		&meta.Extender{Mode: meta.SetMetadata},
	),
).Parser()

// ParseName extracts the name field from YAML
// frontmatter in a markdown document. Returns ""
// if not found or if the value is not a string.
func ParseName(data []byte) string {
	doc := parser.Parse(text.NewReader(data))
	m := doc.OwnerDocument().Meta()
	if m == nil {
		return ""
	}
	v, ok := m["name"]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}
