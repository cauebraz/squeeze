package junit

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
)

var ErrNotJUnit = errors.New("input is not a JUnit XML report: the root element is not testsuites or testsuite")

var bom = []byte("\xef\xbb\xbf")

type Result struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr"`
	Text    string `xml:",chardata"`
}

type Case struct {
	Name         string   `xml:"name,attr"`
	Classname    string   `xml:"classname,attr"`
	File         string   `xml:"file,attr"`
	Line         string   `xml:"line,attr"`
	Failures     []Result `xml:"failure"`
	Errors       []Result `xml:"error"`
	Skipped      []Result `xml:"skipped"`
	FlakyFailure []Result `xml:"flakyFailure"`
	FlakyError   []Result `xml:"flakyError"`
	SystemOut    []string `xml:"system-out"`
	SystemErr    []string `xml:"system-err"`
}

type Suite struct {
	XMLName   xml.Name
	Name      string   `xml:"name,attr"`
	Tests     string   `xml:"tests,attr"`
	Failures  string   `xml:"failures,attr"`
	Errors    string   `xml:"errors,attr"`
	Skipped   string   `xml:"skipped,attr"`
	Cases     []Case   `xml:"testcase"`
	Suites    []Suite  `xml:"testsuite"`
	SystemOut []string `xml:"system-out"`
	SystemErr []string `xml:"system-err"`
}

func trim(input []byte) []byte {
	return bytes.TrimLeft(bytes.TrimPrefix(bytes.TrimLeft(input, " \t\r\n"), bom), " \t\r\n")
}

func decoder(input []byte) *xml.Decoder {
	d := xml.NewDecoder(bytes.NewReader(input))
	d.CharsetReader = func(_ string, r io.Reader) (io.Reader, error) { return r, nil }
	return d
}

func rootElement(name xml.Name) bool {
	return name.Local == "testsuites" || name.Local == "testsuite"
}

func detect(head []byte) bool {
	trimmed := trim(head)
	if len(trimmed) == 0 || trimmed[0] != '<' {
		return false
	}
	d := decoder(trimmed)
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

func Parse(input []byte) (*Suite, error) {
	if !detect(input) {
		return nil, ErrNotJUnit
	}
	var root Suite
	if err := decoder(trim(input)).Decode(&root); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	return &root, nil
}
