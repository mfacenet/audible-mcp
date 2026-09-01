package mcpserver

import "testing"

func TestFirstPathSegment(t *testing.T) {
	t.Parallel()
	tests := []struct {
		uri  string
		want string
	}{
		{uri: "audible://library/B0FVBC49CX", want: "B0FVBC49CX"},
		{uri: "audible://collections/__FAVORITES/items", want: "__FAVORITES"},
		{uri: "audible://content/B0FVBC49CX/metadata", want: "B0FVBC49CX"},
		{uri: "audible://catalog/B0FVBC49CX", want: "B0FVBC49CX"},
	}
	for _, tt := range tests {
		t.Run(tt.uri, func(t *testing.T) {
			t.Parallel()
			if got := firstPathSegment(tt.uri); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}
