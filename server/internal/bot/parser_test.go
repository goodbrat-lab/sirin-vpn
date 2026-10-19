package bot

import (
	"strings"
	"testing"
	"time"
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

func TestMaskUUID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"12345678", "12345678"},
		{"1234567", "1234567"},
		{"abcdef01-2345-6789-abcd-ef0123456789", "abcdef01…"},
	}
	for _, tt := range tests {
		got := maskUUID(tt.input)
		if got != tt.want {
			t.Errorf("maskUUID(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFormatDaysLeft(t *testing.T) {
	if formatDaysLeft(nil) != "бессрочно" {
		t.Errorf("expected бессрочно for nil")
	}

	zero := time.Time{}
	if formatDaysLeft(&zero) != "бессрочно" {
		t.Errorf("expected бессрочно for zero time")
	}

	future := time.Now().Add(5 * 24 * time.Hour)
	if formatDaysLeft(&future) != "5д" {
		t.Errorf("expected 5д, got %q", formatDaysLeft(&future))
	}

	today := time.Now().Add(6 * time.Hour)
	if formatDaysLeft(&today) != "истекает сегодня" {
		t.Errorf("expected истекает сегодня, got %q", formatDaysLeft(&today))
	}

	pastToday := time.Now().Add(-2 * time.Hour)
	if formatDaysLeft(&pastToday) != "просрочен сегодня" {
		t.Errorf("expected просрочен сегодня, got %q", formatDaysLeft(&pastToday))
	}

	pastDays := time.Now().Add(-72 * time.Hour)
	if formatDaysLeft(&pastDays) != "просрочен 3д" {
		t.Errorf("expected просрочен 3д, got %q", formatDaysLeft(&pastDays))
	}
}

func TestParseRenameArgs(t *testing.T) {
	tests := []struct {
		input   string
		wantOld string
		wantNew string
		wantErr bool
	}{
		{
			input:   "Марина Иванова -> Марина И.",
			wantOld: "Марина Иванова",
			wantNew: "Марина И.",
			wantErr: false,
		},
		{
			input:   "/rename Марина Иванова -> Марина И.",
			wantOld: "Марина Иванова",
			wantNew: "Марина И.",
			wantErr: false,
		},
		{
			input:   "/rename@sirin_crp_bot a1b2c3d4 -> Новое Имя",
			wantOld: "a1b2c3d4",
			wantNew: "Новое Имя",
			wantErr: false,
		},
		{
			input:   "Марина",
			wantErr: true,
		},
		{
			input:   "Марина ->",
			wantErr: true,
		},
		{
			input:   "-> Марина",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		old, newN, err := parseRenameArgs(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("parseRenameArgs(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
		}
		if !tt.wantErr {
			if old != tt.wantOld || newN != tt.wantNew {
				t.Errorf("parseRenameArgs(%q) = (%q, %q), want (%q, %q)", tt.input, old, newN, tt.wantOld, tt.wantNew)
			}
		}
	}
}

func TestResolveClient(t *testing.T) {
	clients := []*Client{
		{UUID: "11111111-0000-0000-0000-000000000001", Name: "Марина Кузнецова"},
		{UUID: "22222222-0000-0000-0000-000000000002", Name: "Иван Иванов"},
		{UUID: "33333333-0000-0000-0000-000000000003", Name: "Иван Сидоров"},
	}

	// 1. Exact UUID prefix (>=6)
	c, err, _ := resolveClient(clients, "111111")
	if err != nil || c == nil || c.Name != "Марина Кузнецова" {
		t.Errorf("resolve by prefix 111111 failed: %v", err)
	}

	// 2. Exact name
	c, err, _ = resolveClient(clients, "марина кузнецова")
	if err != nil || c == nil || c.UUID != "11111111-0000-0000-0000-000000000001" {
		t.Errorf("resolve by exact name failed: %v", err)
	}

	// 3. Substring single match
	c, err, _ = resolveClient(clients, "Сидоров")
	if err != nil || c == nil || c.UUID != "33333333-0000-0000-0000-000000000003" {
		t.Errorf("resolve by substring single match failed: %v", err)
	}

	// 4. Substring ambiguous match (Иван matches "Иван Иванов" and "Иван Сидоров")
	_, err, ambiguous := resolveClient(clients, "Иван")
	if err != ErrAmbiguousClient || len(ambiguous) != 2 {
		t.Errorf("resolve by ambiguous should return ErrAmbiguousClient with 2 matches, got err=%v, count=%d", err, len(ambiguous))
	}

	// 5. Not found
	_, err, _ = resolveClient(clients, "Петр")
	if err != ErrClientNotFound {
		t.Errorf("resolve not found failed: got %v, want ErrClientNotFound", err)
	}
}

