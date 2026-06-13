package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/takihito/glasp/internal/browser"
	"golang.org/x/oauth2"
)

// openBrowser opens the specified URL in the default browser of the user.
// Failures are logged only: the login flow keeps waiting for the callback
// and the user can open the printed URL manually.
func openBrowser(url string) {
	if err := browser.Start(url); err != nil {
		log.Printf("Failed to open browser: %v", err)
		log.Printf("Please manually open your browser and go to: %s", url)
	}
}

func generateStateToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("failed to generate state token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

var (
	// テストで差し替えられるよう、関数参照を保持する。
	openBrowserFn        = openBrowser
	generateStateTokenFn = generateStateToken
)

type oauthState struct {
	token string
	used  bool
	mu    sync.Mutex
}

func (s *oauthState) validate(incoming string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.used {
		return fmt.Errorf("state token already used")
	}
	if incoming == "" || incoming != s.token {
		return fmt.Errorf("invalid state in response")
	}
	s.used = true
	return nil
}

// LoginOptions configures optional behavior of the interactive login flow.
type LoginOptions struct {
	// PKCE enables PKCE (Proof Key for Code Exchange) for the OAuth code exchange.
	PKCE bool
}

// Login performs the OAuth2 login flow using the project-local token cache.
func Login(ctx context.Context, config *oauth2.Config) (*http.Client, error) {
	return LoginWithOptions(ctx, config, LoginOptions{})
}

// LoginWithOptions performs the OAuth2 login flow with explicit options.
func LoginWithOptions(ctx context.Context, config *oauth2.Config, opts LoginOptions) (*http.Client, error) {
	cacheFile, err := tokenCacheFile()
	if err != nil {
		return nil, fmt.Errorf("failed to get token cache file path: %w", err)
	}
	return loginWithCachePath(ctx, config, cacheFile, opts.PKCE)
}

// LoginWithCachePath performs the OAuth2 login flow using an explicit cache file path.
func LoginWithCachePath(ctx context.Context, config *oauth2.Config, cacheFile string) (*http.Client, error) {
	if strings.TrimSpace(cacheFile) == "" {
		return nil, fmt.Errorf("token cache file path is required")
	}
	return loginWithCachePath(ctx, config, cacheFile, false)
}

// clientFromCachedToken tries to build a client from the cached token,
// refreshing it through the token source when needed. ok is false when the
// cache is missing or unusable and the full OAuth flow must run instead.
func clientFromCachedToken(ctx context.Context, config *oauth2.Config, cacheFile string) (client *http.Client, ok bool) {
	token, _, _, loadErr := loadToken(cacheFile)
	if loadErr != nil {
		log.Printf("No cached token found or failed to load: %v", loadErr)
		return nil, false
	}
	// config.TokenSource automatically refreshes the token if it's expired
	// and a refresh token is available.
	tokenSource := config.TokenSource(ctx, token)
	freshToken, err := tokenSource.Token()
	if err != nil {
		log.Printf("Cached token or refresh token is invalid, proceeding with full OAuth flow: %v", err)
		return nil, false
	}
	// Save the token back to cache in case it was refreshed.
	if freshToken.AccessToken != token.AccessToken {
		if saveErr := saveToken(cacheFile, freshToken); saveErr != nil {
			log.Printf("Warning: Failed to save refreshed token: %v", saveErr)
		}
	}
	fmt.Fprintf(stdout, "Using cached or refreshed token from %s\n", cacheFile)
	return oauth2.NewClient(ctx, tokenSource), true
}

type authCodeResult struct {
	code string
	err  error
}

// startCallbackServer serves the OAuth redirect endpoint on the listener and
// returns the server together with a channel that receives exactly one
// result: the authorization code, a callback error, or a serve error.
func startCallbackServer(listener net.Listener, state *oauthState) (*http.Server, <-chan authCodeResult) {
	resultChan := make(chan authCodeResult, 1)
	sendResult := func(result authCodeResult) {
		select {
		case resultChan <- result:
		default:
		}
	}

	// Use a local ServeMux to avoid polluting the global http handler.
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth2callback", func(w http.ResponseWriter, r *http.Request) {
		incomingState := r.URL.Query().Get("state")
		if err := state.validate(incomingState); err != nil {
			errmsg := "Invalid state in response."
			http.Error(w, errmsg, http.StatusBadRequest)
			log.Println(errmsg)
			sendResult(authCodeResult{err: err})
			return
		}
		log.Printf("Valid state token received")
		code := r.URL.Query().Get("code")
		if code == "" {
			errmsg := "No code in response. "
			if errParam := r.URL.Query().Get("error"); errParam != "" {
				errmsg += fmt.Sprintf("Google returned error: %s", errParam)
			}
			http.Error(w, errmsg, http.StatusBadRequest)
			log.Println(errmsg)
			sendResult(authCodeResult{err: fmt.Errorf("%s", errmsg)})
			return
		}
		log.Printf("Received authorization code (length: %d)", len(code))
		fmt.Fprintf(w, "Authentication successful! You can close this tab.")
		sendResult(authCodeResult{code: code})
	})

	srv := &http.Server{Handler: mux}
	go func() {
		if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Printf("listen error: %v\n", err)
			sendResult(authCodeResult{err: fmt.Errorf("listen: %w", err)})
		}
	}()
	return srv, resultChan
}

