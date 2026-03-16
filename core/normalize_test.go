package core

import "testing"

func TestNormalizeTitle(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Nikki Glaser: The Stunning Tour", "nikki glaser: the stunning tour"},
		{"Paramount Theatre Club Seating - Nikki Glaser", "nikki glaser"},
		{"Paramount Theatre Club Seating -Nikki Glaser", "nikki glaser"},
		{"Paramount Theatre Club Seating: Nikki Glaser", "nikki glaser"},
		{"Charlie Berens: The Lost & Found Tour", "charlie berens: the lost and found tour"},
		{"CHELSEA HANDLER: THE HIGH AND MIGHTY TOUR", "chelsea handler: the high and mighty tour"},
	}
	for _, tt := range tests {
		got := NormalizeTitle(tt.input)
		if got != tt.want {
			t.Errorf("NormalizeTitle(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeTitleForDedup(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		// Tour suffix stripped.
		{"Nikki Glaser: The Stunning Tour", "nikki glaser"},
		{"John Mulaney: Mister Whatever", "john mulaney"},
		// Club Seating prefix stripped + tour suffix stripped.
		{"Paramount Theatre Club Seating - Nikki Glaser", "nikki glaser"},
		// Plain title unchanged (no colon).
		{"John Mulaney", "john mulaney"},
		// & → and, then colon strip.
		{"Charlie Berens: The Lost & Found Tour", "charlie berens"},
		// Dash-separated tour suffix.
		{"TREY KENNEDY - THE RELATABLE TOUR", "trey kennedy"},
		// Dash suffix without tour keyword — keep it.
		{"Small Town Murder", "small town murder"},
	}
	for _, tt := range tests {
		got := NormalizeTitleForDedup(tt.input)
		if got != tt.want {
			t.Errorf("NormalizeTitleForDedup(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
