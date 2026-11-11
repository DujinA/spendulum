package migrationcheck

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path"
	"regexp"
	"slices"
	"strconv"
)

var fileName = regexp.MustCompile(`^(\d+)_[a-z0-9_]+\.sql$`)

// Check enforces ADR 0008 on the migrations of head against those already on master (base).
func Check(base, head fs.FS) error {
	baseFiles, err := read(base)
	if err != nil {
		return fmt.Errorf("read master migrations: %w", err)
	}

	headFiles, err := read(head)
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}

	var errs []error

	lastOnMaster := int64(0)

	for _, name := range slices.Sorted(maps.Keys(baseFiles)) {
		if v, ok := version(name); ok {
			lastOnMaster = max(lastOnMaster, v)
		}

		content, ok := headFiles[name]

		switch {
		case !ok:
			errs = append(errs, fmt.Errorf("%s: deleted, but it is already on master", name))
		case !bytes.Equal(content, baseFiles[name]):
			errs = append(errs, fmt.Errorf("%s: changed, but it is already on master; add a new migration instead", name))
		}
	}

	seen := map[int64]string{}

	for _, name := range slices.Sorted(maps.Keys(headFiles)) {
		v, ok := version(name)
		if !ok {
			errs = append(errs, fmt.Errorf("%s: name must be <version>_<snake_case>.sql", name))

			continue
		}

		if other, dup := seen[v]; dup {
			errs = append(errs, fmt.Errorf("%s: duplicate version %d, also used by %s", name, v, other))
		}

		seen[v] = name

		if _, onMaster := baseFiles[name]; !onMaster && v <= lastOnMaster {
			errs = append(errs, fmt.Errorf("%s: out of order, version %d is not after %d, the last on master", name, v, lastOnMaster))
		}
	}

	return errors.Join(errs...)
}

func read(fsys fs.FS) (map[string][]byte, error) {
	names, err := fs.Glob(fsys, "*.sql")
	if err != nil {
		return nil, fmt.Errorf("list: %w", err)
	}

	files := make(map[string][]byte, len(names))

	for _, name := range names {
		b, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}

		files[path.Base(name)] = b
	}

	return files, nil
}

func version(name string) (int64, bool) {
	m := fileName.FindStringSubmatch(name)
	if m == nil {
		return 0, false
	}

	v, err := strconv.ParseInt(m[1], 10, 64)

	return v, err == nil
}
