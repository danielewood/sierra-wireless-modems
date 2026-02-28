package firmware

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/danielewood/sierra-wireless-modems/swtool/internal/log"
)

const (
	// FirmwarePage is the Sierra Wireless firmware listing page.
	FirmwarePage = "https://source.sierrawireless.com/resources/airprime/minicard/74xx/em_mc74xx-approved-fw-packages/"

	// MinFirmwareSize is the minimum expected firmware ZIP size in bytes (40MB).
	MinFirmwareSize = 40_000_000
)

// LegacyFirmware holds the known-good legacy firmware URL and filename.
var LegacyFirmware = FirmwareRef{
	URL:      "https://source.sierrawireless.com/-/media/support_downloads/airprime/74xx/fw/7455/swi9x30c_02,-d-,30,-d-,01,-d-,01_generic_002,-d-,045_001.ashx",
	Filename: "SWI9X30C_02.30.01.01_GENERIC_002.045_001.zip",
}

// FirmwareRef holds a firmware download URL and expected filename.
type FirmwareRef struct {
	URL      string
	Filename string
}

// FirmwareFiles holds paths to the extracted firmware files.
type FirmwareFiles struct {
	CWE string // Path to .cwe file
	NVU string // Path to .nvu file
}

// ScrapeLatestURL fetches the Sierra Wireless firmware page and extracts
// the latest EM7455 generic firmware download URL.
func ScrapeLatestURL(l *log.Logger) (*FirmwareRef, error) {
	l.Step("Fetching latest firmware URL from Sierra Wireless...")

	resp, err := http.Get(FirmwarePage)
	if err != nil {
		return nil, fmt.Errorf("fetching firmware page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("firmware page returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading firmware page: %w", err)
	}

	// Match href links containing swi9x30c...generic...ashx (case-insensitive)
	re := regexp.MustCompile(`(?i)href="([^"]*swi9x30c[^"]*generic[^"]*\.ashx)"`)
	matches := re.FindAllStringSubmatch(string(body), -1)

	for _, m := range matches {
		href := m[1]
		if strings.Contains(strings.ToLower(href), "7455") || strings.Contains(strings.ToLower(href), "74xx") {
			url := href
			if !strings.HasPrefix(url, "http") {
				url = "https://source.sierrawireless.com" + url
			}

			filename := DeriveFilename(url)
			l.Infof("Found firmware: %s", filename)

			return &FirmwareRef{
				URL:      url,
				Filename: filename,
			}, nil
		}
	}

	// Fallback: try any swi9x30c generic link
	for _, m := range matches {
		href := m[1]
		url := href
		if !strings.HasPrefix(url, "http") {
			url = "https://source.sierrawireless.com" + url
		}
		filename := DeriveFilename(url)
		l.Infof("Found firmware (fallback): %s", filename)

		return &FirmwareRef{
			URL:      url,
			Filename: filename,
		}, nil
	}

	return nil, fmt.Errorf("no firmware download link found on Sierra Wireless page")
}

// FindLocalFiles scans a directory for existing .cwe and .nvu firmware files.
// Used with --skip-download to find previously extracted firmware.
func FindLocalFiles(dir string) (*FirmwareFiles, error) {
	files := &FirmwareFiles{}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading directory: %w", err)
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		lower := strings.ToLower(e.Name())
		if strings.HasSuffix(lower, ".cwe") && strings.Contains(lower, "swi9x30c") {
			files.CWE = filepath.Join(dir, e.Name())
		}
		if strings.HasSuffix(lower, ".nvu") && strings.Contains(lower, "generic") {
			files.NVU = filepath.Join(dir, e.Name())
		}
	}

	if files.CWE == "" {
		return nil, fmt.Errorf("no .cwe firmware file found in %s", dir)
	}
	if files.NVU == "" {
		return nil, fmt.Errorf("no .nvu firmware file found in %s", dir)
	}

	return files, nil
}

