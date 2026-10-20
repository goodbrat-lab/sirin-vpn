package menu

// Tariff представляет тарифный план подписки.
type Tariff struct {
	ID       string
	Name     string
	Days     int
	PriceRub int
	Discount string
}

// Tariffs содержит доступные тарифные планы.
var Tariffs = []Tariff{
	{ID: "1m", Name: "1 месяц", Days: 30, PriceRub: 150, Discount: ""},
	{ID: "3m", Name: "3 месяца", Days: 90, PriceRub: 400, Discount: "−11%"},
	{ID: "6m", Name: "6 месяцев", Days: 180, PriceRub: 750, Discount: "−17%"},
	{ID: "1y", Name: "1 год", Days: 365, PriceRub: 1350, Discount: "−25%"},
}

// FindTariff ищет тариф по идентификатору.
func FindTariff(id string) (Tariff, bool) {
	for _, t := range Tariffs {
		if t.ID == id {
			return t, true
		}
	}
	return Tariff{}, false
}
