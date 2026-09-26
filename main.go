// Command wallgrab downloads Apple's Aerial wallpapers.
package main

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"maps"
	"math"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

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
		OS:           "macos",
		MacOSVersion: "v27.0",
		Lang:         "en",
		Dest:         "~/Pictures/backgrounds/aerials",
		logger:       func(string, ...any) {},
	}
	switch n := runtime.NumCPU(); {
	case n > 6:
		args.Streams = 8
	case n > 4:
		args.Streams = 6
	}
	ox.RunContext(
		context.Background(),
		ox.Usage("wallgrab", "download Apple Aerial wallpapers"),
		ox.Defaults(),
		ox.From(args),
		ox.Sub(
			ox.Exec(args.doList),
			ox.Usage("list", "list the available wallpapers"),
		),
		ox.Sub(
			ox.Exec(args.doShow),
			ox.Usage("show", "draw a thumbnail of each wallpaper in the terminal"),
		),
		ox.Sub(
			ox.Exec(args.doGrab),
			ox.Usage("grab", "download the wallpapers"),
		),
	)
}

type Args struct {
	Verbose      bool   `ox:"write progress and requests to stderr,short:v"`
	OS           string `ox:"operating system to get wallpapers for,name:os"`
	MacOSVersion string `ox:"major version to get wallpapers for,name:macos-version"`
	Streams      int    `ox:"number of downloads to run at the same time"`
	Sizes        bool   `ox:"show the size of each wallpaper"`
	Dest         string `ox:"directory to write the wallpapers to"`
	M3u          string `ox:"name of the playlist file to write"`
	UserAgent    string `ox:"user agent to send with each request"`
	Lang         string `ox:"language for the wallpaper names"`
	Clear        bool   `ox:"delete the cache directory before running"`

	resURL    string
	pool      *x509.CertPool
	resources *Resources
	loctable  map[string]map[string]any
	logger    func(string, ...any)
}

// setup prepares args for a command. It builds the user agent and the cert
// pool, then finds the URL of the resources tar.
func (args *Args) setup(ctx context.Context) error {
	// set verbose logger
	if args.Verbose {
		args.logger = func(s string, v ...any) {
			fmt.Fprintf(os.Stderr, s+"\n", v...)
		}
	}
	if args.Clear {
		if err := args.clearCache(ctx); err != nil {
			return fmt.Errorf("unable to clear cache: %w", err)
		}
	}
	now := time.Now()
	if err := args.buildUserAgent(ctx); err != nil {
		return fmt.Errorf("unable to build user agent: %w", err)
	}
	args.logger("user-agent: %s (%s)", args.UserAgent, time.Since(now))
	now = time.Now()
	if err := args.buildCertPool(ctx); err != nil {
		return fmt.Errorf("unable to build cert pool: %w", err)
	}
	args.logger("ca bundle: %s (%s)", appleCABundleURL, time.Since(now))
	now = time.Now()
	if err := args.getResURL(ctx); err != nil {
		return fmt.Errorf("unable to get res url: %w", err)
	}
	args.logger("resources: %s (%s)", args.resURL, time.Since(now))
	return nil
}

// doList writes the available wallpapers to stdout.
func (args *Args) doList(ctx context.Context) error {
	if err := args.setup(ctx); err != nil {
		return fmt.Errorf("unable to setup: %w", err)
	}
	if args.Verbose {
		if err := args.listLangs(ctx); err != nil {
			return fmt.Errorf("unable to list langs: %w", err)
		}
	}
	entries, err := args.getEntries(ctx)
	if err != nil {
		return fmt.Errorf("unable to get entries: %w", err)
	}
	if args.Sizes {
		if err := args.getSizes(ctx, entries); err != nil {
			return fmt.Errorf("unable to get sizes: %w", err)
		}
	}
	var total ox.Size
	for i, asset := range entries.Assets {
		var extra string
		if args.Sizes {
			extra = fmt.Sprintf(", %s", asset.Size)
		}
		fmt.Printf("%3d: %s (%s%s)\n", i+1, asset.String(), asset.ShotID, extra)
		total += asset.Size
	}
	if args.Sizes {
		fmt.Println("total:", total)
	}
	return nil
}

