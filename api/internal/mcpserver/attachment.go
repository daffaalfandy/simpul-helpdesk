package mcpserver

import (
	"fmt"
	"slices"
	"strings"
)

// attachmentKind says how read_attachment can return a file.
type attachmentKind int

const (
	kindOther attachmentKind = iota // only a link
	kindImage                       // image content
	kindText                        // text content
)

// Inline caps. A bigger image only gets its link; a bigger text file is cut, with the link.
// Text stays small because MCP clients cap how much of a tool's output the model reads.
const (
	maxInlineImageBytes int64 = 4 << 20
	maxInlineTextBytes  int64 = 64 << 10
	bytesPerKiB               = 1 << 10
	bytesPerMiB               = 1 << 20
)

// Image types every MCP client can show.
var inlineImageTypes = []string{"image/png", "image/jpeg", "image/gif", "image/webp"}

// Text types besides text/*.
var inlineTextTypes = []string{"application/json", "application/xml", "application/x-ndjson", "application/yaml",
	"application/x-yaml", "application/sql", "application/csv"}

const textTypePrefix = "text/"

// classify reads a stored content type (parameters such as charset are ignored).
func classify(contentType string) attachmentKind {
	mt, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(contentType)), ";")
	mt = strings.TrimSpace(mt)
	switch {
	case slices.Contains(inlineImageTypes, mt):
		return kindImage
	case strings.HasPrefix(mt, textTypePrefix), slices.Contains(inlineTextTypes, mt):
		return kindText
	}
	return kindOther
}

// inlineLimit is how many bytes of a kind read_attachment returns; 0 for a link only.
func inlineLimit(k attachmentKind) int64 {
	switch k {
	case kindImage:
		return maxInlineImageBytes
	case kindText:
		return maxInlineTextBytes
	}
	return 0
}

// humanBytes writes a size the way the app does: bytes, KB or MB.
func humanBytes(n int64) string {
	switch {
	case n >= bytesPerMiB:
		return fmt.Sprintf("%.1f MB", float64(n)/bytesPerMiB)
	case n >= bytesPerKiB:
		return fmt.Sprintf("%d KB", n/bytesPerKiB)
	}
	return fmt.Sprintf("%d bytes", n)
}
