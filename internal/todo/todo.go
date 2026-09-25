// Package todo is BurnMon Dev's optional Microsoft To Do panel (UI review
// patch, 2026-09-25, section 9): reads the signed-in user's own open tasks
// via Microsoft Graph, read-only. Ported from perfadvisor's own
// internal/todo (source commit perfadvisor main 2ed8046), with two changes
// from that source:
//
//  1. Its own token cache under %LOCALAPPDATA%\burnmon\ (cachePath below),
//     not perfadvisor's own %LOCALAPPDATA%\perfadvisor\, since the two are
//     separate sign-ins to the same Microsoft account, not a shared one.
//  2. Login is split into StartLogin (one fast HTTP call, returns the
//     device code and verification URL to show immediately) and FinishLogin
//     (the slow polling loop, run in a goroutine). perfadvisor's own Login
//     blocks a console until the user finishes on their phone or browser,
//     which is fine for a CLI but must never run synchronously on a
//     WebView2 binding's calling thread (the same UI-thread-blocking
//     concern cmd\burnmon-dev\main.go's other bdev* bindings already guard
//     against, e.g. bdevActivityHeatmap's own comment).
//
// Never called from internal/devexport or any export/log path: task titles
// are personal data the export bundle's own privacy rule ("never export
// prompt or response text") extends to by omission, not by a redaction
// rule here, so simply never importing this package from that path is
// sufficient (design doc's own "never in exports, logs or screenshots").
package todo

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Public client id used for the device-code sign-in. Defaults to the
// well-known "Microsoft Graph Command Line Tools" app, the same one
// perfadvisor's own package uses; override with BURNMON_TODO_CLIENT_ID if
// your tenant blocks it or IT provides a dedicated app registration.
const defaultClientID = "14d82eec-204b-4c2f-b7e8-296a70dab67e"

const (
	scope     = "Tasks.Read offline_access"
	deviceURL = "https://login.microsoftonline.com/common/oauth2/v2.0/devicecode"
	tokenURL  = "https://login.microsoftonline.com/common/oauth2/v2.0/token"
	graphBase = "https://graph.microsoft.com/v1.0"
)

func clientID() string {
	if v := os.Getenv("BURNMON_TODO_CLIENT_ID"); v != "" {
		return v
	}
	return defaultClientID
}

var httpc = &http.Client{Timeout: 20 * time.Second}

