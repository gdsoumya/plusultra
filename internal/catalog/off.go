package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/model"
	"github.com/gdsoumya/plusultra/internal/version"
)

const (
	offHost    = "world.openfoodfacts.org"
	offURL     = "https://" + offHost + "/api/v2/product/%s.json?fields=product_name,nutriments"
	offTimeout = 5 * time.Second
	offMaxBody = 1 << 20
)

type offResponse struct {
	Status  int `json:"status"`
	Product struct {
		Name      string `json:"product_name"`
		Nutriment struct {
			Kcal    *float64 `json:"energy-kcal_100g"`
			Protein float64  `json:"proteins_100g"`
			Carbs   float64  `json:"carbohydrates_100g"`
			Fat     float64  `json:"fat_100g"`
			Fiber   float64  `json:"fiber_100g"`
		} `json:"nutriments"`
	} `json:"product"`
}

// LookupBarcode returns the food for a barcode. userID's custom foods come first,
// then catalog rows (including earlier Open Food Facts lookups); otherwise the
// product is fetched from Open Food Facts and cached as source "off". Products
// without energy data are not cached. A nil client is NewOFFClient(5 s); a
// custom client must set its own timeout.
func LookupBarcode(ctx context.Context, q *sqlc.Queries, userID *uuid.UUID, client *http.Client, code string) (Food, error) {
	if !barcodeRE.MatchString(code) {
		return Food{}, invalid("barcode must be 8 to 14 digits")
	}
	f, err := q.GetFoodByBarcode(ctx, sqlc.GetFoodByBarcodeParams{Barcode: &code, UserID: userID})
	if err == nil {
		return f, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Food{}, err
	}

	p, err := fetchOFF(ctx, client, code)
	if err != nil {
		return Food{}, err
	}
	n := p.Product.Nutriment
	return foodRow{
		Source: "off", SourceID: code, Name: p.Product.Name, Barcode: code,
		Per100g: model.Macros{Kcal: *n.Kcal, Protein: n.Protein, Carbs: n.Carbs, Fat: n.Fat, Fiber: n.Fiber},
	}.save(ctx, q, nil)
}

// NewOFFClient returns an HTTP client for Open Food Facts that gives up after
// timeout and follows redirects only within the OFF host.
func NewOFFClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 || req.URL.Host != offHost {
			return http.ErrUseLastResponse
		}
		return nil
	}}
}

func fetchOFF(ctx context.Context, client *http.Client, code string) (offResponse, error) {
	if client == nil {
		client = NewOFFClient(offTimeout)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf(offURL, code), http.NoBody)
	if err != nil {
		return offResponse{}, err
	}
	req.Header.Set("User-Agent", "plusultra/"+version.Version+" (self-hosted)")
	resp, err := client.Do(req)
	if err != nil {
		return offResponse{}, fmt.Errorf("%w: %w", ErrUpstream, err)
	}
	defer resp.Body.Close() //nolint:errcheck // read-only
	if resp.StatusCode == http.StatusNotFound {
		return offResponse{}, ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return offResponse{}, fmt.Errorf("%w: open food facts returned %s", ErrUpstream, resp.Status)
	}
	var p offResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, offMaxBody)).Decode(&p); err != nil {
		return offResponse{}, fmt.Errorf("%w: %w", ErrUpstream, err)
	}
	if p.Status != 1 || strings.TrimSpace(p.Product.Name) == "" || p.Product.Nutriment.Kcal == nil {
		return offResponse{}, ErrNotFound
	}
	return p, nil
}
