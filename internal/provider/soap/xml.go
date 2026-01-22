package soap

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
)

// XMLParser provides utilities for parsing and manipulating XML documents.
// It is designed to handle the various XML formats used by Turkish cargo carriers.
type XMLParser struct {
	// StripNamespaces removes namespace prefixes when parsing.
	StripNamespaces bool
	// TrimWhitespace trims whitespace from text content.
	TrimWhitespace bool
}

// NewXMLParser creates a new XMLParser with default settings.
func NewXMLParser() *XMLParser {
	return &XMLParser{
		StripNamespaces: true,
		TrimWhitespace:  true,
	}
}

// Parse parses XML data into a generic map structure.
func (p *XMLParser) Parse(data []byte) (map[string]interface{}, error) {
	if p.StripNamespaces {
		data = p.stripNamespaces(data)
	}

	decoder := xml.NewDecoder(bytes.NewReader(data))
	return p.parseElement(decoder)
}

// ParseInto parses XML data into a struct.
func (p *XMLParser) ParseInto(data []byte, v interface{}) error {
	if p.StripNamespaces {
		data = p.stripNamespaces(data)
	}
	return xml.Unmarshal(data, v)
}

// ExtractElement extracts a specific element from XML data.
func (p *XMLParser) ExtractElement(data []byte, elementName string) ([]byte, error) {
	if p.StripNamespaces {
		data = p.stripNamespaces(data)
	}

	decoder := xml.NewDecoder(bytes.NewReader(data))
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if start, ok := token.(xml.StartElement); ok {
			localName := start.Name.Local
			if localName == elementName {
				var buf bytes.Buffer
				encoder := xml.NewEncoder(&buf)
				if err := p.copyElement(decoder, encoder, start); err != nil {
					return nil, err
				}
				encoder.Flush()
				return buf.Bytes(), nil
			}
		}
	}

	return nil, fmt.Errorf("element %s not found", elementName)
}

// ExtractText extracts the text content of a specific element.
func (p *XMLParser) ExtractText(data []byte, elementName string) (string, error) {
	if p.StripNamespaces {
		data = p.stripNamespaces(data)
	}

	decoder := xml.NewDecoder(bytes.NewReader(data))
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		if start, ok := token.(xml.StartElement); ok {
			localName := start.Name.Local
			if localName == elementName {
				text, err := p.readText(decoder)
				if err != nil {
					return "", err
				}
				if p.TrimWhitespace {
					text = strings.TrimSpace(text)
				}
				return text, nil
			}
		}
	}

	return "", fmt.Errorf("element %s not found", elementName)
}

// ExtractAllText extracts all text content from elements with the given name.
func (p *XMLParser) ExtractAllText(data []byte, elementName string) ([]string, error) {
	if p.StripNamespaces {
		data = p.stripNamespaces(data)
	}

	var results []string
	decoder := xml.NewDecoder(bytes.NewReader(data))
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if start, ok := token.(xml.StartElement); ok {
			localName := start.Name.Local
			if localName == elementName {
				text, err := p.readText(decoder)
				if err != nil {
					return nil, err
				}
				if p.TrimWhitespace {
					text = strings.TrimSpace(text)
				}
				results = append(results, text)
			}
		}
	}

	return results, nil
}

// ExtractAttribute extracts an attribute value from a specific element.
func (p *XMLParser) ExtractAttribute(data []byte, elementName, attrName string) (string, error) {
	if p.StripNamespaces {
		data = p.stripNamespaces(data)
	}

	decoder := xml.NewDecoder(bytes.NewReader(data))
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		if start, ok := token.(xml.StartElement); ok {
			localName := start.Name.Local
			if localName == elementName {
				for _, attr := range start.Attr {
					if attr.Name.Local == attrName {
						return attr.Value, nil
					}
				}
				return "", fmt.Errorf("attribute %s not found in element %s", attrName, elementName)
			}
		}
	}

	return "", fmt.Errorf("element %s not found", elementName)
}

