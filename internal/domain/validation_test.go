package domain

import (
	"testing"
)

func TestValidateIATA(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input   string
		wantErr bool
	}{
		{"AA", false},
		{"KL", false},
		{"U2", false},
		{"9W", false},
		{"6E", false},
		{"3K", false},
		{"u2", false},
		{"42", true},
		{"99", true},
		{"A", true},
		{"KLM", true},
		{"", true},
		{"A!", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			err := ValidateIATA(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateIATA(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestValidateFlightNumber(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input   string
		wantErr bool
	}{
		{"1", false},
		{"123", false},
		{"1234", false},
		{"427A", false},
		{"1B", false},
		{"", true},
		{"0", true},
		{"01", true},
		{"12345", true},
		{"AB", true},
		{"1234AB", true},
		{"1234!", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			err := ValidateFlightNumber(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFlightNumber(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestValidateFlight(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input   string
		wantErr bool
	}{
		{"AA123", false},
		{"DL1234", false},
		{"BA456", false},
		{"LH789", false},
		{"U27898", false},
		{"6E1234", false},
		{"aa123", false},
		{"KL427A", false},
		{"9W1B", false},
		{"", true},
		{"AA", true},
		{"42123", true},
		{"A", true},
		{"AA0", true},
		{"AA01234", true},
		{"AA12345", true},
		{"AAAA123", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			err := ValidateFlight(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFlight(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestFormatFlightNumber(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{"U27898", "U2 7898"},
		{"AA123", "AA 123"},
		{"u2 7898", "U2 7898"},
		{"6e1234", "6E 1234"},
		{"KL427A", "KL 427A"},
		{"AA", "AA"},
		{"A", "A"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			got := FormatFlightNumber(tt.input)
			if got != tt.want {
				t.Errorf("FormatFlightNumber(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
