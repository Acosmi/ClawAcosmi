package config

import "testing"

func TestParseOpenAcosmiVersion(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantNil bool
		wantMaj int
		wantMin int
		wantPat int
		wantRev int
	}{
		{"with v prefix", "v1.2.3", false, 1, 2, 3, 0},
		{"without v prefix", "1.2.3", false, 1, 2, 3, 0},
		{"with revision", "v1.2.3-4", false, 1, 2, 3, 4},
		{"zero version", "v0.0.0", false, 0, 0, 0, 0},
		{"large numbers", "v100.200.300-999", false, 100, 200, 300, 999},
		{"with trailing text", "v1.2.3-4-beta", false, 1, 2, 3, 4},
		{"empty string", "", true, 0, 0, 0, 0},
		{"garbage", "not-a-version", true, 0, 0, 0, 0},
		{"partial", "v1.2", true, 0, 0, 0, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v := ParseOpenAcosmiVersion(tc.input)
			if tc.wantNil {
				if v != nil {
					t.Fatalf("expected nil, got %+v", v)
				}
				return
			}
			if v == nil {
				t.Fatal("expected non-nil version")
			}
			if v.Major != tc.wantMaj || v.Minor != tc.wantMin || v.Patch != tc.wantPat || v.Revision != tc.wantRev {
				t.Fatalf("got %d.%d.%d-%d, want %d.%d.%d-%d",
					v.Major, v.Minor, v.Patch, v.Revision,
					tc.wantMaj, tc.wantMin, tc.wantPat, tc.wantRev)
			}
		})
	}
}

func TestCompareOpenAcosmiVersions(t *testing.T) {
	tests := []struct {
		name   string
		a, b   string
		want   int
		wantOk bool
	}{
		{"equal", "v1.2.3", "v1.2.3", 0, true},
		{"equal with revision", "v1.2.3-4", "v1.2.3-4", 0, true},
		{"major less", "v1.0.0", "v2.0.0", -1, true},
		{"major greater", "v2.0.0", "v1.0.0", 1, true},
		{"minor less", "v1.1.0", "v1.2.0", -1, true},
		{"minor greater", "v1.2.0", "v1.1.0", 1, true},
		{"patch less", "v1.1.1", "v1.1.2", -1, true},
		{"patch greater", "v1.1.2", "v1.1.1", 1, true},
		{"revision less", "v1.1.1-1", "v1.1.1-2", -1, true},
		{"revision greater", "v1.1.1-2", "v1.1.1-1", 1, true},
		{"a invalid", "bad", "v1.0.0", 0, false},
		{"b invalid", "v1.0.0", "bad", 0, false},
		{"both invalid", "bad", "also-bad", 0, false},
		{"a empty", "", "v1.0.0", 0, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := CompareOpenAcosmiVersions(tc.a, tc.b)
			if ok != tc.wantOk {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOk)
			}
			if got != tc.want {
				t.Fatalf("cmp = %d, want %d", got, tc.want)
			}
		})
	}
}