// parseElement recursively parses an XML element into a map.
func (p *XMLParser) parseElement(decoder *xml.Decoder) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		switch t := token.(type) {
		case xml.StartElement:
			name := t.Name.Local
			child, err := p.parseElementContent(decoder, t)
			if err != nil {
				return nil, err
			}

			// Handle multiple elements with the same name
			if existing, ok := result[name]; ok {
				switch v := existing.(type) {
				case []interface{}:
					result[name] = append(v, child)
				default:
					result[name] = []interface{}{v, child}
				}
			} else {
				result[name] = child
			}

		case xml.EndElement:
			return result, nil
		}
	}

	return result, nil
}

// parseElementContent parses the content of an XML element.
func (p *XMLParser) parseElementContent(decoder *xml.Decoder, start xml.StartElement) (interface{}, error) {
	var textContent strings.Builder
	children := make(map[string]interface{})
	hasChildren := false

	// Add attributes
	for _, attr := range start.Attr {
		children["@"+attr.Name.Local] = attr.Value
		hasChildren = true
	}

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		switch t := token.(type) {
		case xml.StartElement:
			hasChildren = true
			name := t.Name.Local
			child, err := p.parseElementContent(decoder, t)
			if err != nil {
				return nil, err
			}

			if existing, ok := children[name]; ok {
				switch v := existing.(type) {
				case []interface{}:
					children[name] = append(v, child)
				default:
					children[name] = []interface{}{v, child}
				}
			} else {
				children[name] = child
			}

		case xml.CharData:
			text := string(t)
			if p.TrimWhitespace {
				text = strings.TrimSpace(text)
			}
			if text != "" {
				textContent.WriteString(text)
			}

		case xml.EndElement:
			if hasChildren {
				if textContent.Len() > 0 {
					children["#text"] = textContent.String()
				}
				return children, nil
			}
			return textContent.String(), nil
		}
	}

	if hasChildren {
		return children, nil
	}
	return textContent.String(), nil
}

// readText reads the text content until the end element.
func (p *XMLParser) readText(decoder *xml.Decoder) (string, error) {
	var text strings.Builder
	depth := 1

	for depth > 0 {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		switch t := token.(type) {
		case xml.CharData:
			text.Write(t)
		case xml.StartElement:
			depth++
		case xml.EndElement:
			depth--
		}
	}

	return text.String(), nil
}

// copyElement copies an element and its content to the encoder.
func (p *XMLParser) copyElement(decoder *xml.Decoder, encoder *xml.Encoder, start xml.StartElement) error {
	if err := encoder.EncodeToken(start); err != nil {
		return err
	}

	depth := 1
	for depth > 0 {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		switch t := token.(type) {
		case xml.StartElement:
			depth++
			if err := encoder.EncodeToken(t); err != nil {
				return err
			}
		case xml.EndElement:
			depth--
			if err := encoder.EncodeToken(t); err != nil {
				return err
			}
		case xml.CharData:
			if err := encoder.EncodeToken(t); err != nil {
				return err
			}
		}
	}

	return nil
}

// stripNamespaces removes namespace prefixes from XML.
var nsRegex = regexp.MustCompile(`(<\/?)[a-zA-Z0-9]+:([a-zA-Z0-9]+)`)
var nsAttrRegex = regexp.MustCompile(`\s+xmlns(:[a-zA-Z0-9]+)?="[^"]*"`)

func (p *XMLParser) stripNamespaces(data []byte) []byte {
	// Remove namespace prefixes from elements
	data = nsRegex.ReplaceAll(data, []byte("$1$2"))
	// Remove xmlns attributes
	data = nsAttrRegex.ReplaceAll(data, []byte(""))
	return data
}

// XMLBuilder provides utilities for building XML documents.
type XMLBuilder struct {
	buf    bytes.Buffer
	indent string
	level  int
}

// NewXMLBuilder creates a new XMLBuilder.
func NewXMLBuilder() *XMLBuilder {
	return &XMLBuilder{
		indent: "  ",
	}
}

// SetIndent sets the indentation string.
func (b *XMLBuilder) SetIndent(indent string) {
	b.indent = indent
}

// WriteHeader writes the XML declaration.
func (b *XMLBuilder) WriteHeader() {
	b.buf.WriteString(xml.Header)
}

// StartElement writes a start element tag.
func (b *XMLBuilder) StartElement(name string, attrs ...Attr) {
	b.writeIndent()
	b.buf.WriteString("<")
	b.buf.WriteString(name)
	for _, attr := range attrs {
		b.buf.WriteString(" ")
		b.buf.WriteString(attr.Name)
		b.buf.WriteString(`="`)
		b.buf.WriteString(escapeXML(attr.Value))
		b.buf.WriteString(`"`)
	}
	b.buf.WriteString(">")
	b.level++
}

