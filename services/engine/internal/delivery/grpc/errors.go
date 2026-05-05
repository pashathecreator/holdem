package grpc

import (
	"errors"

	"github.com/pashathecreator/holdem/services/engine/internal/domain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func domainErrorToGRPC(err error) error {
	switch {
	case errors.Is(err, domain.ErrGameNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrPlayerNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrNotPlayerTurn):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, domain.ErrPlayerNotActive):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, domain.ErrInvalidAction):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrInvalidRaiseAmount):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrInsufficientStack):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, domain.ErrNotEnoughPlayers):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, domain.ErrHandAlreadyStarted):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, domain.ErrHandNotStarted):
		return status.Error(codes.FailedPrecondition, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}
