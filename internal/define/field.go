package define

import "strings"

// FieldDOMTags is shared with the browser bridge. Bare known HTML tag names are
// selector expressions; other bare words are values, not substring tag matches.
const FieldDOMTags = "html base head link meta style title body address article aside footer header h1 h2 h3 h4 h5 h6 main nav section blockquote dd div dl dt figcaption figure hr li menu ol p pre ul a abbr b bdi bdo br cite code data dfn em i kbd mark q rp rt ruby s samp small span strong sub sup time u var wbr area audio img map track video embed iframe object param picture portal source svg math canvas noscript script del ins caption col colgroup table tbody td tfoot th thead tr button datalist fieldset form input label legend meter optgroup option output progress select textarea details dialog summary slot template acronym applet bgsound big blink center content dir font frame frameset image keygen marquee menuitem nobr noembed noframes plaintext rb rtc shadow spacer strike tt xmp"

type FieldExpression struct {
	Value    string
	Selector bool
	Explicit bool
}

// ParseFieldExpression retains selector-looking legacy strings while making
// literal and selector intent explicit through literal: and css: prefixes.
// Whitespace remains selector-looking for legacy descendant selectors; use
// literal: for values such as "Fixed author" or strings containing CSS syntax.
func ParseFieldExpression(field string) FieldExpression {
	field = strings.TrimSpace(field)
	if strings.HasPrefix(field, "literal:") {
		return FieldExpression{Value: strings.TrimPrefix(field, "literal:"), Explicit: true}
	}
	if strings.HasPrefix(field, "css:") {
		return FieldExpression{Value: strings.TrimSpace(strings.TrimPrefix(field, "css:")), Selector: true, Explicit: true}
	}
	selector := field != "" && (strings.ContainsAny(field, ".#[]:>+~,* \t\r\n\f") || strings.Contains(" "+FieldDOMTags+" ", " "+strings.ToLower(field)+" "))
	return FieldExpression{Value: field, Selector: selector}
}
