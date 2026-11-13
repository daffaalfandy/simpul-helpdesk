package tickets

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"
)

var (
	pngHead  = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")
	jpegHead = []byte("\xFF\xD8\xFF\xE0\x00\x10JFIF")
	pdfHead  = []byte("%PDF-1.7\n")
	zipHead  = []byte("PK\x03\x04\x14\x00\x06\x00")
	oleHead  = append([]byte("\xD0\xCF\x11\xE0\xA1\xB1\x1A\xE1"), make([]byte, 24)...)
	textHead = []byte("2026-11-12 08:14 ERROR login failed\n")
	htmlHead = []byte("<html><script>alert(1)</script></html>")
	exeHead  = []byte("MZ\x90\x00\x03\x00")
)

func TestClassify(t *testing.T) {
	tests := []struct {
		name string
		head []byte
		want string
		err  error
	}{
		{"screenshot.png", pngHead, ctPNG, nil},
		{"SCREEN.PNG", pngHead, ctPNG, nil},
		{"photo.jpeg", jpegHead, ctJPEG, nil},
		{"letter.pdf", pdfHead, ctPDF, nil},
		{"report.docx", zipHead, ctDOCX, nil},
		{"budget.xlsx", zipHead, ctXLSX, nil},
		{"logs.zip", zipHead, ctZIP, nil},
		{"old.doc", oleHead, ctDOC, nil},
		{"old.xls", oleHead, ctXLS, nil},
		{"error.log", textHead, ctText, nil},
		{"data.csv", []byte("name,email\nA,a@b.test\n"), ctCSV, nil},
		{"png named jpg.jpg", pngHead, "", ErrTypeNotAllowed},
		{"zip named doc.doc", zipHead, "", ErrTypeNotAllowed},
		{"html in text.txt", htmlHead, "", ErrTypeNotAllowed},
		{"exe named pdf.pdf", exeHead, "", ErrTypeNotAllowed},
		{"drawing.svg", []byte("<svg xmlns='http://www.w3.org/2000/svg'/>"), "", ErrTypeNotAllowed},
		{"setup.exe", exeHead, "", ErrTypeNotAllowed},
		{"a.pdf.exe", exeHead, "", ErrTypeNotAllowed},
		{"noextension", pdfHead, "", ErrTypeNotAllowed},
		{"empty.png", nil, "", ErrEmptyFile},
	}
	for _, tt := range tests {
		got, err := Classify(tt.name, tt.head)
		if got != tt.want || !errors.Is(err, tt.err) {
			t.Errorf("Classify(%q) = %q, %v; want %q, %v", tt.name, got, err, tt.want, tt.err)
		}
	}
	exts := Extensions()
	if len(exts) != len(allowedTypes) || exts[0] != ".csv" {
		t.Errorf("Extensions = %v", exts)
	}
}

func TestSanitizeFilename(t *testing.T) {
	long := strings.Repeat("é", 200) + ".pdf" // 404 bytes
	tests := []struct {
		in, want string
		err      error
	}{
		{"screenshot-login-error.png", "screenshot-login-error.png", nil},
		{"C:\\Users\\hendra\\Desktop\\scan.pdf", "scan.pdf", nil},
		{"../../etc/passwd", "passwd", nil},
		{"  spaced .txt  ", "spaced .txt", nil},
		{"tab\tand\nnewline.txt", "tabandnewline.txt", nil},
		{"invoice\u202Efdp.exe", "invoicefdp.exe", nil},
		{"dir/", "", ErrInvalidFilename},
		{"..", "", ErrInvalidFilename},
		{"   ", "", ErrInvalidFilename},
	}
	for _, tt := range tests {
		got, err := SanitizeFilename(tt.in)
		if got != tt.want || !errors.Is(err, tt.err) {
			t.Errorf("SanitizeFilename(%q) = %q, %v; want %q, %v", tt.in, got, err, tt.want, tt.err)
		}
	}
	got, err := SanitizeFilename(long)
	if err != nil || len(got) > MaxFilenameBytes || !strings.HasSuffix(got, ".pdf") || !utf8.ValidString(got) {
		t.Errorf("long name = %q (%d bytes), %v", got, len(got), err)
	}
}