// doShow draws a thumbnail of each wallpaper in the terminal.
func (args *Args) doShow(ctx context.Context) error {
	if !rasterm.Available() {
		return rasterm.ErrTermGraphicsNotAvailable
	}
	if err := args.setup(ctx); err != nil {
		return fmt.Errorf("unable to setup: %w", err)
	}
	entries, err := args.getEntries(ctx)
	if err != nil {
		return fmt.Errorf("unable to get entries: %w", err)
	}
	if err := args.getSizes(ctx, entries); err != nil {
		return fmt.Errorf("unable to get sizes: %w", err)
	}
	for _, asset := range entries.Assets {
		fmt.Fprintf(os.Stdout, "%s (% .2z):\n", asset.String(), asset.Size)
		body, err := args.get(ctx, asset.PreviewImage)
		if err != nil {
			return fmt.Errorf("unable to get preview image: %w", err)
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

// doGrab downloads the wallpapers.
func (args *Args) doGrab(ctx context.Context) error {
	start := time.Now()
	if err := args.setup(ctx); err != nil {
		return fmt.Errorf("unable to setup: %w", err)
	}
	entries, err := args.getEntries(ctx)
	if err != nil {
		return fmt.Errorf("unable to get entries: %w", err)
	}
	if err := args.getSizes(ctx, entries); err != nil {
		return fmt.Errorf("unable to get sizes: %w", err)
	}
	if err := args.setDL(entries); err != nil {
		return fmt.Errorf("unable to set entries download: %w", err)
	}
	if err := args.getAssets(ctx, entries); err != nil {
		return fmt.Errorf("unable to get assets: %w", err)
	}
	// TODO: move ffprobe duration read into actual asset read, and put as part
	// TODO: of workload, to make go fast, vroom VROOM VROOOOOOOOOOOOM
	if err := args.addDur(ctx, entries); err != nil {
		return fmt.Errorf("unable to add ffmpeg duration: %w", err)
	}
	if err := args.writeM3U(entries); err != nil {
		return fmt.Errorf("unable to write m3u: %w", err)
	}
	args.logger("total: %s", time.Since(start))
	return nil
}

// getSizes reads the size of each wallpaper and stores it on the asset.
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

// setDL marks an asset for download when the local file is absent, or when
// its size differs from the size on the server.
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
			cl, err := args.client(ctx, false)
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
			r, err := bar.ProxyReader(res.Body)
			if err != nil {
				return err
			}
			defer r.Close()
			_, err = io.Copy(f, r)
			return err
		})
	}
	pool.StopAndWait()
	pb.Wait()
	return nil
}

// Resources holds the files read from a resources tar. Apple changed the tar
// layout with macOS v26, so exactly one of loctable and strings is set.
type Resources struct {
	// entries is the raw entries.json asset manifest.
	entries []byte
	// loctable is the combined localization table, used by macOS v26 and
	// later. It holds every language in one binary plist.
	loctable []byte
	// strings holds one plist per language, used by macOS v15 and earlier. It
	// is keyed by language.
	strings map[string][]byte
}

// getResources reads the resources tar and returns the manifest and the
// localized names. It reads the tar once, because the tar holds both and is
// over two megabytes.
func (args *Args) getResources(ctx context.Context) (*Resources, error) {
	if args.resources != nil {
		return args.resources, nil
	}
	res, err := args.readResources(ctx)
	if errors.Is(err, errCorrupt) {
		args.logger("%v (evicting and retrying)", err)
		if err := args.evict(ctx, args.resURL); err != nil {
			return nil, fmt.Errorf("unable to evict %s: %w", args.resURL, err)
		}
		if res, err = args.readResources(ctx); err != nil {
			return nil, fmt.Errorf("%w (run with --clear to delete the cache)", err)
		}
	}
	if err != nil {
		return nil, err
	}
	args.resources = res
	return res, nil
}

