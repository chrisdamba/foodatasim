package repositories

import (
	"context"
	"github.com/chrisdamba/foodatasim/internal/models"
)

type UserRepository interface {
	BulkCreate(ctx context.Context, users []*models.User) error
	Create(ctx context.Context, user *models.User) error
	GetAll(ctx context.Context) ([]*models.User, error)
	Count(ctx context.Context) (int, error)
	DeleteAll(ctx context.Context) error
}

type RestaurantRepository interface {
	BulkCreate(ctx context.Context, users []*models.Restaurant) error
	Create(ctx context.Context, restaurant *models.Restaurant) error
	GetAll(ctx context.Context) (map[string]*models.Restaurant, error)
	Count(ctx context.Context) (int, error)
	DeleteAll(ctx context.Context) error
}

type MenuItemRepository interface {
	BulkCreate(ctx context.Context, users []*models.MenuItem) error
	Create(ctx context.Context, menuItem *models.MenuItem) error
	GetAll(ctx context.Context) (map[string]*models.MenuItem, error)
	GetByRestaurantID(ctx context.Context, restaurantID string) ([]*models.MenuItem, error)
	Count(ctx context.Context) (int, error)
	DeleteAll(ctx context.Context) error
}

type DeliveryPartnerRepository interface {
	BulkCreate(ctx context.Context, users []*models.DeliveryPartner) error
	Create(ctx context.Context, partner *models.DeliveryPartner) error
	GetAll(ctx context.Context) ([]*models.DeliveryPartner, error)
	Count(ctx context.Context) (int, error)
	DeleteAll(ctx context.Context) error
}
