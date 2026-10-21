package config

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// legacyName is what onon was called before 0.8. MigrateLegacyDirs gives
// anyone upgrading their old settings under the new name.
const legacyName = "gopdf"

// legacyWord matches the old name as a word, which is how it appears in
// Lua: the global, require("gopdf.json"), and paths such as ~/.config/gopdf.
var legacyWord = regexp.MustCompile(`\b` + legacyName + `\b`)

// migrationNotes are what MigrateLegacyDirs did, for Open to show once.
var migrationNotes []string

// MigrateLegacyDirs copies each gopdf directory of the user's (config,
// data, cache) to an onon one beside it, where there is none yet, so
// session.sqlite, config.lua, themes and plugins carry over. Lua and
// plugin manifests in the copy are rewritten for the new name. The old
// directory is left as it was, as a backup. A symbolic link is left for
// the user to move, since its target is theirs to manage, often a
// dotfiles repository. Run it before anything opens the session database.
func MigrateLegacyDirs() {
	parents := platformAppDirParents()
	if dir, err := os.UserCacheDir(); err == nil {
		parents = append(parents, dir)
	}
	for _, parent := range unique(parents) {
		note, err := migrateLegacyDir(filepath.Join(parent, legacyName), filepath.Join(parent, "onon"))
		if err != nil {
			note = fmt.Sprintf("could not copy settings from %s: %v", filepath.Join(parent, legacyName), err)
		}
		if note != "" {
			log.Printf("config: %s", note)
			migrationNotes = append(migrationNotes, note)
		}
	}
}

// migrateLegacyDir copies old to dst when old exists and dst does not, and
// says what it did. It copies to a temporary name first, so a copy cut
// short is never taken for a finished one.
func migrateLegacyDir(old, dst string) (string, error) {
	info, err := os.Lstat(old)
	if err != nil {
		return "", nil // nothing to migrate
	}
	if _, err := os.Lstat(dst); !errors.Is(err, fs.ErrNotExist) {
		return "", nil // already migrated, or the user made one
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		return fmt.Sprintf("%s is a link and was not copied; rename it to %s and %q to \"onon\" in its Lua files", old, dst, legacyName), nil
	}
	if !info.IsDir() {
		return "", nil
	}
	tmp := dst + ".migrating"
	if err := os.RemoveAll(tmp); err != nil {
		return "", err
	}
	if err := copyLegacyTree(old, tmp); err != nil {
		os.RemoveAll(tmp)
		return "", err
	}
	if err := os.Rename(tmp, dst); err != nil {
		os.RemoveAll(tmp)
		return "", err
	}
	return fmt.Sprintf("copied settings from %s to %s; the old directory can be deleted", old, dst), nil
}

// copyLegacyTree copies src to dst, renaming gopdf-plugin.json manifests
// and rewriting the old name in them and in Lua files.
func copyLegacyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		name := d.Name()
		if name == legacyName+"-plugin.json" {
			rel = filepath.Join(filepath.Dir(rel), "onon-plugin.json")
		}
		target := filepath.Join(dst, rel)
		info, err := d.Info()
		if err != nil {
			return err
		}
		switch {
		case d.IsDir():
			return os.MkdirAll(target, info.Mode().Perm()|0o700)
		case info.Mode()&fs.ModeSymlink != 0:
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		case !info.Mode().IsRegular():
			return nil // sockets and the like belong to a running process
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.HasSuffix(name, ".lua") || strings.HasSuffix(name, "-plugin.json") {
			data = legacyWord.ReplaceAll(data, []byte("onon"))
		}
		return os.WriteFile(target, data, info.Mode().Perm())
	})
}
