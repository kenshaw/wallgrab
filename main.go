// Command wallgrab downloads Apple's Aerial wallpapers.
package main

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/alitto/pond/v2"
	"github.com/chromedp/verhist"
	"github.com/kenshaw/diskcache"
	"github.com/kenshaw/httplog"
	"github.com/kenshaw/rasterm"
	"github.com/micromdm/plist"
	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
	"github.com/xo/ox"
)

func main() {
	args := &Args{
		MacosMajor: 15,
		MacosMinor: 0,
		Streams:    4,
		Lang:       "en",
		Dest:       "~/Pictures/backgrounds/aerials",
		logger:     func(string, ...any) {},
	}
	switch n := runtime.NumCPU(); {
	case n > 6:
		args.Streams = 8
	case n > 4:
		args.Streams = 6
	}
	ox.RunContext(
		context.Background(),
		ox.Usage("wallgrab", "a apple aerials wallpaper downloader"),
		ox.Defaults(),
		ox.From(args),
		ox.Sub(
			ox.Exec(args.doList),
			ox.Usage("list", "list available aerials"),
		),
		ox.Sub(
			ox.Exec(args.doShow),
			ox.Usage("show", "show available aerial thumbnails with term graphics"),
		),
		ox.Sub(
			ox.Exec(args.doGrab),
			ox.Usage("grab", "grab available aerials"),
		),
	)
}

type Args struct {
	Verbose    bool   `ox:"enable verbose,short:v"`
	Quiet      bool   `ox:"enable quiet,short:q"`
	MacosMajor int    `ox:"macOS major version"`
	MacosMinor int    `ox:"macOS minor version"`
	Streams    int    `ox:"concurrent streams"`
	Dest       string `ox:"dest"`
	M3u        string `ox:"m3u"`
	UserAgent  string `ox:"user agent"`
	Lang       string `ox:"language"`

	resURL string
	logger func(string, ...any)
}

// setup sets up the args.
func (args *Args) setup(ctx context.Context) error {
	// set verbose logger
	if args.Verbose {
		args.logger = func(s string, v ...any) {
			fmt.Fprintf(os.Stderr, s+"\n", v...)
		}
	}
	if err := args.buildUserAgent(ctx); err != nil {
		return err
	}
	now := time.Now()
	args.logger("user-agent: %s (%s)", args.UserAgent, time.Since(now))
	now = time.Now()
	if err := args.getResURL(ctx); err != nil {
		return err
	}
	args.logger("resources: %s (%s)", args.resURL, time.Since(now))
	return nil
}

// doList lists the available assets.
func (args *Args) doList(ctx context.Context) error {
	if err := args.setup(ctx); err != nil {
		return err
	}
	if args.Verbose {
		if err := args.listLangs(ctx); err != nil {
			return err
		}
	}
	entries, err := args.getEntries(ctx)
	if err != nil {
		return err
	}
	for i, asset := range entries.Assets {
		fmt.Printf("%3d: %s (%s)\n", i+1, asset.String(), asset.ShotID)
	}
	return nil
}

// doShow shows the assets in the terminal.
func (args *Args) doShow(ctx context.Context) error {
	if !rasterm.Available() {
		return rasterm.ErrTermGraphicsNotAvailable
	}
	if err := args.setup(ctx); err != nil {
		return err
	}
	entries, err := args.getEntries(ctx)
	if err != nil {
		return err
	}
	if err := args.getSizes(ctx, entries); err != nil {
		return err
	}
	for _, asset := range entries.Assets {
		fmt.Fprintf(os.Stdout, "%s (% .2z):\n", asset.String(), asset.Size)
		body, err := args.get(ctx, asset.PreviewImage, true)
		if err != nil {
			return err
		}
		img, _, err := image.Decode(body)
		if err != nil {
			_ = body.Close()
			fmt.Fprintf(os.Stdout, "error: %v\n\n", err)
			continue
		}
		if err := rasterm.Encode(os.Stdout, img); err != nil {
			fmt.Fprintf(os.Stdout, "error: %v\n\n", err)
			continue
		}
	}
	return nil
}

// doGrab grabs assets.
func (args *Args) doGrab(ctx context.Context) error {
	start := time.Now()
	if err := args.setup(ctx); err != nil {
		return err
	}
	entries, err := args.getEntries(ctx)
	if err != nil {
		return err
	}
	if err := args.getSizes(ctx, entries); err != nil {
		return err
	}
	if err := args.setDL(entries); err != nil {
		return err
	}
	if err := args.getAssets(ctx, entries); err != nil {
		return err
	}
	// TODO: move ffprobe duration read into actual asset read, and put as part
	// TODO: of workload, to make go fast, vroom VROOM VROOOOOOOOOOOOM
	if err := args.addDur(ctx, entries); err != nil {
		return err
	}
	if err := args.writeM3U(entries); err != nil {
		return err
	}
	args.logger("total: %s", time.Since(start))
	return nil
}

