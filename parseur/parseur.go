package parseur

import (
	"strings"
)

type Offset struct {
	Start int
	End   int
}

type Tag struct {
	Name       string
	Attributes map[string]string
	Body       Offset
	Children   []*Tag
	Namespace  string
}

type Parser struct {
	tags         []*Tag
	body         string
	root         *Tag
	current      *Tag
	namespaceTag *Tag
	namespaces   map[string]string
	length       int
	lastIndex    int
	html         bool
	success      bool
}

func (p *Parser) First(name string) *Tag {
	for _, tag := range p.GetTags() {
		if tag.Name == name {
			return tag
		}
	}

	return nil
}

func (p *Parser) Filter(name string) []*Tag {
	tags := make([]*Tag, 0)

	for _, tag := range p.GetTags() {
		if tag.Name == name {
			tags = append(tags, tag)
		}
	}

	return tags
}

func (t *Tag) FindAll(name string) []*Tag {
	children := make([]*Tag, 0)

	for _, c := range t.Children {
		if c.Name == name {
			children = append(children, c)
		}

		children = append(children, c.FindAll(name)...)
	}

	return children
}

func NewParser(body *string) *Parser {
	parser := &Parser{
		body:       *body,
		namespaces: make(map[string]string),
		length:     len(*body),
		tags:       make([]*Tag, 0),
	}

	parser.current = &Tag{Children: make([]*Tag, 0), Name: "root"}
	parser.lastIndex = 0
	parser.root = parser.current
	index := parser.consumeNamespaceTag(parser.skipWhitespace(0))
	if index == -1 {
		index = 0
	}

	p := parser.consumeTag(index)

	for p < parser.length && p != -1 {
		p = parser.consumeTag(p)
	}

	parser.success = p != -1

	return parser
}

func (p *Parser) Success() bool {
	return p.success
}

func (p *Parser) GetBody() string {
	return p.body
}

func (p *Parser) GetSize() int {
	return p.length
}

func (p *Parser) GetTags() []*Tag {
	return p.tags
}

func (p *Parser) GetRoot() *Tag {
	return p.root
}

func (p *Parser) parseDoctype(index int) int {
	if p.body[index] != '!' {
		return -1
	}

	index = p.parseTagName(index + 1)

	if index == -1 {
		return -1
	}

	isNamespaceTag := p.body[index] == '>'

	if isNamespaceTag {
		p.namespaceTag = p.current
	}

	p.html = strings.ToLower(p.current.Name) == "doctype" && p.current.Attributes["html"] == "html"

	return p.updatePointer(index + 1)
}

func (p *Parser) consumeNamespaceTag(index int) int {
	currentIndex := p.skipWhitespace(index)
	if len(p.body)-1 < currentIndex {
		return -1
	}

	if p.body[currentIndex] != '<' {
		return -1
	}

	currentIndex++

	if p.body[currentIndex] != '?' {
		return p.parseDoctype(currentIndex)
	}

	currentIndex++

	currentIndex = p.parseTagName(currentIndex)

	if currentIndex == -1 {
		return -1
	}

	isNamespaceTag := p.body[currentIndex] == '?' && p.body[currentIndex+1] == '>'

	if isNamespaceTag {
		p.namespaceTag = p.current
	}

	return p.updatePointer(currentIndex + 2)
}

func (p *Parser) updatePointer(currentIndex int) int {
	if p.lastIndex < currentIndex {
		p.lastIndex = currentIndex
	}

	return currentIndex
}

func (p *Parser) isWhitespace(index int) bool {
	r := p.body[index]
	return r == ' ' || r == '\t' || r == '\n'
}

func (p *Parser) skipWhitespace(index int) int {
	if index == -1 {
		return index
	}

	for index < p.length && p.isWhitespace(index) {
		index++
	}

	return p.updatePointer(index)
}

