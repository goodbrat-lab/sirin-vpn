'require view';
'require ui';
'require rpc';
'require poll';

var callStatus = rpc.declare({
	object: 'sirin',
	method: 'status',
	expect: {}
});

var callGetConfig = rpc.declare({
	object: 'sirin',
	method: 'get_config',
	expect: {}
});

var callSetConfig = rpc.declare({
	object: 'sirin',
	method: 'set_config',
	params: ['gateway', 'password', 'device_id', 'ru_direct', 'tun_mtu'],
	expect: {}
});

var callStart = rpc.declare({
	object: 'sirin',
	method: 'start',
	expect: {}
});

var callStop = rpc.declare({
	object: 'sirin',
	method: 'stop',
	expect: {}
});

var callRestart = rpc.declare({
	object: 'sirin',
	method: 'restart',
	expect: {}
});

var callEnable = rpc.declare({
	object: 'sirin',
	method: 'enable',
	expect: {}
});

var callDisable = rpc.declare({
	object: 'sirin',
	method: 'disable',
	expect: {}
});

var callGetLogs = rpc.declare({
	object: 'sirin',
	method: 'get_logs',
	params: ['lines'],
	expect: { lines: [] }
});

var callTestConnection = rpc.declare({
	object: 'sirin',
	method: 'test_connection',
	expect: {}
});

function formatUptime(sec) {
	if (!sec || sec <= 0) return '—';
	var days = Math.floor(sec / 86400);
	var hours = Math.floor((sec % 86400) / 3600);
	var minutes = Math.floor((sec % 3600) / 60);
	var seconds = sec % 60;
	var parts = [];
	if (days > 0) parts.push(days + ' д.');
	if (hours > 0 || days > 0) parts.push(hours + ' ч.');
	if (minutes > 0 || hours > 0 || days > 0) parts.push(minutes + ' мин.');
	parts.push(seconds + ' сек.');
	return parts.join(' ');
}

