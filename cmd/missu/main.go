// Command missu is the Miss U More command line: let one person know you miss
// them, without leaving the terminal.
//
//	missu              press the button
//	missu status       print the score, connection and cooldown
//	missu login        store a personal API token for later runs
//	missu logout       forget the stored token
//	missu version      print the build version
//
// It deliberately has no dependencies and no generated code. The whole public
// API is two endpoints behind a bearer header —
//
//	GET  /v1/me/status   the score, connection, cooldown, and a `summary`
//	                     sentence the server renders for us
//	POST /v1/missu       press the button
//
// — and the server owns the prose, so this client never restates it. That is why
// the Go, npm and Python clients in this repository can be independent
// implementations without drifting: there is one sentence, and the server writes
// it.
//
// Auth is a personal API token from Settings → Assistants (MCP) on missu.fyi,
// read from --token, then MISSU_TOKEN, then ~/.config/missu/token (0600).
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// version is stamped at build time: -ldflags "-X main.version=v1.2.3".
var version = "dev"

const (
	defaultBaseURL = "https://missu.fyi"
	// source is the surface ID sent with a tap, so the notification the other
	// person receives can read "… via the command line".
	source = "cli"
)

const usage = `missu — let one person know you miss them.

Usage:
  missu                press the button
  missu status         the score, whether you're Connected, and any cooldown
  missu login          store a personal API token for later runs
  missu logout         forget the stored token
  missu version        print the version

Flags:
  --token <token>      use this token instead of MISSU_TOKEN / the stored one
  --base <url>         API base URL (default $MISSU_BASE_URL or ` + defaultBaseURL + `)
  --json               print raw JSON instead of prose

Mint a token at https://missu.fyi → Settings → Assistants (MCP).
`

func main() {
	var (
		token    string
		base     string
		asJSON   bool
		showHelp bool
	)
	fs := flag.NewFlagSet("missu", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	fs.StringVar(&token, "token", "", "personal API token")
	fs.StringVar(&base, "base", "", "API base URL")
	fs.BoolVar(&asJSON, "json", false, "print raw JSON")
	fs.BoolVar(&showHelp, "help", false, "show usage")
	fs.BoolVar(&showHelp, "h", false, "show usage")

	// Accept the subcommand before or after flags: `missu --json status` and
	// `missu status --json` should both work, because both get typed.
	var cmd string
	var flagArgs []string
	for _, a := range os.Args[1:] {
		if !strings.HasPrefix(a, "-") && cmd == "" {
			cmd = a
			continue
		}
		flagArgs = append(flagArgs, a)
	}
	if err := fs.Parse(flagArgs); err != nil {
		os.Exit(2)
	}
	if showHelp || cmd == "help" {
		fmt.Print(usage)
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, cmd, token, base, asJSON); err != nil {
		fmt.Fprintln(os.Stderr, "missu: "+err.Error())
		os.Exit(1)
	}
}

func run(ctx context.Context, cmd, tokenFlag, baseFlag string, asJSON bool) error {
	base := baseFlag
	if base == "" {
		base = os.Getenv("MISSU_BASE_URL")
	}
	if base == "" {
		base = defaultBaseURL
	}
	base = strings.TrimRight(base, "/")

	switch cmd {
	case "version":
		fmt.Printf("missu %s\n", version)
		return nil

	case "logout":
		path, err := tokenPath()
		if err != nil {
			return err
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		fmt.Println("Token forgotten.")
		return nil

	case "login":
		return login(ctx, base)

	case "", "status":
		token, err := resolveToken(tokenFlag)
		if err != nil {
			return err
		}

		tapped := cmd == ""
		if tapped {
			if _, err := request(ctx, base, token, http.MethodPost, "/v1/missu",
				map[string]string{"source": source}); err != nil {
				return err
			}
		}

		raw, err := request(ctx, base, token, http.MethodGet, "/v1/me/status", nil)
		if err != nil {
			return err
		}
		if asJSON {
			return printJSON(raw)
		}

		var st status
		if err := json.Unmarshal(raw, &st); err != nil {
			return fmt.Errorf("decoding status: %w", err)
		}
		fmt.Println(render(&st, tapped))
		return nil

	default:
		return fmt.Errorf("unknown command %q — run `missu --help`", cmd)
	}
}

// status is the subset of GET /v1/me/status this client reads. Summary is the
// sentence the server renders; the rest is only a fallback for a server that
// predates it.
type status struct {
	Summary string `json:"summary"`
	User    struct {
		Email string `json:"email"`
	} `json:"user"`
	Miss struct {
		Email      string `json:"email"`
		Connected  bool   `json:"connected"`
		Connection struct {
			Email string `json:"email"`
			Name  string `json:"name"`
		} `json:"connection"`
		Stats struct {
			SentCount     int64 `json:"sent_count"`
			ReceivedCount int64 `json:"received_count"`
		} `json:"stats"`
	} `json:"miss"`
}

// render prefers the server's own sentence, so this client holds no copy of the
// prose to drift from.
func render(st *status, tapped bool) string {
	lead := ""
	if tapped {
		lead = "Miss U sent 💛\n\n"
	}
	if st.Summary != "" {
		return lead + st.Summary
	}

	if !st.Miss.Connected {
		if st.Miss.Email == "" {
			return lead + "Nobody's been named yet — open missu.fyi and choose who you miss."
		}
		return lead + fmt.Sprintf(
			"You've named %s, but they haven't named you back (yet) — not Connected.", st.Miss.Email)
	}
	who := st.Miss.Connection.Name
	if who == "" {
		who = st.Miss.Connection.Email
	}
	return lead + fmt.Sprintf("You and %s are Connected 💛 — you've sent %d, they've sent %d.",
		who, st.Miss.Stats.SentCount, st.Miss.Stats.ReceivedCount)
}

var httpClient = &http.Client{Timeout: 15 * time.Second}

// request issues one authenticated call and turns any non-2xx into the API's own
// message, so the user reads "connect to someone first", not "HTTP 403".
func request(ctx context.Context, base, token, method, path string, body any) (json.RawMessage, error) {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, base+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if reader != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	res, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not reach %s: %w", base, err)
	}
	defer res.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		msg := strings.TrimSpace(res.Status)
		var e struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(raw, &e) == nil && e.Error != "" {
			msg = e.Error
		}
		if ra := res.Header.Get("Retry-After"); ra != "" {
			if secs, err := strconv.Atoi(ra); err == nil {
				msg = fmt.Sprintf("%s (retry in %ds)", msg, secs)
			}
		}
		return nil, errors.New(msg)
	}
	return raw, nil
}

