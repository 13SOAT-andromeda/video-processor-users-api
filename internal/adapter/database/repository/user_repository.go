package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/13SOAT-andromeda/video-processor-users-api/internal/adapter/database/model/user"
	"github.com/13SOAT-andromeda/video-processor-users-api/internal/application/ports"
	"github.com/13SOAT-andromeda/video-processor-users-api/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type userRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) ports.UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Create(ctx context.Context, u *domain.User) error {
	m := &user.Model{}
	m.FromDomain(u)
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	if err := r.db.WithContext(ctx).Create(m).Error; err != nil {
		if isUniqueViolation(err) {
			return domain.ErrEmailAlreadyExists
		}
		return err
	}
	u.ID = m.ID
	u.CreatedAt = m.CreatedAt
	u.UpdatedAt = m.UpdatedAt
	return nil
}

func (r *userRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	var m user.Model
	if err := r.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrUserNotFound
		}
		return nil, err
	}
	return m.ToDomain(), nil
}

func (r *userRepository) Update(ctx context.Context, u *domain.User) error {
	m := &user.Model{}
	m.FromDomain(u)
	if err := r.db.WithContext(ctx).Model(m).Where("id = ?", u.ID).Updates(m).Error; err != nil {
		if isUniqueViolation(err) {
			return domain.ErrEmailAlreadyExists
		}
		return err
	}
	return nil
}

func (r *userRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&user.Model{}).Error
}

func (r *userRepository) List(ctx context.Context, limit, offset int) ([]*domain.User, error) {
	var models []user.Model
	if err := r.db.WithContext(ctx).Limit(limit).Offset(offset).Find(&models).Error; err != nil {
		return nil, err
	}
	users := make([]*domain.User, len(models))
	for i, m := range models {
		mc := m
		users[i] = mc.ToDomain()
	}
	return users, nil
}

func isUniqueViolation(err error) bool {
	return strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate")
}
