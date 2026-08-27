package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/Songxwn/rack-auto-ISO/internal/api"
	"github.com/Songxwn/rack-auto-ISO/internal/store"
)

var version = "dev"

func main() {
	dataDir := flag.String("data", envOr("IPXE_DATA", "./data"), "data directory for menus, ISOs and settings")
	listen := flag.String("listen", envOr("IPXE_LISTEN", ":8081"), "HTTP listen address")
	publicURL := flag.String("public-url", envOr("IPXE_PUBLIC_URL", ""), "public URL advertised to iPXE clients (e.g. http://192.168.1.10:8081)")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}

	if err := os.MkdirAll(*dataDir, 0o755); err != nil {
		log.Fatalf("create data dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(*dataDir, "isos"), 0o755); err != nil {
		log.Fatalf("create isos dir: %v", err)
	}

	st, err := store.Open(*dataDir)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	if *publicURL != "" {
		cfg := st.Settings()
		cfg.PublicURL = *publicURL
		if err := st.SaveSettings(cfg); err != nil {
			log.Fatalf("save settings: %v", err)
		}
	}

	srv := api.New(st, version)
	log.Printf("ipxe-manager %s listening on %s (data=%s)", version, *listen, *dataDir)
	if err := http.ListenAndServe(*listen, srv.Handler()); err != nil {
		log.Fatal(err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