func loginWithCachePath(ctx context.Context, config *oauth2.Config, cacheFile string, pkce bool) (*http.Client, error) {
	if client, ok := clientFromCachedToken(ctx, config, cacheFile); ok {
		return client, nil
	}

	// Token not found or invalid, perform full OAuth2 flow

	// Start a local server on a random free port.
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return nil, fmt.Errorf("failed to start local server: %w", err)
	}
	defer listener.Close()

	// Update the RedirectURL with the dynamically chosen port
	port := listener.Addr().(*net.TCPAddr).Port
	config.RedirectURL = fmt.Sprintf("http://localhost:%d/oauth2callback", port)

	stateToken, err := generateStateTokenFn()
	if err != nil {
		return nil, err
	}
	state := &oauthState{token: stateToken}
	var pkceVerifier string
	authCodeOpts := []oauth2.AuthCodeOption{oauth2.AccessTypeOffline}
	if pkce {
		pkceVerifier = oauth2.GenerateVerifier()
		authCodeOpts = append(authCodeOpts, oauth2.S256ChallengeOption(pkceVerifier))
		fmt.Fprintln(stdout, "PKCE enabled: requesting authorization with S256 code challenge")
	}
	authCodeURL := config.AuthCodeURL(stateToken, authCodeOpts...)
	fmt.Fprintf(stdout, "Go to the following link in your browser:\n%s\n", authCodeURL)
	openBrowserFn(authCodeURL)

	srv, resultChan := startCallbackServer(listener, state)
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if message, isError := shutdownServerMessage(srv.Shutdown(shutdownCtx)); message != "" {
			if isError {
				log.Println(message)
			} else {
				fmt.Fprintln(stdout, message)
			}
		}
	}()
	fmt.Fprintln(stdout, "Waiting for authentication flow to complete in browser...")

	// Wait for the auth code or an error
	var result authCodeResult
	select {
	case result = <-resultChan:
	case <-ctx.Done():
		return nil, fmt.Errorf("authentication cancelled: %w", ctx.Err())
	}
	if result.err != nil {
		return nil, fmt.Errorf("authentication failed: %w", result.err)
	}

	// Exchange the code for a token
	var exchangeOpts []oauth2.AuthCodeOption
	if pkceVerifier != "" {
		exchangeOpts = append(exchangeOpts, oauth2.VerifierOption(pkceVerifier))
		fmt.Fprintln(stdout, "PKCE: exchanging authorization code with code_verifier")
	}
	token, err := config.Exchange(ctx, result.code, exchangeOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange authorization code: %w", err)
	}

	// Save the token
	if err := saveToken(cacheFile, token); err != nil {
		return nil, fmt.Errorf("failed to save token: %w", err)
	}

	return oauth2.NewClient(ctx, config.TokenSource(ctx, token)), nil
}

func shutdownServerMessage(err error) (string, bool) {
	if err == nil {
		return "", false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "OAuth callback server shutdown timed out; authentication already completed.", false
	}
	return fmt.Sprintf("Error shutting down server: %v", err), true
}
