package main

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

//go:embed index.tmpl
var tpl string

const (
	rootVarName   = "PASTEBIN_ROOT"
	pasteFile     = "bin.txt"
	maxUploadSize = 50 * 1024 * 1024
)

func main() {
	t, err := template.New("index").Parse(tpl)
	if err != nil {
		log.Fatalf("template parse failed: %v", err)
	}

	rootDir, ok := os.LookupEnv(rootVarName)
	if !ok {
		log.Fatalf("environment variable %s should be set", rootVarName)
	}

	uploadsDirAbs := filepath.Join(rootDir, "uploads/")
	err = os.MkdirAll(uploadsDirAbs, 0750)
	if err != nil {
		log.Fatalf("upload dir creation failed: %v", err)
	}

	pasteFileAbs := filepath.Join(rootDir, pasteFile)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.FileServer(http.Dir(rootDir)))
	mux.Handle("GET /uploads/", http.FileServer(http.Dir(rootDir)))
	mux.HandleFunc("POST /uploads/", func(w http.ResponseWriter, req *http.Request) {
		req.Body = http.MaxBytesReader(w, req.Body, maxUploadSize)
		if err := req.ParseMultipartForm(maxUploadSize); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "400 %s", strings.ToLower(http.StatusText(http.StatusBadRequest)))
			return
		}

		file, fileHeader, err := req.FormFile("userfile")
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "400 %s", strings.ToLower(http.StatusText(http.StatusBadRequest)))
			return
		}

		defer file.Close()

		dst, err := os.Create(filepath.Join(
			uploadsDirAbs,
			fileHeader.Filename,
		))
		if err != nil {
			log.Printf("upload file creation failed: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		defer dst.Close()

		_, err = io.Copy(dst, file)
		if err != nil {
			log.Printf("upload file copying failed: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		http.Redirect(w, req, "/uploads/", http.StatusSeeOther)
	})
	mux.HandleFunc("POST /paste/", func(w http.ResponseWriter, req *http.Request) {
		req.ParseForm()
		t := strings.TrimSpace(
			strings.ToValidUTF8(
				(req.FormValue("paste")),
				"",
			),
		)

		if t == "" {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "400 %s", strings.ToLower(http.StatusText(http.StatusBadRequest)))
			return
		}

		err := os.WriteFile(pasteFileAbs, []byte(t), 0644)
		if err != nil {
			log.Printf("paste file writing failed: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		http.Redirect(w, req, "/", http.StatusSeeOther)
	})
	mux.HandleFunc("GET /", func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/" {
			http.NotFound(w, req)
			return
		}

		var paste []byte
		paste, err = os.ReadFile(pasteFileAbs)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				log.Fatal(err)
			}
		}

		err = t.ExecuteTemplate(w, "index", struct{ Paste string }{string(paste)})
		if err != nil {
			log.Printf("template execution failed: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	})
	srv := &http.Server{
		Addr:         ":8000",
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	go func() {
		<-ctx.Done()
		log.Println("shutting down")
		if err := srv.Shutdown(context.Background()); err != nil {
			log.Printf("server shutdown failed: %v", err)
		}
	}()

	log.Println("serving at port 8000")
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("server listen failed: %v", err)
	}
}
