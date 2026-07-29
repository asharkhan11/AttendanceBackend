package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/school/attendance-backend/internal/apperror"
	"github.com/school/attendance-backend/internal/auth"
	"github.com/school/attendance-backend/internal/dto"
	"github.com/school/attendance-backend/internal/models"
	"github.com/school/attendance-backend/internal/repository"
)

type AuthService struct {
	schoolRepo *repository.SchoolRepo
	userRepo   *repository.UserRepo
	jwtSvc     *auth.JWTService
	passSvc    *auth.PasswordService
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func NewAuthService(
	schoolRepo *repository.SchoolRepo,
	userRepo *repository.UserRepo,
	jwtSvc *auth.JWTService,
	passSvc *auth.PasswordService,
	accessTTL, refreshTTL time.Duration,
) *AuthService {
	return &AuthService{schoolRepo, userRepo, jwtSvc, passSvc, accessTTL, refreshTTL}
}

func (s *AuthService) Login(ctx context.Context, req *dto.LoginRequest) (*dto.LoginResponse, error) {
	// 1. Resolve tenant
	school, err := s.schoolRepo.GetByCode(ctx, req.SchoolCode)
	if err != nil {
		return nil, apperror.Unauthorized("invalid school code or credentials")
	}
	if !school.IsActive {
		return nil, apperror.Unauthorized("school account is inactive")
	}

	// 2. Look up user within the school (bypasses RLS — pre-tenancy)
	user, hash, err := s.userRepo.GetByEmail(ctx, school.ID, req.Email)
	if err != nil {
		return nil, apperror.Unauthorized("invalid school code or credentials")
	}
	if !user.IsActive {
		return nil, apperror.Unauthorized("account is inactive")
	}

	// 3. Verify password
	ok, err := s.passSvc.VerifyPassword(req.Password, hash)
	if err != nil || !ok {
		return nil, apperror.Unauthorized("invalid school code or credentials")
	}

	// 4. Issue tokens
	accessToken, err := s.jwtSvc.IssueAccessToken(user.ID, school.ID, user.Role)
	if err != nil {
		return nil, apperror.Internal(fmt.Errorf("issue access token: %w", err))
	}
	rawRefresh, hashedRefresh, err := auth.GenerateRefreshToken()
	if err != nil {
		return nil, apperror.Internal(fmt.Errorf("generate refresh token: %w", err))
	}
	expiresAt := time.Now().Add(s.refreshTTL)

	// Store hashed refresh token — use pool directly (no RLS tx needed)
	if err := s.userRepo.StoreRefreshToken(ctx, user.ID, hashedRefresh, expiresAt); err != nil {
		return nil, apperror.Internal(fmt.Errorf("store refresh token: %w", err))
	}

	return &dto.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: rawRefresh,
		ExpiresIn:    int(s.accessTTL.Seconds()),
		User:         toUserResp(user),
	}, nil
}

func (s *AuthService) Refresh(ctx context.Context, rawToken string) (*dto.LoginResponse, error) {
	hashed := auth.HashRefreshToken(rawToken)
	userID, err := s.userRepo.GetRefreshToken(ctx, hashed)
	if err != nil {
		return nil, apperror.Unauthorized("invalid or expired refresh token")
	}

	// Revoke old token (rotation)
	if err := s.userRepo.RevokeRefreshToken(ctx, hashed); err != nil {
		return nil, apperror.Internal(err)
	}

	// Get user (need school_id for JWT — use pool directly)
	// We need to look up the user by ID without RLS
	user, schoolID, role, err := s.userRepo.GetByIDNoRLS(ctx, userID)
	if err != nil {
		return nil, apperror.Unauthorized("user not found")
	}

	accessToken, err := s.jwtSvc.IssueAccessToken(user.ID, schoolID, role)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	rawRefresh, hashedRefresh, err := auth.GenerateRefreshToken()
	if err != nil {
		return nil, apperror.Internal(err)
	}
	expiresAt := time.Now().Add(s.refreshTTL)
	if err := s.userRepo.StoreRefreshToken(ctx, user.ID, hashedRefresh, expiresAt); err != nil {
		return nil, apperror.Internal(err)
	}

	return &dto.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: rawRefresh,
		ExpiresIn:    int(s.accessTTL.Seconds()),
		User:         toUserResp(user),
	}, nil
}

func (s *AuthService) Logout(ctx context.Context, rawToken string) error {
	hashed := auth.HashRefreshToken(rawToken)
	return s.userRepo.RevokeRefreshToken(ctx, hashed)
}

func (s *AuthService) GetMe(ctx context.Context, userID uuid.UUID) (*models.User, error) {
	return s.userRepo.GetByID(ctx, userID)
}

func (s *AuthService) UpdateProfile(ctx context.Context, userID uuid.UUID, req *dto.UpdateProfileRequest) (*models.User, error) {
	updated, err := s.userRepo.Update(ctx, userID, req.FullName, req.Phone)
	if err != nil {
		return nil, apperror.Internal(fmt.Errorf("update profile: %w", err))
	}
	return updated, nil
}

func (s *AuthService) ChangePassword(ctx context.Context, userID uuid.UUID, req *dto.ChangePasswordRequest) error {
	currentHash, err := s.userRepo.GetPasswordHashByID(ctx, userID)
	if err != nil {
		return apperror.NotFound("user")
	}

	match, err := s.passSvc.VerifyPassword(req.CurrentPassword, currentHash)
	if err != nil || !match {
		return apperror.Unauthorized("invalid current password")
	}

	newHash, err := s.passSvc.HashPassword(req.NewPassword)
	if err != nil {
		return apperror.Internal(fmt.Errorf("hash new password: %w", err))
	}

	if err := s.userRepo.UpdatePasswordHash(ctx, userID, newHash); err != nil {
		return apperror.Internal(fmt.Errorf("update password: %w", err))
	}

	_ = s.userRepo.RevokeAllRefreshTokens(ctx, userID)
	return nil
}

func toUserResp(u *models.User) dto.UserResp {
	return dto.UserResp{
		ID:       u.ID.String(),
		SchoolID: u.SchoolID.String(),
		Role:     string(u.Role),
		FullName: u.FullName,
		Email:    u.Email,
		Phone:    u.Phone,
		IsActive: u.IsActive,
	}
}
