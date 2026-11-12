package api

import (
	"context"

	"connectrpc.com/connect"

	pb "github.com/gdsoumya/plusultra/gen/go/plusultra/v1"
	"github.com/gdsoumya/plusultra/internal/llmdocs"
)

type referenceServer struct{ docs llmdocs.Docs }

func (s referenceServer) GetProgramLanguageReference(context.Context, *connect.Request[pb.GetProgramLanguageReferenceRequest]) (*connect.Response[pb.GetProgramLanguageReferenceResponse], error) {
	return connect.NewResponse(&pb.GetProgramLanguageReferenceResponse{Markdown: s.docs.ProgramLanguage}), nil
}

func (s referenceServer) GetMealLanguageReference(context.Context, *connect.Request[pb.GetMealLanguageReferenceRequest]) (*connect.Response[pb.GetMealLanguageReferenceResponse], error) {
	return connect.NewResponse(&pb.GetMealLanguageReferenceResponse{Markdown: s.docs.MealLanguage}), nil
}

func (s referenceServer) GetProgramExamples(context.Context, *connect.Request[pb.GetProgramExamplesRequest]) (*connect.Response[pb.GetProgramExamplesResponse], error) {
	return connect.NewResponse(&pb.GetProgramExamplesResponse{Markdown: s.docs.ProgramExamples}), nil
}

func (s referenceServer) GetLogFormatReference(context.Context, *connect.Request[pb.GetLogFormatReferenceRequest]) (*connect.Response[pb.GetLogFormatReferenceResponse], error) {
	return connect.NewResponse(&pb.GetLogFormatReferenceResponse{Markdown: s.docs.LogFormat}), nil
}
