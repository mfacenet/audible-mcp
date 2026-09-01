package marketplace

import "testing"

func TestLookup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		code    string
		domain  string
		wantErr bool
	}{
		{code: "us", domain: "com"},
		{code: "UK", domain: "co.uk"},
		{code: " jp ", domain: "co.jp"},
		{code: "xx", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			t.Parallel()
			got, err := Lookup(tt.code)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("Lookup: %v", err)
			}
			if got.Domain != tt.domain {
				t.Fatalf("domain = %q, want %q", got.Domain, tt.domain)
			}
		})
	}
}

func TestAllowsAudibleUsername(t *testing.T) {
	t.Parallel()

	us, _ := Lookup("us")
	fr, _ := Lookup("fr")
	if !us.AllowsAudibleUsername() {
		t.Fatal("US should allow Audible-username login")
	}
	if fr.AllowsAudibleUsername() {
		t.Fatal("FR should not allow Audible-username login")
	}
}
