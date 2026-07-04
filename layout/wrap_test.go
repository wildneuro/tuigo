package layout

import (
	"reflect"
	"testing"
)

func TestWrapTextBreaksOnWordBoundaries(t *testing.T) {
	got := WrapText("one two three", 7)
	want := []string{"one two", "three"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("WrapText = %v, want %v", got, want)
	}
}

func TestWrapTextHardBreaksLongWord(t *testing.T) {
	got := WrapText("supercalifragilistic", 5)
	want := []string{"super", "calif", "ragil", "istic"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("WrapText = %v, want %v", got, want)
	}
}

func TestWrapTextEmptyStringReturnsOneEmptyLine(t *testing.T) {
	got := WrapText("", 10)
	want := []string{""}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("WrapText(\"\") = %v, want %v", got, want)
	}
}

func TestWrapTextFitsWithinWidthOnOneLine(t *testing.T) {
	got := WrapText("short", 20)
	want := []string{"short"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("WrapText = %v, want %v", got, want)
	}
}