func (p *Parser) parseTagEnd(index int, name string) int {

	isOutOfBounds := index == -1 || index >= p.length
	isNotEndTagOrOutOfBounds := isOutOfBounds || p.body[index] != '<' || p.body[index+1] != '/'

	if isNotEndTagOrOutOfBounds {
		return -1
	}

	length := len(name) + index + 2

	for i := 0; i+index+2 < length; i++ {
		if p.body[index+i+2] != name[i] {
			return -1
		}
	}

	if p.body[length] != '>' {
		return -1
	}

	return p.updatePointer(length + 1)
}

func (p *Parser) doesNotNeedEscape(t *Tag) bool {
	return t.Name == "meta" ||
		t.Name == "link" ||
		t.Name == "img" ||
		t.Name == "input" ||
		t.Name == "br" ||
		t.Name == "hr"
}

func (p *Parser) consumeTag(index int) int {
	currentIndex := p.skipWhitespace(index)
	parent := p.current
	isOutOfBoundsOrNotStartOfTag := currentIndex == -1 ||
		currentIndex >= p.length ||
		p.body[currentIndex] != '<'

	if isOutOfBoundsOrNotStartOfTag {
		return -1
	}

	currentIndex = p.parseTagName(currentIndex + 1)
	self := p.current

	if currentIndex == -1 {
		return -1
	}

	isEndOfTag := p.length > currentIndex+1 && p.body[currentIndex] == '/' && p.body[currentIndex+1] == '>'

	if isEndOfTag {
		currentIndex += 2
	} else if p.html && p.doesNotNeedEscape(self) {
		currentIndex = p.skipWhitespace(currentIndex) + 1
	} else if p.body[currentIndex] == '>' {
		if self.Name != "script" {
			currentIndex = p.parseRegularBody(currentIndex)
		} else {
			currentIndex = p.ffScriptBody(currentIndex)
		}

		if currentIndex == -1 {
			return -1
		}
	} else {
		return -1
	}

	parent.Children = append(parent.Children, self)

	p.tags = append(p.tags, self)
	p.current = parent

	return p.updatePointer(currentIndex)
}

func (p *Parser) ffScriptBody(index int) int {
	size := p.length
	start := index
	for index > -1 && size > index {

		for size > index && p.body[index] != '<' {
			index++
		}

		if size < index+9 {
			return -1
		}

		if p.body[index:index+8] == "</script" {
			k := p.skipWhitespace(index + 8)

			if k == -1 || p.body[k] != '>' {
				index += 1
				continue
			}

			p.current.Body = Offset{start + 1, index}

			return k + 1
		}

		index += 1
	}

	return -1
}

func (p *Parser) parseRegularBody(index int) int {
	return p.skipWhitespace(p.parseBody(index + 1))
}

func (p *Parser) LastPointer() int {
	return p.lastIndex
}

func (p *Parser) parseTagName(index int) int {
	currentIndex := index

	if p.ffLetter(index) == -1 {
		return -1
	}

	if p.body[index] == '"' || p.body[index] == '\'' {
		index = p.checkForLiteral(index, p.body[index])
	} else {
		index = p.skipValidTag(index)
	}

	current := &Tag{}

	if p.body[index] == ':' {
		current.Namespace = p.body[currentIndex:index]
		currentIndex = index + 1
		index = p.skipValidTag(index + 1)
	}

	current.Name = p.body[currentIndex:index]
	p.current = current
	current.Attributes = make(map[string]string)
	currentIndex = p.skipWhitespace(index)

	if currentIndex != index {
		currentIndex = p.parseAttributes(currentIndex)
	}

	return p.updatePointer(currentIndex)
}

func (p *Parser) ff(index int) int {
	for p.length > index && p.body[index] != '<' {
		index++
	}

	return index
}

func (p *Parser) parseBody(index int) int {
	index = p.skipWhitespace(index)
	currentIndex := index
	self := p.current
	size := p.length

	if size <= index {
		return -1
	}

	offset := index

	for index > -1 && size > index {
		index = p.ff(index)

		if index > size {
			return -1
		}

		currentIndex = index

		i := p.parseTagEnd(index, self.Name)

		if i != -1 {
			return i
		}

		index = p.consumeComment(index)

		if index == -1 {
			index = p.consumeTag(currentIndex)
		}

		if index == -1 {
			index = currentIndex + 1
		}

		index = p.skipWhitespace(index)
		currentIndex = index

		i = p.parseTagEnd(index, self.Name)

		if i != -1 {
			return i
		}
	}

	p.current.Body = Offset{offset, currentIndex}

	return p.updatePointer(currentIndex)
}

