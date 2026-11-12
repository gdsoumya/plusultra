package api

import (
	"cmp"
	"context"
	"encoding/json"
	"slices"
	"strings"
	"time"

	"connectrpc.com/connect"

	pb "github.com/gdsoumya/plusultra/gen/go/plusultra/v1"
	"github.com/gdsoumya/plusultra/internal/catalog"
	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/model"
)

const defaultFoodLimit = 20

// offClient fetches barcodes the catalog lacks from Open Food Facts; tests point it at a fake server.
var offClient = catalog.NewOFFClient(5 * time.Second)

type foodServer struct{ q *sqlc.Queries }

// foodPB converts f, naming its concept when it belongs to one.
func (s *foodServer) foodPB(ctx context.Context, f catalog.Food) (*pb.Food, error) {
	if f.ConceptID == nil {
		return foodWith(f, "")
	}
	c, err := s.q.GetFoodConcept(ctx, *f.ConceptID)
	if err != nil {
		return nil, err
	}
	return foodWith(f, c.Name)
}

func foodWith(f catalog.Food, conceptName string) (*pb.Food, error) {
	var per100g model.Macros
	var units []catalog.Unit
	if err := json.Unmarshal(f.Per100g, &per100g); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(f.Units, &units); err != nil {
		return nil, err
	}
	out := &pb.Food{Id: f.ID.String(), Source: f.Source, SourceId: f.SourceID, Name: f.Name, Aliases: f.Aliases,
		Per100G: macrosPB(per100g), Custom: f.UserID != nil, ConceptName: conceptName}
	if f.Barcode != nil {
		out.Barcode = *f.Barcode
	}
	if f.ConceptID != nil {
		out.ConceptId = *f.ConceptID
	}
	for _, u := range units {
		if out.Units == nil {
			out.Units = map[string]float64{}
		}
		out.Units[u.Name] = u.Grams
	}
	return out, nil
}

func customFoodOf(f *pb.Food) catalog.CustomFood {
	c := catalog.CustomFood{Name: f.GetName(), Aliases: f.GetAliases(), Barcode: f.GetBarcode(), Per100g: macrosOf(f.GetPer100G())}
	for name, grams := range f.GetUnits() {
		c.Units = append(c.Units, catalog.Unit{Name: name, Grams: grams})
	}
	slices.SortFunc(c.Units, func(a, b catalog.Unit) int { return strings.Compare(a.Name, b.Name) })
	return c
}

// SearchFoods passes the catalog's ranking through; a concept hit already carries the concept's name.
func (s *foodServer) SearchFoods(ctx context.Context, req *connect.Request[pb.SearchFoodsRequest]) (*connect.Response[pb.SearchFoodsResponse], error) {
	userID, err := callerID(ctx)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	hits, err := catalog.SearchFoods(ctx, s.q, &userID, m.GetQuery(), m.GetSource(), cmp.Or(int(m.GetLimit()), defaultFoodLimit))
	if err != nil {
		return nil, err
	}
	res := &pb.SearchFoodsResponse{}
	for _, h := range hits {
		var concept string
		if h.ConceptID != "" {
			concept = h.Name
		}
		f, err := foodWith(h.Food, concept)
		if err != nil {
			return nil, err
		}
		res.Foods = append(res.Foods, f)
	}
	return connect.NewResponse(res), nil
}

func (s *foodServer) GetFood(ctx context.Context, req *connect.Request[pb.GetFoodRequest]) (*connect.Response[pb.GetFoodResponse], error) {
	userID, id, err := owned(ctx, req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	f, err := catalog.GetFood(ctx, s.q, userID, id)
	if err != nil {
		return nil, err
	}
	out, err := s.foodPB(ctx, f)
	return connect.NewResponse(&pb.GetFoodResponse{Food: out}), err
}

func (s *foodServer) ListConceptMembers(ctx context.Context, req *connect.Request[pb.ListConceptMembersRequest]) (*connect.Response[pb.ListConceptMembersResponse], error) {
	if _, err := callerID(ctx); err != nil {
		return nil, err
	}
	id := req.Msg.GetConceptId()
	c, err := s.q.GetFoodConcept(ctx, id)
	if err != nil {
		return nil, err
	}
	members, err := catalog.ListConceptMembers(ctx, s.q, id)
	if err != nil {
		return nil, err
	}
	res := &pb.ListConceptMembersResponse{}
	for _, m := range members {
		f, err := foodWith(m, c.Name)
		if err != nil {
			return nil, err
		}
		res.Foods = append(res.Foods, f)
	}
	if len(members) > 0 {
		res.PreferredFoodId = members[0].ID.String()
	}
	return connect.NewResponse(res), nil
}

func (s *foodServer) LookupBarcode(ctx context.Context, req *connect.Request[pb.LookupBarcodeRequest]) (*connect.Response[pb.LookupBarcodeResponse], error) {
	userID, err := callerID(ctx)
	if err != nil {
		return nil, err
	}
	f, err := catalog.LookupBarcode(ctx, s.q, &userID, offClient, req.Msg.GetBarcode())
	if err != nil {
		return nil, err
	}
	out, err := s.foodPB(ctx, f)
	return connect.NewResponse(&pb.LookupBarcodeResponse{Food: out}), err
}

func (s *foodServer) CreateCustomFood(ctx context.Context, req *connect.Request[pb.CreateCustomFoodRequest]) (*connect.Response[pb.CreateCustomFoodResponse], error) {
	userID, err := writer(ctx)
	if err != nil {
		return nil, err
	}
	f, err := catalog.CreateCustomFood(ctx, s.q, userID, customFoodOf(req.Msg.GetFood()))
	if err != nil {
		return nil, err
	}
	out, err := foodWith(f, "")
	return connect.NewResponse(&pb.CreateCustomFoodResponse{Food: out}), err
}

func (s *foodServer) UpdateCustomFood(ctx context.Context, req *connect.Request[pb.UpdateCustomFoodRequest]) (*connect.Response[pb.UpdateCustomFoodResponse], error) {
	m := req.Msg.GetFood()
	userID, id, err := ownedWrite(ctx, m.GetId())
	if err != nil {
		return nil, err
	}
	f, err := catalog.UpdateCustomFood(ctx, s.q, userID, id, customFoodOf(m))
	if err != nil {
		return nil, err
	}
	out, err := foodWith(f, "")
	return connect.NewResponse(&pb.UpdateCustomFoodResponse{Food: out}), err
}

func (s *foodServer) DeleteCustomFood(ctx context.Context, req *connect.Request[pb.DeleteCustomFoodRequest]) (*connect.Response[pb.DeleteCustomFoodResponse], error) {
	userID, id, err := ownedWrite(ctx, req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.DeleteCustomFoodResponse{}), catalog.DeleteCustomFood(ctx, s.q, userID, id)
}
