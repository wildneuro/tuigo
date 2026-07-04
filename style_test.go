package tuigo

import (
	"testing"

	"github.com/wildneuro/tuigo/types"
)

func TestStyleOptionsSetExactlyOneField(t *testing.T) {
	tests := []struct {
		name string
		opt  Option
		want types.Style
	}{
		{"Padding", Padding(2), types.Style{Padding: [4]int{2, 2, 2, 2}}},
		{"PaddingTop", PaddingTop(2), types.Style{Padding: [4]int{2, 0, 0, 0}}},
		{"PaddingRight", PaddingRight(2), types.Style{Padding: [4]int{0, 2, 0, 0}}},
		{"PaddingBottom", PaddingBottom(2), types.Style{Padding: [4]int{0, 0, 2, 0}}},
		{"PaddingLeft", PaddingLeft(2), types.Style{Padding: [4]int{0, 0, 0, 2}}},
		{"Margin", Margin(3), types.Style{Margin: [4]int{3, 3, 3, 3}}},
		{"Gap", Gap(1), types.Style{Gap: 1}},
		{"FlexRow", FlexRow(), types.Style{FlexDir: types.FlexDirectionRow}},
		{"FlexColumn", FlexColumn(), types.Style{FlexDir: types.FlexDirectionColumn}},
		{"Width", Width(10), types.Style{Width: 10}},
		{"Height", Height(5), types.Style{Height: 5}},
		{"MinWidth", MinWidth(1), types.Style{MinWidth: 1}},
		{"MinHeight", MinHeight(1), types.Style{MinHeight: 1}},
		{"Border", Border(BorderSingle), types.Style{Border: types.BorderSingle}},
		{"Bold", Bold(), types.Style{Bold: true}},
		{"Italic", Italic(), types.Style{Italic: true}},
		{"Underline", Underline(), types.Style{Underline: true}},
		{"ColorFg", ColorFg(5), types.Style{FgColor: 5}},
		{"ColorBg", ColorBg(7), types.Style{BgColor: 7}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := Box(tt.opt)
			if e.Style != tt.want {
				t.Errorf("%s: Style = %+v, want %+v", tt.name, e.Style, tt.want)
			}
		})
	}
}