// readResources reads the manifest and the localized names from the resources
// tar.
func (args *Args) readResources(ctx context.Context) (*Resources, error) {
	body, err := args.get(ctx, args.resURL)
	if err != nil {
		return nil, err
	}
	defer body.Close()
	args.logger("reading tar: %s", args.resURL)
	res := &Resources{strings: make(map[string][]byte)}
	for r := tar.NewReader(body); ; {
		h, err := r.Next()
		switch {
		case errors.Is(err, io.EOF):
			if res.entries == nil {
				return nil, fmt.Errorf("%w: %s does not contain %s", errCorrupt, args.resURL, entriesName)
			}
			if res.loctable == nil && len(res.strings) == 0 {
				return nil, fmt.Errorf("%w: %s holds no localized names", errCorrupt, args.resURL)
			}
			return res, nil
		case err != nil:
			return nil, fmt.Errorf("%w: unable to read %s: %w", errCorrupt, args.resURL, err)
		}
		name := strings.TrimPrefix(h.Name, "./")
		// macOS copies a bundle with its resource forks, which land in the tar
		// as AppleDouble files. They are not part of the bundle.
		if kind, lang := classify(name); kind != "" && !strings.HasPrefix(path.Base(name), "._") {
			buf, err := io.ReadAll(r)
			if err != nil {
				return nil, fmt.Errorf("%w: unable to read %s from %s: %w", errCorrupt, name, args.resURL, err)
			}
			args.logger("read %s from tar (%d bytes)", name, len(buf))
			switch kind {
			case "entries":
				res.entries = buf
			case "loctable":
				res.loctable = buf
			case "strings":
				res.strings[lang] = buf
			}
		}
	}
}

// classify reports which resource a tar member holds, and for a per language
// plist, which language it holds. It returns an empty kind for every other
// member.
func classify(name string) (string, string) {
	switch {
	case name == entriesName:
		return "entries", ""
	case name == loctableName:
		return "loctable", ""
	case strings.HasPrefix(name, stringsPrefix) && strings.HasSuffix(name, stringsSuffix):
		lang := strings.TrimSuffix(strings.TrimPrefix(name, stringsPrefix), stringsSuffix)
		if lang == "" || strings.Contains(lang, "/") {
			return "", ""
		}
		return "strings", lang
	}
	return "", ""
}

// getLoctable returns the localization table. The table maps a language to its
// localization keys, and each key to its localized string.
func (args *Args) getLoctable(ctx context.Context) (map[string]map[string]any, error) {
	if args.loctable != nil {
		return args.loctable, nil
	}
	res, err := args.getResources(ctx)
	if err != nil {
		return nil, err
	}
	loctable := make(map[string]map[string]any)
	if res.loctable != nil {
		// macOS v26 and later: one binary plist holds every language. It also
		// holds a LocProvenance entry, and the values of that entry are not
		// strings.
		if err := plist.Unmarshal(res.loctable, &loctable); err != nil {
			return nil, fmt.Errorf("unable to read the localization table: %w", err)
		}
		delete(loctable, "LocProvenance")
	} else {
		// macOS v15 and earlier: one plist per language.
		for lang, buf := range res.strings {
			m := make(map[string]string)
			if err := plist.Unmarshal(buf, &m); err != nil {
				return nil, fmt.Errorf("unable to read the names for %s: %w", lang, err)
			}
			loctable[lang] = make(map[string]any, len(m))
			for k, v := range m {
				loctable[lang][k] = v
			}
		}
	}
	args.loctable = loctable
	return loctable, nil
}

