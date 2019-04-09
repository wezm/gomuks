// gomuks - A terminal Matrix client written in Go.
// Copyright (C) 2019 Tulir Asokan
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package html

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/alecthomas/chroma"
	"github.com/alecthomas/chroma/lexers"
	"github.com/alecthomas/chroma/styles"
	"github.com/lucasb-eyer/go-colorful"
	"golang.org/x/net/html"

	"maunium.net/go/mautrix"
	"maunium.net/go/tcell"

	"maunium.net/go/gomuks/matrix/rooms"
	"maunium.net/go/gomuks/ui/widget"
)

var matrixToURL = regexp.MustCompile("^(?:https?://)?(?:www\\.)?matrix\\.to/#/([#@!].*)")

type htmlParser struct {
	room *rooms.Room

	keepLinebreak bool
}

func AdjustStyleBold(style tcell.Style) tcell.Style {
	return style.Bold(true)
}

func AdjustStyleItalic(style tcell.Style) tcell.Style {
	return style.Italic(true)
}

func AdjustStyleUnderline(style tcell.Style) tcell.Style {
	return style.Underline(true)
}

func AdjustStyleStrikethrough(style tcell.Style) tcell.Style {
	return style.Strikethrough(true)
}

func AdjustStyleTextColor(color tcell.Color) func(tcell.Style) tcell.Style {
	return func(style tcell.Style) tcell.Style {
		return style.Foreground(color)
	}
}

func AdjustStyleBackgroundColor(color tcell.Color) func(tcell.Style) tcell.Style {
	return func(style tcell.Style) tcell.Style {
		return style.Background(color)
	}
}

func (parser *htmlParser) getAttribute(node *html.Node, attribute string) string {
	for _, attr := range node.Attr {
		if attr.Key == attribute {
			return attr.Val
		}
	}
	return ""
}

func (parser *htmlParser) listToEntity(node *html.Node) Entity {
	children := parser.nodeToEntities(node.FirstChild)
	ordered := node.Data == "ol"
	start := 1
	if ordered {
		if startRaw := parser.getAttribute(node, "start"); len(startRaw) > 0 {
			var err error
			start, err = strconv.Atoi(startRaw)
			if err != nil {
				start = 1
			}
		}
	}
	listItems := children[:0]
	for _, child := range children {
		if child.GetTag() == "li" {
			listItems = append(listItems, child)
		}
	}
	return NewListEntity(ordered, start, listItems)
}

func (parser *htmlParser) basicFormatToEntity(node *html.Node) Entity {
	entity := &BaseEntity{
		Tag:      node.Data,
		Children: parser.nodeToEntities(node.FirstChild),
	}
	switch node.Data {
	case "b", "strong":
		entity.AdjustStyle(AdjustStyleBold)
	case "i", "em":
		entity.AdjustStyle(AdjustStyleItalic)
	case "s", "del":
		entity.AdjustStyle(AdjustStyleStrikethrough)
	case "u", "ins":
		entity.AdjustStyle(AdjustStyleUnderline)
	case "font":
		fgColor, ok := parser.parseColor(node, "data-mx-color", "color")
		if ok {
			entity.AdjustStyle(AdjustStyleTextColor(fgColor))
		}
		bgColor, ok := parser.parseColor(node, "data-mx-bg-color", "background-color")
		if ok {
			entity.AdjustStyle(AdjustStyleBackgroundColor(bgColor))
		}
	}
	return entity
}

func (parser *htmlParser) parseColor(node *html.Node, mainName, altName string) (color tcell.Color, ok bool) {
	hex := parser.getAttribute(node, mainName)
	if len(hex) == 0 {
		hex = parser.getAttribute(node, altName)
		if len(hex) == 0 {
			return
		}
	}

	cful, err := colorful.Hex(hex)
	if err != nil {
		color2, found := colorMap[strings.ToLower(hex)]
		if !found {
			return
		}
		cful, _ = colorful.MakeColor(color2)
	}

	r, g, b := cful.RGB255()
	return tcell.NewRGBColor(int32(r), int32(g), int32(b)), true
}

func (parser *htmlParser) headerToEntity(node *html.Node) Entity {
	length := int(node.Data[1] - '0')
	prefix := strings.Repeat("#", length) + " "
	return (&BaseEntity{
		Tag:      node.Data,
		Text:     prefix,
		Children: parser.nodeToEntities(node.FirstChild),
	}).AdjustStyle(AdjustStyleBold)
}

func (parser *htmlParser) blockquoteToEntity(node *html.Node) Entity {
	return NewBlockquoteEntity(parser.nodeToEntities(node.FirstChild))
}

func (parser *htmlParser) linkToEntity(node *html.Node) Entity {
	entity := &BaseEntity{
		Tag:      "a",
		Children: parser.nodeToEntities(node.FirstChild),
	}
	href := parser.getAttribute(node, "href")
	if len(href) == 0 {
		return entity
	}
	match := matrixToURL.FindStringSubmatch(href)
	if len(match) == 2 {
		entity.Children = nil
		pillTarget := match[1]
		entity.Text = pillTarget
		if pillTarget[0] == '@' {
			if member := parser.room.GetMember(pillTarget); member != nil {
				entity.Text = member.Displayname
				entity.Style = entity.Style.Foreground(widget.GetHashColor(pillTarget))
			}
		}
	}
	// TODO add click action and underline on hover for links
	return entity
}

func (parser *htmlParser) imageToEntity(node *html.Node) Entity {
	alt := parser.getAttribute(node, "alt")
	if len(alt) == 0 {
		alt = parser.getAttribute(node, "title")
		if len(alt) == 0 {
			alt = "[inline image]"
		}
	}
	entity := &BaseEntity{
		Tag:  "img",
		Text: alt,
	}
	// TODO add click action and underline on hover for inline images
	return entity
}

