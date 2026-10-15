package netboot

import "testing"

func TestResolveAnswerType(t *testing.T) {
	cases := []struct {
		name     string
		overlay  string
		template string
		want     string
	}{
		{"overlay 优先", "winpeshl", "autounattend", "winpeshl"},
		{"overlay 优先（反向）", "autounattend", "winpeshl", "autounattend"},
		{"模板兜底", "", "winpeshl", "winpeshl"},
		{"overlay 空白视为未设", "  ", "autounattend", "autounattend"},
		{"两者皆空", "", "", ""},
		{"overlay 值去空白", " kickstart ", "preseed", "kickstart"},
	}
	for _, c := range cases {
		if got := ResolveAnswerType(c.overlay, c.template); got != c.want {
			t.Errorf("%s: ResolveAnswerType(%q,%q) = %q, want %q", c.name, c.overlay, c.template, got, c.want)
		}
	}
}