// getNames returns the localized names for the language.
func (args *Args) getNames(ctx context.Context) (map[string]string, error) {
	loctable, err := args.getLoctable(ctx)
	if err != nil {
		return nil, err
	}
	lang, err := matchLang(slices.Sorted(maps.Keys(loctable)), args.Lang)
	if err != nil {
		return nil, err
	}
	args.logger("lang: %s", lang)
	m := make(map[string]string)
	for _, k := range slices.Sorted(maps.Keys(loctable[lang])) {
		s, ok := loctable[lang][k].(string)
		if !ok {
			continue
		}
		m[k] = strings.Join(strings.FieldsFunc(s, func(r rune) bool {
			return unicode.IsSpace(r) || !unicode.IsPrint(r)
		}), " ")
		args.logger("%s[%s]: %q", lang, k, m[k])
	}
	return m, nil
}

// matchLang matches the language against the available languages. It matches
// the name exactly, then without case, then by a language prefix that belongs
// to one language only. The prefix "zh" matches nothing, because zh_CN,
// zh_HK, and zh_TW are all available.
func matchLang(langs []string, lang string) (string, error) {
	name := strings.ReplaceAll(lang, "-", "_")
	if slices.Contains(langs, name) {
		return name, nil
	}
	lower := strings.ToLower(name)
	var matched []string
	for _, s := range langs {
		if v := strings.ToLower(s); v == lower || strings.HasPrefix(v, lower+"_") {
			matched = append(matched, s)
		}
	}
	switch {
	case len(matched) == 1:
		return matched[0], nil
	case len(matched) > 1:
		return "", fmt.Errorf("ambiguous language %q (matches: %s)", lang, strings.Join(matched, " "))
	}
	return "", fmt.Errorf("unknown language %q (available: %s)", lang, strings.Join(langs, " "))
}

// localized returns the localized string for the key. It returns the key
// itself when the language has no string for that key.
func localized(names map[string]string, key string) string {
	if s := names[key]; s != "" {
		return s
	}
	return key
}

// categoryPrefixes are the prefixes that Apple puts on a category or
// subcategory localization key, longest first.
var categoryPrefixes = []string{
	"AerialSubcategoryDescription",
	"AerialCategoryDescription",
	"AerialSubcategory",
	"AerialCategory",
}

// categoryName returns the localized name for a category or subcategory key.
// Apple ships some keys with no string for them, as macOS v26 does for
// AerialSubcategoryDescriptionMac. Read the name out of the key in that case,
// rather than show the reader the key itself.
func categoryName(names map[string]string, key string) string {
	if s := names[key]; s != "" {
		return s
	}
	for _, prefix := range categoryPrefixes {
		if s, ok := strings.CutPrefix(key, prefix); ok && s != "" {
			return s
		}
	}
	return key
}

// getEntries returns the asset manifest, with the localized names applied.
func (args *Args) getEntries(ctx context.Context) (*Entries, error) {
	res, err := args.getResources(ctx)
	if err != nil {
		return nil, err
	}
	buf := res.entries
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
		asset.Name = localized(names, asset.LocalizedNameKey)
		// the dynamic wallpapers use one name for both orientations. Add the
		// orientation to keep the two names apart.
		if asset.Variant != nil && asset.Variant.Orientation != "" {
			asset.Name += " (" + asset.Variant.Orientation + ")"
		}
		// add category names
		asset.CategoryNames = make([]string, len(asset.Categories))
		for i, id := range asset.Categories {
			s := categoryName(names, entries.GetCategory(id))
			asset.CategoryNames[i] = s
			args.logger("cat %s %d: %s -> %q", asset.LocalizedNameKey, i, id, s)
		}
		// add subcategory names
		asset.SubcategoryNames = make([]string, len(asset.Subcategories))
		for i, id := range asset.Subcategories {
			s := categoryName(names, entries.GetSubcategory(asset.Categories, id))
			asset.SubcategoryNames[i] = s
			args.logger("subcat %s %d: %s -> %q", asset.LocalizedNameKey, i, id, s)
		}
		entries.Assets[i] = asset
	}
	// some languages give two different wallpapers the same name. Arabic and
	// Slovenian translate both "Hong Kong Skyline" and "Hong Kong Horizon" the
	// same way. Add the shot id to every name that repeats.
	counts := make(map[string]int)
	for _, asset := range entries.Assets {
		counts[asset.String()]++
	}
	for i, asset := range entries.Assets {
		if name := asset.String(); counts[name] > 1 {
			args.logger("%q is not unique, qualifying with %s", name, asset.ShotID)
			asset.Name += " (" + asset.ShotID + ")"
			entries.Assets[i] = asset
		}
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
	loctable, err := args.getLoctable(ctx)
	if err != nil {
		return err
	}
	args.logger("langs: %s", strings.Join(slices.Sorted(maps.Keys(loctable)), " "))
	return nil
}

