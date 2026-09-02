package cli

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/mfacenet/audible-mcp/internal/auth"
	"github.com/mfacenet/audible-mcp/internal/marketplace"
	"github.com/mfacenet/audible-mcp/internal/mcpserver"
	"github.com/mfacenet/audible-mcp/internal/version"
)

// Run is the CLI entrypoint.
func Run(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] == "serve" {
		rest := args
		if len(args) > 0 && args[0] == "serve" {
			rest = args[1:]
		}
		return runServe(ctx, rest)
	}

	switch args[0] {
	case "auth":
		if len(args) < 2 {
			return usageError("auth requires login or refresh")
		}
		switch args[1] {
		case "login":
			return runLogin(ctx, args[2:])
		case "refresh":
			return runRefresh(ctx, args[2:])
		default:
			return usageError("unknown auth command: " + args[1])
		}
	case "help", "--help", "-h":
		printUsage(os.Stdout)
		return nil
	case "version", "--version", "-v":
		fmt.Printf("audible-mcp %s\n", version.Version)
		return nil
	default:
		return usageError("unknown command: " + strings.Join(args, " "))
	}
}

func runServe(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	authFile := fs.String("auth-file", os.Getenv("AUDIBLE_AUTH_FILE"), "path to audible-auth.json")
	baseURL := fs.String("base-url", os.Getenv("AUDIBLE_BASE_URL"), "Audible API base URL")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*authFile) == "" {
		return usageError("Provide an auth file with --auth-file or AUDIBLE_AUTH_FILE.")
	}

	server, err := mcpserver.New(mcpserver.Options{
		AuthFile: *authFile,
		BaseURL:  *baseURL,
	})
	if err != nil {
		return err
	}
	return server.Run(ctx, &mcp.StdioTransport{})
}

func runLogin(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("auth login", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	file := fs.String("file", "audible-auth.json", "output auth bundle path")
	marketplaceCode := fs.String("marketplace", "us", "Audible marketplace code")
	locale := fs.String("locale", "", "alias for --marketplace")
	serial := fs.String("serial", "", "optional device serial")
	noOpen := fs.Bool("no-open", false, "do not open a browser")
	withUsername := fs.Bool("with-username", false, "login with an Audible username")
	if err := fs.Parse(args); err != nil {
		return err
	}
	code := *marketplaceCode
	if *locale != "" {
		code = *locale
	}

	m, err := marketplace.Lookup(code)
	if err != nil {
		return err
	}

	var opts []auth.SessionOption
	if *serial != "" {
		opts = append(opts, auth.WithSerial(*serial))
	}
	if *withUsername {
		opts = append(opts, auth.WithAudibleUsername())
	}
	session, err := auth.NewSession(m, opts...)
	if err != nil {
		return err
	}

	fmt.Printf("Marketplace: %s (%s)\n", m.Code, m.Domain)
	fmt.Printf("Device serial: %s\n\n", session.Serial)
	fmt.Println("Open this URL in a browser and complete the Audible/Amazon login flow:")
	fmt.Println(session.LoginURL)
	fmt.Println()

	if !*noOpen {
		if err := openBrowser(session.LoginURL); err != nil {
			fmt.Fprintf(os.Stderr, "could not open browser: %v\n", err)
		} else {
			fmt.Println("Attempted to open the browser automatically.")
			fmt.Println()
		}
	}

	fmt.Println("After login, copy the final URL from the browser address bar.")
	responseURL, err := prompt("Paste the final maplanding URL: ")
	if err != nil {
		return err
	}
	authorizationCode, err := auth.AuthorizationCode(responseURL)
	if err != nil {
		return err
	}
	registration, err := auth.RegisterDevice(ctx, nil, session, authorizationCode)
	if err != nil {
		return err
	}
	bundle := auth.Bundle(session, registration)
	if err := auth.Save(*file, bundle); err != nil {
		return err
	}

	abs, _ := filepath.Abs(*file)
	fmt.Printf("\nSaved auth bundle to %s\n", abs)
	fmt.Println("This registration creates a virtual Audible device. Reuse this file instead of registering repeatedly.")
	return nil
}

func runRefresh(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("auth refresh", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	file := fs.String("file", "audible-auth.json", "auth bundle path")
	refreshCookies := fs.Bool("refresh-cookies", false, "also refresh website cookies")
	if err := fs.Parse(args); err != nil {
		return err
	}

	bundle, err := auth.Load(*file)
	if err != nil {
		return err
	}
	token, expiresAt, err := auth.RefreshAccessToken(ctx, nil, bundle)
	if err != nil {
		return err
	}
	bundle.AccessToken = token
	bundle.ExpiresAt = expiresAt
	if *refreshCookies {
		cookies, err := auth.RefreshWebsiteCookies(ctx, nil, bundle, bundle.Domain)
		if err != nil {
			return err
		}
		bundle.WebsiteCookies = cookies
	}
	if err := auth.Save(*file, bundle); err != nil {
		return err
	}
	abs, _ := filepath.Abs(*file)
	fmt.Printf("Updated auth bundle at %s\n", abs)
	return nil
}

func prompt(message string) (string, error) {
	fmt.Print(message)
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", err
		}
		return "", fmt.Errorf("no maplanding URL provided")
	}
	return strings.TrimSpace(scanner.Text()), nil
}

func openBrowser(rawURL string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", rawURL)
	case "darwin":
		cmd = exec.Command("open", rawURL)
	default:
		cmd = exec.Command("xdg-open", rawURL)
	}
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Start()
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  audible-mcp serve [--auth-file <path>] [--base-url <url>]")
	fmt.Fprintln(w, "  audible-mcp auth login [--marketplace <code>] [--file <path>] [--no-open] [--serial <serial>] [--with-username]")
	fmt.Fprintln(w, "  audible-mcp auth refresh [--file <path>] [--refresh-cookies]")
	fmt.Fprintln(w, "  audible-mcp version")
}

type usageErr struct{ msg string }

func (e usageErr) Error() string { return e.msg }

func usageError(msg string) error { return usageErr{msg: msg} }

// IsUsage reports whether err is a CLI usage error.
func IsUsage(err error) bool {
	_, ok := err.(usageErr)
	return ok
}