func (p *Parser) consumeComment(index int) int {
	ref := p.body

	if p.length < index+7 {
		return -1
	}

	isStart := ref[index] == '<' && ref[index+1] == '!' && ref[index+2] == '-' && ref[index+3] == '-'

	if !isStart {
		return -1
	}

	terminated := false

	for ; index+1 < p.length && !terminated; index++ {
		terminated = ref[index] == '-' && ref[index+1] == '-' && ref[index+2] == '>'
	}

	return index
}

func (p *Parser) parseAttributes(index int) int {
	currentIndex := index
	for currentIndex != -1 {
		var namespace *string = nil
		var literal = p.body[currentIndex]
		var isAttrLiteral = literal == '"' ||
			literal == '\''

		if isAttrLiteral {
			currentIndex = p.checkForLiteral(currentIndex, p.body[currentIndex])
		} else {
			currentIndex = p.skipValidTag(currentIndex)
		}

		if currentIndex == -1 {
			return -1
		}

		name := p.body[index:currentIndex]

		if p.body[currentIndex] == '>' {
			p.current.Attributes[name] = p.body[index:currentIndex]
			return p.updatePointer(currentIndex)
		}

		isNamespaceAttr :=
			name == "xmlns" &&
				p.body[currentIndex] == ':'

		if isNamespaceAttr {
			index = currentIndex + 1
			currentIndex = p.skipValidTag(currentIndex + 1)
			temp := p.body[index:currentIndex]
			namespace = &temp
		}

		if p.isWhitespace(currentIndex) {
			p.current.Attributes[name] = p.body[index:currentIndex]
			currentIndex = p.skipWhitespace(currentIndex + 1)
			continue
		}

		if p.body[currentIndex] != '=' {
			return -1
		}

		literal = p.body[currentIndex+1]
		isAttrLiteral = literal != '"' &&
			literal != '\''

		if isAttrLiteral {
			return -1
		}

		currentIndex = currentIndex + 2
		index = currentIndex

		for p.body[currentIndex] != literal {
			currentIndex++
		}

		if namespace != nil {
			p.namespaces[*namespace] = p.body[index:currentIndex]
		} else {
			p.current.Attributes[name] = p.body[index:currentIndex]
		}

		currentIndex = p.skipWhitespace(currentIndex + 1)
		index = currentIndex

		if p.body[currentIndex] == '?' ||
			p.body[currentIndex] == '>' ||
			p.body[currentIndex] == '/' && p.body[currentIndex+1] == '>' {
			return p.updatePointer(index)
		}
	}

	return p.updatePointer(currentIndex)
}

func (p *Parser) ffLetter(index int) int {
	if index >= p.length || !p.isAlpha(index) {
		return -1
	}

	return p.updatePointer(index + 1)
}

func (p *Parser) isAlpha(index int) bool {
	r := p.body[index]

	return ('A' <= r && r <= 'Z') || ('a' <= r && r <= 'z')
}

func (p *Parser) checkForLiteral(index int, literal uint8) int {
	r := p.body[index]

	if r != literal {
		return -1
	}

	for index += 1; index < p.length && p.body[index] != 34; {
		index++
	}

	index += 1

	return p.updatePointer(index)
}

func (p *Parser) skipValidTag(index int) int {
	if !p.isValidTagStart(index) {
		return -1
	}

	index += 1

	for index < p.length && p.isValidTagChar(index) {
		index++
	}

	return p.updatePointer(index)
}

func (p *Parser) isValidTagStart(index int) bool {
	r := p.body[index]

	return ('A' <= r && r <= 'Z') || ('a' <= r && r <= 'z')
}

func (p *Parser) isValidTagChar(index int) bool {
	r := p.body[index]

	return ('0' <= r && r <= '9') || ('A' <= r && r <= 'Z') || ('a' <= r && r <= 'z') || (r == '-')
}