// getSize returns the size of an asset. It sends a HEAD request to the URL.
func (args *Args) getSize(ctx context.Context, asset Asset) (ox.Size, error) {
	args.logger("checking: %s %s", asset.ShotID, asset.String())
	args.logger("HEAD %s", asset.URL4kSdr240FPS)
	cl, err := args.client(ctx, true)
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

// addDur reads the duration of each downloaded file with ffprobe.
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

// buildUserAgent sets the user agent to the current stable Chrome user agent.
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
		verhist.PlatformString(),
		verhist.Stable.String(),
		verhist.WithTransport(cache),
	)
	return err
}

// buildCertPool builds the cert pool used to verify Apple's hosts. If the
// cached bundle cannot be read, buildCertPool deletes it from the cache and
// tries once more.
func (args *Args) buildCertPool(ctx context.Context) error {
	pool, err := args.certPool(ctx)
	if errors.Is(err, errCorrupt) {
		args.logger("%v (evicting and retrying)", err)
		if err := args.evict(ctx, appleCABundleURL); err != nil {
			return fmt.Errorf("unable to evict %s: %w", appleCABundleURL, err)
		}
		pool, err = args.certPool(ctx)
	}
	if err != nil {
		return err
	}
	args.pool = pool
	return nil
}

// certPool returns the system cert pool with Apple's root CA bundle added.
// Apple's configuration and asset hosts do not use a publicly trusted root.
func (args *Args) certPool(ctx context.Context) (*x509.CertPool, error) {
	// args.pool is still nil here, so the system roots verify this request
	buf, err := args.getAll(ctx, appleCABundleURL)
	if err != nil {
		return nil, err
	}
	pool, err := x509.SystemCertPool()
	if err != nil {
		return nil, err
	}
	if ok := pool.AppendCertsFromPEM(buf); !ok {
		return nil, fmt.Errorf("%w: %s contains no certificates", errCorrupt, appleCABundleURL)
	}
	return pool, nil
}

// cacheDir returns the artifact cache directory.
func (args *Args) cacheDir(ctx context.Context) (string, error) {
	c, _ := ox.Ctx(ctx)
	if c.Root.Name == "" {
		return "", errors.New("unable to determine cache directory")
	}
	return diskcache.UserCacheDir(c.Root.Name)
}

// clearCache deletes the whole artifact cache directory.
func (args *Args) clearCache(ctx context.Context) error {
	dir, err := args.cacheDir(ctx)
	if err != nil {
		return err
	}
	args.logger("removing: %s", dir)
	return os.RemoveAll(dir)
}

// evict deletes the cached response for one URL.
func (args *Args) evict(ctx context.Context, urlstr string) error {
	c, _ := ox.Ctx(ctx)
	cache, err := newDiskCache(c.Root.Name, http.DefaultTransport)
	if err != nil {
		return err
	}
	req, err := args.newReq(ctx, "GET", urlstr, nil)
	if err != nil {
		return err
	}
	args.logger("evicting: %s", urlstr)
	return cache.Evict(req)
}

