package main

import "testing"

func TestParseSirinDeepLink(t *testing.T) {
	const validUUID = "5f26ebc3-6eb0-45fb-a490-c8c92b86b644"

	cases := []struct {
		in   string
		want string
	}{
		{"sirin://key?uuid=" + validUUID, validUUID},
		{"SIRIN://key?uuid=" + validUUID + "&source=bot", validUUID},
		{"sirin://import?key=" + validUUID, validUUID},
		{"sirin://key/" + validUUID, validUUID},
		{"https://crp.guseynovia.ru/key?uuid=" + validUUID, validUUID},
		{"sirin://key?uuid=not-a-uuid", ""},
		{"javascript:alert(1)", ""},
		{"", ""},
	}

	for _, tc := range cases {
		if got := ParseSirinDeepLink(tc.in); got != tc.want {
			t.Errorf("ParseSirinDeepLink(%q) = %q; want %q", tc.in, got, tc.want)
		}
	}
}