// login prompts for a pasted token, verifies it, then stores it so later runs
// need no environment variable.
func login(ctx context.Context, base string) error {
	fmt.Println("Mint a token at https://missu.fyi → Settings → Assistants (MCP).")
	fmt.Print("Paste it here: ")

	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	token := strings.TrimSpace(line)
	if token == "" {
		if err != nil {
			return fmt.Errorf("no token given")
		}
		return fmt.Errorf("no token given")
	}

	// Verify before saving, so a typo fails now rather than on the next tap.
	verifyCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	raw, err := request(verifyCtx, base, token, http.MethodGet, "/v1/me/status", nil)
	if err != nil {
		return fmt.Errorf("that token did not work: %w", err)
	}
	var st status
	_ = json.Unmarshal(raw, &st)

	path, err := tokenPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(token+"\n"), 0o600); err != nil {
		return err
	}
	fmt.Printf("Signed in as %s. Token saved to %s.\n", st.User.Email, path)
	return nil
}

// resolveToken finds the bearer credential: explicit flag, then environment,
// then the file written by `missu login`.
func resolveToken(flagValue string) (string, error) {
	if flagValue != "" {
		return flagValue, nil
	}
	if env := strings.TrimSpace(os.Getenv("MISSU_TOKEN")); env != "" {
		return env, nil
	}
	path, err := tokenPath()
	if err != nil {
		return "", err
	}
	if b, err := os.ReadFile(path); err == nil {
		if token := strings.TrimSpace(string(b)); token != "" {
			return token, nil
		}
	}
	return "", errors.New("no token — run `missu login`, or set MISSU_TOKEN")
}

// tokenPath is ~/.config/missu/token, honouring XDG_CONFIG_HOME. The npm and
// Python clients in this repository use the same file, so `missu login` once
// works whichever one you reach for.
func tokenPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "missu", "token"), nil
}

// printJSON pretty-prints the raw payload, for scripts that would rather parse
// than read prose.
func printJSON(raw json.RawMessage) error {
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		_, err := os.Stdout.Write(raw)
		return err
	}
	buf.WriteByte('\n')
	_, err := os.Stdout.Write(buf.Bytes())
	return err
}
