package store

import "path"

// matchGlob reports whether name matches pattern using shell-style globbing
// ('*', '?', '[...]'). Redis's own KEYS pattern language is a superset of
// this (it additionally supports '\' escapes); path.Match covers the common
// interactive cases without pulling in a bespoke matcher.
func matchGlob(pattern, name string) (bool, error) {
	return path.Match(pattern, name)
}
