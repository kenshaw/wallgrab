package main

import (
	"archive/tar"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
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
	"github.com/groob/plist"
	"github.com/kenshaw/colors"
	"github.com/kenshaw/diskcache"
	"github.com/kenshaw/httplog"
	"github.com/kenshaw/rasterm"
	"github.com/kenshaw/snaker"
	"github.com/spf13/cobra"
	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
)

var (
	name    = "wallgrab"
	version = "0.0.0-dev"
)

func main() {
	if err := run(context.Background(), name, version, os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, name, version string, cliargs []string) error {
	args := &Args{
		bg:         colors.FromColor(color.Transparent),
		logger:     func(string, ...interface{}) {},
		macosMajor: 15,
		macosMinor: 0,
		streams:    4,
		dest:       "~/Pictures/backgrounds/apple",
	}
	switch n := runtime.NumCPU(); {
	case n > 6:
		args.streams = 8
	case n > 4:
		args.streams = 6
	}
	var (
		bashCompletion       bool
		zshCompletion        bool
		fishCompletion       bool
		powershellCompletion bool
		noDescriptions       bool
	)
	c := &cobra.Command{
		Use:           name + " [flags] <image1> [image2, ..., imageN]",
		Short:         name + ", a command-line image viewer using terminal graphics",
		Version:       version,
		SilenceErrors: true,
		SilenceUsage:  false,
		RunE: func(cmd *cobra.Command, cliargs []string) error {
			// completions and short circuits
			switch {
			case bashCompletion:
				return cmd.GenBashCompletionV2(os.Stdout, !noDescriptions)
			case zshCompletion:
				if noDescriptions {
					return cmd.GenZshCompletionNoDesc(os.Stdout)
				}
				return cmd.GenZshCompletion(os.Stdout)
			case fishCompletion:
				return cmd.GenFishCompletion(os.Stdout, !noDescriptions)
			case powershellCompletion:
				if noDescriptions {
					return cmd.GenPowerShellCompletion(os.Stdout)
				}
				return cmd.GenPowerShellCompletionWithDesc(os.Stdout)
			}
			f := args.setup
			switch {
			case args.list:
				f = args.doList
			case args.show:
				f = args.doShow
			case args.grab:
				f = args.doGrab
			}
			return f(ctx)
		},
	}
	c.SetVersionTemplate("{{ .Name }} {{ .Version }}\n")
	c.SetArgs(cliargs[1:])
	// flags
	flags := c.Flags()
	flags.BoolVarP(&args.verbose, "verbose", "v", args.verbose, "enable verbose")
	flags.BoolVarP(&args.quiet, "quiet", "q", args.quiet, "enable quiet")
	flags.Var(args.bg.Pflag(), "bg", "background color")
	flags.IntVar(&args.macosMajor, "macos-major", args.macosMajor, "macOS major version")
	flags.IntVar(&args.macosMinor, "macos-minor", args.macosMinor, "macOS minor version")
	flags.IntVar(&args.streams, "streams", args.streams, "number of concurrent streams")
	flags.StringVar(&args.dest, "dest", args.dest, "destination path")
	flags.BoolVar(&args.list, "list", args.list, "list resources")
	flags.BoolVar(&args.show, "show", args.show, "show resources")
	flags.BoolVar(&args.grab, "grab", args.grab, "grab resources")
	// completions
	flags.BoolVar(&bashCompletion, "completion-script-bash", false, "output bash completion script and exit")
	flags.BoolVar(&zshCompletion, "completion-script-zsh", false, "output zsh completion script and exit")
	flags.BoolVar(&fishCompletion, "completion-script-fish", false, "output fish completion script and exit")
	flags.BoolVar(&powershellCompletion, "completion-script-powershell", false, "output powershell completion script and exit")
	flags.BoolVar(&noDescriptions, "no-descriptions", false, "disable descriptions in completion scripts")
	// mark hidden
	for _, name := range []string{
		"completion-script-bash", "completion-script-zsh", "completion-script-fish",
		"completion-script-powershell", "no-descriptions",
	} {
		flags.Lookup(name).Hidden = true
	}
	return c.ExecuteContext(ctx)
}

type Args struct {
	verbose    bool
	quiet      bool
	bg         colors.Color
	macosMajor int
	macosMinor int
	streams    int
	dest       string

	userAgent string
	resURL    string

	all  bool
	list bool
	show bool
	grab bool

	logger func(string, ...interface{})
}

// setup sets up the args.
func (args *Args) setup(ctx context.Context) error {
	// set verbose logger
	if args.verbose {
		args.logger = func(s string, v ...interface{}) {
			fmt.Fprintf(os.Stderr, s+"\n", v...)
		}
	}
	if err := args.buildUserAgent(ctx); err != nil {
		return err
	}
	now := time.Now()
	args.logger("user-agent: %s (%s)", args.userAgent, time.Since(now))
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
	entries, err := args.getEntries(ctx)
	if err != nil {
		return err
	}
	for i, asset := range entries.Assets {
		fmt.Printf("%d: %s %q\n", i, asset.Identifier(), asset.Title())
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
		fmt.Fprintf(os.Stdout, "%s (% .2f):\n", asset.Identifier(), decor.SizeB1024(asset.Size))
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
	args.logger("total: %s", time.Since(start))
	return nil
}

// getSizes adds the sizes for the files to the metadata.
func (args *Args) getSizes(ctx context.Context, entries *Entries) error {
	if len(entries.Assets) < 1 {
		return nil
	}
	pool := pond.NewPool(args.streams, pond.WithContext(ctx))
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
					var total int64
					for _, asset := range entries.Assets {
						total += asset.Size
					}
					return fmt.Sprintf("%s (% .2f)", s, decor.SizeB1024(total))
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
	baseDir := expand(u, args.dest)
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return err
	}
	for i, asset := range entries.Assets {
		if asset.Size == 0 {
			return fmt.Errorf("%s has size 0", asset.Identifier())
		}
		size, out := int64(0), filepath.Join(baseDir, asset.File())
		switch fi, err := os.Stat(out); {
		case errors.Is(err, os.ErrNotExist):
		case err != nil:
			return err
		case fi.IsDir():
			return fmt.Errorf("%s is a directory", out)
		default:
			size = fi.Size()
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
	n, total := len(entries.Assets[0].Identifier()), int64(0)
	for _, asset := range entries.Assets[1:] {
		if !asset.DL {
			continue
		}
		n, total = max(n, len(asset.Identifier())), total+asset.Size
	}
	// create task pool and progress bar
	pool := pond.NewPool(args.streams, pond.WithContext(ctx))
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
		args.logger("%s -> %s (% .2f)", asset.Identifier(), asset.Out, decor.SizeB1024(asset.Size))
		wg.Add(1)
		pool.SubmitErr(func() error {
			defer wg.Done()
			// out
			f, err := os.OpenFile(asset.Out, os.O_CREATE|os.O_TRUNC|os.O_WRONLY|os.O_APPEND, 0o644)
			if err != nil {
				return err
			}
			defer f.Close()
			// build client and request
			cl, err := args.client(true, false)
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
				asset.Size,
				mpb.BarStyle(),
				mpb.PrependDecorators(
					decor.Name(fmt.Sprintf("%- *s", n+2, asset.Identifier()+": ")),
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

// getSize gets the size for an asset, by performing a HEAD against the url.
func (args *Args) getSize(ctx context.Context, asset Asset) (int64, error) {
	args.logger("checking: %s (%s)", name, asset.Identifier())
	args.logger("HEAD %s", asset.URL4kSdr240FPS)
	cl, err := args.client(true, true)
	if err != nil {
		return 0, err
	}
	req, err := args.newReq(ctx, "HEAD", asset.URL4kSdr240FPS, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", args.userAgent)
	res, err := cl.Do(req)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()
	return res.ContentLength, nil
}

// getEntries gets the asset entries.
func (args *Args) getEntries(ctx context.Context) (*Entries, error) {
	body, err := args.get(ctx, args.resURL, true)
	if err != nil {
		return nil, err
	}
	defer body.Close()
	r := tar.NewReader(body)
	for {
		switch h, err := r.Next(); {
		case err != nil:
			return nil, err
		case h.Name == "./entries.json":
			entries := new(Entries)
			dec := json.NewDecoder(r)
			dec.DisallowUnknownFields()
			if err := dec.Decode(entries); err != nil {
				return nil, err
			}
			m := make(map[string]int)
			for i, asset := range entries.Assets {
				id := asset.Identifier()
				num, ok := m[id]
				if ok {
					num++
				}
				asset.Num, m[id] = num, num
				entries.Assets[i] = asset
			}
			sort.Slice(entries.Assets, func(i, j int) bool {
				return strings.Compare(entries.Assets[i].Identifier(), entries.Assets[j].Identifier()) < 0
			})
			return entries, nil
		}
	}
}

// buildUserAgent builds the user agent.
func (args *Args) buildUserAgent(ctx context.Context) error {
	if args.userAgent != "" {
		return nil
	}
	cache, err := newDiskCache(name, http.DefaultTransport.(*http.Transport).Clone())
	if err != nil {
		return err
	}
	args.userAgent, err = verhist.UserAgent(
		ctx,
		"linux",
		"stable",
		verhist.WithTransport(cache),
	)
	return err
}

// client returns the http client using the shared cache.
func (args *Args) client(insecure, cache bool) (*http.Client, error) {
	var transport http.RoundTripper = http.DefaultTransport.(*http.Transport).Clone()
	if insecure {
		transport.(*http.Transport).TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
	}
	if cache {
		if args.verbose {
			transport = httplog.NewPrefixedRoundTripLogger(
				transport,
				args.logger,
				httplog.WithReqResBody(false, false),
			)
		}
		var err error
		if transport, err = newDiskCache(name, transport); err != nil {
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
	req.Header.Set("User-Agent", args.userAgent)
	return req, nil
}

// get returns the url using the shared cache.
func (args *Args) get(ctx context.Context, urlstr string, insecure bool) (io.ReadCloser, error) {
	args.logger("GET %s insecure:%t", urlstr, insecure)
	cl, err := args.client(insecure, true)
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
	buf, err := args.getAll(ctx, fmt.Sprintf(resourcesConfigPlistURL, args.macosMajor, args.macosMinor), false)
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

	// state fields (not in json)
	Num  int    `json:"-"`
	Size int64  `json:"-"`
	Out  string `json:"-"`
	DL   bool   `json:"-"`
}

func (a Asset) String() string {
	switch {
	case a.AccessibilityLabel != "":
		return a.AccessibilityLabel
	case a.ShotID != "":
		return a.ShotID
	}
	return a.ID
}

func (a Asset) Title() string {
	if a.Num != 0 {
		return a.String() + " (" + strconv.Itoa(a.Num) + ")"
	}
	return a.String()
}

func (a Asset) Identifier() string {
	return snaker.CamelToSnakeIdentifier(a.String()) + a.NumString()
}

func (a Asset) NumString() string {
	if a.Num != 0 {
		return "_" + strconv.Itoa(a.Num)
	}
	return ""
}

func (a Asset) File() string {
	return a.Identifier() + path.Ext(a.URL4kSdr240FPS)
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

// resourcesConfigPlistURL is the resources config plist URL.
const resourcesConfigPlistURL = "https://configuration.apple.com/configurations/internetservices/aerials/resources-config-%d-%d.plist"