// EndElement writes an end element tag.
func (b *XMLBuilder) EndElement(name string) {
	b.level--
	b.writeIndent()
	b.buf.WriteString("</")
	b.buf.WriteString(name)
	b.buf.WriteString(">")
}

// WriteElement writes a complete element with text content.
func (b *XMLBuilder) WriteElement(name, value string, attrs ...Attr) {
	b.writeIndent()
	b.buf.WriteString("<")
	b.buf.WriteString(name)
	for _, attr := range attrs {
		b.buf.WriteString(" ")
		b.buf.WriteString(attr.Name)
		b.buf.WriteString(`="`)
		b.buf.WriteString(escapeXML(attr.Value))
		b.buf.WriteString(`"`)
	}
	if value == "" {
		b.buf.WriteString("/>")
	} else {
		b.buf.WriteString(">")
		b.buf.WriteString(escapeXML(value))
		b.buf.WriteString("</")
		b.buf.WriteString(name)
		b.buf.WriteString(">")
	}
}

// WriteElementInt writes an element with an integer value.
func (b *XMLBuilder) WriteElementInt(name string, value int) {
	b.WriteElement(name, fmt.Sprintf("%d", value))
}

// WriteElementInt64 writes an element with an int64 value.
func (b *XMLBuilder) WriteElementInt64(name string, value int64) {
	b.WriteElement(name, fmt.Sprintf("%d", value))
}

// WriteElementFloat writes an element with a float value.
func (b *XMLBuilder) WriteElementFloat(name string, value float64, precision int) {
	b.WriteElement(name, fmt.Sprintf("%.*f", precision, value))
}

// WriteElementBool writes an element with a boolean value.
func (b *XMLBuilder) WriteElementBool(name string, value bool) {
	if value {
		b.WriteElement(name, "true")
	} else {
		b.WriteElement(name, "false")
	}
}

// WriteElementTime writes an element with a time value.
func (b *XMLBuilder) WriteElementTime(name string, value time.Time, format string) {
	b.WriteElement(name, value.Format(format))
}

// WriteRaw writes raw XML content.
func (b *XMLBuilder) WriteRaw(content string) {
	b.buf.WriteString(content)
}

// WriteCDATA writes a CDATA section.
func (b *XMLBuilder) WriteCDATA(content string) {
	b.buf.WriteString("<![CDATA[")
	b.buf.WriteString(content)
	b.buf.WriteString("]]>")
}

// WriteComment writes an XML comment.
func (b *XMLBuilder) WriteComment(comment string) {
	b.writeIndent()
	b.buf.WriteString("<!-- ")
	b.buf.WriteString(comment)
	b.buf.WriteString(" -->")
}

// Bytes returns the built XML as bytes.
func (b *XMLBuilder) Bytes() []byte {
	return b.buf.Bytes()
}

// String returns the built XML as a string.
func (b *XMLBuilder) String() string {
	return b.buf.String()
}

// Reset resets the builder.
func (b *XMLBuilder) Reset() {
	b.buf.Reset()
	b.level = 0
}

func (b *XMLBuilder) writeIndent() {
	if b.indent != "" && b.level > 0 {
		b.buf.WriteString("\n")
		for i := 0; i < b.level; i++ {
			b.buf.WriteString(b.indent)
		}
	}
}

// Attr represents an XML attribute.
type Attr struct {
	Name  string
	Value string
}

// escapeXML escapes special XML characters.
func escapeXML(s string) string {
	var buf bytes.Buffer
	xml.EscapeText(&buf, []byte(s))
	return buf.String()
}

// SOAPEnvelopeBuilder builds SOAP envelopes.
type SOAPEnvelopeBuilder struct {
	version     SOAPVersion
	namespaces  map[string]string
	headerElems []interface{}
	bodyElem    interface{}
}

// NewSOAPEnvelopeBuilder creates a new SOAPEnvelopeBuilder.
func NewSOAPEnvelopeBuilder(version SOAPVersion) *SOAPEnvelopeBuilder {
	return &SOAPEnvelopeBuilder{
		version:    version,
		namespaces: make(map[string]string),
	}
}

