package bot

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestParseNewKeyArgs(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantName    string
		wantDays    int
		wantClamped bool
		wantErr     bool
	}{
		{
			name:        "multi-word name with days",
			input:       "Марина Иванова 365",
			wantName:    "Марина Иванова",
			wantDays:    365,
			wantClamped: false,
			wantErr:     false,
		},
		{
			name:        "multi-word name without days",
			input:       "Марина Иванова",
			wantName:    "Марина Иванова",
			wantDays:    30,
			wantClamped: false,
			wantErr:     false,
		},
		{
			name:        "single-word name with days",
			input:       "Марина 365",
			wantName:    "Марина",
			wantDays:    365,
			wantClamped: false,
			wantErr:     false,
		},
		{
			name:        "single-word name without days",
			input:       "Марина",
			wantName:    "Марина",
			wantDays:    30,
			wantClamped: false,
			wantErr:     false,
		},
		{
			name:        "numeric name single token",
			input:       "365",
			wantName:    "365",
			wantDays:    30,
			wantClamped: false,
			wantErr:     false,
		},
		{
			name:        "empty input",
			input:       "",
			wantErr:     true,
		},
		{
			name:        "spaces only",
			input:       "   ",
			wantErr:     true,
		},
		{
			name:        "command only",
			input:       "/newkey",
			wantErr:     true,
		},
		{
			name:        "command with bot name only",
			input:       "/newkey@sirin_crp_bot",
			wantErr:     true,
		},
		{
			name:        "command with multi-word name and days",
			input:       "/newkey Марина Иванова 365",
			wantName:    "Марина Иванова",
			wantDays:    365,
			wantClamped: false,
			wantErr:     false,
		},
		{
			name:        "command with bot mention",
			input:       "/newkey@sirin_crp_bot Марина Иванова 365",
			wantName:    "Марина Иванова",
			wantDays:    365,
			wantClamped: false,
			wantErr:     false,
		},
		{
			name:        "extra spaces collapsed",
			input:       "  Марина   Иванова   7  ",
			wantName:    "Марина Иванова",
			wantDays:    7,
			wantClamped: false,
			wantErr:     false,
		},
		{
			name:        "zero days fallback to name token",
			input:       "Марина 0",
			wantName:    "Марина 0",
			wantDays:    30,
			wantClamped: false,
			wantErr:     false,
		},
		{
			name:        "negative days fallback to name token",
			input:       "Марина -5",
			wantName:    "Марина -5",
			wantDays:    30,
			wantClamped: false,
			wantErr:     false,
		},
		{
			name:        "clamp large days to 3650",
			input:       "Марина 100000",
			wantName:    "Марина",
			wantDays:    3650,
			wantClamped: true,
			wantErr:     false,
		},
		{
			name:        "very long name truncated to 64 runes",
			input:       strings.Repeat("Александр ", 10) + " 90",
			wantDays:    90,
			wantClamped: false,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotName, gotDays, gotClamped, err := parseNewKeyArgsDetailed(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseNewKeyArgsDetailed(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if tt.name == "very long name truncated to 64 runes" {
				if utf8.RuneCountInString(gotName) > MaxNameLen {
					t.Errorf("len(gotName) = %d runes, want <= %d", utf8.RuneCountInString(gotName), MaxNameLen)
				}
			} else if gotName != tt.wantName {
				t.Errorf("gotName = %q, want %q", gotName, tt.wantName)
			}
			if gotDays != tt.wantDays {
				t.Errorf("gotDays = %d, want %d", gotDays, tt.wantDays)
			}
			if gotClamped != tt.wantClamped {
				t.Errorf("gotClamped = %v, want %v", gotClamped, tt.wantClamped)
			}

			// Also verify standard 3-return value signature parseNewKeyArgs
			stdName, stdDays, stdErr := parseNewKeyArgs(tt.input)
			if (stdErr != nil) != tt.wantErr {
				t.Errorf("parseNewKeyArgs(%q) error = %v, wantErr %v", tt.input, stdErr, tt.wantErr)
			}
			if !tt.wantErr {
				if stdName != gotName || stdDays != gotDays {
					t.Errorf("parseNewKeyArgs returned (%q, %d), detailed returned (%q, %d)", stdName, stdDays, gotName, gotDays)
				}
			}
		})
	}
}
