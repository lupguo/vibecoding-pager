package agents

import "testing"

func TestSelectByLabel_KnownLabels(t *testing.T) {
	tests := []struct {
		label  string
		wantOK bool
	}{
		{"CC", true},
		{"CC-Internal", true},
		{"CodeBuddy", true},
		{"Unknown", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			a, ok := SelectByLabel(tt.label)
			if ok != tt.wantOK {
				t.Errorf("SelectByLabel(%q) ok = %v, want %v", tt.label, ok, tt.wantOK)
			}
			if tt.wantOK && a == nil {
				t.Errorf("SelectByLabel(%q) returned nil agent", tt.label)
			}
		})
	}
}

func TestSelectByLabel_CCAndCodeBuddyMapToSameType(t *testing.T) {
	cc, _ := SelectByLabel("CC")
	cb, _ := SelectByLabel("CodeBuddy")
	if cc.ID() != cb.ID() {
		t.Errorf("CC.ID()=%q, CodeBuddy.ID()=%q, want same", cc.ID(), cb.ID())
	}
}

func TestAll_SortedByLabel(t *testing.T) {
	all := All()
	if len(all) < 3 {
		t.Errorf("len(All()) = %d, want >= 3", len(all))
	}
	for i := 1; i < len(all); i++ {
		if all[i-1].Label > all[i].Label {
			t.Errorf("All() not sorted: %q > %q", all[i-1].Label, all[i].Label)
		}
	}
}
