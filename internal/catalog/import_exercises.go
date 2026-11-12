package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gdsoumya/plusultra/internal/db/sqlc"
)

const (
	sourceCurated = "curated"
	sourceFreeDB  = "free-exercise-db"
)

// freeEquipment maps free-exercise-db equipment to the tokens used in aliases.csv.
var freeEquipment = map[string]string{
	"body only": "bodyweight", "e-z curl bar": "ez bar", "kettlebells": "kettlebell",
	"bands": "band", "other": "",
}

type freeEntry struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Equipment        string   `json:"equipment"`
	Category         string   `json:"category"`
	PrimaryMuscles   []string `json:"primaryMuscles"`
	SecondaryMuscles []string `json:"secondaryMuscles"`
	Images           []string `json:"images"`
	Instructions     []string `json:"instructions"`
}

// ImportFreeExerciseDB loads the curated catalog and, from a free-exercise-db
// checkout in dir, enriches it with muscles, categories and images (copied to
// imagesDir/<slug>/). Entries the curated list does not cover are imported
// under their own dataset name with source "free-exercise-db"; they never get
// the bare-name alias, so a curated default always wins resolution. The slug is
// the identity: re-running is idempotent and catalog rows the import no longer
// produces are pruned unless a user has data on them.
func ImportFreeExerciseDB(ctx context.Context, q *sqlc.Queries, dir, imagesDir string) (int, error) {
	rows, err := CuratedRows()
	if err != nil {
		return 0, err
	}
	return importRows(ctx, q, rows, dir, imagesDir)
}

func importRows(ctx context.Context, q *sqlc.Queries, rows []Row, dir, imagesDir string) (int, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "dist", "exercises.json")) //nolint:gosec // operator-supplied path
	if err != nil {
		return 0, fmt.Errorf("catalog: read dataset: %w", err)
	}
	var entries []freeEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return 0, fmt.Errorf("catalog: parse dataset: %w", err)
	}

	drafts := map[string]*sqlc.UpsertExerciseParams{}
	bySource := map[string]string{}
	for _, r := range rows {
		p := r.params()
		drafts[p.Slug] = p
		if r.SourceID != "" {
			bySource[r.SourceID] = p.Slug
		}
	}
	filled := map[string]bool{}
	for _, e := range entries {
		slug, curated := bySource[e.ID]
		if !curated {
			name := strings.Join(strings.Fields(e.Name), " ")
			eq := normalizeToken(e.Equipment)
			if m, ok := freeEquipment[eq]; ok {
				eq = m
			}
			slug = slugify(name, eq)
			if _, taken := drafts[slug]; taken {
				continue
			}
			drafts[slug] = &sqlc.UpsertExerciseParams{Slug: slug, Name: name, Equipment: eq, Source: sourceFreeDB}
		}
		if !filled[slug] {
			filled[slug] = true
			if err := enrich(drafts[slug], e, dir, imagesDir); err != nil {
				return 0, err
			}
		}
	}

	fillStubs(rows, drafts)
	keep := slices.Sorted(maps.Keys(drafts))
	for _, slug := range keep {
		if _, err := q.UpsertExercise(ctx, *drafts[slug]); err != nil {
			return 0, fmt.Errorf("catalog: upsert %s: %w", slug, err)
		}
	}
	return len(keep), prune(ctx, q, keep)
}

