package main

import (
	"bytes"
	"flag"
	"testing"
)

func TestFlagDocIndent(t *testing.T) {
	// Tests the indent() function, which differs by Go version,
	// due to differences in "flags" package across versions.
	// Run with multiple versions of Go to ensure that doc output
	// is properly indented, regardless of Go version.

	var fs flag.FlagSet
	var buf bytes.Buffer
	fs.SetOutput(&buf)

	fs.String("foo", "", prettify(`
		This is a flag doc string.
		It has multiple lines.
		More than two, actually.`))
	fs.Int("bar", 100, prettify(`
		This is a simple flag doc string.`))
	fs.Bool("baz", false, prettify(`
		This is another long doc string.
		It also has multiple lines. But not as long as the first one.`))

	fs.PrintDefaults()

	expected := `  -bar int
    	This is a simple flag doc string. (default 100)
  -baz
    	This is another long doc string.
    	It also has multiple lines. But not as long as the first one.
  -foo string
    	This is a flag doc string.
    	It has multiple lines.
    	More than two, actually.
`

	if buf.String() != expected {
		t.Errorf("Flag output had wrong indentation.\nExpecting:\n%s\nGot:\n%s", expected, buf.String())
	}
}
