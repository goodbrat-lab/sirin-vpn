# Sirin Router v1.1.1 — теперь с веб-интерфейсом LuCI

Привет! Мы зарелизили версию **Sirin Router v1.1.1** для OpenWrt — теперь клиентом можно управлять прямо из браузера.

### Что нового:
- **Веб-интерфейс LuCI**: статус соединения, ввод ключа, выбор MTU, переключение RuDirect, диагностика и логи — в несколько кликов без терминала.
- **Критический фикс**: устранена проблема сохранения маскированного ключа из UI, реальный UUID больше не затирается.
- **Поддержка всех архитектур**: MIPS, MIPSLE, ARMv7, ARM64 и x86_64 на OpenWrt 24.x (`opkg`) и 25.x (`apk`).

### Как установить (по SSH на роутере):
```bash
wget -O - https://raw.githubusercontent.com/goodbrat-lab/sirin-vpn/master/apps/router/install.sh | sh
```

### Как открыть:
Перейдите в браузере: `http://<ip-роутера>/cgi-bin/luci/admin/services/sirin` (меню **Службы** → **Sirin VPN**).

Если что-то пошло не так — создайте Issue по шаблону [router_bug](https://github.com/goodbrat-lab/sirin-vpn/issues/new?template=router_bug.yml) или напишите нам, приложив вывод команды `logread | grep sirin`.

Подробная инструкция: [docs/ROUTER.md](ROUTER.md)