// AddNamespace adds a namespace declaration.
func (b *SOAPEnvelopeBuilder) AddNamespace(prefix, uri string) *SOAPEnvelopeBuilder {
	b.namespaces[prefix] = uri
	return b
}

// AddHeader adds a header element.
func (b *SOAPEnvelopeBuilder) AddHeader(elem interface{}) *SOAPEnvelopeBuilder {
	b.headerElems = append(b.headerElems, elem)
	return b
}

// SetBody sets the body element.
func (b *SOAPEnvelopeBuilder) SetBody(elem interface{}) *SOAPEnvelopeBuilder {
	b.bodyElem = elem
	return b
}

// Build builds the SOAP envelope.
func (b *SOAPEnvelopeBuilder) Build() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString(xml.Header)

	// Start envelope
	buf.WriteString(`<soap:Envelope xmlns:soap="`)
	buf.WriteString(b.version.Namespace())
	buf.WriteString(`"`)

	// Add custom namespaces
	for prefix, uri := range b.namespaces {
		buf.WriteString(` xmlns:`)
		buf.WriteString(prefix)
		buf.WriteString(`="`)
		buf.WriteString(uri)
		buf.WriteString(`"`)
	}
	buf.WriteString(`>`)

	// Add header if present
	if len(b.headerElems) > 0 {
		buf.WriteString(`<soap:Header>`)
		for _, elem := range b.headerElems {
			headerBytes, err := xml.Marshal(elem)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal header element: %w", err)
			}
			buf.Write(headerBytes)
		}
		buf.WriteString(`</soap:Header>`)
	}

	// Add body
	buf.WriteString(`<soap:Body>`)
	if b.bodyElem != nil {
		bodyBytes, err := xml.Marshal(b.bodyElem)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal body element: %w", err)
		}
		buf.Write(bodyBytes)
	}
	buf.WriteString(`</soap:Body>`)

	// End envelope
	buf.WriteString(`</soap:Envelope>`)

	return buf.Bytes(), nil
}

// XMLPath provides XPath-like access to XML data.
type XMLPath struct {
	data map[string]interface{}
}

// NewXMLPath creates a new XMLPath from parsed XML data.
func NewXMLPath(data map[string]interface{}) *XMLPath {
	return &XMLPath{data: data}
}

// Get retrieves a value at the given path.
// Path elements are separated by "/".
// Example: "Envelope/Body/Response/Status"
func (p *XMLPath) Get(path string) (interface{}, bool) {
	parts := strings.Split(path, "/")
	current := interface{}(p.data)

	for _, part := range parts {
		if part == "" {
			continue
		}

		switch v := current.(type) {
		case map[string]interface{}:
			val, ok := v[part]
			if !ok {
				return nil, false
			}
			current = val
		default:
			return nil, false
		}
	}

	return current, true
}

// GetString retrieves a string value at the given path.
func (p *XMLPath) GetString(path string) (string, bool) {
	val, ok := p.Get(path)
	if !ok {
		return "", false
	}
	str, ok := val.(string)
	return str, ok
}

// GetInt retrieves an integer value at the given path.
func (p *XMLPath) GetInt(path string) (int, bool) {
	str, ok := p.GetString(path)
	if !ok {
		return 0, false
	}
	var i int
	_, err := fmt.Sscanf(str, "%d", &i)
	return i, err == nil
}

// GetBool retrieves a boolean value at the given path.
func (p *XMLPath) GetBool(path string) (bool, bool) {
	str, ok := p.GetString(path)
	if !ok {
		return false, false
	}
	str = strings.ToLower(str)
	return str == "true" || str == "1" || str == "yes", true
}

// GetArray retrieves an array value at the given path.
func (p *XMLPath) GetArray(path string) ([]interface{}, bool) {
	val, ok := p.Get(path)
	if !ok {
		return nil, false
	}
	arr, ok := val.([]interface{})
	if ok {
		return arr, true
	}
	// Single element, wrap in array
	return []interface{}{val}, true
}

// GetMap retrieves a map value at the given path.
func (p *XMLPath) GetMap(path string) (map[string]interface{}, bool) {
	val, ok := p.Get(path)
	if !ok {
		return nil, false
	}
	m, ok := val.(map[string]interface{})
	return m, ok
}