// Download fetches a firmware ZIP file to the specified directory.
// Returns the path to the downloaded file. Skips download if a matching
// file already exists with the correct size.
func Download(l *log.Logger, ref *FirmwareRef, outputDir string) (string, error) {
	outPath := filepath.Join(outputDir, ref.Filename)

	// Check remote file size and status
	l.Debugf("HEAD %s", ref.URL)
	headResp, err := http.Head(ref.URL)
	if err != nil {
		return "", fmt.Errorf("checking firmware URL: %w", err)
	}
	headResp.Body.Close()

	if headResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("firmware URL returned HTTP %d", headResp.StatusCode)
	}

	remoteSize := headResp.ContentLength

	if remoteSize > 0 && remoteSize < MinFirmwareSize {
		return "", fmt.Errorf("remote file size (%d bytes) is unexpectedly small", remoteSize)
	}

	// Check if already downloaded with matching size
	if info, err := os.Stat(outPath); err == nil && remoteSize > 0 && info.Size() == remoteSize {
		l.Infof("Already downloaded %s (%d bytes)", ref.Filename, info.Size())
		return outPath, nil
	}

	// Download
	l.Step(fmt.Sprintf("Downloading %s ...", ref.Filename))
	if remoteSize > 0 {
		l.Infof("  Size: %d bytes (%.1f MB)", remoteSize, float64(remoteSize)/1048576)
	}

	resp, err := http.Get(ref.URL)
	if err != nil {
		return "", fmt.Errorf("downloading firmware: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("firmware download returned HTTP %d", resp.StatusCode)
	}

	f, err := os.Create(outPath)
	if err != nil {
		return "", fmt.Errorf("creating output file: %w", err)
	}
	defer f.Close()

	var src io.Reader = resp.Body
	spin := l.StartSpinner(fmt.Sprintf("Downloading... (%.1f MB)", float64(remoteSize)/1048576))
	defer spin.Stop()
	if remoteSize > 0 {
		src = &progressReader{r: resp.Body, total: remoteSize, spin: spin}
	}

	written, err := io.Copy(f, src)
	if err != nil {
		os.Remove(outPath)
		return "", fmt.Errorf("writing firmware file: %w", err)
	}

	// Verify size if known
	if remoteSize > 0 && written != remoteSize {
		os.Remove(outPath)
		return "", fmt.Errorf("download size mismatch: expected %d bytes, got %d", remoteSize, written)
	}

	l.Success(fmt.Sprintf("Downloaded %s (%d bytes)", ref.Filename, written))
	return outPath, nil
}

// Extract unzips a firmware ZIP and extracts .cwe and .nvu files to outputDir.
// Returns the paths to the extracted firmware files.
func Extract(l *log.Logger, zipPath, outputDir string) (*FirmwareFiles, error) {
	l.Step(fmt.Sprintf("Extracting %s...", filepath.Base(zipPath)))

	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, fmt.Errorf("opening zip: %w", err)
	}
	defer r.Close()

	files := &FirmwareFiles{}

	for _, f := range r.File {
		name := f.Name
		lower := strings.ToLower(name)

		var destPath *string
		if strings.HasSuffix(lower, ".cwe") {
			destPath = &files.CWE
		} else if strings.HasSuffix(lower, ".nvu") {
			destPath = &files.NVU
		} else {
			continue
		}

		outPath := filepath.Join(outputDir, filepath.Base(name))

		if err := extractZipFile(f, outPath); err != nil {
			return nil, fmt.Errorf("extracting %s: %w", name, err)
		}

		*destPath = outPath
		l.Infof("  Extracted: %s", filepath.Base(name))
	}

	if files.CWE == "" {
		return nil, fmt.Errorf("no .cwe file found in %s", filepath.Base(zipPath))
	}
	if files.NVU == "" {
		return nil, fmt.Errorf("no .nvu file found in %s", filepath.Base(zipPath))
	}

	l.Success("Firmware extracted successfully")
	return files, nil
}

// extractZipFile extracts a single file from the zip archive to destPath.
func extractZipFile(f *zip.File, destPath string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, rc)
	return err
}

// progressReader wraps an io.Reader and updates a spinner with download progress.
type progressReader struct {
	r       io.Reader
	total   int64
	read    int64
	lastPct int
	spin    *log.Spinner
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.r.Read(p)
	pr.read += int64(n)
	pct := int(pr.read * 100 / pr.total)
	if pct != pr.lastPct {
		pr.spin.Update(fmt.Sprintf("Downloading... %d%% (%.1f / %.1f MB)",
			pct, float64(pr.read)/1048576, float64(pr.total)/1048576))
		pr.lastPct = pct
	}
	return n, err
}

// DeriveFilename converts a Sierra Wireless .ashx URL to a readable filename.
// Input:  ".../swi9x30c_02,-d-,24,-d-,05,-d-,06_generic_002,-d-,026_000.ashx"
// Output: "SWI9X30C_02.24.05.06_GENERIC_002.026_000.zip"
func DeriveFilename(url string) string {
	base := filepath.Base(url)
	base = strings.TrimSuffix(base, ".ashx")
	base = strings.ReplaceAll(base, ",-d-,", ".")
	return strings.ToUpper(base) + ".zip"
}