// getSizes adds the sizes for the files to the metadata.
func (args *Args) getSizes(ctx context.Context, entries *Entries) error {
	if len(entries.Assets) < 1 {
		return nil
	}
	pool := pond.NewPool(args.Streams, pond.WithContext(ctx))
	var wg sync.WaitGroup
	pb := mpb.NewWithContext(
		ctx,
		mpb.WithWidth(48),
		mpb.WithWaitGroup(&wg),
		mpb.WithAutoRefresh(),
	)
	bar := pb.New(
		int64(len(entries.Assets)),
		mpb.BarStyle(),
		mpb.PrependDecorators(decor.Name("(metadata)")),
		mpb.AppendDecorators(
			decor.OnCompleteMeta(
				decor.CountersNoUnit("%d / %d", decor.WCSyncWidth),
				func(s string) string {
					var total ox.Size
					for _, asset := range entries.Assets {
						total += asset.Size
					}
					return fmt.Sprintf("%s (% .2z)", s, total)
				},
			),
		),
	)
	for i, asset := range entries.Assets {
		wg.Add(1)
		pool.SubmitErr(func() error {
			defer bar.Increment()
			defer wg.Done()
			var err error
			if asset.Size, err = args.getSize(ctx, asset); err != nil {
				return err
			}
			entries.Assets[i] = asset
			return nil
		})
	}
	pool.StopAndWait()
	pb.Wait()
	return nil
}

// setDL sets whether or not to download the assets.
func (args *Args) setDL(entries *Entries) error {
	u, err := user.Current()
	if err != nil {
		return err
	}
	baseDir := expand(u, args.Dest)
	for i, asset := range entries.Assets {
		if asset.Size == 0 {
			return fmt.Errorf("%s has size 0", asset.String())
		}
		size, out := ox.Size(0), filepath.Join(baseDir, asset.String())
		switch fi, err := os.Stat(out); {
		case errors.Is(err, os.ErrNotExist):
		case err != nil:
			return err
		case fi.IsDir():
			return fmt.Errorf("%s is a directory", out)
		default:
			size = ox.Size(fi.Size())
		}
		asset.Out, asset.DL = out, size != asset.Size
		entries.Assets[i] = asset
	}
	return nil
}

func (args *Args) getAssets(ctx context.Context, entries *Entries) error {
	if len(entries.Assets) < 1 {
		return nil
	}
	// determine longest name and total size
	n, total := len(entries.Assets[0].String()), ox.Size(0)
	for _, asset := range entries.Assets[1:] {
		if !asset.DL {
			continue
		}
		n, total = max(n, len(asset.String())), total+asset.Size
	}
	// create task pool and progress bar
	pool := pond.NewPool(args.Streams, pond.WithContext(ctx))
	var wg sync.WaitGroup
	pb := mpb.NewWithContext(
		ctx,
		mpb.WithWidth(48),
		mpb.WithWaitGroup(&wg),
		mpb.WithAutoRefresh(),
	)
	for _, asset := range entries.Assets {
		if !asset.DL {
			continue
		}
		args.logger("%s -> %s (% .2z)", asset.ShotID, asset.Out, asset.Size)
		wg.Add(1)
		pool.SubmitErr(func() error {
			defer wg.Done()
			if err := os.MkdirAll(filepath.Dir(asset.Out), 0o755); err != nil {
				return err
			}
			// out
			f, err := os.OpenFile(asset.Out, os.O_CREATE|os.O_TRUNC|os.O_WRONLY|os.O_APPEND, 0o644)
			if err != nil {
				return err
			}
			defer f.Close()
			// build client and request
			cl, err := args.client(ctx, true, false)
			if err != nil {
				return err
			}
			args.logger("GET %s", asset.URL4kSdr240FPS)
			req, err := args.newReq(ctx, "GET", asset.URL4kSdr240FPS, nil)
			if err != nil {
				return err
			}
			// execute
			res, err := cl.Do(req)
			if err != nil {
				return err
			}
			defer res.Body.Close()
			// progress bar
			bar := pb.New(
				int64(asset.Size),
				mpb.BarStyle(),
				mpb.PrependDecorators(
					decor.Name(fmt.Sprintf("%- *s", n+2, asset.String()+": ")),
				),
				mpb.AppendDecorators(
					decor.OnComplete(
						decor.EwmaSpeed(decor.SizeB1024(0), "%- 4.2f", 0),
						"done",
					),
				),
			)
			// copy
			r := bar.ProxyReader(res.Body)
			defer r.Close()
			_, err = io.Copy(f, r)
			return err
		})
	}
	pool.StopAndWait()
	pb.Wait()
	return nil
}

