package service

import (
	"errors"

	"github.com/chatapp/pkg/jwt"
)

type AuthService interface {
	ValidateToken(tokenString string) (int, error)
}

type authService struct {
	jwtService jwt.Service
}

func NewAuthService(jwtService jwt.Service) AuthService {
	return &authService{jwtService: jwtService}
}

func (s *authService) ValidateToken(tokenString string) (int, error) {
	if tokenString == "" {
		return 0, errors.New("empty token")
	}

	claims, err := s.jwtService.ValidateToken(tokenString)
	if err != nil {
		return 0, err
	}

	userID, ok := claims["id"].(float64)
	if !ok {
		return 0, errors.New("invalid user ID in token")
	}

	return int(userID), nil
}
