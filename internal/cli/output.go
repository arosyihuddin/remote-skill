package cli

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"

	"github.com/pstar7/remote-skill/internal/proto"
)

func printScreenshot(raw json.RawMessage, savePath string) {
	var r proto.ScreenshotResult
	if err := json.Unmarshal(raw, &r); err != nil {
		fmt.Fprintf(os.Stderr, "error decoding screenshot: %v\n", err)
		os.Exit(1)
	}
	data, err := base64.StdEncoding.DecodeString(r.Base64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error decoding base64: %v\n", err)
		os.Exit(1)
	}
	path := savePath
	if path == "" {
		path = saveTemp("rsk-screenshot-*.png", data)
	} else {
		if err := os.WriteFile(path, data, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "error saving: %v\n", err)
			os.Exit(1)
		}
	}
	fmt.Printf("%s (%dx%d)\n", path, r.Width, r.Height)
}

func printRead(raw json.RawMessage, savePath string) {
	var r proto.ReadFileResult
	if err := json.Unmarshal(raw, &r); err != nil {
		fmt.Fprintf(os.Stderr, "error decoding read result: %v\n", err)
		os.Exit(1)
	}
	if !r.Base64 && savePath == "" {
		fmt.Println(r.Content)
		return
	}
	var data []byte
	if r.Base64 {
		var err error
		data, err = base64.StdEncoding.DecodeString(r.Content)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error decoding base64: %v\n", err)
			os.Exit(1)
		}
	} else {
		data = []byte(r.Content)
	}
	path := savePath
	if path == "" {
		path = saveTemp("rsk-read-*.bin", data)
	} else {
		if err := os.WriteFile(path, data, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "error saving: %v\n", err)
			os.Exit(1)
		}
	}
	fmt.Printf("%s (%d bytes)\n", path, r.SizeBytes)
}

func saveTemp(prefix string, data []byte) string {
	f, err := os.CreateTemp("", prefix)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating temp file: %v\n", err)
		os.Exit(1)
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		fmt.Fprintf(os.Stderr, "error writing temp file: %v\n", err)
		os.Exit(1)
	}
	_ = f.Close()
	return f.Name()
}
