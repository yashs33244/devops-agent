package secrets

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// File is a Secrets backend that reads a YAML file, falling back to
// environment variables when a path is not present in the file.
//
// YAML shape:
//
//	secret/example/api-key:
//	  value: abc123
//	secret/nightshift/worker-hmac:
//	  secret: deadbeef
//
// Environment-variable fallback: if the YAML has no entry for path,
// the backend transforms path into an env-var name (slashes → _, upper,
// `NS_SECRET_` prefix) and returns `{"value": os.Getenv(...)}` if set.
type File struct {
	path string
	data map[string]map[string]string
}

// NewFile loads path. If path is empty or missing, the backend works
// in env-var-only mode.
func NewFile(path string) (*File, error) {
	f := &File{path: path, data: map[string]map[string]string{}}
	if path == "" {
		return f, nil
	}
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return f, nil
	}
	if err != nil {
		return nil, fmt.Errorf("secrets: read %s: %w", path, err)
	}
	if err := yaml.Unmarshal(raw, &f.data); err != nil {
		return nil, fmt.Errorf("secrets: parse %s: %w", path, err)
	}
	return f, nil
}

// Get returns the KV payload at path.
func (f *File) Get(ctx context.Context, path string) (map[string]string, error) {
	if v, ok := f.data[path]; ok {
		// Defensive copy so callers can't mutate the backing map.
		out := make(map[string]string, len(v))
		for k, val := range v {
			out[k] = val
		}
		return out, nil
	}
	if envVal := os.Getenv(envName(path)); envVal != "" {
		return map[string]string{"value": envVal}, nil
	}
	return nil, ErrNotFound
}

// Put returns ErrNotImplemented — the file backend is read-only.
// Operators wanting per-user credential writes (chunk 11
// SetConnectorStaticToken) must enable the OpenBao backend.
func (f *File) Put(ctx context.Context, path string, kv map[string]string) error {
	return ErrNotImplemented
}

// Delete returns ErrNotImplemented — see Put.
func (f *File) Delete(ctx context.Context, path string) error {
	return ErrNotImplemented
}

// List returns the immediate children of prefix from the in-memory
// map. Read-only; no env-var fallback (env keys aren't enumerable).
func (f *File) List(ctx context.Context, prefix string) ([]string, error) {
	if prefix == "" {
		return nil, nil
	}
	p := strings.TrimRight(prefix, "/") + "/"
	seen := map[string]struct{}{}
	for k := range f.data {
		if !strings.HasPrefix(k, p) {
			continue
		}
		rest := k[len(p):]
		if i := strings.IndexByte(rest, '/'); i >= 0 {
			rest = rest[:i]
		}
		if rest != "" {
			seen[rest] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}
	return out, nil
}

// envName transforms "secret/nightshift/worker-hmac" into
// "NS_SECRET_SECRET_NIGHTSHIFT_WORKER_HMAC". Matches the operator
// pattern used in deploy/charts/nightshift.
func envName(path string) string {
	r := strings.NewReplacer("/", "_", "-", "_", ".", "_")
	return "NS_SECRET_" + strings.ToUpper(r.Replace(path))
}