type storedToken struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type tokenResp struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	ExpiresIn        int    `json:"expires_in"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// cachePath is burnmon-dev's own token cache, under the same
// %LOCALAPPDATA%\burnmon\ folder as burnmon.db and burnmon-dev.db (cmd\
// burnmon-dev\app.go's appDataDir), not perfadvisor's own data folder.
func cachePath() (string, error) {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = home
	}
	dir := filepath.Join(base, "burnmon")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return filepath.Join(dir, "burnmon-dev-msgraph-token.json"), nil
}

// tokenMu guards saveToken/loadToken/Logout: the 5-minute task poll, a
// status check moving to signed-in, and a login's own FinishLogin can all
// touch the same file from different goroutines (found by review,
// 2026-09-25).
var tokenMu sync.Mutex

func saveToken(tr tokenResp) error {
	p, err := cachePath()
	if err != nil {
		return err
	}
	t := storedToken{
		AccessToken:  tr.AccessToken,
		RefreshToken: tr.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(tr.ExpiresIn-60) * time.Second),
	}
	data, err := json.Marshal(t)
	if err != nil {
		return err
	}
	tokenMu.Lock()
	defer tokenMu.Unlock()
	// Write to a temp file and rename over the real one: a reader
	// (loadToken, from a different goroutine) can never observe a
	// partially-written file this way, only the old or the new complete
	// one (found by review, 2026-09-25).
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

func loadToken() (*storedToken, error) {
	p, err := cachePath()
	if err != nil {
		return nil, err
	}
	tokenMu.Lock()
	data, err := os.ReadFile(p)
	tokenMu.Unlock()
	if err != nil {
		return nil, err
	}
	var t storedToken
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

// SignedIn reports whether a cached token exists at all (not whether it is
// still valid: accessToken below refreshes an expired one), so the panel
// can tell "never signed in" (show the sign-in button) apart from "signed
// in, showing tasks or a transient error".
func SignedIn() bool {
	_, err := loadToken()
	return err == nil
}

// Logout removes the cached token.
func Logout() error {
	p, err := cachePath()
	if err != nil {
		return err
	}
	tokenMu.Lock()
	defer tokenMu.Unlock()
	err = os.Remove(p)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func postForm(u string, form url.Values, out any) error {
	resp, err := httpc.PostForm(u, form)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, out)
}

// DeviceCodeInfo is StartLogin's result: enough to show the user code and
// verification URL in the panel and open the browser immediately, plus the
// fields FinishLogin's own polling loop needs (kept on the same struct so
// the panel binding only has to hold one value between the two calls).
type DeviceCodeInfo struct {
	UserCode        string
	VerificationURI string
	Message         string

	deviceCode string
	interval   int
	expiresIn  int
}

// StartLogin makes the one fast HTTP call the OAuth device-code flow starts
// with: the panel shows UserCode/VerificationURI/Message and opens the
// browser to VerificationURI as soon as this returns, then calls
// FinishLogin (in a goroutine) with the same DeviceCodeInfo to actually
// wait for the user to approve it.
func StartLogin() (DeviceCodeInfo, error) {
	var dc struct {
		DeviceCode      string `json:"device_code"`
		UserCode        string `json:"user_code"`
		VerificationURI string `json:"verification_uri"`
		Interval        int    `json:"interval"`
		ExpiresIn       int    `json:"expires_in"`
		Message         string `json:"message"`
	}
	form := url.Values{"client_id": {clientID()}, "scope": {scope}}
	if err := postForm(deviceURL, form, &dc); err != nil {
		return DeviceCodeInfo{}, err
	}
	if dc.DeviceCode == "" {
		return DeviceCodeInfo{}, errors.New("could not start device sign-in; your tenant may block this client id (set BURNMON_TODO_CLIENT_ID)")
	}
	msg := dc.Message
	if msg == "" {
		msg = fmt.Sprintf("Open %s and enter the code %s", dc.VerificationURI, dc.UserCode)
	}
	return DeviceCodeInfo{
		UserCode: dc.UserCode, VerificationURI: dc.VerificationURI, Message: msg,
		deviceCode: dc.DeviceCode, interval: dc.Interval, expiresIn: dc.ExpiresIn,
	}, nil
}

// FinishLogin polls until the user approves the sign-in (or it times out),
// then caches the resulting token. Blocks for as long as the sign-in takes
// (the interactive step, "open a browser and enter the code"), so a caller
// on a WebView2 binding thread must run this in a goroutine, exactly the
// same pattern this file's own package doc explains.
func FinishLogin(info DeviceCodeInfo) error {
	interval := info.interval
	if interval <= 0 {
		interval = 5
	}
	expiresIn := info.expiresIn
	if expiresIn <= 0 {
		expiresIn = 900 // Microsoft's own device-code default, in case the response omitted it
	}
	deadline := time.Now().Add(time.Duration(expiresIn) * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(time.Duration(interval) * time.Second)
		var tr tokenResp
		form := url.Values{
			"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
			"client_id":   {clientID()},
			"device_code": {info.deviceCode},
		}
		if err := postForm(tokenURL, form, &tr); err != nil {
			return err
		}
		if tr.AccessToken != "" {
			return saveToken(tr)
		}
		switch tr.Error {
		case "authorization_pending":
			// keep polling
		case "slow_down":
			interval += 5
		default:
			if tr.ErrorDescription != "" {
				return errors.New(tr.ErrorDescription)
			}
			return errors.New("sign-in failed: " + tr.Error)
		}
	}
	return errors.New("sign-in timed out")
}

func accessToken() (string, error) {
	t, err := loadToken()
	if err != nil {
		return "", errors.New("not signed in")
	}
	if time.Now().Before(t.ExpiresAt) && t.AccessToken != "" {
		return t.AccessToken, nil
	}
	var tr tokenResp
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {clientID()},
		"refresh_token": {t.RefreshToken},
		"scope":         {scope},
	}
	if err := postForm(tokenURL, form, &tr); err != nil {
		return "", err
	}
	if tr.AccessToken == "" {
		// The refresh token itself was rejected (revoked or expired), not a
		// transient network error: forget the cached token so SignedIn
		// reports false and the panel's own sign-in button reappears
		// instead of the panel silently getting stuck (found by review,
		// 2026-09-25).
		_ = Logout()
		return "", errors.New("sign-in expired: sign in again")
	}
	if err := saveToken(tr); err != nil {
		return "", err
	}
	return tr.AccessToken, nil
}

func get(u, tok string, out any) error {
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := httpc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("Microsoft Graph returned %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// Item is one task shown in the panel.
type Item struct {
	Title   string
	List    string
	DueDate string // yyyy-mm-dd, empty when the task has no due date
	Overdue bool
}

// TodayTasks returns open tasks due today or overdue, across all lists.
// Note: Graph does not expose To Do's "My Day", so due date is the
// criterion (same limitation perfadvisor's own package documents).
func TodayTasks() ([]Item, error) {
	tok, err := accessToken()
	if err != nil {
		return nil, err
	}
	var lists struct {
		Value []struct {
			ID          string `json:"id"`
			DisplayName string `json:"displayName"`
		} `json:"value"`
	}
	if err := get(graphBase+"/me/todo/lists?$top=20", tok, &lists); err != nil {
		return nil, err
	}
	today := time.Now().Format("2006-01-02")
	var out []Item
	for i, l := range lists.Value {
		if i >= 10 {
			break
		}
		var tasks struct {
			Value []struct {
				Title       string `json:"title"`
				Status      string `json:"status"`
				DueDateTime *struct {
					DateTime string `json:"dateTime"`
				} `json:"dueDateTime"`
			} `json:"value"`
		}
		u := graphBase + "/me/todo/lists/" + url.PathEscape(l.ID) +
			"/tasks?$top=100&$filter=" + url.QueryEscape("status ne 'completed'")
		if err := get(u, tok, &tasks); err != nil {
			continue // one broken list should not kill the panel
		}
		for _, t := range tasks.Value {
			if t.DueDateTime == nil || len(t.DueDateTime.DateTime) < 10 {
				continue
			}
			due := t.DueDateTime.DateTime[:10]
			if due > today {
				continue
			}
			out = append(out, Item{Title: t.Title, List: l.DisplayName, DueDate: due, Overdue: due < today})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Overdue != out[j].Overdue {
			return out[i].Overdue
		}
		return out[i].DueDate < out[j].DueDate
	})
	return out, nil
}
