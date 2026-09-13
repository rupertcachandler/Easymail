package wbxml

// Tag is a single name↔token entry within a code page.
type Tag struct {
	Token byte
	Name  string
}

// CodePage is one of the 25 EAS code pages defined by MS-ASWBXML.
type CodePage struct {
	ID   byte
	Name string
	Tags []Tag

	byToken map[byte]string
	byName  map[string]byte
}

// Build prepares the lookup maps. It must be called once for each page during
// package init; calling it again is a no-op.
func (p *CodePage) build() {
	if p.byToken != nil {
		return
	}
	p.byToken = make(map[byte]string, len(p.Tags))
	p.byName = make(map[string]byte, len(p.Tags))
	for _, t := range p.Tags {
		p.byToken[t.Token] = t.Name
		p.byName[t.Name] = t.Token
	}
}

var pages = map[byte]*CodePage{}

func registerPage(p *CodePage) {
	p.build()
	pages[p.ID] = p
}

// PageByID returns the registered code page for id, if any.
func PageByID(id byte) (*CodePage, bool) {
	p, ok := pages[id]
	return p, ok
}

// PageByName returns the registered code page with the given name, if any.
func PageByName(name string) (*CodePage, bool) {
	for _, p := range pages {
		if p.Name == name {
			return p, true
		}
	}
	return nil, false
}

// TagByToken returns the tag name for the given (page, token), if any.
func TagByToken(page byte, token byte) (string, bool) {
	p, ok := pages[page]
	if !ok {
		return "", false
	}
	name, ok := p.byToken[token]
	return name, ok
}

// TokenByTag returns the token byte for (page, tag name), if any.
func TokenByTag(page byte, name string) (byte, bool) {
	p, ok := pages[page]
	if !ok {
		return 0, false
	}
	tok, ok := p.byName[name]
	return tok, ok
}

// AllPageIDs returns the IDs of every registered code page.
func AllPageIDs() []byte {
	out := make([]byte, 0, len(pages))
	for id := range pages {
		out = append(out, id)
	}
	return out
}