func colourToColor(colour chroma.Colour) tcell.Color {
	if !colour.IsSet() {
		return tcell.ColorDefault
	}
	return tcell.NewRGBColor(int32(colour.Red()), int32(colour.Green()), int32(colour.Blue()))
}

func styleEntryToStyle(se chroma.StyleEntry) tcell.Style {
	return tcell.StyleDefault.
		Bold(se.Bold == chroma.Yes).
		Italic(se.Italic == chroma.Yes).
		Underline(se.Underline == chroma.Yes).
		Foreground(colourToColor(se.Colour)).
		Background(colourToColor(se.Background))
}

func (parser *htmlParser) syntaxHighlight(text, language string) Entity {
	lexer := lexers.Get(strings.ToLower(language))
	if lexer == nil {
		return nil
	}
	iter, err := lexer.Tokenise(nil, text)
	if err != nil {
		return nil
	}
	// TODO allow changing theme
	style := styles.SolarizedDark

	tokens := iter.Tokens()
	children := make([]Entity, len(tokens))
	for i, token := range tokens {
		if token.Value == "\n" {
			children[i] = &BaseEntity{Block: true, Tag: "br"}
		} else {
			children[i] = &BaseEntity{
				Tag:   token.Type.String(),
				Text:  token.Value,
				Style: styleEntryToStyle(style.Get(token.Type)),

				DefaultHeight: 1,
			}
		}
	}
	return NewCodeBlockEntity(children, styleEntryToStyle(style.Get(chroma.Background)))
}

func (parser *htmlParser) codeblockToEntity(node *html.Node) Entity {
	lang := "plaintext"
	// TODO allow disabling syntax highlighting
	if node.FirstChild.Type == html.ElementNode && node.FirstChild.Data == "code" {
		node = node.FirstChild
		attr := parser.getAttribute(node, "class")
		for _, class := range strings.Split(attr, " ") {
			if strings.HasPrefix(class, "language-") {
				lang = class[len("language-"):]
				break
			}
		}
	}
	parser.keepLinebreak = true
	text := (&BaseEntity{
		Children: parser.nodeToEntities(node.FirstChild),
	}).PlainText()
	parser.keepLinebreak = false
	return parser.syntaxHighlight(text, lang)
}

func (parser *htmlParser) tagNodeToEntity(node *html.Node) Entity {
	switch node.Data {
	case "blockquote":
		return parser.blockquoteToEntity(node)
	case "ol", "ul":
		return parser.listToEntity(node)
	case "h1", "h2", "h3", "h4", "h5", "h6":
		return parser.headerToEntity(node)
	case "br":
		return NewBreakEntity()
	case "b", "strong", "i", "em", "s", "del", "u", "ins", "font":
		return parser.basicFormatToEntity(node)
	case "a":
		return parser.linkToEntity(node)
	case "img":
		return parser.imageToEntity(node)
	case "pre":
		return parser.codeblockToEntity(node)
	default:
		return &BaseEntity{
			Tag:      node.Data,
			Children: parser.nodeToEntities(node.FirstChild),
			Block:    parser.isBlockTag(node.Data),
		}
	}
}

func (parser *htmlParser) singleNodeToEntity(node *html.Node) Entity {
	switch node.Type {
	case html.TextNode:
		if !parser.keepLinebreak {
			node.Data = strings.ReplaceAll(node.Data, "\n", "")
		}
		return &BaseEntity{
			Tag:  "text",
			Text: node.Data,
		}
	case html.ElementNode:
		return parser.tagNodeToEntity(node)
	case html.DocumentNode:
		if node.FirstChild.Data == "html" && node.FirstChild.NextSibling == nil {
			return parser.singleNodeToEntity(node.FirstChild)
		}
		return &BaseEntity{
			Tag:      "html",
			Children: parser.nodeToEntities(node.FirstChild),
			Block:    true,
		}
	default:
		return nil
	}
}

func (parser *htmlParser) nodeToEntities(node *html.Node) (entities []Entity) {
	for ; node != nil; node = node.NextSibling {
		if entity := parser.singleNodeToEntity(node); entity != nil {
			entities = append(entities, entity)
		}
	}
	return
}

var BlockTags = []string{"p", "h1", "h2", "h3", "h4", "h5", "h6", "ol", "ul", "li", "pre", "blockquote", "div", "hr", "table"}

func (parser *htmlParser) isBlockTag(tag string) bool {
	for _, blockTag := range BlockTags {
		if tag == blockTag {
			return true
		}
	}
	return false
}

func (parser *htmlParser) Parse(htmlData string) Entity {
	node, _ := html.Parse(strings.NewReader(htmlData))
	return parser.singleNodeToEntity(node)
}

const TabLength = 4

// Parse parses a HTML-formatted Matrix event into a UIMessage.
func Parse(room *rooms.Room, evt *mautrix.Event, senderDisplayname string) Entity {
	htmlData := evt.Content.FormattedBody
	if evt.Content.Format != mautrix.FormatHTML {
		htmlData = strings.ReplaceAll(html.EscapeString(evt.Content.Body), "\n", "<br/>")
	}
	htmlData = strings.Replace(htmlData, "\t", strings.Repeat(" ", TabLength), -1)

	parser := htmlParser{room: room}
	root := parser.Parse(htmlData)
	root.(*BaseEntity).Block = false

	if evt.Content.MsgType == mautrix.MsgEmote {
		root = &BaseEntity{
			Tag: "emote",
			Children: []Entity{
				NewTextEntity("* "),
				NewTextEntity(senderDisplayname).AdjustStyle(AdjustStyleTextColor(widget.GetHashColor(evt.Sender))),
				NewTextEntity(" "),
				root,
			},
		}
	}

	return root
}