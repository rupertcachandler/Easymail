//go:build android

package app

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Over-the-air update check for the Android build. The manifest lives on
// mail.btl.io (plain HTTP is fine: the manifest is Ed25519-signed with the
// key whose public half is pinned below — a network MITM cannot forge an
// update, and the APK sha256 is inside the signed payload).
//
// Publish flow (scripts/publish-android-update.sh): build APK, upload it,
// sign its sha256, write manifest.json. The app checks on launch and every
// 6h; a newer version raises "update-available"; the banner in the UI calls
// InstallUpdate(), which downloads, verifies and hands the APK to Android's
// package installer (user-approved, like any sideload).

const (
	updateManifestURL = "http://mail.btl.io/boi/manifest.json"
	updatePubKeyHex   = "124623905cd15178677ed425d4568f335810db932ba3a09787c2c5257dd56628"
)

type updateManifest struct {
	Version string `json:"version"`
	URL     string `json:"url"`
	SHA256  string `json:"sha256"`
	Notes   string `json:"notes"`
	Sig     string `json:"sig"` // base64 Ed25519 over the sha256 hex string
}

var (
	updateMu       sync.Mutex
	updatePending  *updateManifest
	updateDownload string // path of the verified downloaded APK, if any
)

func parseVer(s string) []int {
	var out []int
	for _, p := range strings.Split(strings.TrimPrefix(strings.TrimSpace(s), "v"), ".") {
		n := 0
		fmt.Sscanf(p, "%d", &n)
		out = append(out, n)
	}
	return out
}

func versionNewer(candidate, current string) bool {
	a, b := parseVer(candidate), parseVer(current)
	for i := 0; i < len(a) || i < len(b); i++ {
		x, y := 0, 0
		if i < len(a) {
			x = a[i]
		}
		if i < len(b) {
			y = b[i]
		}
		if x != y {
			return x > y
		}
	}
	return false
}

func fetchUpdateManifest() (*updateManifest, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(updateManifestURL + "?t=" + fmt.Sprint(time.Now().Unix()))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var m updateManifest
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, err
	}
	if m.Version == "" || m.URL == "" || m.SHA256 == "" || m.Sig == "" {
		return nil, fmt.Errorf("manifest missing fields")
	}
	pub, err := hex.DecodeString(updatePubKeyHex)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("bad pinned pubkey")
	}
	sig, err := base64.StdEncoding.DecodeString(m.Sig)
	if err != nil || len(sig) != ed25519.SignatureSize {
		return nil, fmt.Errorf("bad signature encoding")
	}
	if !ed25519.Verify(ed25519.PublicKey(pub), []byte(strings.ToLower(m.SHA256)), sig) {
		return nil, fmt.Errorf("manifest signature invalid")
	}
	return &m, nil
}

func startUpdateWatcher(a *App) {
	// Let the app settle (first sync, DB open) before the first check.
	time.Sleep(30 * time.Second)
	for {
		if m, err := fetchUpdateManifest(); err != nil {
			log.Printf("update check: %v", err)
		} else if versionNewer(m.Version, version) {
			updateMu.Lock()
			newer := updatePending == nil || updatePending.Version != m.Version
			updateMu.Unlock()
			if newer {
				updateMu.Lock()
				updatePending = m
				updateMu.Unlock()
				a.emitEvent("update-available", map[string]string{"version": m.Version, "notes": m.Notes})
				log.Printf("update available: %s (running %s)", m.Version, version)
			}
		}
		time.Sleep(10 * time.Minute)
	}
}

func updateApkDir() (string, error) {
	base := application.Android.StoragePath()
	if base == "" {
		return "", fmt.Errorf("android storage path unavailable")
	}
	dir := filepath.Join(base, "updates")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	return dir, nil
}

// InstallUpdate is the binding behind the in-app update banner: downloads
// the pending update (if not already), verifies its sha256, then hands it to
// Android's package installer via WailsBridge.installUpdate. Progress and
// failures surface as "update-progress" / "update-error" events.
func (a *App) InstallUpdate() {
	go func() {
		updateMu.Lock()
		m := updatePending
		updateMu.Unlock()
		if m == nil {
			if fm, err := fetchUpdateManifest(); err != nil || !versionNewer(fm.Version, version) {
				a.emitEvent("update-error", "no update pending")
				return
			} else {
				m = fm
				updateMu.Lock()
				updatePending = m
				updateMu.Unlock()
			}
		}

		updateMu.Lock()
		dl := updateDownload
		updateMu.Unlock()
		if dl == "" {
			a.emitEvent("update-progress", "downloading")
			dir, err := updateApkDir()
			if err != nil {
				a.emitEvent("update-error", err.Error())
				return
			}
			tmp := dir + "/boi-update.apk.part"
			if err := httpDownloadTo(m.URL, tmp); err != nil {
				a.emitEvent("update-error", "download failed: "+err.Error())
				return
			}
			sum, err := fileSHA256(tmp)
			if err != nil || !strings.EqualFold(sum, m.SHA256) {
				os.Remove(tmp)
				a.emitEvent("update-error", "checksum mismatch")
				return
			}
			dl = strings.TrimSuffix(tmp, ".part")
			os.Rename(tmp, dl)
			updateMu.Lock()
			updateDownload = dl
			updateMu.Unlock()
		}
		a.emitEvent("update-progress", "launching installer")
		platformInstallUpdate(dl)
	}()
}

func httpDownloadTo(url, dest string) error {
	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
