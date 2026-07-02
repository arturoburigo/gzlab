package gitlab

import "testing"

func TestDiscussion_ResolveState(t *testing.T) {
	cases := []struct {
		name       string
		notes      []Note
		resolvable bool
		resolved   bool
	}{
		{"plain comment", []Note{{Resolvable: false}}, false, false},
		{"open thread", []Note{{Resolvable: true, Resolved: false}}, true, false},
		{"resolved thread", []Note{{Resolvable: true, Resolved: true}}, true, true},
		{"partially resolved is unresolved", []Note{{Resolvable: true, Resolved: true}, {Resolvable: true, Resolved: false}}, true, false},
		{"system-only", []Note{{System: true, Resolvable: false}}, false, false},
		{"no notes", nil, false, false},
	}
	for _, c := range cases {
		d := Discussion{Notes: c.notes}
		if got := d.Resolvable(); got != c.resolvable {
			t.Errorf("%s: Resolvable() = %v, want %v", c.name, got, c.resolvable)
		}
		if got := d.Resolved(); got != c.resolved {
			t.Errorf("%s: Resolved() = %v, want %v", c.name, got, c.resolved)
		}
	}
}
