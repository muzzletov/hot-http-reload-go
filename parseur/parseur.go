package parseur

import (
	"strings"
)

type Offset struct {
	Start int
	End   int
}

type Tag struct {
	Start      int
	Name       string
	Attributes map[string]string
	Body       Offset
	Children   []*Tag
	Namespace  string
}

type Parser struct {
	tags         []*Tag
	body         string
	current      *Tag
	namespaceTag *Tag
	namespaces   map[string]string
	length       int
	lastIndex    int
	html         bool
}

func NewParser(body string) *Parser {
	parser := &Parser{
		body:       body,
		namespaces: make(map[string]string),
		length:     len(body),
		tags:       make([]*Tag, 0),
	}
	parser.lastIndex = 0
	index := parser.consumeNamespaceTag(parser.skipWhitespace(0))
	if index == -1 {
		index = 0
	}
	parser.consumeTag(index)

	return parser
}

func (p *Parser) GetSize() int {
	return p.length
}

func (p *Parser) GetTags() []*Tag {
	return p.tags
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
		println(p.body)
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

func (p *Parser) tagEnd(index int) int {

	isOutOfBounds := index == -1 || index >= p.length
	isNotEndTagOrOutOfBounds := isOutOfBounds || p.body[index] != '<' || p.body[index+1] != '/'

	if isNotEndTagOrOutOfBounds {
		return -1
	}

	length := len(p.current.Name) + index + 2

	for i := 0; i+index+2 < length; i++ {
		if p.body[index+i+2] != p.current.Name[i] {
			return -1
		}
	}

	if p.body[length] != '>' {
		return -1
	}

	return p.updatePointer(length + 1)
}

func (p *Parser) isMETAorLINKtag(t *Tag) bool {
	return t.Name == "meta" || t.Name == "link"
}

func (p *Parser) consumeTag(index int) int {
	currentIndex := p.skipWhitespace(index)
	isOutOfBoundsOrNotStartOfTag := currentIndex == -1 ||
		currentIndex >= p.length ||
		p.body[currentIndex] != '<'

	if isOutOfBoundsOrNotStartOfTag {
		return -1
	}

	currentIndex = p.parseTagName(currentIndex + 1)
	p.tags = append(p.tags, p.current)

	if currentIndex == -1 {
		return -1
	}

	isEndOfTag := p.length > currentIndex+1 && p.body[currentIndex] == '/' && p.body[currentIndex+1] == '>'

	if p.html && p.isMETAorLINKtag(p.current) {
		currentIndex += 1
	} else if isEndOfTag {
		currentIndex += 2
	} else if p.body[currentIndex] == '>' {
		currentIndex = p.parseBody(currentIndex + 1)
		currentIndex = p.tagEnd(currentIndex)
		currentIndex = p.skipWhitespace(currentIndex)

		if currentIndex == -1 {
			return -1
		}
	} else {
		return -1
	}

	current := p.tags[len(p.tags)-1]

	if len(p.tags) > 0 {
		p.current = p.tags[len(p.tags)-1]
		p.current.Children = append(p.current.Children, current)
	}

	return p.updatePointer(currentIndex)
}

func (p *Parser) LastPointer() int {
	return p.lastIndex
}

func (p *Parser) parseTagName(index int) int {
	currentIndex := index
	start := index

	if p.ffLetter(index) == -1 {
		return -1
	}

	index = p.skipValidTag(index)

	current := &Tag{}
	current.Start = start
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

func (p *Parser) parseBody(index int) int {
	index = p.skipWhitespace(index)
	currentIndex := index
	current := p.current
	size := p.length

	if size <= index {
		return -1
	}

	offset := index

	for index > -1 && size > index {

		for p.body[index] != '<' && size > index {
			index++
		}

		if index > size {
			return -1
		}

		currentIndex = index

		if p.end(index) {
			break
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

		if p.end(index) {
			break
		}
	}

	current.Body = Offset{offset, currentIndex}

	p.current = current

	return p.updatePointer(currentIndex)
}

func (p *Parser) end(index int) bool {
	return p.length > index+1 && p.body[index] == '<' && p.body[index+1] == '/'
}

func (p *Parser) consumeComment(index int) int {
	ref := p.body

	if p.length < index+4 {
		return -1
	}

	hasStart := ref[index] == '<' && ref[index+1] == '!' && ref[index+2] == '-' && ref[index+3] == '-'

	if hasStart {
		terminated := false
		for ; index+1 < p.length && !terminated; index++ {
			terminated = ref[index] == '-' && ref[index+1] == '-' && ref[index+2] == '>'
		}
	} else {
		return -1
	}

	return index

}

func (p *Parser) parseAttributes(index int) int {
	currentIndex := index
	for currentIndex != -1 {
		var namespace *string = nil
		currentIndex = p.skipValidTag(currentIndex)

		if currentIndex == -1 {
			return -1
		}

		name := p.body[index:currentIndex]

		if p.body[currentIndex] == '>' {
			p.current.Attributes[name] = p.body[index:currentIndex]
			return p.updatePointer(currentIndex)
		}

		if name == "xmlns" &&
			p.body[currentIndex] == ':' {
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

		if p.body[currentIndex] != '=' &&
			p.body[currentIndex+1] != '"' {
			return -1
		}

		currentIndex = currentIndex + 2
		index = currentIndex

		for p.body[currentIndex] != '"' {
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
	if index < p.length && p.isAlpha(index) {
		return p.updatePointer(index + 1)
	}
	return -1
}

func (p *Parser) isAlpha(index int) bool {
	r := p.body[index]
	return ('A' <= r && r <= 'Z') || ('a' <= r && r <= 'z')
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
