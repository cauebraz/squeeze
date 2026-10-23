package junit

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

var errNotJUnit = errors.New("input is not a JUnit XML report: the root element is not testsuites or testsuite")

var bom = []byte("\xef\xbb\xbf")

type rawResult struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr"`
	Text    string `xml:",chardata"`
}

type rawCase struct {
	Name         string      `xml:"name,attr"`
	Classname    string      `xml:"classname,attr"`
	File         string      `xml:"file,attr"`
	Line         string      `xml:"line,attr"`
	Failures     []rawResult `xml:"failure"`
	Errors       []rawResult `xml:"error"`
	Skipped      []rawResult `xml:"skipped"`
	FlakyFailure []rawResult `xml:"flakyFailure"`
	FlakyError   []rawResult `xml:"flakyError"`
	SystemOut    []string    `xml:"system-out"`
	SystemErr    []string    `xml:"system-err"`
}

type rawSuite struct {
	XMLName   xml.Name
	Name      string     `xml:"name,attr"`
	Tests     string     `xml:"tests,attr"`
	Failures  string     `xml:"failures,attr"`
	Errors    string     `xml:"errors,attr"`
	Skipped   string     `xml:"skipped,attr"`
	Cases     []rawCase  `xml:"testcase"`
	Suites    []rawSuite `xml:"testsuite"`
	SystemOut []string   `xml:"system-out"`
	SystemErr []string   `xml:"system-err"`
}

func trim(input []byte) []byte {
	return bytes.TrimLeft(bytes.TrimPrefix(bytes.TrimLeft(input, " \t\r\n"), bom), " \t\r\n")
}

func decoder(input []byte, charset func(string, io.Reader) (io.Reader, error)) *xml.Decoder {
	d := xml.NewDecoder(bytes.NewReader(input))
	d.CharsetReader = charset
	return d
}

func anyCharset(_ string, r io.Reader) (io.Reader, error) { return r, nil }

func convertCharset(label string, r io.Reader) (io.Reader, error) {
	switch strings.ToLower(label) {
	case "us-ascii", "ascii":
		return r, nil
	case "iso-8859-1", "iso8859-1", "iso_8859-1", "latin1", "latin-1", "l1":
		data, err := io.ReadAll(r)
		if err != nil {
			return nil, err
		}
		out := make([]byte, 0, len(data)*2)
		for _, c := range data {
			out = utf8.AppendRune(out, rune(c))
		}
		return bytes.NewReader(out), nil
	}
	return nil, fmt.Errorf("unsupported encoding %q: squeeze reads UTF-8, US-ASCII and ISO-8859-1", label)
}

func rootElement(name xml.Name) bool {
	return name.Local == "testsuites" || name.Local == "testsuite"
}

func detect(head []byte) bool {
	trimmed := trim(head)
	if len(trimmed) == 0 || trimmed[0] != '<' {
		return false
	}
	d := decoder(trimmed, anyCharset)
	for {
		tok, err := d.RawToken()
		if err != nil {
			return false
		}
		switch t := tok.(type) {
		case xml.StartElement:
			return rootElement(t.Name)
		case xml.CharData:
			if len(bytes.TrimSpace(t)) > 0 {
				return false
			}
		}
	}
}

func parse(input []byte) (*rawSuite, error) {
	if !detect(input) {
		return nil, errNotJUnit
	}
	var root rawSuite
	if err := decoder(trim(input), convertCharset).Decode(&root); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	return &root, nil
}
