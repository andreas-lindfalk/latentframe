// The assembler service owns pipeline stage 6 (ASSEMBLE): turn a property's
// artifacts into the shareable property page, and serve it with lead capture.
//
//	assembler build --spec property.json --out site/index.html          # deployable page
//	assembler build --spec property.json --out frag.html --fragment     # for embedding/preview
//	assembler serve --site site/index.html --leads leads.jsonl --addr :8080
//
// `build` inlines every asset (clip, stills) as a data URI, so the output is one
// self-contained file.
package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/andreas-lindfalk/latentframe/services/assembler/internal/site"
)

func main() {
	log.SetFlags(0)
	if len(os.Args) < 2 {
		log.Fatal("usage: assembler <build|serve> ...")
	}
	switch os.Args[1] {
	case "build":
		build(os.Args[2:])
	case "serve":
		serve(os.Args[2:])
	default:
		log.Fatalf("unknown command %q (use: build | serve)", os.Args[1])
	}
}

// spec is the on-disk property description consumed by `build`.
type spec struct {
	Title    string `json:"title"`
	Location string `json:"location"`
	Price    string `json:"price"`
	Blurb    string `json:"blurb"`
	Agent    string `json:"agent"`
	Beds     int    `json:"beds"`
	Baths    int    `json:"baths"`
	Area     int    `json:"area"`
	HeroClip string `json:"heroClip"`
	Rooms    []struct {
		Label  string `json:"label"`
		Before string `json:"before"`
		After  string `json:"after"`
	} `json:"rooms"`
}

func build(args []string) {
	fs := flag.NewFlagSet("build", flag.ExitOnError)
	specPath := fs.String("spec", "", "path to the property spec JSON (required)")
	out := fs.String("out", "", "output HTML path (required)")
	leadEndpoint := fs.String("lead-endpoint", "", "URL the enquiry form POSTs to (empty = client-only demo)")
	fragment := fs.Bool("fragment", false, "emit a body-only fragment instead of a full HTML document")
	_ = fs.Parse(args)
	if *specPath == "" || *out == "" {
		fs.Usage()
		log.Fatal("\n--spec and --out are required")
	}

	raw, err := os.ReadFile(*specPath)
	if err != nil {
		log.Fatal(err)
	}
	var s spec
	if err := json.Unmarshal(raw, &s); err != nil {
		log.Fatalf("parse spec: %v", err)
	}
	dir := filepath.Dir(*specPath)

	listing := site.Listing{
		Title: s.Title, Location: s.Location, Price: s.Price, Blurb: s.Blurb,
		Agent: s.Agent, Beds: s.Beds, Baths: s.Baths, Area: s.Area,
		HeroClip: template.URL(dataURI(resolve(dir, s.HeroClip))), LeadEndpoint: *leadEndpoint,
	}
	for _, r := range s.Rooms {
		listing.Rooms = append(listing.Rooms, site.RoomView{
			Label:  r.Label,
			Before: template.URL(dataURI(resolve(dir, r.Before))),
			After:  template.URL(dataURI(resolve(dir, r.After))),
		})
	}

	render := site.Standalone
	if *fragment {
		render = site.Render
	}
	html, err := render(listing)
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(*out, []byte(html), 0o644); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("✓ wrote %s (%d KB)\n", *out, len(html)/1024)
}

func serve(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	sitePath := fs.String("site", "", "path to the built property page (required)")
	leads := fs.String("leads", "leads.jsonl", "file to append captured leads to")
	addr := fs.String("addr", ":8080", "listen address")
	_ = fs.Parse(args)
	if *sitePath == "" {
		fs.Usage()
		log.Fatal("\n--site is required")
	}

	// One long-lived, mutex-guarded writer so concurrent enquiries append atomically.
	leadFile, err := os.OpenFile(*leads, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		log.Fatalf("open leads file: %v", err)
	}
	defer leadFile.Close()
	var leadMu sync.Mutex

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, *sitePath)
	})
	http.HandleFunc("/lead", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
			http.Error(w, "expected application/json", http.StatusUnsupportedMediaType)
			return
		}

		var lead struct {
			Name    string `json:"name"`
			Email   string `json:"email"`
			Phone   string `json:"phone"`
			Message string `json:"message"`
		}
		// Cap the body so an abusive or accidental payload can't blow up memory.
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&lead); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		lead.Name = strings.TrimSpace(lead.Name)
		lead.Email = strings.TrimSpace(lead.Email)
		if lead.Name == "" || !strings.Contains(lead.Email, "@") {
			http.Error(w, "name and a valid email are required", http.StatusBadRequest)
			return
		}

		record, _ := json.Marshal(struct {
			Name       string `json:"name"`
			Email      string `json:"email"`
			Phone      string `json:"phone"`
			Message    string `json:"message"`
			ReceivedAt string `json:"received_at"`
		}{lead.Name, lead.Email, strings.TrimSpace(lead.Phone), strings.TrimSpace(lead.Message),
			time.Now().UTC().Format(time.RFC3339)})

		leadMu.Lock()
		_, werr := leadFile.Write(append(record, '\n'))
		leadMu.Unlock()
		if werr != nil {
			http.Error(w, "storage error", http.StatusInternalServerError)
			return
		}
		log.Printf("lead captured: %s <%s>", lead.Name, lead.Email)
		w.WriteHeader(http.StatusNoContent)
	})

	log.Printf("serving %s on %s (leads → %s)", *sitePath, *addr, *leads)
	log.Fatal(http.ListenAndServe(*addr, nil))
}

func resolve(dir, p string) string {
	if strings.TrimSpace(p) == "" {
		return "" // missing asset path — keep it empty so dataURI reports it, rather than joining to (and reading) dir
	}
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(dir, p)
}

// dataURI reads a file and returns a data: URI, detecting the media type from bytes.
func dataURI(path string) string {
	if strings.TrimSpace(path) == "" {
		log.Fatal("assembler: a required asset path is missing in property.json (check heroClip and every room's before/after)")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("read asset %s: %v", path, err)
	}
	mime := http.DetectContentType(raw)
	// DetectContentType doesn't sniff mp4 reliably; trust the extension for video.
	if strings.EqualFold(filepath.Ext(path), ".mp4") {
		mime = "video/mp4"
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(raw)
}
