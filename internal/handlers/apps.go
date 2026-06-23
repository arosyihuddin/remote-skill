//go:build linux

package handlers

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pstar7/remote-skill/internal/handler"
	"github.com/pstar7/remote-skill/internal/proto"
)

func ListApps(ctx context.Context, raw json.RawMessage, _ handler.StreamWriter) (any, error) {
	var req proto.AppListRequest
	if raw != nil {
		json.Unmarshal(raw, &req)
	}

	dirs := []string{
		filepath.Join(os.Getenv("HOME"), ".local", "share", "applications"),
		"/usr/share/applications",
		"/usr/local/share/applications",
	}

	seen := make(map[string]bool)
	var apps []proto.AppInfo

	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".desktop") {
				continue
			}
			app := parseDesktopFile(filepath.Join(dir, e.Name()))
			if app == nil {
				continue
			}
			if seen[app.Name] {
				continue
			}
			seen[app.Name] = true
			if req.Filter != "" {
				lower := strings.ToLower(req.Filter)
				nameMatch := strings.Contains(strings.ToLower(app.Name), lower)
				commentMatch := strings.Contains(strings.ToLower(app.Comment), lower)
				catMatch := strings.Contains(strings.ToLower(app.Categories), lower)
				if !nameMatch && !commentMatch && !catMatch {
					continue
				}
			}
			apps = append(apps, proto.AppInfo{
				Name:       app.Name,
				Exec:       app.Exec,
				Icon:       app.Icon,
				Comment:    app.Comment,
				Categories: app.Categories,
				Terminal:   app.Terminal,
			})
		}
	}

	return proto.AppListResult{Apps: apps}, nil
}

func LaunchApp(ctx context.Context, raw json.RawMessage, _ handler.StreamWriter) (any, error) {
	var req proto.AppLaunchRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, fmt.Errorf("bad request: %w", err)
	}
	if req.Name == "" {
		return nil, fmt.Errorf("app name required")
	}

	app := findDesktopApp(req.Name)
	if app == nil {
		return nil, fmt.Errorf("app %q not found", req.Name)
	}

	execLine := parseExecLine(app.Exec, req.Args)
	cmd := exec.CommandContext(ctx, "sh", "-c", execLine)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("launch %q: %w", req.Name, err)
	}

	go cmd.Wait()
	return proto.EmptyResult{OK: true}, nil
}

type desktopEntry struct {
	Name       string
	Exec       string
	Icon       string
	Comment    string
	Categories string
	NoDisplay  bool
	Terminal   bool
	Type       string
}

func findDesktopApp(name string) *desktopEntry {
	dirs := []string{
		filepath.Join(os.Getenv("HOME"), ".local", "share", "applications"),
		"/usr/share/applications",
		"/usr/local/share/applications",
	}

	lower := strings.ToLower(name)
	var candidates []*desktopEntry

	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".desktop") {
				continue
			}
			app := parseDesktopFile(filepath.Join(dir, e.Name()))
			if app == nil {
				continue
			}
			if strings.EqualFold(app.Name, name) {
				return app
			}
			if strings.Contains(strings.ToLower(app.Name), lower) {
				candidates = append(candidates, app)
			}
		}
	}

	if len(candidates) > 0 {
		return candidates[0]
	}
	return nil
}

func parseDesktopFile(path string) *desktopEntry {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var e desktopEntry
	inDesktop := false
	sc := bufio.NewScanner(f)

	for sc.Scan() {
		line := sc.Text()
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "[") {
			inDesktop = line == "[Desktop Entry]"
			continue
		}
		if !inDesktop {
			continue
		}

		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)

		switch key {
		case "Type":
			e.Type = val
		case "NoDisplay":
			e.NoDisplay = val == "true"
		case "Terminal":
			e.Terminal = val == "true"
		case "Name":
			if e.Name == "" {
				e.Name = val
			}
		case "Icon":
			e.Icon = val
		case "Comment":
			e.Comment = val
		case "Categories":
			e.Categories = val
		case "Exec":
			e.Exec = val
		}
	}

	if e.Type != "Application" || e.NoDisplay || e.Name == "" || e.Exec == "" {
		return nil
	}

	return &e
}

func parseExecLine(template string, args []string) string {
	if !strings.Contains(template, "%") {
		if len(args) > 0 {
			return template + " " + strings.Join(args, " ")
		}
		return template
	}

	var b strings.Builder
	i := 0
	for i < len(template) {
		if template[i] == '%' && i+1 < len(template) {
			switch template[i+1] {
			case '%':
				b.WriteByte('%')
			case 'f', 'F', 'u', 'U':
				if len(args) > 0 {
					b.WriteString(args[0])
				}
			case 'i':
			case 'c':
			case 'k':
			default:
				b.WriteByte(template[i+1])
			}
			i += 2
		} else {
			b.WriteByte(template[i])
			i++
		}
	}
	return b.String()
}
