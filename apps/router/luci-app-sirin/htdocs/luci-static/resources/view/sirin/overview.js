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
	params: ['gateway', 'password', 'device_id', 'ru_direct', 'tun_mtu', 'kill_switch'],
	expect: {}
});

var callGetSplitConfig = rpc.declare({
	object: 'sirin',
	method: 'get_split_config',
	expect: {}
});

var callSetSplitConfig = rpc.declare({
	object: 'sirin',
	method: 'set_split_config',
	params: ['mode', 'use_ru_direct', 'cidrs', 'domains'],
	expect: {}
});

var callGetRuDirectInfo = rpc.declare({
	object: 'sirin',
	method: 'get_ru_direct_info',
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
			callGetConfig().catch(function() { return {}; }),
			callGetSplitConfig().catch(function() { return {}; }),
			callGetRuDirectInfo().catch(function() { return {}; })
		]);
	},

	render: function(data) {
		var status = data[0] || {};
		var config = data[1] || {};
		var splitConfig = data[2] || {};
		var ruInfo = data[3] || {};

		var ruCount = ruInfo.count || 8641;

		var statusNode = E('span', { class: 'label' }, 'Опрос...');
		var infoTable = E('table', { class: 'table' }, [
			E('tr', { class: 'tr' }, [
				E('td', { class: 'td', style: 'width:30%' }, 'Статус службы:'),
				E('td', { class: 'td' }, statusNode)
			]),
			E('tr', { class: 'tr' }, [
				E('td', { class: 'td' }, 'Версия клиента:'),
				E('td', { class: 'td', id: 'sirin-version' }, status.version || '1.2.0')
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
			if (elVer) elVer.textContent = currentStatus.version || '1.2.0';

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

		// Section 2: Connection settings
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

		var passWarnNode = E('div', {
			class: 'cbi-value-description',
			style: 'color: #d99b26; display: none; margin-top: 4px;'
		}, 'Предупреждение: ключ не похож на UUID v4 (формат xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx)');

		function checkPasswordWarn() {
			var val = inPassword.value.trim();
			var uuidRegex = /^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-4[0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$/;
			if (val && val.indexOf('…') === -1 && val.indexOf('...') === -1 && !uuidRegex.test(val)) {
				passWarnNode.style.display = 'block';
			} else {
				passWarnNode.style.display = 'none';
			}
		}

		inPassword.addEventListener('input', checkPasswordWarn);
		inPassword.addEventListener('change', checkPasswordWarn);

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

		var chkKillSwitch = E('input', {
			class: 'cbi-input-checkbox',
			type: 'checkbox',
			checked: (config.kill_switch !== undefined ? !!config.kill_switch : true)
		});

		var btnSaveConfig = E('button', {
			class: 'btn cbi-button cbi-button-save',
			style: 'margin-right: 8px;',
			click: function() {
				var gw = inGateway.value.trim();
				var pw = inPassword.value.trim();
				var did = inDeviceId.value.trim();
				var mtu = parseInt(inTunMtu.value, 10);
				var rud = (splitConfig.use_ru_direct !== undefined ? !!splitConfig.use_ru_direct : true);
				var ks = chkKillSwitch.checked;

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
				return callSetConfig(gw, pw, did, rud, mtu, ks).then(function(res) {
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
						chkKillSwitch.checked = (cfg.kill_switch !== undefined ? !!cfg.kill_switch : true);
						ui.addNotification(null, E('p', {}, 'Значения сброшены до текущих настроек файла'), 'info');
					}
				}).catch(function() {
					btnCancelConfig.disabled = false;
				});
			}
		}, 'Отмена');

		// Section 3: Split Tunneling
		var splitMode = (splitConfig.mode || 'bypass').toLowerCase();
		var radBypass = E('input', {
			type: 'radio',
			name: 'split_mode',
			value: 'bypass',
			checked: splitMode !== 'only'
		});
		var radOnly = E('input', {
			type: 'radio',
			name: 'split_mode',
			value: 'only',
			checked: splitMode === 'only'
		});

		var chkRuDirect = E('input', {
			type: 'checkbox',
			checked: (splitConfig.use_ru_direct !== undefined ? !!splitConfig.use_ru_direct : true)
		});

		var ruDirectWarnNode = E('div', {
			class: 'cbi-value-description',
			style: 'color: #d99b26; display: ' + (splitMode === 'only' ? 'block' : 'none') + '; margin-top: 4px;'
		}, 'Внимание: список RuDirect игнорируется в режиме «Only» (через VPN пойдут только адреса из списков ниже).');

		function updateModeUI() {
			if (radOnly.checked) {
				ruDirectWarnNode.style.display = 'block';
			} else {
				ruDirectWarnNode.style.display = 'none';
			}
		}

		radBypass.addEventListener('change', updateModeUI);
		radOnly.addEventListener('change', updateModeUI);

		var txtCIDRs = E('textarea', {
			class: 'cbi-input-textarea',
			rows: 5,
			style: 'width: 100%; font-family: monospace;',
			placeholder: '1.1.1.1/32\n104.16.0.0/12'
		}, (splitConfig.cidrs || []).join('\n'));

		var txtDomains = E('textarea', {
			class: 'cbi-input-textarea',
			rows: 5,
			style: 'width: 100%; font-family: monospace;',
			placeholder: 'rutracker.org\nkinopoisk.ru'
		}, (splitConfig.domains || []).join('\n'));

		var btnSaveSplit = E('button', {
			class: 'btn cbi-button cbi-button-save',
			style: 'margin-right: 8px;',
			click: function() {
				var mode = radOnly.checked ? 'only' : 'bypass';
				var useRu = chkRuDirect.checked;

				// Parse and validate CIDRs
				var rawCidrs = txtCIDRs.value.split('\n');
				var validCidrs = [];
				var cidrRegex = /^[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}(\/[0-9]{1,2})?$/;
				for (var i = 0; i < rawCidrs.length; i++) {
					var c = rawCidrs[i].trim();
					if (!c) continue;
					if (!cidrRegex.test(c)) {
						ui.addNotification(null, E('p', {}, 'Некорректный формат CIDR или IP: ' + c), 'error');
						return;
					}
					validCidrs.push(c);
				}

				// Parse and validate domains
				var rawDomains = txtDomains.value.split('\n');
				var validDomains = [];
				var domainRegex = /^([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$/;
				for (var j = 0; j < rawDomains.length; j++) {
					var d = rawDomains[j].trim().toLowerCase();
					if (!d) continue;
					if (!domainRegex.test(d)) {
						ui.addNotification(null, E('p', {}, 'Некорректный формат доменного имени: ' + d), 'error');
						return;
					}
					validDomains.push(d);
				}

				btnSaveSplit.disabled = true;
				return callSetSplitConfig(mode, useRu, validCidrs, validDomains).then(function(res) {
					btnSaveSplit.disabled = false;
					if (res && res.ok) {
						ui.addNotification(null, E('p', {}, 'Параметры раздельного туннелирования сохранены'), 'info');
						ui.showModal('Перезапустить службу Sirin?', [
							E('p', {}, 'Параметры разделения сохранены. Для применения маршрутизации требуется перезапуск службы.'),
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
						ui.addNotification(null, E('p', {}, (res && res.error) || 'Ошибка сохранения разделения'), 'error');
					}
				}).catch(function(e) {
					btnSaveSplit.disabled = false;
					ui.addNotification(null, E('p', {}, e.message || 'Ошибка RPC сохранения'), 'error');
				});
			}
		}, 'Сохранить раздельное туннелирование');

		var btnCancelSplit = E('button', {
			class: 'btn cbi-button cbi-button-reset',
			click: function() {
				btnCancelSplit.disabled = true;
				return callGetSplitConfig().then(function(sc) {
					btnCancelSplit.disabled = false;
					sc = sc || {};
					if (sc.mode === 'only') {
						radOnly.checked = true;
					} else {
						radBypass.checked = true;
					}
					updateModeUI();
					chkRuDirect.checked = (sc.use_ru_direct !== undefined ? !!sc.use_ru_direct : true);
					txtCIDRs.value = (sc.cidrs || []).join('\n');
					txtDomains.value = (sc.domains || []).join('\n');
					ui.addNotification(null, E('p', {}, 'Настройки разделения сброшены'), 'info');
				}).catch(function() {
					btnCancelSplit.disabled = false;
				});
			}
		}, 'Отмена');

		// Section 4: Diagnostics
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
					var rudText = res.raw || (res.rudirect_via_tunnel ? 'РФ-трафик направлен в туннель' : 'RuDirect напрямую через WAN (ок)');

					diagResult.innerHTML = '';
					diagResult.appendChild(E('table', { class: 'table' }, [
						E('tr', { class: 'tr' }, [
							E('td', { class: 'td', style: 'width: 35%;' }, 'Внешний IP (через туннель):'),
							E('td', { class: 'td' }, E('strong', {}, res.public_ip || 'нет ответа'))
						]),
						E('tr', { class: 'tr' }, [
							E('td', { class: 'td' }, 'Режим разделения трафика:'),
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
						passWarnNode,
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
					E('label', { class: 'cbi-value-title' }, 'Kill Switch'),
					E('div', { class: 'cbi-value-field' }, [
						E('label', { style: 'cursor: pointer;' }, [
							chkKillSwitch,
							E('span', { style: 'margin-left: 6px;' }, E('strong', {}, 'Kill Switch (блокировать при обрыве)'))
						]),
						E('div', { class: 'cbi-value-description' }, 'Блокировать трафик локальной сети при обрыве туннеля для предотвращения утечек в WAN')
					])
				]),
				E('div', { style: 'margin-top: 16px;' }, [
					btnSaveConfig,
					btnCancelConfig
				])
			]),

			E('div', { class: 'cbi-section' }, [
				E('h3', {}, '3. Раздельное туннелирование (Split Tunneling)'),
				E('div', { class: 'cbi-value' }, [
					E('label', { class: 'cbi-value-title' }, 'Режим работы'),
					E('div', { class: 'cbi-value-field' }, [
						E('label', { style: 'margin-right: 16px; cursor: pointer;' }, [
							radBypass,
							E('span', { style: 'margin-left: 6px;' }, E('strong', {}, 'Bypass (Обход)')),
							E('span', { style: 'color: #888; margin-left: 4px;' }, '— весь трафик в VPN, кроме списка')
						]),
						E('br'),
						E('label', { style: 'cursor: pointer; margin-top: 6px; display: inline-block;' }, [
							radOnly,
							E('span', { style: 'margin-left: 6px;' }, E('strong', {}, 'Only (Только)')),
							E('span', { style: 'color: #888; margin-left: 4px;' }, '— только адреса из списка через VPN, остальное напрямую')
						])
					])
				]),
				E('div', { class: 'cbi-value' }, [
					E('label', { class: 'cbi-value-title' }, 'Прямой доступ в РФ (RuDirect)'),
					E('div', { class: 'cbi-value-field' }, [
						chkRuDirect,
						E('span', { style: 'margin-left: 8px;' }, 'Использовать встроенный список RuDirect (' + ruCount + ' диапазонов РФ напрямую через WAN)'),
						ruDirectWarnNode
					])
				]),
				E('div', { class: 'cbi-value' }, [
					E('label', { class: 'cbi-value-title' }, 'Дополнительные IP / CIDR'),
					E('div', { class: 'cbi-value-field' }, [
						txtCIDRs,
						E('div', { class: 'cbi-value-description' }, 'По одному адресу или подсети на строку (например 1.1.1.1/32 или 104.16.0.0/12)')
					])
				]),
				E('div', { class: 'cbi-value' }, [
					E('label', { class: 'cbi-value-title' }, 'Доменные имена'),
					E('div', { class: 'cbi-value-field' }, [
						txtDomains,
						E('div', { class: 'cbi-value-description' }, 'По одному домену на строку (например rutracker.org). IP-адреса разрешаются при запуске службы.')
					])
				]),
				E('div', { style: 'margin-top: 16px;' }, [
					btnSaveSplit,
					btnCancelSplit
				])
			]),

			E('div', { class: 'cbi-section' }, [
				E('h3', {}, '4. Диагностика и мониторинг'),
				E('div', {}, [
					btnTestConn,
					btnShowLogs
				]),
				diagResult
			])
		]);
	}
});
