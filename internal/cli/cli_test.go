package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestHelp(t *testing.T) {
	t.Parallel()
	if err := Run(context.Background(), []string{"help"}); err != nil {
		t.Fatal(err)
	}
}

func TestUnknownCommand(t *testing.T) {
	t.Parallel()
	err := Run(context.Background(), []string{"frobnicate"})
	if err == nil || !IsUsage(err) {
		t.Fatalf("err = %v", err)
	}
}

func TestServeRequiresAuthFile(t *testing.T) {
	t.Setenv("AUDIBLE_AUTH_FILE", "")
	err := Run(context.Background(), []string{"serve"})
	if err == nil || !IsUsage(err) {
		t.Fatalf("err = %v", err)
	}
}

func TestPrintUsage(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	printUsage(&buf)
	if !strings.Contains(buf.String(), "audible-mcp serve") {
		t.Fatalf("usage = %s", buf.String())
	}
}