// client returns an http client. It reads and writes the shared disk cache
// when cache is true.
func (args *Args) client(ctx context.Context, cache bool) (*http.Client, error) {
	var transport http.RoundTripper = http.DefaultTransport.(*http.Transport).Clone()
	transport.(*http.Transport).TLSClientConfig = &tls.Config{
		InsecureSkipVerify: false,
		// args.pool is nil until the Apple bundle arrives. A nil pool means
		// the system roots, which verify the download of the bundle itself.
		RootCAs: args.pool,
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

// newReq creates a request with the user agent set.
func (args *Args) newReq(ctx context.Context, method, urlstr string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, urlstr, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", args.UserAgent)
	return req, nil
}

// get returns the body of the URL, using the shared cache.
func (args *Args) get(ctx context.Context, urlstr string) (io.ReadCloser, error) {
	args.logger("GET %s", urlstr)
	cl, err := args.client(ctx, true)
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
	if res.StatusCode != http.StatusOK {
		_ = res.Body.Close()
		return nil, fmt.Errorf("%s: %s", urlstr, res.Status)
	}
	return res.Body, nil
}

func (args *Args) getAll(ctx context.Context, urlstr string) ([]byte, error) {
	body, err := args.get(ctx, urlstr)
	if err != nil {
		return nil, err
	}
	defer body.Close()
	return io.ReadAll(body)
}

// getResURL finds the URL of the resources tar for the release.
func (args *Args) getResURL(ctx context.Context) error {
	if args.resURL != "" {
		return nil
	}
	release, err := matchRelease(args.OS, args.MacOSVersion)
	if err != nil {
		return err
	}
	args.logger("release: %s %d (%s)", release.OS, release.Major, release.Config)
	buf, err := args.getAll(ctx, release.Config)
	if err != nil {
		return err
	}
	var v struct {
		ResourcesURL string `plist:"resources-url"`
	}
	if err := plist.Unmarshal(buf, &v); err != nil {
		return fmt.Errorf("unable to read %s: %w", release.Config, err)
	}
	if v.ResourcesURL == "" {
		return fmt.Errorf("%s holds no resources-url", release.Config)
	}
	args.resURL = v.ResourcesURL
	return nil
}

// Release describes one operating system release that Apple publishes aerial
// wallpapers for.
type Release struct {
	// OS is the operating system name.
	OS string
	// Major is the major version of the release.
	Major int
	// Config is the URL of the configuration plist, which names the resources
	// tar for the release.
	Config string
}

// releases lists the releases that Apple publishes a configuration for. Apple
// keeps an old configuration in place after a new one appears, so a release
// stays in this list once it is added.
//
// tvOS shares the configuration of macOS. The two operating systems read the
// same aerial wallpapers from the same tar, and the bundle of localized names
// inside that tar is still called TVIdleScreenStrings. Apple publishes no
// separate tvOS path.
var releases = []Release{
	// macOS v14 has no configuration of its own. The unversioned
	// configuration names the v14 tar, and is what a client that sends no
	// version receives.
	{OS: "macos", Major: 14, Config: resourcesConfigURL + "resources-config.plist"},
	{OS: "macos", Major: 15, Config: resourcesConfigURL + "resources-config-15-0.plist"},
	{OS: "macos", Major: 26, Config: resourcesConfigURL + "resources-config-26-0.plist"},
	{OS: "macos", Major: 27, Config: resourcesConfigURL + "resources-config-27-0.plist"},
}

// matchRelease returns the release for the operating system and version.
func matchRelease(osName, version string) (Release, error) {
	osName = strings.ToLower(strings.TrimSpace(osName))
	// tvOS reads the macOS configuration. See the note on releases.
	if osName == "tvos" {
		osName = "macos"
	}
	major, err := majorVersion(version)
	if err != nil {
		return Release{}, err
	}
	var known []string
	for _, release := range releases {
		if release.OS != osName {
			continue
		}
		if release.Major == major {
			return release, nil
		}
		known = append(known, strconv.Itoa(release.Major))
	}
	if len(known) == 0 {
		return Release{}, fmt.Errorf("unknown operating system %q (available: macos tvos)", osName)
	}
	return Release{}, fmt.Errorf("Apple publishes no aerial wallpapers for %s %d (available: %s)", osName, major, strings.Join(known, " "))
}

// majorVersion returns the major version number from a version string such as
// "v27.0", "27.0", or "27".
func majorVersion(version string) (int, error) {
	v := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(version)), "v")
	if i := strings.IndexRune(v, '.'); i != -1 {
		v = v[:i]
	}
	major, err := strconv.Atoi(v)
	if err != nil || major <= 0 {
		return 0, fmt.Errorf("invalid version %q", version)
	}
	return major, nil
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
	return id
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
	return id
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
	Variant            *Variant          `json:"variant"`
	VideoGravity       string            `json:"videoGravity"`

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

// Variant describes an asset that exists in more than one appearance or
// orientation. Only the dynamic wallpapers use it.
type Variant struct {
	Appearance  string `json:"appearance"`
	Orientation string `json:"orientation"`
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
	CombineVariants         bool   `json:"combineVariants"`
}

// newDiskCache creates a disk cache in the user cache directory.
func newDiskCache(name string, transport http.RoundTripper) (*diskcache.Cache, error) {
	cache, err := diskcache.New(
		diskcache.WithAppCacheDir(name),
		diskcache.WithMethod("GET", "HEAD"),
		diskcache.WithTTL(30*24*time.Hour),
		diskcache.WithHeaderWhitelist("Date", "Content-Type", "Content-Length"),
		diskcache.WithErrorTruncator(),
		diskcache.WithGzipCompression(),
		diskcache.WithTransport(transport),
		// diskcache matches a content type against the response header
		// exactly. A type that is not listed here keeps the 30 day TTL above.
		diskcache.WithContentTypeTTL(7*24*time.Hour, "text/xml", "application/octet-stream", "video/quicktime", "text/plain; charset=utf-8"),
	)
	return cache, err
}

// expand replaces a leading tilde (~) in a file name with the home
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

// ffprobeDuration returns the duration of a file in seconds. It runs ffprobe.
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

// errCorrupt reports that an artifact cannot be read, which means the cached
// response is truncated or corrupt. The caller deletes the cache entry and
// tries once more.
var errCorrupt = errors.New("corrupt")

const (
	// resourcesConfigURL is the URL that Apple publishes the aerial
	// configurations under.
	resourcesConfigURL = "https://configuration.apple.com/configurations/internetservices/aerials/"
	// appleCABundleURL is the URL of Apple's root CA bundle, published by
	// github.com/tls-inspector/rootca. This is the copy committed to the
	// repository, not the release asset. Both hold the same bytes, but the
	// release asset redirects to a signed URL that expires within the hour,
	// so nothing can cache it.
	appleCABundleURL = "https://raw.githubusercontent.com/tls-inspector/rootca/main/bundles/apple_ca_bundle.pem"
	// entriesName is the name of the asset manifest within the resources tar.
	entriesName = "entries.json"
	// loctableName is the name of the localization table in the resources tar.
	// Before macOS v27, each language had its own Localizable.nocache.strings
	// plist next to the Contents directory of the bundle.
	loctableName = "TVIdleScreenStrings.bundle/Contents/Resources/Localizable.nocache.loctable"
	// stringsPrefix and stringsSuffix wrap the language in the name of a per
	// language plist, as used by macOS v15 and earlier.
	stringsPrefix = "TVIdleScreenStrings.bundle/"
	stringsSuffix = ".lproj/Localizable.nocache.strings"
)
