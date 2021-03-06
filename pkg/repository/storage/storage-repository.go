package storagerepository

import (
	"context"
	"database/sql"
	"time"

	v1 "github.com/GameComponent/economy-service/pkg/api/v1"
	"github.com/golang/protobuf/ptypes"
)

// StorageRepository struct
type StorageRepository struct {
	db *sql.DB
}

// NewStorageRepository constructor
func NewStorageRepository(db *sql.DB) *StorageRepository {
	return &StorageRepository{
		db: db,
	}
}

// Create a storage
func (r *StorageRepository) Create(ctx context.Context, playerID string, name string) (*v1.Storage, error) {
	// Add item to the databased return the generated UUID
	lastInsertUUID := ""
	err := r.db.QueryRowContext(
		ctx,
		`INSERT INTO storage(player_id, name) VALUES ($1, $2) RETURNING id`,
		playerID,
		name,
	).Scan(&lastInsertUUID)

	if err != nil {
		return nil, err
	}

	storage := &v1.Storage{
		Id:       lastInsertUUID,
		PlayerId: playerID,
		Name:     name,
	}

	return storage, nil
}

// Get a storage
func (r *StorageRepository) Get(ctx context.Context, storageID string) (*v1.Storage, error) {
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT
      storage.id as storageId,
      storage.name as storageName,
      storage.metadata as storageData,
      storage.player_id as playerId,
      storage_item.id as storageItemId,
      storage_item.metadata as storageItemData,
      item.id as itemId,
      item.name as itemName,
      item.metadata as itemData
    FROM storage 
    LEFT JOIN storage_item on (storage.id = storage_item.storage_id)
    LEFT JOIN item on (storage_item.item_id = item.id)
    WHERE storage.id = $1`,
		storageID,
	)

	if err != nil {
		return nil, err
	}

	storageItems := []*v1.StorageItem{}

	type row struct {
		StorageID           string
		StorageName         string
		StorageData         string
		PlayerID            string
		StorageItemID       sql.NullString
		StorageItemItemData sql.NullString
		ItemID              sql.NullString
		ItemName            sql.NullString
		ItemData            sql.NullString
	}

	var res row
	for rows.Next() {
		err = rows.Scan(
			&res.StorageID,
			&res.StorageName,
			&res.StorageData,
			&res.PlayerID,
			&res.StorageItemID,
			&res.StorageItemItemData,
			&res.ItemID,
			&res.ItemName,
			&res.ItemData,
		)

		if err != nil {
			return nil, err
		}

		// Extract the Item
		item := &v1.Item{}
		if res.ItemID.Valid && res.ItemName.Valid {
			item.Id = res.ItemID.String
			item.Name = res.ItemName.String
		}

		// Extract the StorageItem
		storageItem := &v1.StorageItem{}
		if res.StorageItemID.Valid {
			storageItem.Id = res.StorageItemID.String
			storageItem.Item = item
		}

		// Add object to the storageItem if it is set
		if storageItem.Id != "" {
			storageItems = append(storageItems, storageItem)
		}
	}

	storage := &v1.Storage{
		Id:       res.StorageID,
		PlayerId: res.PlayerID,
		Name:     res.StorageName,
		Items:    storageItems,
	}

	return storage, nil
}

// GiveItem to a storage
func (r *StorageRepository) GiveItem(ctx context.Context, storageID string, itemID string) (*string, error) {
	// Add item to the databased return the generated UUID
	lastInsertUUID := ""
	err := r.db.QueryRowContext(
		ctx,
		`INSERT INTO storage_item(item_id, storage_id) VALUES ($1, $2) RETURNING id`,
		itemID,
		storageID,
	).Scan(&lastInsertUUID)

	if err != nil {
		return nil, err
	}

	return &lastInsertUUID, nil
}

// GiveCurrency to a storage
func (r *StorageRepository) GiveCurrency(ctx context.Context, storageID string, currencyID string, amount int64) (*v1.StorageCurrency, error) {
	// Add item to the databased return the generated UUID
	storageCurrencyUUID := ""
	storageCurrencyAmount := int64(0)

	err := r.db.QueryRowContext(
		ctx,
		`
      INSERT INTO storage_currency(currency_id, storage_id, amount)
      VALUES($1, $2, $3)
      ON CONFLICT(currency_id,storage_id) DO UPDATE
      SET amount = storage_currency.amount + EXCLUDED.amount
      RETURNING id, amount
    `,
		currencyID,
		storageID,
		amount,
	).Scan(&storageCurrencyUUID, &storageCurrencyAmount)

	if err != nil {
		return nil, err
	}

	storageCurrency := &v1.StorageCurrency{
		Id:         storageCurrencyUUID,
		CurrencyId: currencyID,
		Amount:     storageCurrencyAmount,
	}

	return storageCurrency, nil
}

// List all storages
func (r *StorageRepository) List(
	ctx context.Context,
	limit int32,
	offset int32,
) (
	[]*v1.Storage,
	int32,
	error,
) {
	// Query items from the database
	rows, err := r.db.QueryContext(
		ctx,
		`
			SELECT 
				id,
				name,
				player_id,
				created_at,
				updated_at,
				(SELECT COUNT(DISTINCT id) FROM storage) AS total_size
			FROM storage
			LIMIT $1
			OFFSET $2
		`,
		limit,
		offset,
	)

	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	// Unwrap rows into items
	storages := []*v1.Storage{}
	totalSize := int32(0)

	for rows.Next() {
		storage := v1.Storage{}
		createdAt := time.Time{}
		updatedAt := time.Time{}

		err := rows.Scan(
			&storage.Id,
			&storage.Name,
			&storage.PlayerId,
			&createdAt,
			&updatedAt,
			&totalSize,
		)
		if err != nil {
			return nil, 0, err
		}

		// Convert created_at to timestamp
		storage.CreatedAt, _ = ptypes.TimestampProto(createdAt)
		storage.UpdatedAt, _ = ptypes.TimestampProto(updatedAt)

		storages = append(storages, &storage)
	}

	return storages, totalSize, nil
}
