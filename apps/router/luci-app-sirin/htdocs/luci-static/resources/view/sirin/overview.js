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
	params: ['gateway', 'password', 'device_id', 'ru_direct', 'tun_mtu', 'kill_switch', 'normalize_ttl', 'target_ttl', 'intercept_dns', 'block_dot', 'dead_drop_urls'],
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
	params: ['mode', 'use_ru_direct', 'cidrs', 'domains', 'ports', 'port_mode'],
	expect: {}
});

var callGetAntiDPIConfig = rpc.declare({
	object: 'sirin',
	method: 'get_antidpi_config',
	expect: {}
});

var callSetAntiDPIConfig = rpc.declare({
	object: 'sirin',
	method: 'set_antidpi_config',
	params: ['enabled', 'port', 'split_pos', 'quic_reject', 'youtube', 'discord', 'custom_domains'],
	expect: {}
});

var callGetWhitelistConfig = rpc.declare({
	object: 'sirin',
	method: 'get_whitelist_config',
	expect: {}
});

var callSetWhitelistConfig = rpc.declare({
	object: 'sirin',
	method: 'set_whitelist_config',
	params: ['enabled', 'mode', 'gateway_port', 'use_dtls', 'turn_backup', 'turn_realm', 'relay_target', 'vk_hashes', 'dion_slugs'],
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

var callCheckUpdate = rpc.declare({
	object: 'sirin',
	method: 'check_update',
	expect: {}
});

var callUpgrade = rpc.declare({
	object: 'sirin',
	method: 'upgrade',
	params: ['url'],
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
			callGetRuDirectInfo().catch(function() { return {}; }),
			callGetAntiDPIConfig().catch(function() { return {}; }),
			callGetWhitelistConfig().catch(function() { return {}; })
		]);
	},

	render: function(data) {
		var status = data[0] || {};
		var config = data[1] || {};
		var splitConfig = data[2] || {};
		var ruInfo = data[3] || {};
		var antiDPI = data[4] || {};
		var wlConfig = data[5] || {};

		var ruCount = ruInfo.count || 8641;

		var updateStatusSpan = E('span', {
			id: 'sirin-update-badge',
			style: 'margin-left: 10px; display: inline-flex; align-items: center;'
		});

		function showUpgradeModal(info) {
			var modalBody = E('div', { class: 'cbi-modal-body' }, [
				E('p', {}, 'Доступна новая версия пакета Sirin VPN:'),
				E('table', { class: 'table', style: 'margin: 10px 0;' }, [
					E('tr', { class: 'tr' }, [
						E('td', { class: 'td', style: 'width: 35%; font-weight: bold;' }, 'Текущая версия:'),
						E('td', { class: 'td' }, info.current_version || '1.6.0')
					]),
					E('tr', { class: 'tr' }, [
						E('td', { class: 'td', style: 'font-weight: bold;' }, 'Новая версия:'),
						E('td', { class: 'td', style: 'color: #5cb85c; font-weight: bold;' }, info.latest_version || info.tag)
					]),
					E('tr', { class: 'tr' }, [
						E('td', { class: 'td', style: 'font-weight: bold;' }, 'Архитектура / Менеджер:'),
						E('td', { class: 'td' }, (info.arch || 'mipsle') + ' (' + (info.pkg_mgr || 'opkg') + ')')
					]),
					E('tr', { class: 'tr' }, [
						E('td', { class: 'td', style: 'font-weight: bold;' }, 'Файл обновления:'),
						E('td', { class: 'td', style: 'word-break: break-all;' }, info.asset_name || 'sirin-full')
					])
				]),
				info.release_notes ? E('div', { style: 'background: #f8f9fa; border: 1px solid #e9ecef; border-radius: 4px; padding: 8px; margin: 10px 0; max-height: 120px; overflow-y: auto; font-size: 12px;' }, [
					E('strong', {}, 'Информация о выпуске:'),
					E('p', { style: 'margin: 4px 0 0 0;' }, info.release_notes)
				]) : E('span', {}),
				E('p', { style: 'color: #f0ad4e; margin-top: 10px;' }, '⚠ Во время обновления служба VPN будет перезапущена. Не выключайте роутер.')
			]);

			var progressDiv = E('div', { style: 'display:none; text-align:center; padding: 20px 0;' }, [
				E('div', { class: 'spinning', style: 'display: inline-block; margin-bottom: 12px;' }),
				E('p', { id: 'sirin-upgrade-status-text', style: 'font-weight: bold;' }, 'Загрузка и установка обновления... Пожалуйста, подождите.')
			]);

			var btnApply = E('button', {
				class: 'btn cbi-button cbi-button-action',
				click: function() {
					btnApply.disabled = true;
					btnCancel.disabled = true;
					modalBody.style.display = 'none';
					progressDiv.style.display = 'block';

					return callUpgrade(info.asset_url).then(function(res) {
						if (res && res.ok) {
							progressDiv.innerHTML = '<div style="color: #5cb85c; font-size: 16px; font-weight: bold; margin-bottom: 8px;">✓ ' +
								(res.message || 'Обновление успешно установлено!') + '</div>' +
								'<p>Перезагрузка страницы через 3 секунды...</p>';
							setTimeout(function() {
								location.reload();
							}, 3000);
						} else {
							progressDiv.innerHTML = '<div style="color: #d9534f; font-size: 15px; font-weight: bold; margin-bottom: 8px;">✗ Ошибка обновления</div>' +
								'<p style="word-break: break-all;">' + ((res && res.error) || 'Неизвестная ошибка') + '</p>';
							btnCancel.disabled = false;
							btnCancel.textContent = 'Закрыть';
						}
					}).catch(function(e) {
						progressDiv.innerHTML = '<div style="color: #d9534f; font-size: 15px; font-weight: bold; margin-bottom: 8px;">✗ Ошибка выполнения запроса</div>' +
							'<p>' + (e.message || 'Ошибка RPC') + '</p>';
						btnCancel.disabled = false;
						btnCancel.textContent = 'Закрыть';
					});
				}
			}, 'Установить обновление');

			var btnCancel = E('button', {
				class: 'btn cbi-button cbi-button-neutral',
				click: ui.hideModal
			}, 'Отмена');

			ui.showModal('Обновление Sirin VPN', [
				modalBody,
				progressDiv,
				E('div', { class: 'right', style: 'margin-top: 15px;' }, [
					btnCancel,
					' ',
					btnApply
				])
			]);
		}

		var btnCheckUpdate = E('button', {
			class: 'btn cbi-button cbi-button-neutral',
			style: 'padding: 2px 8px; font-size: 12px; margin-left: 8px;',
			click: function() {
				btnCheckUpdate.disabled = true;
				updateStatusSpan.textContent = ' Проверка...';
				return callCheckUpdate().then(function(res) {
					btnCheckUpdate.disabled = false;
					if (!res || !res.ok) {
						updateStatusSpan.innerHTML = '<span style="color:#d9534f; margin-left:6px;">✗ ' + ((res && res.error) || 'Ошибка проверки') + '</span>';
						return;
					}
					if (res.update_available) {
						var btnDoUpgrade = E('button', {
							class: 'btn cbi-button cbi-button-action',
							style: 'margin-left: 8px; padding: 2px 8px; font-size: 12px;',
							click: function() {
								showUpgradeModal(res);
							}
						}, 'Обновить до ' + (res.tag || ('v' + res.latest_version)));

						updateStatusSpan.innerHTML = '';
						updateStatusSpan.appendChild(E('span', { style: 'color:#5cb85c; font-weight:bold; margin-left:6px;' }, 'Доступно: ' + res.latest_version));
						updateStatusSpan.appendChild(btnDoUpgrade);
					} else {
						updateStatusSpan.innerHTML = '<span style="color:#5cb85c; margin-left:6px;">✓ У вас последняя версия (' + res.current_version + ')</span>';
						setTimeout(function() {
							updateStatusSpan.innerHTML = '';
						}, 6000);
					}
				}).catch(function(e) {
					btnCheckUpdate.disabled = false;
					updateStatusSpan.innerHTML = '<span style="color:#d9534f; margin-left:6px;">✗ ' + (e.message || 'Ошибка RPC') + '</span>';
				});
			}
		}, 'Проверить обновления');

		var statusNode = E('span', { class: 'label' }, 'Опрос...');
		var infoTable = E('table', { class: 'table' }, [
			E('tr', { class: 'tr' }, [
				E('td', { class: 'td', style: 'width:30%' }, 'Статус службы:'),
				E('td', { class: 'td' }, statusNode)
			]),
			E('tr', { class: 'tr' }, [
				E('td', { class: 'td' }, 'Версия клиента:'),
				E('td', { class: 'td' }, [
					E('span', { id: 'sirin-version', style: 'font-weight: bold;' }, status.version || '1.6.0'),
					btnCheckUpdate,
					updateStatusSpan
				])
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

		var chkNormalizeTTL = E('input', {
			class: 'cbi-input-checkbox',
			type: 'checkbox',
			checked: (config.normalize_ttl !== undefined ? !!config.normalize_ttl : true)
		});

		var inTargetTTL = E('input', {
			class: 'cbi-input-text',
			type: 'number',
			min: '1',
			max: '255',
			style: 'width: 80px;',
			value: config.target_ttl || 64
		});

		var chkInterceptDNS = E('input', {
			class: 'cbi-input-checkbox',
			type: 'checkbox',
			checked: (config.intercept_dns !== undefined ? !!config.intercept_dns : true)
		});

		var chkBlockDoT = E('input', {
			class: 'cbi-input-checkbox',
			type: 'checkbox',
			checked: (config.block_dot !== undefined ? !!config.block_dot : true)
		});

		var txtDeadDrop = E('textarea', {
			class: 'cbi-input-textarea',
			rows: 3,
			style: 'width: 100%; font-family: monospace;',
			placeholder: 'https://disk.yandex.ru/d/...\nhttps://bucket.storage.yandexcloud.net/params.json'
		}, (config.dead_drop_urls || []).join('\n'));

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
				var normTTL = chkNormalizeTTL.checked;
				var targTTL = parseInt(inTargetTTL.value, 10);
				if (isNaN(targTTL) || targTTL < 1 || targTTL > 255) targTTL = 64;
				var intDNS = chkInterceptDNS.checked;
				var blkDoT = chkBlockDoT.checked;

				var rawDD = txtDeadDrop.value.split('\n');
				var validDD = [];
				for (var d = 0; d < rawDD.length; d++) {
					var item = rawDD[d].trim();
					if (item && (item.indexOf('http://') === 0 || item.indexOf('https://') === 0)) {
						validDD.push(item);
					}
				}

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
				return callSetConfig(gw, pw, did, rud, mtu, ks, normTTL, targTTL, intDNS, blkDoT, validDD).then(function(res) {
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
						chkNormalizeTTL.checked = (cfg.normalize_ttl !== undefined ? !!cfg.normalize_ttl : true);
						inTargetTTL.value = cfg.target_ttl || 64;
						chkInterceptDNS.checked = (cfg.intercept_dns !== undefined ? !!cfg.intercept_dns : true);
						chkBlockDoT.checked = (cfg.block_dot !== undefined ? !!cfg.block_dot : true);
						txtDeadDrop.value = (cfg.dead_drop_urls || []).join('\n');
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

		var portMode = (splitConfig.port_mode || 'only').toLowerCase();
		var radPortOnly = E('input', {
			type: 'radio',
			name: 'split_port_mode',
			value: 'only',
			checked: portMode !== 'bypass'
		});
		var radPortBypass = E('input', {
			type: 'radio',
			name: 'split_port_mode',
			value: 'bypass',
			checked: portMode === 'bypass'
		});

		var txtPorts = E('textarea', {
			class: 'cbi-input-textarea',
			rows: 4,
			style: 'width: 100%; font-family: monospace;',
			placeholder: '80\n443\n8080-8090\nudp:27000-27050'
		}, (splitConfig.ports || []).join('\n'));

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

				// Parse and validate ports
				var rawPorts = txtPorts.value.split('\n');
				var validPorts = [];
				var portRegex = /^(tcp:|udp:)?[0-9]{1,5}(-[0-9]{1,5})?$/i;
				for (var k = 0; k < rawPorts.length; k++) {
					var p = rawPorts[k].trim();
					if (!p) continue;
					if (!portRegex.test(p)) {
						ui.addNotification(null, E('p', {}, 'Некорректный формат порта или диапазона: ' + p), 'error');
						return;
					}
					var checkPart = p;
					if (checkPart.indexOf(':') !== -1) {
						checkPart = checkPart.split(':')[1];
					}
					if (checkPart.indexOf('-') !== -1) {
						var rangeParts = checkPart.split('-');
						var pStart = parseInt(rangeParts[0], 10);
						var pEnd = parseInt(rangeParts[1], 10);
						if (pStart < 1 || pStart > 65535 || pEnd < 1 || pEnd > 65535 || pStart >= pEnd) {
							ui.addNotification(null, E('p', {}, 'Некорректный диапазон портов (1-65535, start < end): ' + p), 'error');
							return;
						}
					} else {
						var pVal = parseInt(checkPart, 10);
						if (pVal < 1 || pVal > 65535) {
							ui.addNotification(null, E('p', {}, 'Номер порта вне диапазона 1-65535: ' + p), 'error');
							return;
						}
					}
					validPorts.push(p);
				}
				var selectedPortMode = radPortBypass.checked ? 'bypass' : 'only';

				btnSaveSplit.disabled = true;
				return callSetSplitConfig(mode, useRu, validCidrs, validDomains, validPorts, selectedPortMode).then(function(res) {
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
					txtPorts.value = (sc.ports || []).join('\n');
					if (sc.port_mode === 'bypass') {
						radPortBypass.checked = true;
					} else {
						radPortOnly.checked = true;
					}
					ui.addNotification(null, E('p', {}, 'Настройки разделения сброшены'), 'info');
				}).catch(function() {
					btnCancelSplit.disabled = false;
				});
			}
		}, 'Отмена');

		// Section 4: Anti-DPI (YouTube / Discord)
		var chkADEnabled = E('input', {
			type: 'checkbox',
			checked: (antiDPI.enabled !== undefined ? !!antiDPI.enabled : true)
		});

		var chkADYouTube = E('input', {
			type: 'checkbox',
			checked: (antiDPI.youtube !== undefined ? !!antiDPI.youtube : true)
		});

		var chkADDiscord = E('input', {
			type: 'checkbox',
			checked: (antiDPI.discord !== undefined ? !!antiDPI.discord : true)
		});

		var chkADQUICReject = E('input', {
			type: 'checkbox',
			checked: (antiDPI.quic_reject !== undefined ? !!antiDPI.quic_reject : true)
		});

		var inADSplitPos = E('input', {
			type: 'number',
			class: 'cbi-input-text',
			min: 1,
			max: 100,
			value: antiDPI.split_pos || 2,
			style: 'width: 100px;'
		});

		var inADPort = E('input', {
			type: 'number',
			class: 'cbi-input-text',
			min: 1024,
			max: 65535,
			value: antiDPI.port || 56100,
			style: 'width: 120px;'
		});

		var txtADCustomDomains = E('textarea', {
			class: 'cbi-input-textarea',
			rows: 4,
			style: 'width: 100%; font-family: monospace;',
			placeholder: 'example.com\ncdn.example.org'
		}, (antiDPI.custom_domains || []).join('\n'));

		var btnSaveAntiDPI = E('button', {
			class: 'btn cbi-button cbi-button-save',
			style: 'margin-right: 8px;',
			click: function() {
				var enabled = chkADEnabled.checked;
				var yt = chkADYouTube.checked;
				var dc = chkADDiscord.checked;
				var quic = chkADQUICReject.checked;
				var sp = parseInt(inADSplitPos.value, 10);
				if (isNaN(sp) || sp < 1 || sp > 100) {
					ui.addNotification(null, E('p', {}, 'Некорректная позиция фрагментации ClientHello (1-100)'), 'error');
					return;
				}
				var port = parseInt(inADPort.value, 10);
				if (isNaN(port) || port < 1024 || port > 65535) {
					ui.addNotification(null, E('p', {}, 'Некорректный порт локального прокси Anti-DPI (1024-65535)'), 'error');
					return;
				}

				var rawDomains = txtADCustomDomains.value.split('\n');
				var validDomains = [];
				var domainRegex = /^([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$/;
				for (var i = 0; i < rawDomains.length; i++) {
					var d = rawDomains[i].trim().toLowerCase();
					if (!d) continue;
					if (!domainRegex.test(d)) {
						ui.addNotification(null, E('p', {}, 'Некорректный формат доменного имени Anti-DPI: ' + d), 'error');
						return;
					}
					validDomains.push(d);
				}

				btnSaveAntiDPI.disabled = true;
				return callSetAntiDPIConfig(enabled, port, sp, quic, yt, dc, validDomains).then(function(res) {
					btnSaveAntiDPI.disabled = false;
					if (res && res.ok) {
						ui.addNotification(null, E('p', {}, 'Параметры Anti-DPI сохранены'), 'info');
						ui.showModal('Перезапустить службу Sirin?', [
							E('p', {}, 'Параметры Anti-DPI сохранены. Для применения сетевых правил nftables и dnsmasq требуется перезапуск службы.'),
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
						ui.addNotification(null, E('p', {}, (res && res.error) || 'Ошибка сохранения Anti-DPI'), 'error');
					}
				}).catch(function(e) {
					btnSaveAntiDPI.disabled = false;
					ui.addNotification(null, E('p', {}, e.message || 'Ошибка RPC сохранения Anti-DPI'), 'error');
				});
			}
		}, 'Сохранить параметры Anti-DPI');

		var btnCancelAntiDPI = E('button', {
			class: 'btn cbi-button cbi-button-reset',
			click: function() {
				btnCancelAntiDPI.disabled = true;
				return callGetAntiDPIConfig().then(function(ad) {
					btnCancelAntiDPI.disabled = false;
					ad = ad || {};
					chkADEnabled.checked = (ad.enabled !== undefined ? !!ad.enabled : true);
					chkADYouTube.checked = (ad.youtube !== undefined ? !!ad.youtube : true);
					chkADDiscord.checked = (ad.discord !== undefined ? !!ad.discord : true);
					chkADQUICReject.checked = (ad.quic_reject !== undefined ? !!ad.quic_reject : true);
					inADSplitPos.value = ad.split_pos || 2;
					inADPort.value = ad.port || 56100;
					txtADCustomDomains.value = (ad.custom_domains || []).join('\n');
					ui.addNotification(null, E('p', {}, 'Настройки Anti-DPI сброшены'), 'info');
				}).catch(function() {
					btnCancelAntiDPI.disabled = false;
				});
			}
		}, 'Отмена');

		// Section 5: Whitelist Fallback (VK Calls / DION / S3)
		var chkWLEnabled = E('input', {
			type: 'checkbox',
			checked: (wlConfig.enabled !== undefined ? !!wlConfig.enabled : true)
		});

		var wlMode = (wlConfig.mode || 'auto').toLowerCase();
		var radWLAuto = E('input', {
			type: 'radio',
			name: 'wl_mode',
			value: 'auto',
			checked: wlMode !== 'always'
		});
		var radWLAlways = E('input', {
			type: 'radio',
			name: 'wl_mode',
			value: 'always',
			checked: wlMode === 'always'
		});

		var inWLPort = E('input', {
			type: 'number',
			class: 'cbi-input-text',
			min: 1,
			max: 65535,
			value: wlConfig.gateway_port || 56005,
			style: 'width: 100px;'
		});

		var chkWLDTLS = E('input', {
			type: 'checkbox',
			checked: !!wlConfig.use_dtls
		});

		var inWLTurnBackup = E('input', {
			type: 'text',
			class: 'cbi-input-text',
			value: wlConfig.turn_backup || '193.203.43.1:19302',
			style: 'width: 250px;'
		});

		var inWLTurnRealm = E('input', {
			type: 'text',
			class: 'cbi-input-text',
			value: wlConfig.turn_realm || 'vkcalls',
			style: 'width: 150px;'
		});

		var inWLRelayTarget = E('input', {
			type: 'text',
			class: 'cbi-input-text',
			value: wlConfig.relay_target || '',
			placeholder: '200.10.40.239',
			style: 'width: 200px;'
		});

		var txtWLVKHashes = E('textarea', {
			class: 'cbi-input-textarea',
			rows: 4,
			style: 'width: 100%; font-family: monospace;',
			placeholder: 'https://vk.com/call/join/abcdef123456\nhttps://vk.com/call/join/uvwxyz789012'
		}, (wlConfig.vk_hashes || []).join('\n'));

		var txtWLDionSlugs = E('textarea', {
			class: 'cbi-input-textarea',
			rows: 4,
			style: 'width: 100%; font-family: monospace;',
			placeholder: 'https://dion.vc/event/my-room-slug\nanother-slug'
		}, (wlConfig.dion_slugs || []).join('\n'));

		var btnSaveWhitelist = E('button', {
			class: 'btn cbi-button cbi-button-save',
			style: 'margin-right: 8px;',
			click: function() {
				var enabled = chkWLEnabled.checked;
				var mode = radWLAlways.checked ? 'always' : 'auto';
				var port = parseInt(inWLPort.value, 10);
				if (isNaN(port) || port < 1 || port > 65535) {
					ui.addNotification(null, E('p', {}, 'Некорректный порт шлюза для релея (1-65535)'), 'error');
					return;
				}
				var dtls = chkWLDTLS.checked;
				var turn = inWLTurnBackup.value.trim() || '193.203.43.1:19302';
				var realm = inWLTurnRealm.value.trim() || 'vkcalls';
				var target = inWLRelayTarget.value.trim();

				var rawVK = txtWLVKHashes.value.split('\n');
				var validVK = [];
				for (var i = 0; i < rawVK.length; i++) {
					var item = rawVK[i].trim();
					if (item) validVK.push(item);
				}

				var rawDion = txtWLDionSlugs.value.split('\n');
				var validDion = [];
				for (var j = 0; j < rawDion.length; j++) {
					var dItem = rawDion[j].trim();
					if (dItem) validDion.push(dItem);
				}

				btnSaveWhitelist.disabled = true;
				return callSetWhitelistConfig(enabled, mode, port, dtls, turn, realm, target, validVK, validDion).then(function(res) {
					btnSaveWhitelist.disabled = false;
					if (res && res.ok) {
						ui.addNotification(null, E('p', {}, 'Параметры Whitelist Fallback сохранены'), 'info');
						ui.showModal('Перезапустить службу Sirin?', [
							E('p', {}, 'Параметры аварийного канала сохранены. Для применения конфигурации требуется перезапуск службы.'),
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
						ui.addNotification(null, E('p', {}, (res && res.error) || 'Ошибка сохранения Whitelist Fallback'), 'error');
					}
				}).catch(function(e) {
					btnSaveWhitelist.disabled = false;
					ui.addNotification(null, E('p', {}, e.message || 'Ошибка RPC сохранения'), 'error');
				});
			}
		}, 'Сохранить параметры Whitelist Fallback');

		var btnCancelWhitelist = E('button', {
			class: 'btn cbi-button cbi-button-reset',
			click: function() {
				btnCancelWhitelist.disabled = true;
				return callGetWhitelistConfig().then(function(wl) {
					btnCancelWhitelist.disabled = false;
					wl = wl || {};
					chkWLEnabled.checked = (wl.enabled !== undefined ? !!wl.enabled : true);
					if (wl.mode === 'always') {
						radWLAlways.checked = true;
					} else {
						radWLAuto.checked = true;
					}
					inWLPort.value = wl.gateway_port || 56005;
					chkWLDTLS.checked = !!wl.use_dtls;
					inWLTurnBackup.value = wl.turn_backup || '193.203.43.1:19302';
					inWLTurnRealm.value = wl.turn_realm || 'vkcalls';
					inWLRelayTarget.value = wl.relay_target || '';
					txtWLVKHashes.value = (wl.vk_hashes || []).join('\n');
					txtWLDionSlugs.value = (wl.dion_slugs || []).join('\n');
					ui.addNotification(null, E('p', {}, 'Настройки Whitelist Fallback сброшены'), 'info');
				}).catch(function() {
					btnCancelWhitelist.disabled = false;
				});
			}
		}, 'Отмена');

		// Section 6: Diagnostics
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
				E('div', { class: 'cbi-value' }, [
					E('label', { class: 'cbi-value-title' }, 'OpSec: Нормализация TTL'),
					E('div', { class: 'cbi-value-field' }, [
						E('label', { style: 'cursor: pointer;' }, [
							chkNormalizeTTL,
							E('span', { style: 'margin-left: 6px;' }, E('strong', {}, 'Нормализовать TTL исходящих пакетов'))
						]),
						E('div', { style: 'margin-top: 6px; display: flex; align-items: center;' }, [
							E('span', { style: 'margin-right: 8px;' }, 'Целевое значение TTL:'),
							inTargetTTL,
							E('span', { style: 'color: #888; margin-left: 8px;' }, '(по умолчанию 64; для Windows 128)')
						]),
						E('div', { class: 'cbi-value-description' }, 'Маскирует роутер и факт раздачи интернета (ставит фиксированный TTL на выходе из туннеля и выравнивает TCP MSS под MTU).')
					])
				]),
				E('div', { class: 'cbi-value' }, [
					E('label', { class: 'cbi-value-title' }, 'OpSec: Anti-DNS-Leak'),
					E('div', { class: 'cbi-value-field' }, [
						E('label', { style: 'cursor: pointer;' }, [
							chkInterceptDNS,
							E('span', { style: 'margin-left: 6px;' }, E('strong', {}, 'Перехват всех DNS-запросов (порт 53)'))
						]),
						E('div', { class: 'cbi-value-description' }, 'Принудительно заворачивает любые DNS-запросы LAN-клиентов на локальный dnsmasq роутера, исключая утечки.')
					])
				]),
				E('div', { class: 'cbi-value' }, [
					E('label', { class: 'cbi-value-title' }, 'OpSec: Блокировка DoT'),
					E('div', { class: 'cbi-value-field' }, [
						E('label', { style: 'cursor: pointer;' }, [
							chkBlockDoT,
							E('span', { style: 'margin-left: 6px;' }, E('strong', {}, 'Блокировать DNS-over-TLS (порт 853)'))
						]),
						E('div', { class: 'cbi-value-description' }, 'Запрещает соединения на порт 853, не позволяя Android Private DNS обходить локальный DNS роутера.')
					])
				]),
				E('div', { class: 'cbi-value' }, [
					E('label', { class: 'cbi-value-title' }, 'Резервный Dead Drop (Яндекс Диск / S3)'),
					E('div', { class: 'cbi-value-field' }, [
						txtDeadDrop,
						E('div', { class: 'cbi-value-description' }, 'Резервные публичные ссылки Яндекс Диска (https://disk.yandex.ru/d/...) или S3 URL для получения конфигурации при блокировке центрального API.')
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
				E('div', { class: 'cbi-value' }, [
					E('label', { class: 'cbi-value-title' }, 'Режим разделения портов'),
					E('div', { class: 'cbi-value-field' }, [
						E('label', { style: 'margin-right: 16px; cursor: pointer;' }, [
							radPortOnly,
							E('span', { style: 'margin-left: 6px;' }, E('strong', {}, 'Only (Веб-порты)')),
							E('span', { style: 'color: #888; margin-left: 4px;' }, '— только указанные порты в VPN, всё остальное напрямую в WAN')
						]),
						E('br'),
						E('label', { style: 'cursor: pointer; margin-top: 6px; display: inline-block;' }, [
							radPortBypass,
							E('span', { style: 'margin-left: 6px;' }, E('strong', {}, 'Bypass (Gaming Bypass)')),
							E('span', { style: 'color: #888; margin-left: 4px;' }, '— исключать указанные порты из VPN (игры напрямую в WAN)')
						])
					])
				]),
				E('div', { class: 'cbi-value' }, [
					E('label', { class: 'cbi-value-title' }, 'Порты назначения (Split Ports)'),
					E('div', { class: 'cbi-value-field' }, [
						txtPorts,
						E('div', { class: 'cbi-value-description' }, 'По одному порту или диапазону на строку (например 80, 443, 8080-8090, tcp:80, udp:27000-27050). Оставьте пустым, чтобы туннелировать весь трафик без фильтрации портов.')
					])
				]),
				E('div', { style: 'margin-top: 16px;' }, [
					btnSaveSplit,
					btnCancelSplit
				])
			]),

			E('div', { class: 'cbi-section' }, [
				E('h3', {}, '4. Обход замедления без VPN (Anti-DPI: YouTube / Discord)'),
				E('p', { class: 'cbi-section-descr' }, 'Фрагментация TLS ClientHello и блокировка QUIC (UDP 443) на уровне роутера. Позволяет смотреть YouTube в 4K/8K и пользоваться Discord на максимальной скорости провайдера напрямую через WAN, без нагрузки на внешний VPN-сервер.'),
				E('div', { class: 'cbi-value' }, [
					E('label', { class: 'cbi-value-title' }, 'Активация Anti-DPI'),
					E('div', { class: 'cbi-value-field' }, [
						E('label', { style: 'cursor: pointer;' }, [
							chkADEnabled,
							E('span', { style: 'margin-left: 6px;' }, E('strong', {}, 'Включить встроенный модуль Anti-DPI'))
						]),
						E('div', { class: 'cbi-value-description' }, 'Автоматически перенаправляет TCP 80/443 целевых сервисов на локальный прозрачный прокси фрагментации пакетов.')
					])
				]),
				E('div', { class: 'cbi-value' }, [
					E('label', { class: 'cbi-value-title' }, 'Целевые сервисы'),
					E('div', { class: 'cbi-value-field' }, [
						E('label', { style: 'cursor: pointer; margin-right: 16px;' }, [
							chkADYouTube,
							E('span', { style: 'margin-left: 6px;' }, 'YouTube & Google Video CDN (googlevideo.com, ytimg.com, etc.)')
						]),
						E('br'),
						E('label', { style: 'cursor: pointer; margin-top: 6px; display: inline-block;' }, [
							chkADDiscord,
							E('span', { style: 'margin-left: 6px;' }, 'Discord (discord.com, discord.gg, gateway, voice servers)')
						])
					])
				]),
				E('div', { class: 'cbi-value' }, [
					E('label', { class: 'cbi-value-title' }, 'Блокировка QUIC (UDP 443)'),
					E('div', { class: 'cbi-value-field' }, [
						E('label', { style: 'cursor: pointer;' }, [
							chkADQUICReject,
							E('span', { style: 'margin-left: 6px;' }, E('strong', {}, 'Принудительный откат на TCP (Reject QUIC)'))
						]),
						E('div', { class: 'cbi-value-description' }, 'Блокирует протокол QUIC для целевых доменов, заставляя браузер мгновенно переключаться на защищаемый TCP/TLS HTTP/2.')
					])
				]),
				E('div', { class: 'cbi-value' }, [
					E('label', { class: 'cbi-value-title' }, 'Позиция сплита ClientHello'),
					E('div', { class: 'cbi-value-field' }, [
						inADSplitPos,
						E('div', { class: 'cbi-value-description' }, 'Байт разделения первого TLS-пакета (по умолчанию 2). Разбивает заголовок TLS, скрывая SNI от ТСПУ.')
					])
				]),
				E('div', { class: 'cbi-value' }, [
					E('label', { class: 'cbi-value-title' }, 'Пользовательские домены'),
					E('div', { class: 'cbi-value-field' }, [
						txtADCustomDomains,
						E('div', { class: 'cbi-value-description' }, 'Дополнительные домены для прямого Anti-DPI обхода (по одному на строку, например hdrezka.ag).')
					])
				]),
				E('div', { class: 'cbi-value' }, [
					E('label', { class: 'cbi-value-title' }, 'Локальный порт Anti-DPI'),
					E('div', { class: 'cbi-value-field' }, [
						inADPort,
						E('div', { class: 'cbi-value-description' }, 'Внутренний порт прокси-сервера фрагментации на роутере (по умолчанию 56100).')
					])
				]),
				E('div', { style: 'margin-top: 16px;' }, [
					btnSaveAntiDPI,
					btnCancelAntiDPI
				])
			]),

			E('div', { class: 'cbi-section' }, [
				E('h3', {}, '5. Аварийный обход (Whitelist Fallback: WebRTC TURN VK / DION)'),
				E('p', { class: 'cbi-section-descr' }, 'Аварийный резервный туннель через разрешённые отечественные сервисы (WebRTC TURN VK Calls и DION Video Calls). Применяется при тотальных блокировках протоколов, изоляции внешнего интернета и введении «белых списков» (трафик идёт на доверенные российские адреса api.vk.me, calls.okcdn.ru, api.dion.vc).'),
				E('div', { class: 'cbi-value' }, [
					E('label', { class: 'cbi-value-title' }, 'Аварийный канал'),
					E('div', { class: 'cbi-value-field' }, [
						E('label', { style: 'cursor: pointer;' }, [
							chkWLEnabled,
							E('span', { style: 'margin-left: 6px;' }, E('strong', {}, 'Включить Whitelist Fallback'))
						]),
						E('div', { class: 'cbi-value-description' }, 'Разрешает автоматический переход на WebRTC TURN релеи при недоступности прямого туннеля.')
					])
				]),
				E('div', { class: 'cbi-value' }, [
					E('label', { class: 'cbi-value-title' }, 'Режим работы'),
					E('div', { class: 'cbi-value-field' }, [
						E('label', { style: 'margin-right: 16px; cursor: pointer;' }, [
							radWLAuto,
							E('span', { style: 'margin-left: 6px;' }, E('strong', {}, 'Auto (Аварийный откат)')),
							E('span', { style: 'color: #888; margin-left: 4px;' }, '— включается последней ступенью при отказе прямых соединений')
						]),
						E('br'),
						E('label', { style: 'cursor: pointer; margin-top: 6px; display: inline-block;' }, [
							radWLAlways,
							E('span', { style: 'margin-left: 6px;' }, E('strong', {}, 'Always (Только белый список)')),
							E('span', { style: 'color: #888; margin-left: 4px;' }, '— принудительно направлять трафик исключительно через отечественные WebRTC релеи')
						])
					])
				]),
				E('div', { class: 'cbi-value' }, [
					E('label', { class: 'cbi-value-title' }, 'Ссылки / хеши VK Calls'),
					E('div', { class: 'cbi-value-field' }, [
						txtWLVKHashes,
						E('div', { class: 'cbi-value-description' }, 'Ссылки на групповые звонки VK (https://vk.com/call/join/...) или голые хеши. Клиент анонимно запрашивает свежие TURN-токены через api.vk.me без необходимости входа в аккаунт.')
					])
				]),
				E('div', { class: 'cbi-value' }, [
					E('label', { class: 'cbi-value-title' }, 'Комнаты DION (ВТБ)'),
					E('div', { class: 'cbi-value-field' }, [
						txtWLDionSlugs,
						E('div', { class: 'cbi-value-description' }, 'Ссылки на конференции DION (https://dion.vc/event/...) или slug комнат. Запасной отечественный источник WebRTC TURN после VK.')
					])
				]),
				E('div', { class: 'cbi-value' }, [
					E('label', { class: 'cbi-value-title' }, 'Запасной TURN сервер'),
					E('div', { class: 'cbi-value-field' }, [
						inWLTurnBackup,
						E('div', { class: 'cbi-value-description' }, 'Резервный IP:порт TURN-сервера (по умолчанию 193.203.43.1:19302).')
					])
				]),
				E('div', { class: 'cbi-value' }, [
					E('label', { class: 'cbi-value-title' }, 'Порт шлюза'),
					E('div', { class: 'cbi-value-field' }, [
						inWLPort,
						E('div', { class: 'cbi-value-description' }, 'Порт назначения на сервере при прохождении через релей (по умолчанию 56005).')
					])
				]),
				E('div', { style: 'margin-top: 16px;' }, [
					btnSaveWhitelist,
					btnCancelWhitelist
				])
			]),

			E('div', { class: 'cbi-section' }, [
				E('h3', {}, '6. Диагностика и мониторинг'),
				E('div', {}, [
					btnTestConn,
					btnShowLogs
				]),
				diagResult
			])
		]);
	}
});