return view.extend({
	load: function() {
		return Promise.all([
			callStatus().catch(function() { return {}; }),
			callGetConfig().catch(function() { return {}; })
		]);
	},

	render: function(data) {
		var status = data[0] || {};
		var config = data[1] || {};

		var statusNode = E('span', { class: 'label' }, 'Опрос...');
		var infoTable = E('table', { class: 'table' }, [
			E('tr', { class: 'tr' }, [
				E('td', { class: 'td', style: 'width:30%' }, 'Статус службы:'),
				E('td', { class: 'td' }, statusNode)
			]),
			E('tr', { class: 'tr' }, [
				E('td', { class: 'td' }, 'Версия клиента:'),
				E('td', { class: 'td', id: 'sirin-version' }, status.version || '1.1.0')
			]),
			E('tr', { class: 'tr' }, [
				E('td', { class: 'td' }, 'Активный транспорт:'),
				E('td', { class: 'td', id: 'sirin-transport' }, status.transport ? status.transport.toUpperCase() : '—')
			]),
			E('tr', { class: 'tr' }, [
				E('td', { class: 'td' }, 'Время непрерывной работы:'),
				E('td', { class: 'td', id: 'sirin-uptime' }, formatUptime(status.uptime_sec))
			]),
			E('tr', { class: 'tr' }, [
				E('td', { class: 'td' }, 'Автозапуск при старте:'),
				E('td', { class: 'td', id: 'sirin-enabled' }, status.enabled ? 'Включен' : 'Отключен')
			])
		]);

		var btnStart = E('button', {
			class: 'btn cbi-button cbi-button-action',
			style: 'margin-right: 8px;',
			click: function() {
				btnStart.disabled = true;
				return callStart().then(function(res) {
					if (res && res.ok) {
						ui.addNotification(null, E('p', {}, 'Служба Sirin запущена'), 'info');
					} else {
						ui.addNotification(null, E('p', {}, (res && res.error) || 'Ошибка запуска'), 'error');
					}
					updateStatusView();
				}).catch(function(e) {
					ui.addNotification(null, E('p', {}, e.message || 'Ошибка RPC'), 'error');
					updateStatusView();
				});
			}
		}, 'Запустить');

		var btnStop = E('button', {
			class: 'btn cbi-button cbi-button-reset',
			style: 'margin-right: 8px;',
			click: function() {
				btnStop.disabled = true;
				return callStop().then(function(res) {
					if (res && res.ok) {
						ui.addNotification(null, E('p', {}, 'Служба Sirin остановлена'), 'info');
					} else {
						ui.addNotification(null, E('p', {}, (res && res.error) || 'Ошибка остановки'), 'error');
					}
					updateStatusView();
				}).catch(function(e) {
					ui.addNotification(null, E('p', {}, e.message || 'Ошибка RPC'), 'error');
					updateStatusView();
				});
			}
		}, 'Остановить');

		var btnRestart = E('button', {
			class: 'btn cbi-button cbi-button-apply',
			style: 'margin-right: 8px;',
			click: function() {
				btnRestart.disabled = true;
				return callRestart().then(function(res) {
					if (res && res.ok) {
						ui.addNotification(null, E('p', {}, 'Служба Sirin перезапущена'), 'info');
					} else {
						ui.addNotification(null, E('p', {}, (res && res.error) || 'Ошибка перезапуска'), 'error');
					}
					updateStatusView();
				}).catch(function(e) {
					ui.addNotification(null, E('p', {}, e.message || 'Ошибка RPC'), 'error');
					updateStatusView();
				});
			}
		}, 'Перезапустить');

		var btnAutostart = E('button', {
			class: 'btn cbi-button',
			click: function() {
				btnAutostart.disabled = true;
				var act = currentStatus.enabled ? callDisable : callEnable;
				var text = currentStatus.enabled ? 'отключен' : 'включен';
				return act().then(function(res) {
					if (res && res.ok) {
						ui.addNotification(null, E('p', {}, 'Автозапуск ' + text), 'info');
					} else {
						ui.addNotification(null, E('p', {}, (res && res.error) || 'Ошибка изменения автозапуска'), 'error');
					}
					updateStatusView();
				}).catch(function(e) {
					ui.addNotification(null, E('p', {}, e.message || 'Ошибка RPC'), 'error');
					updateStatusView();
				});
			}
		}, 'Автозапуск');

		var currentStatus = status;
		function applyStatus(st) {
			currentStatus = st || {};
			var isRunning = !!currentStatus.running;
			statusNode.className = isRunning ? 'label success' : 'label warning';
			statusNode.textContent = isRunning ? ('Работает (PID: ' + (currentStatus.pid || '—') + ')') : 'Остановлен';

			var elVer = document.getElementById('sirin-version');
			if (elVer) elVer.textContent = currentStatus.version || '1.1.0';

			var elTrans = document.getElementById('sirin-transport');
			if (elTrans) elTrans.textContent = isRunning ? (currentStatus.transport ? currentStatus.transport.toUpperCase() : 'Определение...') : '—';

			var elUptime = document.getElementById('sirin-uptime');
			if (elUptime) elUptime.textContent = isRunning ? formatUptime(currentStatus.uptime_sec) : '—';

			var elEnabled = document.getElementById('sirin-enabled');
			if (elEnabled) elEnabled.textContent = currentStatus.enabled ? 'Включен' : 'Отключен';

			btnStart.disabled = isRunning;
			btnStop.disabled = !isRunning;
			btnRestart.disabled = false;
			btnAutostart.disabled = false;
			btnAutostart.textContent = currentStatus.enabled ? 'Отключить автозапуск' : 'Включить автозапуск';
		}
		applyStatus(status);

		function updateStatusView() {
			return callStatus().then(function(st) {
				applyStatus(st);
			}).catch(function() {});
		}

		poll.add(function() {
			return updateStatusView();
		}, 5);

		var inGateway = E('input', {
			class: 'cbi-input-text',
			type: 'text',
			placeholder: 'crp.guseynovia.ru',
			value: config.gateway || ''
		});

		var inPassword = E('input', {
			class: 'cbi-input-text',
			type: 'password',
			placeholder: 'UUID из @sirin_crp_bot',
			value: config.password || ''
		});

		var btnTogglePass = E('button', {
			class: 'btn cbi-button',
			type: 'button',
			style: 'margin-left: 8px;',
			click: function() {
				if (inPassword.type === 'password') {
					inPassword.type = 'text';
					btnTogglePass.textContent = 'Скрыть';
				} else {
					inPassword.type = 'password';
					btnTogglePass.textContent = 'Показать';
				}
			}
		}, 'Показать');

		var inDeviceId = E('input', {
			class: 'cbi-input-text',
			type: 'text',
			placeholder: 'router-01',
			value: config.device_id || 'router-01'
		});

		var inTunMtu = E('input', {
			class: 'cbi-input-text',
			type: 'number',
			min: '576',
			max: '1500',
			value: config.tun_mtu || 1280
		});

		var inRuDirect = E('input', {
			type: 'checkbox',
			checked: (config.ru_direct !== undefined ? !!config.ru_direct : true)
		});

		var btnSaveConfig = E('button', {
			class: 'btn cbi-button cbi-button-save',
			style: 'margin-right: 8px;',
			click: function() {
				var gw = inGateway.value.trim();
				var pw = inPassword.value.trim();
				var did = inDeviceId.value.trim();
				var mtu = parseInt(inTunMtu.value, 10);
				var rud = inRuDirect.checked;

				if (!gw || /[^a-zA-Z0-9.-]/.test(gw)) {
					ui.addNotification(null, E('p', {}, 'Ошибка в поле «Шлюз»: укажите валидное доменное имя или IP-адрес'), 'error');
					return;
				}
				if (!pw) {
					ui.addNotification(null, E('p', {}, 'Ошибка в поле «Ключ доступа»: поле не может быть пустым'), 'error');
					return;
				}
				if (!did || /[^a-zA-Z0-9._-]/.test(did) || did.length > 64) {
					ui.addNotification(null, E('p', {}, 'Ошибка в поле «ID устройства»: допустимы символы a-z, 0-9, ., _, -, до 64 символов'), 'error');
					return;
				}
				if (isNaN(mtu) || mtu < 576 || mtu > 1500) {
					ui.addNotification(null, E('p', {}, 'Ошибка в поле «MTU»: значение должно быть от 576 до 1500'), 'error');
					return;
				}

				btnSaveConfig.disabled = true;
				return callSetConfig(gw, pw, did, rud, mtu).then(function(res) {
					btnSaveConfig.disabled = false;
					if (res && res.ok) {
						ui.addNotification(null, E('p', {}, 'Настройки успешно сохранены в /etc/sirin/client.toml'), 'info');
						ui.showModal('Перезапустить службу Sirin?', [
							E('p', {}, 'Настройки сохранены. Желаете перезапустить туннель прямо сейчас, чтобы применить новые параметры?'),
							E('div', { class: 'right' }, [
								E('button', {
									class: 'btn cbi-button cbi-button-neutral',
									style: 'margin-right: 8px;',
									click: function() {
										ui.hideModal();
									}
								}, 'Нет, позже'),
								E('button', {
									class: 'btn cbi-button cbi-button-apply',
									click: function() {
										ui.hideModal();
										callRestart().then(function() {
											ui.addNotification(null, E('p', {}, 'Служба Sirin перезапущена'), 'info');
											updateStatusView();
										});
									}
								}, 'Да, перезапустить')
							])
						]);
					} else {
						ui.addNotification(null, E('p', {}, (res && res.error) || 'Ошибка сохранения настроек'), 'error');
					}
				}).catch(function(e) {
					btnSaveConfig.disabled = false;
					ui.addNotification(null, E('p', {}, e.message || 'Ошибка RPC сохранения'), 'error');
				});
			}
		}, 'Сохранить');

		var btnCancelConfig = E('button', {
			class: 'btn cbi-button cbi-button-reset',
			click: function() {
				btnCancelConfig.disabled = true;
				return callGetConfig().then(function(cfg) {
					btnCancelConfig.disabled = false;
					if (cfg) {
						inGateway.value = cfg.gateway || '';
						inPassword.value = cfg.password || '';
						inDeviceId.value = cfg.device_id || 'router-01';
						inTunMtu.value = cfg.tun_mtu || 1280;
						inRuDirect.checked = (cfg.ru_direct !== undefined ? !!cfg.ru_direct : true);
						ui.addNotification(null, E('p', {}, 'Значения сброшены до текущих настроек файла'), 'info');
					}
				}).catch(function() {
					btnCancelConfig.disabled = false;
				});
			}
		}, 'Отмена');

		var diagResult = E('div', { style: 'margin-top: 12px; display: none;' });

		var btnTestConn = E('button', {
			class: 'btn cbi-button cbi-button-action',
			style: 'margin-right: 8px;',
			click: function() {
				btnTestConn.disabled = true;
				diagResult.style.display = 'block';
				diagResult.innerHTML = '<p><em>Выполняется проверка сетевых маршрутов...</em></p>';

				return callTestConnection().then(function(res) {
					btnTestConn.disabled = false;
					res = res || {};
					var rudClass = res.rudirect_via_tunnel ? 'label warning' : 'label success';
					var rudText = res.rudirect_via_tunnel ? 'ОШИБКА: РФ-трафик направлен в туннель' : 'RuDirect напрямую через WAN (ок)';

					diagResult.innerHTML = '';
					diagResult.appendChild(E('table', { class: 'table' }, [
						E('tr', { class: 'tr' }, [
							E('td', { class: 'td', style: 'width: 35%;' }, 'Внешний IP (через туннель):'),
							E('td', { class: 'td' }, E('strong', {}, res.public_ip || 'нет ответа'))
						]),
						E('tr', { class: 'tr' }, [
							E('td', { class: 'td' }, 'Раздельное туннелирование (RuDirect):'),
							E('td', { class: 'td' }, E('span', { class: rudClass }, rudText))
						])
					]));
				}).catch(function(e) {
					btnTestConn.disabled = false;
					diagResult.innerHTML = '<p class="error">Ошибка проверки: ' + (e.message || 'RPC timeout') + '</p>';
				});
			}
		}, 'Проверить подключение');

		var btnShowLogs = E('button', {
			class: 'btn cbi-button cbi-button-neutral',
			click: function() {
				function fetchAndShowLogs() {
					return callGetLogs(200).then(function(res) {
						var lines = (res && res.lines) ? res.lines.join('\n') : 'Журнал пуст';
						var logPre = E('pre', {
							style: 'max-height: 450px; overflow-y: auto; background: #1e1e1e; color: #dcdcdc; padding: 10px; font-family: monospace; font-size: 12px; border-radius: 4px; white-space: pre-wrap; word-break: break-all;'
						}, lines);

						ui.showModal('Журнал работы Sirin VPN', [
							logPre,
							E('div', { class: 'right', style: 'margin-top: 10px;' }, [
								E('button', {
									class: 'btn cbi-button cbi-button-action',
									style: 'margin-right: 8px;',
									click: function() {
										fetchAndShowLogs();
									}
								}, 'Обновить'),
								E('button', {
									class: 'btn cbi-button cbi-button-neutral',
									click: function() {
										ui.hideModal();
									}
								}, 'Закрыть')
							])
						]);
					}).catch(function(e) {
						ui.addNotification(null, E('p', {}, 'Ошибка загрузки логов: ' + (e.message || 'RPC failure')), 'error');
					});
				}
				fetchAndShowLogs();
			}
		}, 'Показать логи');

		return E('div', { class: 'view' }, [
			E('h2', {}, 'Sirin VPN — Управление Router-клиентом'),
			E('p', { class: 'cbi-section-descr' }, 'Управление защищённым VPN-туннелем для домашней сети на базе протоколов UDP Noise NK и VLESS Reality.'),

			E('div', { class: 'cbi-section' }, [
				E('h3', {}, '1. Состояние службы'),
				infoTable,
				E('div', { style: 'margin-top: 12px;' }, [
					btnStart,
					btnStop,
					btnRestart,
					btnAutostart
				])
			]),

			E('div', { class: 'cbi-section' }, [
				E('h3', {}, '2. Настройки подключения'),
				E('div', { class: 'cbi-value' }, [
					E('label', { class: 'cbi-value-title' }, 'Шлюз (Gateway)'),
					E('div', { class: 'cbi-value-field' }, [
						inGateway,
						E('div', { class: 'cbi-value-description' }, 'Домен или IP-адрес шлюза VPN (по умолчанию crp.guseynovia.ru)')
					])
				]),
				E('div', { class: 'cbi-value' }, [
					E('label', { class: 'cbi-value-title' }, 'Ключ доступа (UUID)'),
					E('div', { class: 'cbi-value-field' }, [
						E('div', { style: 'display: flex; align-items: center;' }, [
							inPassword,
							btnTogglePass
						]),
						E('div', { class: 'cbi-value-description' }, 'Персональный UUID-ключ клиента, выданный ботом @sirin_crp_bot')
					])
				]),
				E('div', { class: 'cbi-value' }, [
					E('label', { class: 'cbi-value-title' }, 'ID устройства'),
					E('div', { class: 'cbi-value-field' }, [
						inDeviceId,
						E('div', { class: 'cbi-value-description' }, 'Идентификатор устройства в сессии (до 64 символов: a-z, 0-9, ., _, -)')
					])
				]),
				E('div', { class: 'cbi-value' }, [
					E('label', { class: 'cbi-value-title' }, 'TUN MTU'),
					E('div', { class: 'cbi-value-field' }, [
						inTunMtu,
						E('div', { class: 'cbi-value-description' }, 'Размер MTU виртуального сетевого интерфейса sirin0 (от 576 до 1500, по умолчанию 1280)')
					])
				]),
				E('div', { class: 'cbi-value' }, [
					E('label', { class: 'cbi-value-title' }, 'Прямой доступ в РФ (RuDirect)'),
					E('div', { class: 'cbi-value-field' }, [
						inRuDirect,
						E('span', { style: 'margin-left: 8px;' }, 'Направлять трафик к 4945 диапазонам IP РФ напрямую через WAN в обход VPN')
					])
				]),
				E('div', { style: 'margin-top: 16px;' }, [
					btnSaveConfig,
					btnCancelConfig
				])
			]),

			E('div', { class: 'cbi-section' }, [
				E('h3', {}, '3. Диагностика и мониторинг'),
				E('div', {}, [
					btnTestConn,
					btnShowLogs
				]),
				diagResult
			])
		]);
	}
});
