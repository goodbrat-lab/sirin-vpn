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
	params: ['enabled', 'port', 'split_pos', 'quic_reject', 'youtube', 'instagram', 'discord', 'custom_domains'],
	expect: {}
});

var callGetForceVPNConfig = rpc.declare({
	object: 'sirin',
	method: 'get_force_vpn_config',
	expect: {}
});

var callSetForceVPNConfig = rpc.declare({
	object: 'sirin',
	method: 'set_force_vpn_config',
	params: ['enabled', 'services', 'domains', 'cidrs'],
	expect: {}
});

var callGetWhitelistConfig = rpc.declare({
	object: 'sirin',
	method: 'get_whitelist_config',
	expect: {}
});

var callGetPSNProxyConfig = rpc.declare({
	object: 'sirin',
	method: 'get_psn_proxy_config',
	expect: {}
});

var callSetPSNProxyConfig = rpc.declare({
	object: 'sirin',
	method: 'set_psn_proxy_config',
	params: ['enabled', 'type', 'host', 'port', 'username', 'password', 'via', 'include_downloads'],
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

function formatTransportHuman(tr) {
	if (!tr) return 'Определение...';
	var u = tr.toUpperCase();
	if (u.indexOf('ZAPRET2 (LAZY VPN)') !== -1) {
		return 'Zapret 2 Активен (VPN в спящем режиме — 0 Байт на сервер)';
	}
	if (u.indexOf('VLESS') !== -1) {
		return 'Zapret 2 + Защищённый туннель VLESS Reality';
	}
	if (u.indexOf('UDP') !== -1) {
		return 'Zapret 2 + Скоростной туннель Noise UDP';
	}
	return u;
}

return view.extend({
	load: function() {
		return Promise.all([
			callStatus().catch(function() { return {}; }),
			callGetConfig().catch(function() { return {}; }),
			callGetSplitConfig().catch(function() { return {}; }),
			callGetRuDirectInfo().catch(function() { return {}; }),
			callGetAntiDPIConfig().catch(function() { return {}; }),
			callGetWhitelistConfig().catch(function() { return {}; }),
			callGetForceVPNConfig().catch(function() { return {}; }),
			callGetPSNProxyConfig().catch(function() { return {}; })
		]);
	},

	render: function(data) {
		var status = data[0] || {};
		var config = data[1] || {};
		var splitConfig = data[2] || {};
		var ruInfo = data[3] || {};
		var antiDPI = data[4] || {};
		var wlConfig = data[5] || {};
		var forceVPN = data[6] || {};
		var psnProxy = data[7] || {};

		var ruCount = ruInfo.count || 8641;
		var isExpert = false;
		try {
			isExpert = localStorage.getItem('sirin_ui_expert') === '1';
		} catch (e) {}

		var hasUnsavedChanges = false;
		var saveBanner = E('div', {
			style: 'position: sticky; bottom: 12px; z-index: 50; background: #1a2433; color: #fff; padding: 12px 18px; border-radius: 8px; box-shadow: 0 4px 16px rgba(0,0,0,0.25); display: flex; align-items: center; justify-content: space-between; margin-top: 16px; border: 1px solid #2b4c7e;'
		});

		var saveBannerText = E('span', { style: 'font-size: 13px; font-weight: bold;' },
			'Проверьте параметры и нажмите «Сохранить и применить», чтобы активировать настройки.'
		);

		function markDirty(msg) {
			hasUnsavedChanges = true;
			saveBanner.style.background = '#123524';
			saveBanner.style.borderColor = '#28a745';
			saveBannerText.textContent = msg || '● Есть несохранённые изменения — нажмите «Сохранить и применить»';
		}

		// --- UPDATE MODAL ---
		var updateStatusSpan = E('span', {
			id: 'sirin-update-badge',
			style: 'margin-left: 8px; display: inline-flex; align-items: center;'
		});

		function showUpgradeModal(info) {
			var modalBody = E('div', { class: 'cbi-modal-body' }, [
				E('p', {}, 'Доступна новая версия пакета Sirin VPN:'),
				E('table', { class: 'table', style: 'margin: 10px 0;' }, [
					E('tr', { class: 'tr' }, [
						E('td', { class: 'td', style: 'width: 35%; font-weight: bold;' }, 'Текущая версия:'),
						E('td', { class: 'td' }, info.current_version || '1.8.0')
					]),
					E('tr', { class: 'tr' }, [
						E('td', { class: 'td', style: 'font-weight: bold;' }, 'Новая версия:'),
						E('td', { class: 'td', style: 'color: #28a745; font-weight: bold;' }, info.latest_version || info.tag)
					]),
					E('tr', { class: 'tr' }, [
						E('td', { class: 'td', style: 'font-weight: bold;' }, 'Устройство:'),
						E('td', { class: 'td' }, (info.arch || 'arm64') + ' (' + (info.pkg_mgr || 'opkg') + ')')
					])
				]),
				info.release_notes ? E('div', { style: 'background: #f8f9fa; border: 1px solid #e9ecef; border-radius: 4px; padding: 8px; margin: 10px 0; max-height: 120px; overflow-y: auto; font-size: 12px;' }, [
					E('strong', {}, 'Что нового:'),
					E('p', { style: 'margin: 4px 0 0 0;' }, info.release_notes)
				]) : E('span', {}),
				E('p', { style: 'color: #d99b26; margin-top: 10px;' }, '⚠ Во время установки обновления служба VPN будет перезапущена на 3–5 секунд.')
			]);

			var progressDiv = E('div', { style: 'display:none; text-align:center; padding: 20px 0;' }, [
				E('div', { class: 'spinning', style: 'display: inline-block; margin-bottom: 12px;' }),
				E('p', { style: 'font-weight: bold;' }, 'Скачивание и установка обновления... Пожалуйста, подождите.')
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
							progressDiv.innerHTML = '<div style="color: #28a745; font-size: 16px; font-weight: bold; margin-bottom: 8px;">✓ ' +
								(res.message || 'Обновление успешно установлено!') + '</div>' +
								'<p>Обновление страницы через 3 секунды...</p>';
							setTimeout(function() { location.reload(); }, 3000);
						} else {
							progressDiv.innerHTML = '<div style="color: #d9534f; font-size: 15px; font-weight: bold; margin-bottom: 8px;">✗ Ошибка обновления</div>' +
								'<p style="word-break: break-all;">' + ((res && res.error) || 'Неизвестная ошибка') + '</p>';
							btnCancel.disabled = false;
							btnCancel.textContent = 'Закрыть';
						}
					}).catch(function(e) {
						progressDiv.innerHTML = '<div style="color: #d9534f; font-size: 15px; font-weight: bold; margin-bottom: 8px;">✗ Ошибка запроса</div>' +
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
				E('div', { class: 'right', style: 'margin-top: 15px;' }, [btnCancel, ' ', btnApply])
			]);
		}

		var btnCheckUpdate = E('button', {
			class: 'btn cbi-button cbi-button-neutral',
			style: 'padding: 2px 10px; font-size: 12px; margin-left: 8px;',
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
							click: function() { showUpgradeModal(res); }
						}, 'Обновить до ' + (res.tag || ('v' + res.latest_version)));
						updateStatusSpan.innerHTML = '';
						updateStatusSpan.appendChild(E('span', { style: 'color:#28a745; font-weight:bold; margin-left:6px;' }, 'Доступна v' + res.latest_version));
						updateStatusSpan.appendChild(btnDoUpgrade);
					} else {
						updateStatusSpan.innerHTML = '<span style="color:#28a745; margin-left:6px; font-weight:bold;">✓ Установлена последняя версия (' + res.current_version + ')</span>';
						setTimeout(function() { updateStatusSpan.innerHTML = ''; }, 6000);
					}
				}).catch(function(e) {
					btnCheckUpdate.disabled = false;
					updateStatusSpan.innerHTML = '<span style="color:#d9534f; margin-left:6px;">✗ ' + (e.message || 'Ошибка RPC') + '</span>';
				});
			}
		}, 'Проверить обновления');

		// --- HEADER STATUS CONTROLS ---
		var statusBadge = E('span', {
			class: 'label',
			style: 'font-size: 13px; padding: 4px 10px; border-radius: 12px;'
		}, 'Опрос...');

		var transportNode = E('span', {
			id: 'sirin-transport',
			style: 'font-weight: bold; color: #0069d9;'
		}, status.transport ? formatTransportHuman(status.transport) : '—');

		var uptimeNode = E('span', { id: 'sirin-uptime' }, formatUptime(status.uptime_sec));
		var versionNode = E('span', { id: 'sirin-version', style: 'font-weight: bold;' }, status.version || '1.8.0');

		var btnStart = E('button', {
			class: 'btn cbi-button cbi-button-action',
			style: 'margin-right: 6px;',
			click: function() {
				btnStart.disabled = true;
				return callStart().then(function(res) {
					if (res && res.ok) ui.addNotification(null, E('p', {}, 'Защита Sirin VPN включена'), 'info');
					else ui.addNotification(null, E('p', {}, (res && res.error) || 'Ошибка запуска'), 'error');
					updateStatusView();
				}).catch(function() { updateStatusView(); });
			}
		}, '▶ Включить защиту');

		var btnStop = E('button', {
			class: 'btn cbi-button cbi-button-reset',
			style: 'margin-right: 6px;',
			click: function() {
				btnStop.disabled = true;
				return callStop().then(function(res) {
					if (res && res.ok) ui.addNotification(null, E('p', {}, 'Служба Sirin VPN остановлена'), 'info');
					else ui.addNotification(null, E('p', {}, (res && res.error) || 'Ошибка остановки'), 'error');
					updateStatusView();
				}).catch(function() { updateStatusView(); });
			}
		}, '■ Остановить');

		var btnRestart = E('button', {
			class: 'btn cbi-button cbi-button-apply',
			style: 'margin-right: 6px;',
			click: function() {
				btnRestart.disabled = true;
				return callRestart().then(function(res) {
					if (res && res.ok) ui.addNotification(null, E('p', {}, 'Служба Sirin VPN перезапущена'), 'info');
					else ui.addNotification(null, E('p', {}, (res && res.error) || 'Ошибка перезапуска'), 'error');
					updateStatusView();
				}).catch(function() { updateStatusView(); });
			}
		}, '↻ Перезапустить');

		var currentStatus = status;
		var btnAutostart = E('button', {
			class: 'btn cbi-button',
			click: function() {
				btnAutostart.disabled = true;
				var act = currentStatus.enabled ? callDisable : callEnable;
				var text = currentStatus.enabled ? 'отключен' : 'включен';
				return act().then(function(res) {
					if (res && res.ok) ui.addNotification(null, E('p', {}, 'Автозапуск при включении роутера ' + text), 'info');
					updateStatusView();
				}).catch(function() { updateStatusView(); });
			}
		}, 'Автозапуск');

		function applyStatus(st) {
			currentStatus = st || {};
			var isRunning = !!currentStatus.running;
			statusBadge.className = isRunning ? 'label success' : 'label warning';
			statusBadge.textContent = isRunning ? ('Защита активна (PID: ' + (currentStatus.pid || '—') + ')') : 'Защита остановлена';

			versionNode.textContent = currentStatus.version || '1.8.0';
			transportNode.textContent = isRunning ? formatTransportHuman(currentStatus.transport) : '—';
			uptimeNode.textContent = isRunning ? formatUptime(currentStatus.uptime_sec) : '—';

			btnStart.disabled = isRunning;
			btnStop.disabled = !isRunning;
			btnRestart.disabled = false;
			btnAutostart.disabled = false;
			btnAutostart.textContent = currentStatus.enabled ? '✓ Автозапуск включен' : 'Включить автозапуск';
		}
		applyStatus(status);

		function updateStatusView() {
			return callStatus().then(function(st) { applyStatus(st); }).catch(function() {});
		}
		poll.add(function() { return updateStatusView(); }, 5);

		// --- EXPERT MODE TOGGLE ---
		var expertBlocks = [];
		function registerExpertBlock(el) {
			expertBlocks.push(el);
			el.style.display = isExpert ? 'block' : 'none';
			return el;
		}

		var btnModeToggle = E('button', {
			class: 'btn cbi-button',
			style: 'font-weight: bold;',
			click: function() {
				isExpert = !isExpert;
				try { localStorage.setItem('sirin_ui_expert', isExpert ? '1' : '0'); } catch (e) {}
				btnModeToggle.textContent = isExpert ? '🛠 Режим: Экспертный (показаны все тонкие настройки)' : '👤 Режим: Простой (понятные настройки)';
				for (var i = 0; i < expertBlocks.length; i++) {
					expertBlocks[i].style.display = isExpert ? 'block' : 'none';
				}
			}
		}, isExpert ? '🛠 Режим: Экспертный (показаны все тонкие настройки)' : '👤 Режим: Простой (понятные настройки)');

		// --- FORM INPUTS (SHARED ACROSS TABS & PRESETS) ---
		var inGateway = E('input', {
			class: 'cbi-input-text',
			type: 'text',
			placeholder: 'crp.guseynovia.ru',
			value: config.gateway || 'crp.guseynovia.ru',
			change: function() { markDirty(); }
		});

		var inPassword = E('input', {
			class: 'cbi-input-text',
			type: 'password',
			style: 'max-width: 340px;',
			placeholder: 'Вставьте UUID-ключ из @sirin_crp_bot',
			value: config.password || '',
			input: function() { checkPasswordWarn(); markDirty(); }
		});

		var passWarnNode = E('div', {
			class: 'cbi-value-description',
			style: 'color: #d99b26; display: none; margin-top: 4px;'
		}, 'Проверьте формат: ключ должен быть в формате UUID (xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx)');

		function checkPasswordWarn() {
			var val = inPassword.value.trim();
			var uuidRegex = /^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-4[0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$/;
			if (val && val.indexOf('…') === -1 && val.indexOf('...') === -1 && !uuidRegex.test(val)) {
				passWarnNode.style.display = 'block';
			} else {
				passWarnNode.style.display = 'none';
			}
		}

		var btnTogglePass = E('button', {
			class: 'btn cbi-button',
			type: 'button',
			style: 'margin-left: 8px;',
			click: function() {
				if (inPassword.type === 'password') {
					inPassword.type = 'text';
					btnTogglePass.textContent = 'Скрыть ключ';
				} else {
					inPassword.type = 'password';
					btnTogglePass.textContent = 'Показать ключ';
				}
			}
		}, 'Показать ключ');

		var inDeviceId = E('input', {
			class: 'cbi-input-text',
			type: 'text',
			placeholder: 'router-01',
			value: config.device_id || 'router-01',
			change: function() { markDirty(); }
		});

		var inTunMtu = E('input', {
			class: 'cbi-input-text',
			type: 'number',
			min: '576',
			max: '1500',
			style: 'width: 110px;',
			value: config.tun_mtu || 1280,
			change: function() { markDirty(); }
		});

		// Protection & OpSec inputs
		var chkKillSwitch = E('input', {
			type: 'checkbox',
			checked: ((config.kill_switch !== undefined ? !!config.kill_switch : true)) ? 'checked' : null,
			change: function() { markDirty(); }
		});

		var opsecInitial = (config.normalize_ttl !== undefined ? !!config.normalize_ttl : true) &&
			(config.intercept_dns !== undefined ? !!config.intercept_dns : true);

		var chkSimpleOpSec = E('input', {
			type: 'checkbox',
			checked: (opsecInitial) ? 'checked' : null,
			change: function() {
				chkNormalizeTTL.checked = chkSimpleOpSec.checked;
				chkInterceptDNS.checked = chkSimpleOpSec.checked;
				chkBlockDoT.checked = chkSimpleOpSec.checked;
				markDirty();
			}
		});

		var chkNormalizeTTL = E('input', {
			type: 'checkbox',
			checked: ((config.normalize_ttl !== undefined ? !!config.normalize_ttl : true)) ? 'checked' : null,
			change: function() { markDirty(); }
		});

		var inTargetTTL = E('input', {
			class: 'cbi-input-text',
			type: 'number',
			min: '1',
			max: '255',
			style: 'width: 80px;',
			value: config.target_ttl || 64,
			change: function() { markDirty(); }
		});

		var chkInterceptDNS = E('input', {
			type: 'checkbox',
			checked: ((config.intercept_dns !== undefined ? !!config.intercept_dns : true)) ? 'checked' : null,
			change: function() { markDirty(); }
		});

		var chkBlockDoT = E('input', {
			type: 'checkbox',
			checked: ((config.block_dot !== undefined ? !!config.block_dot : true)) ? 'checked' : null,
			change: function() { markDirty(); }
		});

		var txtDeadDrop = E('textarea', {
			class: 'cbi-input-textarea',
			rows: 3,
			style: 'width: 100%; font-family: monospace;',
			placeholder: 'https://disk.yandex.ru/d/...\nhttps://bucket.storage.yandexcloud.net/params.json',
			change: function() { markDirty(); }
		}, (config.dead_drop_urls || []).join('\n'));

		// Anti-DPI (Zapret 2) inputs
		var chkADEnabled = E('input', {
			type: 'checkbox',
			checked: ((antiDPI.enabled !== undefined ? !!antiDPI.enabled : true)) ? 'checked' : null,
			change: function() { markDirty(); updatePresetHighlight(); }
		});
		var chkADYouTube = E('input', {
			type: 'checkbox',
			checked: ((antiDPI.youtube !== undefined ? !!antiDPI.youtube : true)) ? 'checked' : null,
			change: function() { markDirty(); }
		});
		var chkADInstagram = E('input', {
			type: 'checkbox',
			checked: ((antiDPI.instagram !== undefined ? !!antiDPI.instagram : true)) ? 'checked' : null,
			change: function() { markDirty(); }
		});
		var chkADDiscord = E('input', {
			type: 'checkbox',
			checked: ((antiDPI.discord !== undefined ? !!antiDPI.discord : true)) ? 'checked' : null,
			change: function() { markDirty(); }
		});
		var chkADQUICReject = E('input', {
			type: 'checkbox',
			checked: ((antiDPI.quic_reject !== undefined ? !!antiDPI.quic_reject : true)) ? 'checked' : null,
			change: function() { markDirty(); }
		});
		var inADSplitPos = E('input', {
			type: 'number',
			class: 'cbi-input-text',
			min: 1,
			max: 100,
			value: antiDPI.split_pos || 2,
			style: 'width: 90px;',
			change: function() { markDirty(); }
		});
		var inADPort = E('input', {
			type: 'number',
			class: 'cbi-input-text',
			min: 1024,
			max: 65535,
			value: antiDPI.port || 56100,
			style: 'width: 110px;',
			change: function() { markDirty(); }
		});
		var txtADCustomDomains = E('textarea', {
			class: 'cbi-input-textarea',
			rows: 3,
			style: 'width: 100%; font-family: monospace;',
			placeholder: 'hdrezka.ag\nexample.org',
			change: function() { markDirty(); }
		}, (antiDPI.custom_domains || []).join('\n'));

		// Force VPN (Geo-blocked services) inputs
		var fvServices = forceVPN.services || ['ai', 'spotify', 'autodesk', 'adobe', 'jetbrains', 'figma', 'notion', 'canva', 'nvidia'];
		function hasFvService(name) {
			for (var i = 0; i < fvServices.length; i++) {
				if (fvServices[i] === name) return true;
			}
			return false;
		}

		var chkFVEnabled = E('input', {
			type: 'checkbox',
			checked: ((forceVPN.enabled !== undefined ? !!forceVPN.enabled : true)) ? 'checked' : null,
			change: function() { markDirty(); }
		});
		var chkFVAI = E('input', {
			type: 'checkbox',
			checked: (hasFvService('ai')) ? 'checked' : null,
			change: function() { markDirty(); }
		});
		var chkFVSpotify = E('input', {
			type: 'checkbox',
			checked: (hasFvService('spotify')) ? 'checked' : null,
			change: function() { markDirty(); }
		});
		var chkFVWork = E('input', {
			type: 'checkbox',
			checked: (hasFvService('autodesk') || hasFvService('adobe') || hasFvService('jetbrains') || hasFvService('figma') || hasFvService('notion') || hasFvService('canva')) ? 'checked' : null,
			change: function() { markDirty(); }
		});
		var chkFVPlayStation = E('input', {
			type: 'checkbox',
			checked: (hasFvService('playstation')) ? 'checked' : null,
			change: function() { markDirty(); }
		});

		// PSN Proxy inputs
		var chkPSNEnabled = E('input', {
			type: 'checkbox',
			checked: psnProxy.enabled ? 'checked' : null,
			change: function() { markDirty(); }
		});
		var selPSNType = E('select', { class: 'cbi-input-select', change: function() { markDirty(); } }, [
			E('option', { value: 'socks5', selected: (psnProxy.type !== 'http') ? 'selected' : null }, 'SOCKS5'),
			E('option', { value: 'http', selected: (psnProxy.type === 'http') ? 'selected' : null }, 'HTTP(S) CONNECT')
		]);
		var inPSNHost = E('input', { type: 'text', class: 'cbi-input-text', style: 'width: 220px;', placeholder: '185.x.x.x', value: psnProxy.host || '', change: function() { markDirty(); } });
		var inPSNPort = E('input', { type: 'number', class: 'cbi-input-text', style: 'width: 100px;', min: 1, max: 65535, placeholder: '8000', value: psnProxy.port || '', change: function() { markDirty(); } });
		var inPSNUser = E('input', { type: 'text', class: 'cbi-input-text', style: 'width: 180px;', placeholder: 'логин', value: psnProxy.username || '', change: function() { markDirty(); } });
		var inPSNPass = E('input', { type: 'password', class: 'cbi-input-text', style: 'width: 180px;', placeholder: 'пароль', value: psnProxy.password || '', change: function() { markDirty(); } });
		var chkPSNViaVPN = E('input', {
			type: 'checkbox',
			checked: (psnProxy.via === 'vpn') ? 'checked' : null,
			change: function() { markDirty(); }
		});
		var chkPSNDownloads = E('input', {
			type: 'checkbox',
			checked: psnProxy.include_downloads ? 'checked' : null,
			change: function() { markDirty(); }
		});

		var txtFVDomains = E('textarea', {
			class: 'cbi-input-textarea',
			rows: 5,
			style: 'width: 100%; font-family: monospace;',
			placeholder: 'bbc.com\nmeduza.io\nlinkedin.com',
			change: function() { markDirty(); }
		}, (forceVPN.domains || []).join('\n'));

		// Split Tunneling inputs
		var splitMode = (splitConfig.mode || 'bypass').toLowerCase();
		var radBypass = E('input', {
			type: 'radio',
			name: 'split_mode',
			value: 'bypass',
			checked: (splitMode !== 'only') ? 'checked' : null,
			change: function() { markDirty(); updatePresetHighlight(); }
		});
		var radOnly = E('input', {
			type: 'radio',
			name: 'split_mode',
			value: 'only',
			checked: (splitMode === 'only') ? 'checked' : null,
			change: function() { markDirty(); updatePresetHighlight(); }
		});

		var chkRuDirect = E('input', {
			type: 'checkbox',
			checked: ((splitConfig.use_ru_direct !== undefined ? !!splitConfig.use_ru_direct : true)) ? 'checked' : null,
			change: function() { markDirty(); updatePresetHighlight(); }
		});

		var portMode = (splitConfig.port_mode || 'only').toLowerCase();
		var hasGamingPorts = portMode === 'bypass' && (splitConfig.ports || []).length > 0;
		var chkGamingBypass = E('input', {
			type: 'checkbox',
			checked: (hasGamingPorts) ? 'checked' : null,
			change: function() {
				if (chkGamingBypass.checked) {
					radPortBypass.checked = true;
					if (!txtPorts.value.trim()) {
						txtPorts.value = 'udp:27000-27100\nudp:3478-3480\nudp:3074\n6881-6889';
					}
				} else {
					radPortOnly.checked = true;
					txtPorts.value = '';
				}
				markDirty();
				updatePresetHighlight();
			}
		});

		var radPortOnly = E('input', {
			type: 'radio',
			name: 'split_port_mode',
			value: 'only',
			checked: (portMode !== 'bypass') ? 'checked' : null,
			change: function() { markDirty(); }
		});
		var radPortBypass = E('input', {
			type: 'radio',
			name: 'split_port_mode',
			value: 'bypass',
			checked: (portMode === 'bypass') ? 'checked' : null,
			change: function() { markDirty(); }
		});

		var txtDomains = E('textarea', {
			class: 'cbi-input-textarea',
			rows: 5,
			style: 'width: 100%; font-family: monospace;',
			placeholder: 'gosuslugi.ru\nmybank.ru\nkinopoisk.ru',
			change: function() { markDirty(); }
		}, (splitConfig.domains || []).join('\n'));

		var txtCIDRs = E('textarea', {
			class: 'cbi-input-textarea',
			rows: 4,
			style: 'width: 100%; font-family: monospace;',
			placeholder: '1.1.1.1/32\n104.16.0.0/12',
			change: function() { markDirty(); }
		}, (splitConfig.cidrs || []).join('\n'));

		var txtPorts = E('textarea', {
			class: 'cbi-input-textarea',
			rows: 3,
			style: 'width: 100%; font-family: monospace;',
			placeholder: 'udp:27000-27100\n6881-6889',
			change: function() { markDirty(); }
		}, (splitConfig.ports || []).join('\n'));

		// Whitelist Fallback inputs
		var chkWLEnabled = E('input', {
			type: 'checkbox',
			checked: ((wlConfig.enabled !== undefined ? !!wlConfig.enabled : true)) ? 'checked' : null,
			change: function() { markDirty(); updatePresetHighlight(); }
		});
		var wlMode = (wlConfig.mode || 'auto').toLowerCase();
		var radWLAuto = E('input', {
			type: 'radio',
			name: 'wl_mode',
			value: 'auto',
			checked: (wlMode !== 'always') ? 'checked' : null,
			change: function() { markDirty(); updatePresetHighlight(); }
		});
		var radWLAlways = E('input', {
			type: 'radio',
			name: 'wl_mode',
			value: 'always',
			checked: (wlMode === 'always') ? 'checked' : null,
			change: function() { markDirty(); updatePresetHighlight(); }
		});
		var inWLPort = E('input', {
			type: 'number',
			class: 'cbi-input-text',
			min: 1,
			max: 65535,
			value: wlConfig.gateway_port || 56005,
			style: 'width: 100px;',
			change: function() { markDirty(); }
		});
		var chkWLDTLS = E('input', {
			type: 'checkbox',
			checked: (!!wlConfig.use_dtls) ? 'checked' : null,
			change: function() { markDirty(); }
		});
		var inWLTurnBackup = E('input', {
			type: 'text',
			class: 'cbi-input-text',
			value: wlConfig.turn_backup || '193.203.43.1:19302',
			style: 'width: 240px;',
			change: function() { markDirty(); }
		});
		var inWLTurnRealm = E('input', {
			type: 'text',
			class: 'cbi-input-text',
			value: wlConfig.turn_realm || 'vkcalls',
			style: 'width: 140px;',
			change: function() { markDirty(); }
		});
		var inWLRelayTarget = E('input', {
			type: 'text',
			class: 'cbi-input-text',
			value: wlConfig.relay_target || '',
			placeholder: 'Авто',
			style: 'width: 180px;',
			change: function() { markDirty(); }
		});
		var txtWLVKHashes = E('textarea', {
			class: 'cbi-input-textarea',
			rows: 3,
			style: 'width: 100%; font-family: monospace;',
			placeholder: 'https://vk.com/call/join/...',
			change: function() { markDirty(); }
		}, (wlConfig.vk_hashes || []).join('\n'));
		var txtWLDionSlugs = E('textarea', {
			class: 'cbi-input-textarea',
			rows: 3,
			style: 'width: 100%; font-family: monospace;',
			placeholder: 'https://dion.vc/event/...',
			change: function() { markDirty(); }
		}, (wlConfig.dion_slugs || []).join('\n'));

		// --- SCENARIO PRESETS (TAB 1) ---
		var presetCards = {};
		function detectActivePreset() {
			if (chkWLEnabled.checked && radWLAlways.checked) return 'emergency';
			if (!chkRuDirect.checked && !chkADEnabled.checked && radBypass.checked) return 'privacy';
			if (chkRuDirect.checked && chkADEnabled.checked && chkGamingBypass.checked) return 'gaming';
			if (chkRuDirect.checked && chkADEnabled.checked) return 'smart';
			return 'custom';
		}

		function applyPreset(id) {
			if (id === 'smart') {
				radBypass.checked = true;
				chkRuDirect.checked = true;
				chkGamingBypass.checked = false;
				radPortOnly.checked = true;
				txtPorts.value = '';
				chkADEnabled.checked = true;
				chkADYouTube.checked = true;
				chkADInstagram.checked = true;
				chkADDiscord.checked = true;
				chkFVEnabled.checked = true;
				chkFVAI.checked = true;
				chkFVSpotify.checked = true;
				chkFVWork.checked = true;
				chkFVPlayStation.checked = true;
				chkWLEnabled.checked = true;
				radWLAuto.checked = true;
				markDirty('Выбран профиль «Умный режим (Рекомендуется)» — нажмите «Сохранить и применить»');
			} else if (id === 'gaming') {
				radBypass.checked = true;
				chkRuDirect.checked = true;
				chkGamingBypass.checked = true;
				radPortBypass.checked = true;
				txtPorts.value = 'udp:27000-27100\nudp:3478-3480\nudp:3074\n6881-6889';
				chkADEnabled.checked = true;
				chkADYouTube.checked = true;
				chkADInstagram.checked = true;
				chkADDiscord.checked = true;
				chkFVEnabled.checked = true;
				chkFVAI.checked = true;
				chkFVPlayStation.checked = true;
				chkWLEnabled.checked = true;
				radWLAuto.checked = true;
				markDirty('Выбран профиль «Игры и Стриминг без задержек» — нажмите «Сохранить и применить»');
			} else if (id === 'privacy') {
				radBypass.checked = true;
				chkRuDirect.checked = false;
				chkGamingBypass.checked = false;
				radPortOnly.checked = true;
				txtPorts.value = '';
				chkADEnabled.checked = false;
				chkKillSwitch.checked = true;
				chkSimpleOpSec.checked = true;
				chkNormalizeTTL.checked = true;
				chkInterceptDNS.checked = true;
				chkBlockDoT.checked = true;
				radWLAuto.checked = true;
				markDirty('Выбран профиль «Максимальная приватность (Весь трафик в VPN)» — нажмите «Сохранить и применить»');
			} else if (id === 'emergency') {
				chkWLEnabled.checked = true;
				radWLAlways.checked = true;
				chkRuDirect.checked = true;
				markDirty('Выбран профиль «Экстренная связь (Белые списки VK / DION)» — нажмите «Сохранить и применить»');
			}
			updatePresetHighlight();
		}

		function updatePresetHighlight() {
			var active = detectActivePreset();
			var ids = ['smart', 'gaming', 'privacy', 'emergency'];
			for (var i = 0; i < ids.length; i++) {
				var k = ids[i];
				var card = presetCards[k];
				if (!card) continue;
				if (k === active) {
					card.style.border = '2px solid #0069d9';
					card.style.background = '#f0f7ff';
					card.querySelector('.preset-badge').style.display = 'inline-block';
				} else {
					card.style.border = '1px solid #d8dee9';
					card.style.background = '#ffffff';
					card.querySelector('.preset-badge').style.display = 'none';
				}
			}
		}

		function buildPresetCard(id, icon, title, subtitle, bullets) {
			var badge = E('span', {
				class: 'preset-badge label success',
				style: 'display: none; margin-left: 8px; font-size: 11px; padding: 2px 8px; border-radius: 10px;'
			}, '✓ Выбран');

			var ul = E('ul', { style: 'margin: 8px 0 0 18px; padding: 0; font-size: 12px; color: #495057;' });
			for (var i = 0; i < bullets.length; i++) {
				ul.appendChild(E('li', { style: 'margin-bottom: 3px;' }, bullets[i]));
			}

			var card = E('div', {
				style: 'border: 1px solid #d8dee9; border-radius: 8px; padding: 14px 16px; cursor: pointer; background: #fff; transition: all 0.15s ease;',
				click: function() { applyPreset(id); }
			}, [
				E('div', { style: 'display: flex; align-items: center; justify-content: space-between;' }, [
					E('strong', { style: 'font-size: 15px; color: #212529;' }, icon + ' ' + title),
					badge
				]),
				E('div', { style: 'font-size: 12px; color: #6c757d; margin-top: 4px;' }, subtitle),
				ul
			]);
			presetCards[id] = card;
			return card;
		}

		var presetsGrid = E('div', {
			style: 'display: grid; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr)); gap: 12px; margin-top: 10px;'
		}, [
			buildPresetCard(
				'smart',
				'🌟',
				'Умный режим (Рекомендуется)',
				'Идеальный баланс скорости и защиты на каждый день',
				[
					'Банки РФ, Госуслуги и маркетплейсы открываются напрямую',
					'YouTube (4K), Instagram и Discord работают через Zapret 2 (0 Байт на VPS)',
					'ChatGPT, Claude, Gemini и Spotify автоматически идут через VPN'
				]
			),
			buildPresetCard(
				'gaming',
				'🎮',
				'Игры и Стриминг без задержек',
				'Минимальный пинг в онлайн-играх и высокая скорость загрузки',
				[
					'Все преимущества «Умного режима» для YouTube, Discord и ИИ',
					'Игровые порты (Steam, CS2, Dota 2, PSN, Xbox) и торренты идут напрямую мимо VPN'
				]
			),
			buildPresetCard(
				'privacy',
				'🔒',
				'Максимальная приватность',
				'Полное шифрование всего трафика домашней сети',
				[
					'Все сайты и приложения направляются через зарубежный VPN-сервер',
					'Включена строгая блокировка утечек при обрыве (Kill Switch)'
				]
			),
			buildPresetCard(
				'emergency',
				'🚨',
				'Экстренная связь (Белые списки)',
				'Для работы при блокировке зарубежного интернета и шатдаунах',
				[
					'Трафик маскируется под видеозвонки ВКонтакте (VK Звонки) и DION',
					'Проходит через разрешённые российские серверы из белого списка'
				]
			)
		]);
		updatePresetHighlight();

		// --- DIAGNOSTICS & LOGS (TAB 4) ---
		var diagTrafficLights = E('div', { style: 'margin-top: 14px; display: none;' });
		var inlineLogPre = E('pre', {
			style: 'max-height: 360px; overflow-y: auto; background: #171c26; color: #dbe2ef; padding: 12px; font-family: monospace; font-size: 12px; border-radius: 6px; white-space: pre-wrap; word-break: break-all; margin-top: 10px;'
		}, 'Нажмите «Обновить журнал», чтобы загрузить последние записи...');

		function refreshInlineLogs() {
			return callGetLogs(120).then(function(res) {
				var lines = (res && res.lines && res.lines.length) ? res.lines.join('\n') : 'Системный журнал пуст';
				inlineLogPre.textContent = lines;
			}).catch(function(e) {
				inlineLogPre.textContent = 'Ошибка чтения журнала: ' + (e.message || 'RPC error');
			});
		}

		var btnTestConn = E('button', {
			class: 'btn cbi-button cbi-button-action',
			style: 'margin-right: 8px;',
			click: function() {
				btnTestConn.disabled = true;
				diagTrafficLights.style.display = 'block';
				diagTrafficLights.innerHTML = '<div style="padding: 12px; background: #f8f9fa; border-radius: 6px;">⏳ <em>Проверяем маршруты до российских сайтов, Zapret 2 и зарубежного VPN...</em></div>';

				return callTestConnection().then(function(res) {
					btnTestConn.disabled = false;
					res = res || {};
					var ruOk = !res.rudirect_via_tunnel;
					var zapEnabled = res.zapret_enabled !== false;
					var zapOk = (res.zapret_ok !== undefined) ? !!res.zapret_ok : zapEnabled;
					var vpnOk = !!res.ip_ok;

					function makeRow(dot, title, statusText, desc) {
						return E('div', {
							style: 'display: flex; align-items: flex-start; padding: 10px 12px; border: 1px solid #e2e8f0; border-radius: 6px; margin-bottom: 8px; background: #fff;'
						}, [
							E('span', { style: 'font-size: 18px; margin-right: 12px; line-height: 1.2;' }, dot),
							E('div', {}, [
								E('div', { style: 'font-weight: bold; font-size: 14px; color: #212529;' }, title + ' — ' + statusText),
								E('div', { style: 'font-size: 12px; color: #6c757d; margin-top: 2px;' }, desc)
							])
						]);
					}

					diagTrafficLights.innerHTML = '';
					diagTrafficLights.appendChild(makeRow(
						ruOk ? '🟢' : '🔵',
						'1. Российские сайты и банки (Яндекс, Госуслуги)',
						ruOk ? 'Работают напрямую через провайдера' : 'Идут через защищённый VPN',
						res.ru_status || res.raw || 'Маршрут проверен по ядерной таблице маршрутизации (77.88.8.8)'
					));
					diagTrafficLights.appendChild(makeRow(
						!zapEnabled ? '⚪' : (zapOk ? '🟢' : '🔴'),
						'2. YouTube, Instagram, Discord и X (Движок Zapret 2)',
						!zapEnabled ? 'Прямой обход отключён' : (zapOk ? 'Доступ подтверждён (DNS + HTTPS OK)' : 'Обнаружена проблема доступа!'),
						res.zapret_status || 'Адаптивная десинхронизация пакетов ТСПУ РКН (4 стратегии)'
					));
					diagTrafficLights.appendChild(makeRow(
						vpnOk ? '🟢' : '🟡',
						'3. Зарубежные сервисы и ИИ (ChatGPT, Claude, Gemini, Spotify)',
						vpnOk ? ('Защищённый канал доступен (Внешний IP: ' + res.public_ip + ')') : 'Спящий режим / Ожидание трафика',
						'Туннель поднимается автоматически за миллисекунды при обращении к зарубежным ИИ-сервисам'
					));
				}).catch(function(e) {
					btnTestConn.disabled = false;
					diagTrafficLights.innerHTML = '<p class="error">Ошибка проверки: ' + (e.message || 'RPC timeout') + '</p>';
				});
			}
		}, '🔍 Проверить работу всех направлений');

		var btnCopySupportReport = E('button', {
			class: 'btn cbi-button',
			style: 'margin-right: 8px;',
			click: function() {
				return callGetLogs(40).then(function(res) {
					var lines = (res && res.lines) ? res.lines.join('\n') : '';
					var report = [
						'=== Отчёт диагностики Sirin VPN Router ===',
						'Версия: ' + (currentStatus.version || '1.8.0'),
						'Статус: ' + (currentStatus.running ? 'Работает (PID ' + currentStatus.pid + ')' : 'Остановлен'),
						'Транспорт: ' + (currentStatus.transport || '—'),
						'Профиль: ' + detectActivePreset(),
						'--- Последние записи журнала ---',
						lines
					].join('\n');

					if (navigator.clipboard && navigator.clipboard.writeText) {
						navigator.clipboard.writeText(report).then(function() {
							ui.addNotification(null, E('p', {}, '✓ Отчёт для техподдержки скопирован в буфер обмена (ваш секретный ключ скрыт)'), 'info');
						});
					} else {
						ui.showModal('Отчёт для техподдержки (скопируйте текст)', [
							E('textarea', { style: 'width:100%; height:260px; font-family:monospace; font-size:12px;' }, report),
							E('div', { class: 'right', style: 'margin-top:10px;' }, [
								E('button', { class: 'btn cbi-button', click: ui.hideModal }, 'Закрыть')
							])
						]);
					}
				});
			}
		}, '📋 Скопировать отчёт для поддержки');

		var btnRefreshLogs = E('button', {
			class: 'btn cbi-button cbi-button-neutral',
			click: function() { refreshInlineLogs(); }
		}, '↻ Обновить журнал');

		// --- UNIFIED SAVE & APPLY HANDLER ---
		function parseLinesList(rawText, regex, errPrefix) {
			var lines = (rawText || '').split('\n');
			var out = [];
			for (var i = 0; i < lines.length; i++) {
				var v = lines[i].trim();
				if (!v) continue;
				if (regex && !regex.test(v)) {
					throw new Error(errPrefix + ': ' + v);
				}
				out.push(v);
			}
			return out;
		}

		var btnUnifiedSave = E('button', {
			class: 'btn cbi-button cbi-button-apply',
			style: 'font-weight: bold; padding: 6px 18px;',
			click: function() {
				try {
					var gw = inGateway.value.trim() || 'crp.guseynovia.ru';
					var pw = inPassword.value.trim();
					var did = inDeviceId.value.trim() || 'router-01';
					var mtu = parseInt(inTunMtu.value, 10) || 1280;
					var ks = chkKillSwitch.checked;
					var normTTL = chkNormalizeTTL.checked;
					var targTTL = parseInt(inTargetTTL.value, 10) || 64;
					var intDNS = chkInterceptDNS.checked;
					var blkDoT = chkBlockDoT.checked;

					if (!pw) {
						ui.addNotification(null, E('p', {}, 'Укажите ваш Ключ доступа (UUID из бота @sirin_crp_bot) на вкладке «Главная»'), 'error');
						return;
					}

					var validDD = [];
					var rawDD = txtDeadDrop.value.split('\n');
					for (var d = 0; d < rawDD.length; d++) {
						var item = rawDD[d].trim();
						if (item && (item.indexOf('http://') === 0 || item.indexOf('https://') === 0)) {
							validDD.push(item);
						}
					}

					var domainRegex = /^([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$/;
					var cidrRegex = /^[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}(\/[0-9]{1,2})?$/;
					var portRegex = /^(tcp:|udp:)?[0-9]{1,5}(-[0-9]{1,5})?$/i;

					var sMode = radOnly.checked ? 'only' : 'bypass';
					var useRu = chkRuDirect.checked;
					var validCidrs = parseLinesList(txtCIDRs.value, cidrRegex, 'Некорректный формат IP/CIDR');
					var validSplitDomains = parseLinesList(txtDomains.value.toLowerCase(), domainRegex, 'Некорректный домен в списке прямого доступа');
					var validPorts = parseLinesList(txtPorts.value, portRegex, 'Некорректный формат порта');
					var sPortMode = radPortBypass.checked ? 'bypass' : 'only';

					var adEnabled = chkADEnabled.checked;
					var adPort = parseInt(inADPort.value, 10) || 56100;
					var adSplitPos = parseInt(inADSplitPos.value, 10) || 2;
					var adQuic = chkADQUICReject.checked;
					var adYT = chkADYouTube.checked;
					var adIG = chkADInstagram.checked;
					var adDC = chkADDiscord.checked;
					var validADDomains = parseLinesList(txtADCustomDomains.value.toLowerCase(), domainRegex, 'Некорректный домен в списке Zapret 2');

					var fvEnabled = chkFVEnabled.checked;
					var selectedFvServices = [];
					if (chkFVAI.checked) selectedFvServices.push('ai');
					if (chkFVSpotify.checked) selectedFvServices.push('spotify');
					if (chkFVWork.checked) {
						selectedFvServices.push('autodesk', 'adobe', 'jetbrains', 'figma', 'notion', 'canva', 'nvidia');
					}
					if (chkFVPlayStation.checked) selectedFvServices.push('playstation');
					var validFVDomains = parseLinesList(txtFVDomains.value.toLowerCase(), domainRegex, 'Некорректный домен в списке «Всегда через VPN»');

					var wlEn = chkWLEnabled.checked;
					var wlMd = radWLAlways.checked ? 'always' : 'auto';
					var wlPrt = parseInt(inWLPort.value, 10) || 56005;
					var wlDtls = chkWLDTLS.checked;
					var wlTurn = inWLTurnBackup.value.trim() || '193.203.43.1:19302';
					var wlRealm = inWLTurnRealm.value.trim() || 'vkcalls';
					var wlTarget = inWLRelayTarget.value.trim();
					var validVK = parseLinesList(txtWLVKHashes.value, null, '');
					var validDion = parseLinesList(txtWLDionSlugs.value, null, '');

					btnUnifiedSave.disabled = true;
					btnUnifiedSave.textContent = '⏳ Сохранение и перезапуск...';

					return callSetConfig(gw, pw, did, useRu, mtu, ks, normTTL, targTTL, intDNS, blkDoT, validDD)
						.then(function() {
							return callSetSplitConfig(sMode, useRu, validCidrs, validSplitDomains, validPorts, sPortMode);
						})
						.then(function() {
							return callSetAntiDPIConfig(adEnabled, adPort, adSplitPos, adQuic, adYT, adIG, adDC, validADDomains);
						})
						.then(function() {
							return callSetForceVPNConfig(fvEnabled, selectedFvServices, validFVDomains, []);
						})
						.then(function() {
							return callSetWhitelistConfig(wlEn, wlMd, wlPrt, wlDtls, wlTurn, wlRealm, wlTarget, validVK, validDion);
						})
						.then(function() {
							return callSetPSNProxyConfig(
								chkPSNEnabled.checked,
								selPSNType.value,
								inPSNHost.value.trim(),
								parseInt(inPSNPort.value, 10) || 0,
								inPSNUser.value.trim(),
								inPSNPass.value,
								chkPSNViaVPN.checked ? 'vpn' : 'wan',
								chkPSNDownloads.checked
							).then(function(res) {
								if (res && res.ok === false) throw new Error(res.error || 'PSN-прокси: ошибка сохранения');
							});
						})
						.then(function() {
							return callRestart();
						})
						.then(function() {
							btnUnifiedSave.disabled = false;
							btnUnifiedSave.textContent = '💾 Сохранить и применить настройки';
							hasUnsavedChanges = false;
							saveBanner.style.background = '#14532d';
							saveBanner.style.borderColor = '#28a745';
							saveBannerText.textContent = '✓ Настройки сохранены и применены! Служба перезапущена.';
							ui.addNotification(null, E('p', {}, '✓ Настройки сохранены, служба Sirin VPN перезапущена!'), 'info');
							updateStatusView();
						})
						.catch(function(e) {
							btnUnifiedSave.disabled = false;
							btnUnifiedSave.textContent = '💾 Сохранить и применить настройки';
							ui.addNotification(null, E('p', {}, 'Ошибка сохранения: ' + (e.message || 'RPC error')), 'error');
						});
				} catch (validationErr) {
					ui.addNotification(null, E('p', {}, validationErr.message), 'error');
				}
			}
		}, '💾 Сохранить и применить настройки');

		saveBanner.appendChild(saveBannerText);
		saveBanner.appendChild(btnUnifiedSave);

		// --- BUILD 4 TABS ---
		var tab1Content = E('div', {}, [
			E('div', { class: 'cbi-section' }, [
				E('h3', {}, '🔑 Ваш ключ доступа к Sirin VPN'),
				E('p', { class: 'cbi-section-descr' }, 'Вставьте персональный ключ (UUID), полученный в Telegram-боте @sirin_crp_bot.'),
				E('div', { class: 'cbi-value' }, [
					E('label', { class: 'cbi-value-title' }, 'Ключ доступа (UUID)'),
					E('div', { class: 'cbi-value-field' }, [
						E('div', { style: 'display: flex; align-items: center; flex-wrap: wrap;' }, [
							inPassword,
							btnTogglePass
						]),
						passWarnNode
					])
				]),
				registerExpertBlock(E('div', { style: 'border-top: 1px dashed #dee2e6; margin-top: 12px; padding-top: 12px;' }, [
					E('div', { class: 'cbi-value' }, [
						E('label', { class: 'cbi-value-title' }, 'Адрес управляющего шлюза (Gateway)'),
						E('div', { class: 'cbi-value-field' }, [
							inGateway,
							E('div', { class: 'cbi-value-description' }, 'Оставьте crp.guseynovia.ru (меняется только при использовании собственного сервера)')
						])
					]),
					E('div', { class: 'cbi-value' }, [
						E('label', { class: 'cbi-value-title' }, 'Имя этого роутера (Device ID)'),
						E('div', { class: 'cbi-value-field' }, [inDeviceId])
					])
				]))
			]),

			E('div', { class: 'cbi-section' }, [
				E('h3', {}, '⚡ Выберите готовый сценарий работы (в 1 клик)'),
				E('p', { class: 'cbi-section-descr' }, 'Выберите подходящий режим — роутер сам настроит все правила для сайтов, игр, YouTube и нейросетей. После выбора нажмите «Сохранить и применить» внизу экрана.'),
				presetsGrid
			])
		]);

		var tab2Content = E('div', {}, [
			E('div', { class: 'cbi-section' }, [
				E('h3', {}, '⚡ Ускорение без нагрузки на VPN (Технология Zapret 2)'),
				E('p', { class: 'cbi-section-descr' }, 'Разблокирует YouTube в 4K/8K, Instagram и голосовые чаты Discord напрямую через вашего провайдера без расхода трафика зарубежного сервера. Если провайдер жестко заблокирует узел по IP — роутер сам бесшовно переключит его на VPN.'),
				E('div', { class: 'cbi-value' }, [
					E('label', { class: 'cbi-value-title' }, 'Прямой обход замедления'),
					E('div', { class: 'cbi-value-field' }, [
						E('label', { style: 'cursor: pointer;' }, [
							chkADEnabled,
							E('span', { style: 'margin-left: 8px; font-weight: bold;' }, 'Включить ускорение Zapret 2 без VPN (рекомендуется)')
						])
					])
				]),
				E('div', { class: 'cbi-value' }, [
					E('label', { class: 'cbi-value-title' }, 'Какие сервисы ускорять напрямую'),
					E('div', { class: 'cbi-value-field' }, [
						E('label', { style: 'cursor: pointer; display: block; margin-bottom: 6px;' }, [
							chkADYouTube,
							E('span', { style: 'margin-left: 8px;' }, '📺 YouTube и Google Видео (быстрая загрузка видео в 4K без буферизации)')
						]),
						E('label', { style: 'cursor: pointer; display: block; margin-bottom: 6px;' }, [
							chkADInstagram,
							E('span', { style: 'margin-left: 8px;' }, '📸 Instagram, Facebook (Meta) и X / Twitter')
						]),
						E('label', { style: 'cursor: pointer; display: block;' }, [
							chkADDiscord,
							E('span', { style: 'margin-left: 8px;' }, '🎧 Discord (голосовые комнаты без задержек, стримы и чат)')
						])
					])
				]),
				registerExpertBlock(E('div', { style: 'border-top: 1px dashed #dee2e6; margin-top: 12px; padding-top: 12px;' }, [
					E('div', { class: 'cbi-value' }, [
						E('label', { class: 'cbi-value-title' }, 'Подавление QUIC (UDP 443)'),
						E('div', { class: 'cbi-value-field' }, [
							chkADQUICReject,
							E('span', { style: 'margin-left: 8px;' }, 'Переводить браузеры с UDP QUIC на TCP TLS 1.3 для корректного разрезания SNI')
						])
					]),
					E('div', { class: 'cbi-value' }, [
						E('label', { class: 'cbi-value-title' }, 'Позиция разреза пакета / Порт прокси'),
						E('div', { class: 'cbi-value-field' }, [
							E('span', {}, 'Байт сплита: '), inADSplitPos,
							E('span', { style: 'margin-left: 16px;' }, 'Локальный порт: '), inADPort
						])
					]),
					E('div', { class: 'cbi-value' }, [
						E('label', { class: 'cbi-value-title' }, 'Дополнительные сайты для Zapret 2'),
						E('div', { class: 'cbi-value-field' }, [
							txtADCustomDomains,
							E('div', { class: 'cbi-value-description' }, 'Сайты, которые нужно пробивать напрямую через Zapret 2 без VPN (по одному домену на строку)')
						])
					])
				]))
			]),

			E('div', { class: 'cbi-section' }, [
				E('h3', {}, '🤖 Зарубежные сервисы, не пускающие пользователей из РФ (Авто-VPN)'),
				E('p', { class: 'cbi-section-descr' }, 'Эти сервисы сами блокируют вход с российских IP-адресов. Роутер автоматически направляет их через зарубежный VPN-сервер.'),
				E('div', { class: 'cbi-value' }, [
					E('label', { class: 'cbi-value-title' }, 'Обход геоблокировок'),
					E('div', { class: 'cbi-value-field' }, [
						E('label', { style: 'cursor: pointer; display: block; margin-bottom: 8px;' }, [
							chkFVEnabled,
							E('span', { style: 'margin-left: 8px; font-weight: bold;' }, 'Автоматически включать VPN для зарубежных ИИ и сервисов')
						]),
						E('label', { style: 'cursor: pointer; display: block; margin-bottom: 6px;' }, [
							chkFVAI,
							E('span', { style: 'margin-left: 8px;' }, '🧠 Нейросети и ИИ (ChatGPT / OpenAI, Claude, Google Gemini, Grok, Copilot, Perplexity, Cursor)')
						]),
						E('label', { style: 'cursor: pointer; display: block; margin-bottom: 6px;' }, [
							chkFVSpotify,
							E('span', { style: 'margin-left: 8px;' }, '🎵 Музыка и Подкасты (Spotify)')
						]),
						E('label', { style: 'cursor: pointer; display: block; margin-bottom: 6px;' }, [
							chkFVWork,
							E('span', { style: 'margin-left: 8px;' }, '📐 Рабочий и инженерный софт (Autodesk / AutoCAD, Adobe, JetBrains, Figma, Notion, Canva, NVIDIA)')
						]),
						E('label', { style: 'cursor: pointer; display: block;' }, [
							chkFVPlayStation,
							E('span', { style: 'margin-left: 8px;' }, '🎮 PlayStation Store и PSN (PS4 / PS5, оплата подписок, аккаунт Sony)')
						])
					])
				])
			]),

			E('div', { class: 'cbi-section' }, [
				E('h3', {}, '🎮 PlayStation через внешний прокси (PSN Proxy)'),
				E('p', { class: 'cbi-section-descr' }, 'Весь трафик PS Store, PSN и аккаунта Sony со всех устройств сети (PS5, PS App на телефоне) идёт через указанный SOCKS5/HTTP-прокси, например турецкий. Имеет приоритет над «Авто-VPN». Загрузки игр по умолчанию идут мимо прокси. Результат проверки прокси (внешний IP и страна) — в журнале на вкладке диагностики.'),
				E('div', { class: 'cbi-value' }, [
					E('label', { class: 'cbi-value-title' }, 'Включить'),
					E('div', { class: 'cbi-value-field' }, [
						E('label', { style: 'cursor: pointer;' }, [
							chkPSNEnabled,
							E('span', { style: 'margin-left: 8px; font-weight: bold;' }, 'Пускать PlayStation через прокси')
						])
					])
				]),
				E('div', { class: 'cbi-value' }, [
					E('label', { class: 'cbi-value-title' }, 'Прокси'),
					E('div', { class: 'cbi-value-field' }, [
						selPSNType, ' ', inPSNHost, E('span', {}, ' : '), inPSNPort
					])
				]),
				E('div', { class: 'cbi-value' }, [
					E('label', { class: 'cbi-value-title' }, 'Логин / пароль'),
					E('div', { class: 'cbi-value-field' }, [
						inPSNUser, ' ', inPSNPass,
						E('div', { class: 'cbi-value-description' }, 'Оставьте пустыми, если прокси авторизует по IP.')
					])
				]),
				E('div', { class: 'cbi-value' }, [
					E('label', { class: 'cbi-value-title' }, 'Дополнительно'),
					E('div', { class: 'cbi-value-field' }, [
						E('label', { style: 'cursor: pointer; display: block; margin-bottom: 6px;' }, [
							chkPSNViaVPN,
							E('span', { style: 'margin-left: 8px;' }, 'Подключаться к прокси через VPN-туннель (если провайдер блокирует адрес прокси)')
						]),
						E('label', { style: 'cursor: pointer; display: block;' }, [
							chkPSNDownloads,
							E('span', { style: 'margin-left: 8px;' }, 'Пускать через прокси и загрузки игр/обновлений (много трафика)')
						])
					])
				])
			]),

			E('div', { class: 'cbi-section' }, [
				E('h3', {}, '🇷🇺 Российские сайты, Банки и Игры'),
				E('div', { class: 'cbi-value' }, [
					E('label', { class: 'cbi-value-title' }, 'Российские сайты напрямую'),
					E('div', { class: 'cbi-value-field' }, [
						E('label', { style: 'cursor: pointer;' }, [
							chkRuDirect,
							E('span', { style: 'margin-left: 8px; font-weight: bold;' }, 'Открывать российские сайты, банки и Госуслуги напрямую без VPN (' + ruCount + ' подсетей РФ)')
						]),
						E('div', { class: 'cbi-value-description' }, 'Сбербанк, Т-Банк, Госуслуги, Кинопоиск, Ozon и Wildberries работают на максимальной скорости провайдера без капчи.')
					])
				]),
				E('div', { class: 'cbi-value' }, [
					E('label', { class: 'cbi-value-title' }, 'Онлайн-игры и торренты'),
					E('div', { class: 'cbi-value-field' }, [
						E('label', { style: 'cursor: pointer;' }, [
							chkGamingBypass,
							E('span', { style: 'margin-left: 8px; font-weight: bold;' }, 'Пускать игровой трафик и торренты напрямую мимо VPN (минимальный пинг)')
						])
					])
				])
			]),

			E('div', { class: 'cbi-section' }, [
				E('h3', {}, '➕ Мои списки сайтов'),
				E('p', { class: 'cbi-section-descr' }, 'Если какого-то сайта нет в стандартных наборах, просто добавьте его адрес (например, example.com) в нужную колонку — по одному сайту на строку.'),
				E('div', { style: 'display: grid; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr)); gap: 16px;' }, [
					E('div', { style: 'background: #f8f9fa; padding: 12px; border-radius: 6px; border: 1px solid #e2e8f0;' }, [
						E('strong', { style: 'display: block; margin-bottom: 4px; color: #0069d9;' }, '🌐 Всегда открывать ЧЕРЕЗ VPN'),
						E('div', { style: 'font-size: 12px; color: #6c757d; margin-bottom: 8px;' }, 'Сайты, которые заблокированы или не открываются без зарубежного IP:'),
						txtFVDomains
					]),
					E('div', { style: 'background: #f8f9fa; padding: 12px; border-radius: 6px; border: 1px solid #e2e8f0;' }, [
						E('strong', { style: 'display: block; margin-bottom: 4px; color: #28a745;' }, '🏠 Всегда открывать НАПРЯМУЮ (без VPN)'),
						E('div', { style: 'font-size: 12px; color: #6c757d; margin-bottom: 8px;' }, 'Сайты, которые должны видеть ваш домашний российский IP-адрес:'),
						txtDomains
					])
				]),
				registerExpertBlock(E('div', { style: 'border-top: 1px dashed #dee2e6; margin-top: 14px; padding-top: 12px;' }, [
					E('div', { class: 'cbi-value' }, [
						E('label', { class: 'cbi-value-title' }, 'Режим таблицы Split Tunneling'),
						E('div', { class: 'cbi-value-field' }, [
							E('label', { style: 'margin-right: 16px;' }, [radBypass, ' Bypass (Исключать список из VPN)']),
							E('label', {}, [radOnly, ' Only (Туннелировать ТОЛЬКО список)'])
						])
					]),
					E('div', { class: 'cbi-value' }, [
						E('label', { class: 'cbi-value-title' }, 'Пользовательские IP / CIDR'),
						E('div', { class: 'cbi-value-field' }, [txtCIDRs])
					]),
					E('div', { class: 'cbi-value' }, [
						E('label', { class: 'cbi-value-title' }, 'Фильтр портов (Split Ports)'),
						E('div', { class: 'cbi-value-field' }, [
							E('div', { style: 'margin-bottom: 6px;' }, [
								E('label', { style: 'margin-right: 14px;' }, [radPortOnly, ' Only (Только эти порты в VPN)']),
								E('label', {}, [radPortBypass, ' Bypass (Исключить эти порты из VPN)'])
							]),
							txtPorts
						])
					])
				]))
			])
		]);

		var tab3Content = E('div', {}, [
			E('div', { class: 'cbi-section' }, [
				E('h3', {}, '🛡️ Автоматическая защита домашней сети'),
				E('p', { class: 'cbi-section-descr' }, 'Рекомендуется оставить все три переключателя включёнными для надёжной и незаметной работы роутера.'),
				E('div', { class: 'cbi-value' }, [
					E('label', { class: 'cbi-value-title' }, '1. Защита при обрыве (Kill Switch)'),
					E('div', { class: 'cbi-value-field' }, [
						E('label', { style: 'cursor: pointer;' }, [
							chkKillSwitch,
							E('span', { style: 'margin-left: 8px; font-weight: bold;' }, 'Не выпускать трафик в открытую сеть при случайном обрыве VPN')
						]),
						E('div', { class: 'cbi-value-description' }, 'Предотвращает утечку вашего реального IP-адреса на зарубежных сайтах при перезапуске соединения.')
					])
				]),
				E('div', { class: 'cbi-value' }, [
					E('label', { class: 'cbi-value-title' }, '2. Скрытие роутера и защита DNS'),
					E('div', { class: 'cbi-value-field' }, [
						E('label', { style: 'cursor: pointer;' }, [
							chkSimpleOpSec,
							E('span', { style: 'margin-left: 8px; font-weight: bold;' }, 'Защищать от подмены сайтов провайдером и скрывать работу VPN (OpSec)')
						]),
						E('div', { class: 'cbi-value-description' }, 'Автоматически выравнивает сетевой TTL пакетов и защищает DNS-запросы смартфонов и ПК от перехвата провайдером.')
					])
				]),
				E('div', { class: 'cbi-value' }, [
					E('label', { class: 'cbi-value-title' }, '3. Обход «Белых списков» и резервная связь'),
					E('div', { class: 'cbi-value-field' }, [
						E('label', { style: 'cursor: pointer;' }, [
							chkWLEnabled,
							E('span', { style: 'margin-left: 8px; font-weight: bold;' }, 'Автоматически переходить на резервный канал через звонки ВКонтакте / DION при шатдаунах')
						]),
						E('div', { class: 'cbi-value-description' }, 'Если провайдер заблокирует все зарубежные IP-адреса, роутер сам установит связь через разрешённые российские серверы.')
					])
				]),

				registerExpertBlock(E('div', { style: 'border-top: 1px dashed #dee2e6; margin-top: 14px; padding-top: 12px;' }, [
					E('h4', { style: 'margin: 0 0 10px 0;' }, 'Тонкие сетевые параметры (Экспертный режим)'),
					E('div', { class: 'cbi-value' }, [
						E('label', { class: 'cbi-value-title' }, 'Размер пакета TUN MTU'),
						E('div', { class: 'cbi-value-field' }, [
							inTunMtu,
							E('span', { style: 'color:#888; margin-left:8px;' }, '(576–1500, оптимально 1280)')
						])
					]),
					E('div', { class: 'cbi-value' }, [
						E('label', { class: 'cbi-value-title' }, 'Детальные опции OpSec'),
						E('div', { class: 'cbi-value-field' }, [
							E('label', { style: 'display:block; margin-bottom:4px;' }, [chkNormalizeTTL, ' Фиксировать исходящий TTL = ', inTargetTTL]),
							E('label', { style: 'display:block; margin-bottom:4px;' }, [chkInterceptDNS, ' Перехватывать порт 53 (Anti-DNS-Leak)']),
							E('label', { style: 'display:block;' }, [chkBlockDoT, ' Блокировать DNS-over-TLS (порт 853)'])
						])
					]),
					E('div', { class: 'cbi-value' }, [
						E('label', { class: 'cbi-value-title' }, 'Резервные ссылки Dead Drop (Яндекс Диск / S3)'),
						E('div', { class: 'cbi-value-field' }, [txtDeadDrop])
					]),
					E('div', { class: 'cbi-value' }, [
						E('label', { class: 'cbi-value-title' }, 'Режим Белых списков (VK / DION)'),
						E('div', { class: 'cbi-value-field' }, [
							E('label', { style: 'margin-right:16px;' }, [radWLAuto, ' Auto (Только при отказе основных каналов)']),
							E('label', {}, [radWLAlways, ' Always (Всегда через ВК / DION)'])
						])
					]),
					E('div', { class: 'cbi-value' }, [
						E('label', { class: 'cbi-value-title' }, 'Ссылки на звонки VK Calls'),
						E('div', { class: 'cbi-value-field' }, [txtWLVKHashes])
					]),
					E('div', { class: 'cbi-value' }, [
						E('label', { class: 'cbi-value-title' }, 'Комнаты конференций DION'),
						E('div', { class: 'cbi-value-field' }, [txtWLDionSlugs])
					]),
					E('div', { class: 'cbi-value' }, [
						E('label', { class: 'cbi-value-title' }, 'Запасной TURN сервер / Порт шлюза'),
						E('div', { class: 'cbi-value-field' }, [
							inWLTurnBackup,
							E('span', { style: 'margin-left:12px;' }, 'Порт: '),
							inWLPort
						])
					])
				]))
			])
		]);

		var tab4Content = E('div', {}, [
			E('div', { class: 'cbi-section' }, [
				E('h3', {}, '📊 Самодиагностика интернета (Светофор направлений)'),
				E('p', { class: 'cbi-section-descr' }, 'Нажмите кнопку ниже, чтобы роутер автоматически проверил работу российских сайтов, прямого ускорения YouTube/Instagram (Zapret 2) и зарубежного VPN.'),
				E('div', {}, [
					btnTestConn,
					btnCopySupportReport
				]),
				diagTrafficLights
			]),
			E('div', { class: 'cbi-section' }, [
				E('div', { style: 'display: flex; align-items: center; justify-content: space-between;' }, [
					E('h3', { style: 'margin: 0;' }, '📜 Системный журнал работы (logread)'),
					btnRefreshLogs
				]),
				inlineLogPre
			])
		]);

		// --- TAB SWITCHER ---
		var tabs = [
			{ id: 'home', label: '🏠 1. Главная и Сценарии', node: tab1Content },
			{ id: 'sites', label: '🌍 2. Сайты и Приложения', node: tab2Content },
			{ id: 'shield', label: '🛡️ 3. Связь и Защита', node: tab3Content },
			{ id: 'diag', label: '📊 4. Проверка и Журнал', node: tab4Content }
		];

		var tabButtons = {};
		var tabMenu = E('ul', {
			class: 'cbi-tabmenu',
			style: 'display: flex; flex-wrap: wrap; list-style: none; padding: 0; margin: 16px 0 0 0; border-bottom: 2px solid #dee2e6; gap: 6px;'
		});

		function selectTab(activeId) {
			for (var i = 0; i < tabs.length; i++) {
				var t = tabs[i];
				var isAct = (t.id === activeId);
				t.node.style.display = isAct ? 'block' : 'none';
				var li = tabButtons[t.id];
				if (li) {
					li.className = isAct ? 'cbi-tab' : 'cbi-tab-disabled';
					li.style.background = isAct ? '#0069d9' : '#e9ecef';
					li.style.color = isAct ? '#ffffff' : '#333333';
				}
			}
			if (activeId === 'diag') {
				refreshInlineLogs();
			}
		}

		for (var tIdx = 0; tIdx < tabs.length; tIdx++) {
			(function(t) {
				var li = E('li', {
					style: 'padding: 9px 16px; border-radius: 6px 6px 0 0; cursor: pointer; font-weight: bold; font-size: 13px; transition: all 0.15s ease;',
					click: function() { selectTab(t.id); }
				}, t.label);
				tabButtons[t.id] = li;
				tabMenu.appendChild(li);
			})(tabs[tIdx]);
		}
		selectTab('home');

		return E('div', { class: 'view' }, [
			E('div', { style: 'display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 10px; margin-bottom: 8px;' }, [
				E('div', {}, [
					E('h2', { style: 'margin: 0;' }, 'Sirin VPN — Центр управления домашней сетью'),
					E('div', { style: 'color: #6c757d; font-size: 13px; margin-top: 2px;' },
						'Умный обход блокировок: Zapret 2 (YouTube / Instagram / Discord без нагрузки на VPS) + Авто-VPN для ИИ и зарубежных сервисов.')
				]),
				btnModeToggle
			]),

			// Compact Sticky Header Card
			E('div', { class: 'cbi-section', style: 'margin-bottom: 12px; padding: 14px 18px;' }, [
				E('div', { style: 'display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 12px;' }, [
					E('div', { style: 'display: flex; align-items: center; flex-wrap: wrap; gap: 14px;' }, [
						statusBadge,
						E('span', { style: 'font-size: 13px;' }, ['Режим: ', transportNode]),
						E('span', { style: 'font-size: 13px; color: #6c757d;' }, ['Работает: ', uptimeNode]),
						E('span', { style: 'font-size: 13px;' }, ['Версия: ', versionNode, btnCheckUpdate, updateStatusSpan])
					]),
					E('div', { style: 'display: flex; align-items: center; flex-wrap: wrap; gap: 4px;' }, [
						btnStart,
						btnStop,
						btnRestart,
						btnAutostart
					])
				])
			]),

			// 4-Tab Bar
			tabMenu,

			// Tab Panels
			tab1Content,
			tab2Content,
			tab3Content,
			tab4Content,

			// Unified Sticky Bottom Save Bar
			saveBanner
		]);
	},

	handleSave: null,
	handleSaveApply: null,
	handleReset: null
});
