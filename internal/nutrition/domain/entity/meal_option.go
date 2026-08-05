package entity

type IngredientGram struct {
	ingredientName    string
	amountGram        float64
	isNutiFoodProduct bool
}

func NewIngredientGram(ingredientName string, amountGram float64, isNutiFoodProduct bool) IngredientGram {
	return IngredientGram{
		ingredientName:    ingredientName,
		amountGram:        amountGram,
		isNutiFoodProduct: isNutiFoodProduct,
	}
}

func (i IngredientGram) IngredientName() string  { return i.ingredientName }
func (i IngredientGram) AmountGram() float64     { return i.amountGram }
func (i IngredientGram) IsNutiFoodProduct() bool { return i.isNutiFoodProduct }

type MealOption struct {
	id                       string
	recipeName               string
	cookingStyle             string
	ingredients              []IngredientGram
	cookingSteps             []string
	supplementaryIngredients []IngredientGram
	ingredientHash           string
}

func NewMealOption(
	id, recipeName, cookingStyle string,
	ingredients []IngredientGram,
	cookingSteps []string,
	supplementaryIngredients []IngredientGram,
	ingredientHash string,
) MealOption {
	ingCopy := make([]IngredientGram, len(ingredients))
	copy(ingCopy, ingredients)

	stepsCopy := make([]string, len(cookingSteps))
	copy(stepsCopy, cookingSteps)

	suppCopy := make([]IngredientGram, len(supplementaryIngredients))
	copy(suppCopy, supplementaryIngredients)

	return MealOption{
		id:                       id,
		recipeName:               recipeName,
		cookingStyle:             cookingStyle,
		ingredients:              ingCopy,
		cookingSteps:             stepsCopy,
		supplementaryIngredients: suppCopy,
		ingredientHash:           ingredientHash,
	}
}

func (o MealOption) ID() string                                 { return o.id }
func (o MealOption) RecipeName() string                         { return o.recipeName }
func (o MealOption) CookingStyle() string                       { return o.cookingStyle }
func (o MealOption) IngredientHash() string                     { return o.ingredientHash }
func (o MealOption) CookingSteps() []string                     { return o.cookingSteps }
func (o MealOption) Ingredients() []IngredientGram              { return o.ingredients }
func (o MealOption) SupplementaryIngredients() []IngredientGram { return o.supplementaryIngredients }
