package menu

import "sirin/server/internal/bot"

// Tariff представляет тарифный план подписки.
type Tariff = bot.Tariff

// Tariffs содержит доступные тарифные планы.
var Tariffs = bot.Tariffs

// FindTariff ищет тариф по идентификатору.
var FindTariff = bot.FindTariff
