package bot

// Tariff представляет тарифный план подписки.
type Tariff struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Days     int    `json:"days"`
	PriceRub int    `json:"price_rub"`
	Stars    int    `json:"stars"`
	Discount string `json:"discount,omitempty"`
}

// Tariffs содержит доступные тарифные планы Sirin VPN.
var Tariffs = []Tariff{
	{ID: "1m", Name: "1 месяц", Days: 30, PriceRub: 150, Stars: 75, Discount: ""},
	{ID: "3m", Name: "3 месяца", Days: 90, PriceRub: 400, Stars: 200, Discount: "−11%"},
	{ID: "6m", Name: "6 месяцев", Days: 180, PriceRub: 750, Stars: 375, Discount: "−17%"},
	{ID: "1y", Name: "1 год", Days: 365, PriceRub: 1350, Stars: 700, Discount: "−25%"},
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
