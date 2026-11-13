package tickets

import (
	"bytes"
	"errors"
	"mime"
	"net/http"
	"path"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Attachment limits. The byte limit matches the attachments.size_bytes CHECK constraint.
const (
	MaxAttachmentBytes           int64 = 10 << 20 // 10 MiB
	MaxAttachmentsPerMessage           = 5
	MaxPendingAttachmentsPerUser       = 20
	// PendingAttachmentTTL is how long an upload waits to be linked to a ticket or message.
	PendingAttachmentTTL    = 24 * time.Hour
	AttachmentSweepInterval = time.Hour
	// MaxFilenameBytes keeps names within the attachments.filename CHECK (255 characters).
	MaxFilenameBytes = 255
	// SniffBytes is how much of a file content checks look at (http.DetectContentType's limit).
	SniffBytes = 512
)

// Attachment policy errors.
var (
	ErrTypeNotAllowed  = errors.New("this file type is not allowed")
	ErrEmptyFile       = errors.New("the file is empty")
	ErrInvalidFilename = errors.New("the file needs a name")
	ErrFileTooLarge    = errors.New("the file is larger than the limit")
)

// Content types stored for allowed files. Downloads are always served as octet-stream; the
// stored type is informational (icons in the UI).
const (
	ctPNG  = "image/png"
	ctJPEG = "image/jpeg"
	ctGIF  = "image/gif"
	ctWebP = "image/webp"
	ctPDF  = "application/pdf"
	ctZIP  = "application/zip"
	ctText = "text/plain"
	ctCSV  = "text/csv"
	ctDOCX = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	ctXLSX = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	ctPPTX = "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	ctODT  = "application/vnd.oasis.opendocument.text"
	ctODS  = "application/vnd.oasis.opendocument.spreadsheet"
	ctODP  = "application/vnd.oasis.opendocument.presentation"
	ctDOC  = "application/msword"
	ctXLS  = "application/vnd.ms-excel"
	ctPPT  = "application/vnd.ms-powerpoint"
)

// oleMagic starts legacy Office files (.doc, .xls, .ppt). http.DetectContentType has no
// signature for them.
var oleMagic = []byte("\xD0\xCF\x11\xE0\xA1\xB1\x1A\xE1")

// contentCheck says whether a file's first bytes fit its extension.
type contentCheck func(sniffed string, head []byte) bool

func sniffedAs(want string) contentCheck {
	return func(sniffed string, _ []byte) bool { return sniffed == want }
}

func isOLE(_ string, head []byte) bool { return bytes.HasPrefix(head, oleMagic) }

type fileType struct {
	contentType string
	check       contentCheck
}

// allowedTypes is the upload allowlist by lower-case extension: screenshots, documents,
// spreadsheets, archives and logs. SVG and HTML (scriptable), executables and anything else
// are refused. Office Open XML and OpenDocument files are zip archives.
var allowedTypes = map[string]fileType{
	".png":  {ctPNG, sniffedAs(ctPNG)},
	".jpg":  {ctJPEG, sniffedAs(ctJPEG)},
	".jpeg": {ctJPEG, sniffedAs(ctJPEG)},
	".gif":  {ctGIF, sniffedAs(ctGIF)},
	".webp": {ctWebP, sniffedAs(ctWebP)},
	".pdf":  {ctPDF, sniffedAs(ctPDF)},
	".zip":  {ctZIP, sniffedAs(ctZIP)},
	".docx": {ctDOCX, sniffedAs(ctZIP)},
	".xlsx": {ctXLSX, sniffedAs(ctZIP)},
	".pptx": {ctPPTX, sniffedAs(ctZIP)},
	".odt":  {ctODT, sniffedAs(ctZIP)},
	".ods":  {ctODS, sniffedAs(ctZIP)},
	".odp":  {ctODP, sniffedAs(ctZIP)},
	".doc":  {ctDOC, isOLE},
	".xls":  {ctXLS, isOLE},
	".ppt":  {ctPPT, isOLE},
	".txt":  {ctText, sniffedAs(ctText)},
	".log":  {ctText, sniffedAs(ctText)},
	".csv":  {ctCSV, sniffedAs(ctText)},
}

// Extensions lists the allowed extensions, sorted, for the attachment policy endpoint.
func Extensions() []string {
	out := make([]string, 0, len(allowedTypes))
	for ext := range allowedTypes {
		out = append(out, ext)
	}
	slices.Sort(out)
	return out
}

// Classify checks a file's extension against the allowlist and its first bytes against the
// extension, and returns the content type to store.
func Classify(filename string, head []byte) (string, error) {
	if len(head) == 0 {
		return "", ErrEmptyFile
	}
	ft, ok := allowedTypes[strings.ToLower(path.Ext(filename))]
	if !ok {
		return "", ErrTypeNotAllowed
	}
	sniffed, _, err := mime.ParseMediaType(http.DetectContentType(head))
	if err != nil || !ft.check(sniffed, head) {
		return "", ErrTypeNotAllowed
	}
	return ft.contentType, nil
}

// SanitizeFilename keeps only the base name, drops control and bidirectional-override
// characters (which can disguise "gpj.exe" as "exe.jpg") and trims the name to
// MaxFilenameBytes, keeping the extension.
func SanitizeFilename(name string) (string, error) {
	name = strings.ToValidUTF8(name, "")
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		name = name[i+1:]
	}
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.Is(unicode.Bidi_Control, r) {
			return -1
		}
		return r
	}, name)
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." {
		return "", ErrInvalidFilename
	}
	if len(name) <= MaxFilenameBytes {
		return name, nil
	}
	ext := path.Ext(name)
	if len(ext) >= MaxFilenameBytes {
		return "", ErrInvalidFilename
	}
	base := name[:MaxFilenameBytes-len(ext)]
	for !utf8.ValidString(base) {
		base = base[:len(base)-1]
	}
	return base + ext, nil
}
