// Package tgfmt implements typed HTML formatting for Telegram messages.
package tgfmt

import "html"

// HTML is escaped text or trusted markup. Dynamic input must use Escape.
type HTML string

func (h HTML) String() string { return string(h) }

func Escape(text string) HTML { return HTML(html.EscapeString(text)) }