// fillStubs gives a curated row without muscles those of another equipment
// variant of the same movement, so no stub counts as "unknown", and a row
// without images the images (and instructions, if it has none) of the default
// variant, or else of the first imaged one; the photos show the same movement.
// Image paths are shared, not copied: the route serves any <slug>/<file>.
func fillStubs(rows []Row, drafts map[string]*sqlc.UpsertExerciseParams) {
	byName := map[string][]*sqlc.UpsertExerciseParams{}
	for _, r := range rows {
		p := drafts[slugify(r.Name, r.Equipment)]
		if r.Default {
			byName[r.Name] = append([]*sqlc.UpsertExerciseParams{p}, byName[r.Name]...)
		} else {
			byName[r.Name] = append(byName[r.Name], p)
		}
	}
	for _, variants := range byName {
		for _, p := range variants {
			if len(p.PrimaryMuscles) == 0 {
				for _, o := range variants {
					if len(o.PrimaryMuscles) > 0 {
						p.PrimaryMuscles, p.SecondaryMuscles = o.PrimaryMuscles, o.SecondaryMuscles
						break
					}
				}
			}
			if len(p.Images) == 0 {
				for _, o := range variants {
					if len(o.Images) > 0 {
						p.Images = o.Images
						if len(p.Instructions) == 0 {
							p.Instructions = o.Instructions
						}
						break
					}
				}
			}
		}
	}
}

// prune drops catalog rows no longer in keep. Rows a user has data on stay, with a warning.
func prune(ctx context.Context, q *sqlc.Queries, keep []string) error {
	stale, err := q.ListStaleCatalog(ctx, keep)
	if err != nil {
		return err
	}
	var drop []string
	for _, s := range stale {
		if s.Referenced {
			slog.Warn("catalog exercise is no longer imported but has user data, keeping it", "slug", s.Slug)
			continue
		}
		drop = append(drop, s.Slug)
	}
	if len(drop) == 0 {
		return nil
	}
	return q.DeleteCatalogExercises(ctx, drop)
}

// params is the draft for a curated row. The default variant answers to the
// bare name and to "Name, Equipment"; the others are named with the suffix.
func (r Row) params() *sqlc.UpsertExerciseParams {
	p := &sqlc.UpsertExerciseParams{Slug: slugify(r.Name, r.Equipment), Name: r.Name, Equipment: r.Equipment, Source: sourceCurated,
		PrimaryMuscles: r.Muscles, SecondaryMuscles: r.Secondary}
	if r.Equipment == "" {
		return p
	}
	full := r.Name + ", " + equipmentTitle(r.Equipment)
	if r.Default {
		p.Aliases = []string{full}
	} else {
		p.Name = full
	}
	return p
}

// enrich fills p from a dataset entry and copies the entry's images.
func enrich(p *sqlc.UpsertExerciseParams, e freeEntry, dir, imagesDir string) error {
	images := make([]string, 0, len(e.Images))
	for _, img := range e.Images {
		if !filepath.IsLocal(img) {
			return fmt.Errorf("catalog: image path %q escapes the dataset", img)
		}
		data, err := os.ReadFile(filepath.Join(dir, "exercises", img)) //nolint:gosec // checked with IsLocal
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("catalog: read image: %w", err)
		}
		base := filepath.Base(img)
		if err := os.MkdirAll(filepath.Join(imagesDir, p.Slug), 0o750); err != nil {
			return fmt.Errorf("catalog: copy image: %w", err)
		}
		if err := os.WriteFile(filepath.Join(imagesDir, p.Slug, base), data, 0o600); err != nil { //nolint:gosec // slug is generated by slugify, base from filepath.Base
			return fmt.Errorf("catalog: copy image: %w", err)
		}
		images = append(images, p.Slug+"/"+base)
	}
	if name := strings.Join(strings.Fields(e.Name), " "); name != p.Name {
		p.Aliases = append(p.Aliases, name)
	}
	p.PrimaryMuscles, p.SecondaryMuscles = lowerAll(e.PrimaryMuscles), lowerAll(e.SecondaryMuscles)
	p.Category, p.Images, p.Instructions = e.Category, images, e.Instructions
	for i, step := range p.Instructions {
		p.Instructions[i] = truncateStep(step)
	}
	return nil
}

// maxStepLen is Exercise.instructions items max_len in exercise.proto; the
// dataset has longer steps.
const maxStepLen = 2000

func truncateStep(s string) string {
	r := []rune(s)
	if len(r) <= maxStepLen {
		return s
	}
	return string(r[:maxStepLen-1]) + "…"
}

func lowerAll(ss []string) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = normalizeToken(s)
	}
	return out
}