func (args *Args) getNames(ctx context.Context) (map[string]string, error) {
	buf, err := args.getTarFile(ctx, "./TVIdleScreenStrings.bundle/"+args.Lang+".lproj/Localizable.nocache.strings")
	if err != nil {
		return nil, fmt.Errorf("could not find plist for language %s", args.Lang)
	}
	m := make(map[string]string)
	if err := plist.Unmarshal(buf, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// getEntries gets the asset entries.
func (args *Args) getEntries(ctx context.Context) (*Entries, error) {
	buf, err := args.getTarFile(ctx, "./entries.json")
	if err != nil {
		return nil, err
	}
	entries := new(Entries)
	dec := json.NewDecoder(bytes.NewReader(buf))
	dec.DisallowUnknownFields()
	if err := dec.Decode(entries); err != nil {
		return nil, err
	}
	names, err := args.getNames(ctx)
	if err != nil {
		return nil, err
	}
	for i, asset := range entries.Assets {
		asset.Name = names[asset.LocalizedNameKey]
		// add category names
		asset.CategoryNames = make([]string, len(asset.Categories))
		for i, id := range asset.Categories {
			asset.CategoryNames[i] = names[entries.GetCategory(id)]
		}
		// add subcategory names
		asset.SubcategoryNames = make([]string, len(asset.Subcategories))
		for i, id := range asset.Subcategories {
			asset.SubcategoryNames[i] = names[entries.GetSubcategory(asset.Categories, id)]
		}
		entries.Assets[i] = asset
	}
	m := make(map[string]bool)
	for _, asset := range entries.Assets {
		name := asset.String()
		if _, ok := m[name]; ok {
			return nil, fmt.Errorf("%s is not unique: %q", asset.ShotID, name)
		}
		m[name] = true
	}
	sort.Slice(entries.Assets, func(i, j int) bool {
		return entries.Assets[i].String() < entries.Assets[j].String()
	})
	return entries, nil
}

func (args *Args) listLangs(ctx context.Context) error {
	body, err := args.get(ctx, args.resURL, true)
	if err != nil {
		return err
	}
	defer body.Close()
	for r := tar.NewReader(body); ; {
		switch h, err := r.Next(); {
		case err != nil && errors.Is(err, io.EOF):
			return nil
		case err != nil:
			return err
		case strings.HasPrefix(h.Name, "./TVIdleScreenStrings.bundle/") && strings.HasSuffix(h.Name, ".lproj/Localizable.nocache.strings"):
			args.logger("lang: %s",
				strings.TrimSuffix(strings.TrimPrefix(h.Name, "./TVIdleScreenStrings.bundle/"), ".lproj/Localizable.nocache.strings"),
			)
		}
	}
}

func (args *Args) getTarFile(ctx context.Context, name string) ([]byte, error) {
	body, err := args.get(ctx, args.resURL, true)
	if err != nil {
		return nil, err
	}
	defer body.Close()
	args.logger("reading tar for: %s", name)
	for r := tar.NewReader(body); ; {
		switch h, err := r.Next(); {
		case err != nil:
			return nil, err
		case h.Name == name:
			return io.ReadAll(r)
		}
	}
}

// getSize gets the size for an asset, by performing a HEAD against the url.
func (args *Args) getSize(ctx context.Context, asset Asset) (ox.Size, error) {
	args.logger("checking: %s %s", asset.ShotID, asset.String())
	args.logger("HEAD %s", asset.URL4kSdr240FPS)
	cl, err := args.client(ctx, true, true)
	if err != nil {
		return 0, err
	}
	req, err := args.newReq(ctx, "HEAD", asset.URL4kSdr240FPS, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", args.UserAgent)
	res, err := cl.Do(req)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()
	return ox.Size(res.ContentLength), nil
}

func (args *Args) writeM3U(entries *Entries) error {
	if args.M3u == "" {
		return nil
	}
	u, err := user.Current()
	if err != nil {
		return err
	}
	baseDir := expand(u, args.Dest)
	out := filepath.Join(baseDir, args.M3u)
	if baseDir != filepath.Dir(out) {
		return fmt.Errorf("invalid m3u file name %q", args.M3u)
	}
	f, err := os.OpenFile(out, os.O_CREATE|os.O_TRUNC|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	fmt.Fprintln(f, "#EXTM3U")
	// title
	fmt.Fprintln(f, "#PLAYLIST: Wallpapers")
	for _, asset := range entries.Assets {
		fmt.Fprintf(f, "#EXTINF:%d,%s\n", int(asset.Dur.Seconds()), asset.Name)
		fmt.Fprintln(f, asset.String())
	}
	return f.Close()
}

// addDur loads the durations of the files using ffprobe.
func (args *Args) addDur(ctx context.Context, entries *Entries) error {
	for i, asset := range entries.Assets {
		dur, err := ffprobeDuration(ctx, asset.Out)
		if err != nil {
			return err
		}
		asset.Dur = time.Duration(dur) * time.Second
		args.logger("%s duration %s", asset.Out, asset.Dur)
		entries.Assets[i] = asset
	}
	return nil
}

// buildUserAgent builds the user agent.
func (args *Args) buildUserAgent(ctx context.Context) error {
	if args.UserAgent != "" {
		return nil
	}
	c, _ := ox.Ctx(ctx)
	cache, err := newDiskCache(c.Root.Name, http.DefaultTransport.(*http.Transport).Clone())
	if err != nil {
		return err
	}
	args.UserAgent, err = verhist.UserAgent(
		ctx,
		"linux",
		"stable",
		verhist.WithTransport(cache),
	)
	return err
}

// client returns the http client using the shared cache.
func (args *Args) client(ctx context.Context, insecure, cache bool) (*http.Client, error) {
	var transport http.RoundTripper = http.DefaultTransport.(*http.Transport).Clone()
	if insecure {
		transport.(*http.Transport).TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
	}
	if cache {
		if args.Verbose {
			transport = httplog.NewPrefixedRoundTripLogger(
				transport,
				args.logger,
				httplog.WithReqResBody(false, false),
			)
		}
		var err error
		c, _ := ox.Ctx(ctx)
		if transport, err = newDiskCache(c.Root.Name, transport); err != nil {
			return nil, err
		}
	}
	return &http.Client{
		Transport: transport,
	}, nil
}

// newReq creates a new request
func (args *Args) newReq(ctx context.Context, method, urlstr string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, urlstr, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", args.UserAgent)
	return req, nil
}

// get returns the url using the shared cache.
func (args *Args) get(ctx context.Context, urlstr string, insecure bool) (io.ReadCloser, error) {
	args.logger("GET %s insecure:%t", urlstr, insecure)
	cl, err := args.client(ctx, insecure, true)
	if err != nil {
		return nil, err
	}
	req, err := args.newReq(ctx, "GET", urlstr, nil)
	if err != nil {
		return nil, err
	}
	res, err := cl.Do(req)
	if err != nil {
		return nil, err
	}
	return res.Body, nil
}

func (args *Args) getAll(ctx context.Context, urlstr string, insecure bool) ([]byte, error) {
	body, err := args.get(ctx, urlstr, insecure)
	if err != nil {
		return nil, err
	}
	defer body.Close()
	return io.ReadAll(body)
}

// getResURL gets the resources url.
func (args *Args) getResURL(ctx context.Context) error {
	if args.resURL != "" {
		return nil
	}
	buf, err := args.getAll(ctx, fmt.Sprintf(resourcesConfigPlistURL, args.MacosMajor, args.MacosMinor), false)
	if err != nil {
		return err
	}
	var v struct {
		ResourcesURL string `plist:"resources-url"`
	}
	if err := plist.Unmarshal(buf, &v); err != nil {
		return err
	}
	args.resURL = v.ResourcesURL
	return nil
}

// Entries is the top level container for entries.json.
type Entries struct {
	Version             int        `json:"version"`
	LocalizationVersion string     `json:"localizationVersion"`
	Assets              []Asset    `json:"assets"`
	InitialAssetCount   int        `json:"initialAssetCount"`
	Categories          []Category `json:"categories"`
}

func (entries *Entries) GetCategory(id string) string {
	for _, category := range entries.Categories {
		if category.ID == id {
			return category.LocalizedNameKey
		}
	}
	panic(fmt.Sprintf("could not find category %s", id))
}

func (entries *Entries) GetSubcategory(categories []string, id string) string {
	if len(categories) == 1 {
		for _, category := range entries.Categories {
			if category.ID == categories[0] {
				for _, subcategory := range category.Subcategories {
					if subcategory.ID == id {
						return subcategory.LocalizedNameKey
					}
				}
			}
		}
	}
	panic(fmt.Sprintf("could not find subcategory %s", id))
}

// Asset contains asset information for entries.json.
type Asset struct {
	ID                 string            `json:"id"`
	ShowInTopLevel     bool              `json:"showInTopLevel"`
	ShotID             string            `json:"shotID"`
	LocalizedNameKey   string            `json:"localizedNameKey"`
	AccessibilityLabel string            `json:"accessibilityLabel"`
	PointsOfInterest   map[string]string `json:"pointsOfInterest"`
	PreviewImage       string            `json:"previewImage"`
	IncludeInShuffle   bool              `json:"includeInShuffle"`
	URL4kSdr240FPS     string            `json:"url-4K-SDR-240FPS"`
	Subcategories      []string          `json:"subcategories"`
	PreferredOrder     int               `json:"preferredOrder"`
	Categories         []string          `json:"categories"`
	Group              string            `json:"group"`

	// names
	Name             string   `json:"-"`
	CategoryNames    []string `json:"-"`
	SubcategoryNames []string `json:"-"`

	// state fields (not in json)
	Size ox.Size       `json:"-"`
	Out  string        `json:"-"`
	DL   bool          `json:"-"`
	Dur  time.Duration `json:"-"`
}

func (a Asset) Names() []string {
	return append(a.CategoryNames, append(a.SubcategoryNames, a.Name)...)
}

func (a Asset) String() string {
	return strings.Join(a.Names(), "/") + path.Ext(a.URL4kSdr240FPS)
}

// Category contains category information for entries.json.
type Category struct {
	ID                      string        `json:"id"`
	PreferredOrder          int           `json:"preferredOrder"`
	RepresentativeAssetID   string        `json:"representativeAssetID"`
	LocalizedNameKey        string        `json:"localizedNameKey"`
	Subcategories           []Subcategory `json:"subcategories"`
	LocalizedDescriptionKey string        `json:"localizedDescriptionKey"`
	PreviewImage            string        `json:"previewImage"`
}

// Subcategory contains subcategory information for entries.json.
type Subcategory struct {
	ID                      string `json:"id"`
	PreviewImage            string `json:"previewImage"`
	LocalizedNameKey        string `json:"localizedNameKey"`
	PreferredOrder          int    `json:"preferredOrder"`
	LocalizedDescriptionKey string `json:"localizedDescriptionKey"`
	RepresentativeAssetID   string `json:"representativeAssetID"`
}

// newDiskCache creates the a new disk cache.
func newDiskCache(name string, transport http.RoundTripper) (http.RoundTripper, error) {
	cache, err := diskcache.New(
		diskcache.WithAppCacheDir(name),
		diskcache.WithMethod("GET", "HEAD"),
		diskcache.WithTTL(30*24*time.Hour),
		diskcache.WithHeaderWhitelist("Date", "Content-Type", "Content-Length"),
		diskcache.WithErrorTruncator(),
		diskcache.WithGzipCompression(),
		diskcache.WithTransport(transport),
		diskcache.WithContentTypeTTL(7*24*time.Hour, "text/xml", "application/octet-stream", "video/quicktime"),
	)
	return cache, err
}

// expand expands the beginning tilde (~) in a file name to the provided home
// directory.
func expand(u *user.User, name string) string {
	switch {
	case name == "~":
		return u.HomeDir
	case strings.HasPrefix(name, "~/"):
		return filepath.Join(u.HomeDir, strings.TrimPrefix(name, "~/"))
	}
	return name
}

// ffprobeDuration uses ffprobe to determine the duration in seconds of a file.
func ffprobeDuration(ctx context.Context, name string) (int64, error) {
	ffprobeOnce.Do(func() {
		ffprobePath, _ = exec.LookPath("ffprobe")
	})
	if ffprobePath == "" {
		return -1, nil
	}
	// ffprobe -show_entries format=duration -of default=noprint_wrappers=1:nokey=1 california_wildflowers.mov 2>/dev/null
	cmd := exec.CommandContext(
		ctx,
		ffprobePath,
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		name,
	)
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, io.Discard
	if err := cmd.Run(); err != nil {
		return -1, err
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(buf.String()), 64)
	switch {
	case err != nil:
		return -1, err
	case f <= 0.0:
		return -1, fmt.Errorf("unable to determine duration for %q", name)
	}
	return int64(math.Ceil(f)), err
}

// ffprobe vars.
var (
	ffprobePath string
	ffprobeOnce sync.Once
)

// resourcesConfigPlistURL is the resources config plist URL.
const resourcesConfigPlistURL = "https://configuration.apple.com/configurations/internetservices/aerials/resources-config-%d-%d.plist"
