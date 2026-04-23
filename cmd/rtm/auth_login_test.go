package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/morozov/rtm-cli-go/internal/rtm"
)

func TestBuildAuthURL(t *testing.T) {
	t.Parallel()

	c := rtm.NewClient("the-key", "the-secret", "")
	got := buildAuthURL(c, "the-frob", "write")

	u, err := url.Parse(got)
	require.NoError(t, err)
	assert.Equal(t, "https", u.Scheme)
	assert.Equal(t, "www.rememberthemilk.com", u.Host)
	assert.Equal(t, "/services/auth/", u.Path)

	q := u.Query()
	assert.Equal(t, "the-key", q.Get("api_key"))
	assert.Equal(t, "write", q.Get("perms"))
	assert.Equal(t, "the-frob", q.Get("frob"))

	// The signature must be deterministic and match the client's own Sign,
	// which is the same function RTM validates against.
	want := url.Values{}
	want.Set("api_key", "the-key")
	want.Set("perms", "write")
	want.Set("frob", "the-frob")
	assert.Equal(t, c.Sign(want), q.Get("api_sig"))
}

type rtmStub struct {
	liveTokens   map[string]bool   // auth_token → true means checkToken returns ok
	frob         string            // frob returned by getFrob
	tokenForFrob map[string]string // frob → token issued by getToken (empty = "" token leak)
}

func newRTMStub(t *testing.T, s rtmStub) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		q := req.URL.Query()
		switch q.Get("method") {
		case "rtm.auth.checkToken":
			tok := q.Get("auth_token")
			if s.liveTokens[tok] {
				writeOK(w, map[string]any{
					"auth": map[string]any{
						"token": tok,
						"perms": "read",
						"user":  map[string]any{"id": "1", "username": "bob", "fullname": "Bob M."},
					},
				})
				return
			}
			writeFail(w, "98", "Login failed / Invalid auth token")
		case "rtm.auth.getFrob":
			writeOK(w, map[string]any{"frob": s.frob})
		case "rtm.auth.getToken":
			tok, ok := s.tokenForFrob[q.Get("frob")]
			if !ok {
				http.Error(w, "unknown frob", http.StatusBadRequest)
				return
			}
			writeOK(w, map[string]any{
				"auth": map[string]any{
					"token": tok,
					"perms": "read",
					"user":  map[string]any{"id": "1", "username": "bob", "fullname": "Bob M."},
				},
			})
		default:
			t.Errorf("unexpected RTM method: %q", q.Get("method"))
			http.Error(w, "bad", http.StatusInternalServerError)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func writeOK(w http.ResponseWriter, fields map[string]any) {
	fields["stat"] = "ok"
	_ = json.NewEncoder(w).Encode(map[string]any{"rsp": fields})
}

func writeFail(w http.ResponseWriter, code, msg string) {
	_ = json.NewEncoder(w).Encode(map[string]any{
		"rsp": map[string]any{
			"stat": "fail",
			"err":  map[string]any{"code": code, "msg": msg},
		},
	})
}

func newClientFor(t *testing.T, srv *httptest.Server, authToken string) *rtm.Client {
	t.Helper()
	c := rtm.NewClient("key", "secret", authToken)
	c.BaseURL = srv.URL + "/"
	return c
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}

func seedConfig(t *testing.T, path, body string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
}

func TestRunAuthLoginNoExistingWritesToken(t *testing.T) {
	stub := newRTMStub(t, rtmStub{
		frob:         "FROB-1",
		tokenForFrob: map[string]string{"FROB-1": "fresh-token"},
		liveTokens:   map[string]bool{"fresh-token": true},
	})
	client := newClientFor(t, stub, "")

	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv("RTM_CONFIG_FILE", cfgPath)

	var progress strings.Builder
	err := runAuthLogin(context.Background(), client, &progress, strings.NewReader("\n"), authLoginOptions{
		Perms:     "read",
		NoBrowser: true,
	})
	require.NoError(t, err)

	assert.Contains(t, progress.String(), "Verified: logged in as bob")
	assert.Contains(t, progress.String(), "Wrote token to "+cfgPath)
	assert.Contains(t, readFile(t, cfgPath), "auth_token: fresh-token")
}

func TestRunAuthLoginExistingWorkingTokenRefusesWithoutForce(t *testing.T) {
	stub := newRTMStub(t, rtmStub{
		liveTokens: map[string]bool{"existing-token": true},
	})
	client := newClientFor(t, stub, "")

	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv("RTM_CONFIG_FILE", cfgPath)
	seedConfig(t, cfgPath, "auth_token: existing-token\n")

	err := runAuthLogin(context.Background(), client, new(strings.Builder), strings.NewReader("\n"), authLoginOptions{
		Perms:     "read",
		NoBrowser: true,
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrExistingToken), "expected ErrExistingToken, got %v", err)

	// Config file must be untouched.
	assert.Equal(t, "auth_token: existing-token\n", readFile(t, cfgPath))
}

func TestRunAuthLoginExistingDeadTokenProceedsSilently(t *testing.T) {
	stub := newRTMStub(t, rtmStub{
		frob:         "FROB-2",
		tokenForFrob: map[string]string{"FROB-2": "new-token"},
		liveTokens:   map[string]bool{"new-token": true}, // old dead token is NOT in the set
	})
	client := newClientFor(t, stub, "")

	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv("RTM_CONFIG_FILE", cfgPath)
	seedConfig(t, cfgPath, "auth_token: dead-token\n")

	err := runAuthLogin(context.Background(), client, new(strings.Builder), strings.NewReader("\n"), authLoginOptions{
		Perms:     "read",
		NoBrowser: true,
	})
	require.NoError(t, err)
	assert.Contains(t, readFile(t, cfgPath), "auth_token: new-token")
}

func TestRunAuthLoginForceReplacesWorkingToken(t *testing.T) {
	stub := newRTMStub(t, rtmStub{
		frob:         "FROB-3",
		tokenForFrob: map[string]string{"FROB-3": "replacement-token"},
		liveTokens: map[string]bool{
			"existing-token":    true,
			"replacement-token": true,
		},
	})
	client := newClientFor(t, stub, "")

	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv("RTM_CONFIG_FILE", cfgPath)
	seedConfig(t, cfgPath, "auth_token: existing-token\n")

	err := runAuthLogin(context.Background(), client, new(strings.Builder), strings.NewReader("\n"), authLoginOptions{
		Perms:     "read",
		NoBrowser: true,
		Force:     true,
	})
	require.NoError(t, err)
	assert.Contains(t, readFile(t, cfgPath), "auth_token: replacement-token")
}

func TestRunAuthLoginEmptyTokenFromRTM(t *testing.T) {
	stub := newRTMStub(t, rtmStub{
		frob:         "FROB-4",
		tokenForFrob: map[string]string{"FROB-4": ""},
	})
	client := newClientFor(t, stub, "")

	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv("RTM_CONFIG_FILE", cfgPath)

	err := runAuthLogin(context.Background(), client, new(strings.Builder), strings.NewReader("\n"), authLoginOptions{
		Perms:     "read",
		NoBrowser: true,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty token")
}
