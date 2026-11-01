package seeder

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	nutritionv1msg "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/nutrition/v1/message"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
)

//go:embed data/food_items_seed.json
var foodItemsSeedJSON []byte

// foodItemSeedRecord defines the JSON schema for a seed food item.
type foodItemSeedRecord struct {
	Name              string   `json:"name"`
	Category          string   `json:"category"`
	CaloriesPer100g   float32  `json:"calories_per_100g"`
	ProteinPer100g    float32  `json:"protein_per_100g"`
	CarbsPer100g      float32  `json:"carbs_per_100g"`
	FatPer100g        float32  `json:"fat_per_100g"`
	AllergenTags      []string `json:"allergen_tags"`
	ProteinSource     string   `json:"protein_source"`
	CarbSource        string   `json:"carb_source"`
	IsNutiFoodProduct bool     `json:"is_nutifood_product"`
	Status            string   `json:"status"`
}

// FoodRegistrar defines the interface required to create and approve food items.
// Implemented by *nutritionGRPC.GRPCHandler.
type FoodRegistrar interface {
	CreateFoodItem(ctx context.Context, req *nutritionv1msg.CreateFoodItemRequest) (*nutritionv1msg.CreateFoodItemResponse, error)
	ApproveFoodItem(ctx context.Context, req *nutritionv1msg.ApproveFoodItemRequest) (*nutritionv1msg.ApproveFoodItemResponse, error)
}

// SeedFoodItems seeds default food catalog and NutiFood partner products via the Nutrition API:
// 1. Checks if items already exist in nutrition.food_items; skips if fully seeded.
// 2. Pre-loads existing food names to ensure idempotent execution.
// 3. Calls foodService.CreateFoodItem followed by foodService.ApproveFoodItem to set status Active.
func SeedFoodItems(ctx context.Context, db *sql.DB, foodService FoodRegistrar) error {
	if foodService == nil {
		return errors.New("foodService cannot be nil")
	}

	var records []foodItemSeedRecord
	if err := json.Unmarshal(foodItemsSeedJSON, &records); err != nil {
		return fmt.Errorf("unmarshal food items seed json: %w", err)
	}

	totalRecords := len(records)
	if totalRecords == 0 {
		log.Println("🥗 No food item seed records found. Skipping.")
		return nil
	}

	existingNames := make(map[string]bool, totalRecords)
	if db != nil {
		var currentCount int
		err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM nutrition.food_items`).Scan(&currentCount)
		if err == nil && currentCount >= totalRecords {
			log.Printf("🥗 Food catalog already fully seeded (%d items present). Skipping food seeder.", currentCount)
			return nil
		}

		rows, err := db.QueryContext(ctx, `SELECT LOWER(name) FROM nutrition.food_items`)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var name string
				if scanErr := rows.Scan(&name); scanErr == nil {
					existingNames[name] = true
				}
			}
		}
	}

	log.Printf("🥗 Starting Food catalog seeding (%d total items, %d already existing)...", totalRecords, len(existingNames))

	adminCtx := context.WithValue(ctx, middleware.UserIDKey, "system-seeder-admin")
	adminCtx = context.WithValue(adminCtx, middleware.UserRoleKey, "Admin")

	createdCount := 0
	for i, record := range records {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		trimmedName := strings.TrimSpace(record.Name)
		if existingNames[strings.ToLower(trimmedName)] {
			continue
		}

		req := &nutritionv1msg.CreateFoodItemRequest{
			Name:              trimmedName,
			Category:          record.Category,
			CaloriesPer_100G:  record.CaloriesPer100g,
			ProteinPer_100G:   record.ProteinPer100g,
			CarbsPer_100G:     record.CarbsPer100g,
			FatPer_100G:       record.FatPer100g,
			AllergenTags:      record.AllergenTags,
			ProteinSource:     record.ProteinSource,
			CarbSource:        record.CarbSource,
			IsNutifoodProduct: record.IsNutiFoodProduct,
		}

		createResp, err := foodService.CreateFoodItem(adminCtx, req)
		if err != nil {
			log.Printf("  ⚠️ Failed to create food item %q: %v", record.Name, err)
			continue
		}

		if createResp.GetFoodItemId() != "" {
			_, err = foodService.ApproveFoodItem(adminCtx, &nutritionv1msg.ApproveFoodItemRequest{
				FoodItemId: createResp.GetFoodItemId(),
			})
			if err != nil {
				log.Printf("  ⚠️ Failed to approve food item %q (ID %s): %v", record.Name, createResp.GetFoodItemId(), err)
			}
		}

		existingNames[strings.ToLower(trimmedName)] = true
		createdCount++

		if createdCount%25 == 0 {
			time.Sleep(5 * time.Millisecond)
		}

		if (i+1)%50 == 0 || i+1 == totalRecords {
			log.Printf("  🥗 Seeded %d/%d food items (newly inserted: %d)...", i+1, totalRecords, createdCount)
		}
	}

	log.Printf("🎉 Completed Food catalog seeding: %d new items created (Total catalog: %d).", createdCount, len(existingNames))
	return nil
}
