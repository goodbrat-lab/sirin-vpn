import { SirinFeather } from './sirin-feather.js';
import * as App from '../bindings/sirin/desktop/app';
import { Events } from '@wailsio/runtime';

const canvas = document.getElementById('feather') as HTMLCanvasElement;
const feather = new SirinFeather(canvas, 'disconnected');

const statusLabel = document.getElementById('status-label')!;
const stateText = document.getElementById('state-text')!;
const transportText = document.getElementById('transport-text')!;
const connectBtn = document.getElementById('connect-btn') as HTMLButtonElement;

let currentState = 'disconnected';

connectBtn.addEventListener('click', async () => {
    if (currentState === 'connected') {
        await App.Disconnect();
    } else if (currentState === 'disconnected' || currentState === 'error') {
        const key = await App.GetKey();
        if (!key) {
            openSettings();
            return;
        }
        await App.Connect(key);
    }
});

function handleStateUpdate(payload: any) {
    const state = payload?.state || 'disconnected';
    currentState = state;
    feather.setState(state);
    
    switch (state) {
        case 'disconnected': 
            statusLabel.textContent = 'Отключено';
            stateText.textContent = 'Не подключено';
            connectBtn.textContent = 'Подключить';
            connectBtn.classList.remove('danger');
            transportText.textContent = '';
            break;
        case 'connecting':
            statusLabel.textContent = 'Подключение...';
            stateText.textContent = 'Подключение...';
            connectBtn.textContent = 'Отмена';
            break;
        case 'connected':
            statusLabel.textContent = 'Защищено';
            stateText.textContent = 'Защищено';
            connectBtn.textContent = 'Отключить';
            connectBtn.classList.add('danger');
            refreshStatus();
            break;
        case 'error':
            statusLabel.textContent = 'Ошибка';
            stateText.textContent = payload.error || 'Ошибка';
            connectBtn.textContent = 'Повторить';
            break;
    }
}

// Слушаем события через Wails v3 runtime
Events.On('vpn:state', (ev: any) => {
    handleStateUpdate(ev?.data !== undefined ? ev.data : ev);
});

Events.On('vpn:log', (ev: any) => {
    const data = ev?.data !== undefined ? ev.data : ev;
    console.log('[log]', data?.message || data);
});

if ((window as any).runtime?.EventsOn) {
    (window as any).runtime.EventsOn('vpn:state', handleStateUpdate);
    (window as any).runtime.EventsOn('vpn:log', (payload: any) => {
        console.log('[log]', payload?.message || payload);
    });
}

async function refreshStatus() {
    const status = await App.GetStatus();
    if (status) {
        transportText.textContent = status.winner ? `via ${status.winner}` : '';
    }
}

// Настройки
let currentLatestUpdate: any = null;

async function openSettings() {
    const dialog = document.getElementById('settings-dialog') as HTMLDialogElement;
    
    try {
        const ver = await App.GetAppVersion();
        const verLabel = document.getElementById('app-version-label');
        if (verLabel && ver) verLabel.textContent = `Версия: ${ver}`;
    } catch (e) {
        console.error('GetAppVersion error:', e);
    }

    try {
        const key = await App.GetKey();
        (document.getElementById('key-input') as HTMLInputElement).value = key || '';
    } catch (e) {
        console.error('GetKey error:', e);
    }

    try {
        const cfg: any = await App.GetRoutingConfig();
        if (cfg) {
            const ruToggle = document.getElementById('rudirect-toggle') as HTMLInputElement;
            if (ruToggle) ruToggle.checked = cfg.ru_direct !== false;

            const mode = cfg.split_mode || 'bypass';
            const radio = document.querySelector(`input[name="split-mode"][value="${mode}"]`) as HTMLInputElement;
            if (radio) radio.checked = true;

            const domainsInput = document.getElementById('domains-input') as HTMLTextAreaElement;
            if (domainsInput) domainsInput.value = (cfg.split_domains || []).join('\n');

            const cidrsInput = document.getElementById('cidrs-input') as HTMLTextAreaElement;
            if (cidrsInput) cidrsInput.value = (cfg.split_cidrs || []).join('\n');
        }
    } catch (e) {
        console.error('GetRoutingConfig error:', e);
    }

    dialog.showModal();
}

// Проверка обновлений десктопа
const checkUpdateBtn = document.getElementById('check-update-btn') as HTMLButtonElement;
const updateBox = document.getElementById('update-box') as HTMLDivElement;
const updateMessage = document.getElementById('update-message') as HTMLDivElement;
const updateNotes = document.getElementById('update-notes') as HTMLDivElement;
const applyUpdateBtn = document.getElementById('apply-update-btn') as HTMLButtonElement;

if (checkUpdateBtn) {
    checkUpdateBtn.addEventListener('click', async () => {
        checkUpdateBtn.disabled = true;
        checkUpdateBtn.textContent = 'Проверка...';
        updateBox.style.display = 'block';
        updateMessage.textContent = 'Поиск обновлений на GitHub...';
        updateMessage.style.color = 'var(--m-dim)';
        updateNotes.style.display = 'none';
        applyUpdateBtn.style.display = 'none';

        try {
            const info: any = await App.CheckForUpdate();
            checkUpdateBtn.disabled = false;
            checkUpdateBtn.textContent = 'Проверить';

            if (!info) {
                updateMessage.textContent = 'Не удалось получить данные об обновлениях';
                return;
            }

            if (info.available) {
                currentLatestUpdate = info;
                updateMessage.innerHTML = `<span style="color:#5cb85c; font-weight:bold;">Доступна версия ${info.latest_version}</span> (текущая: ${info.current_version})`;
                if (info.release_notes) {
                    updateNotes.textContent = info.release_notes;
                    updateNotes.style.display = 'block';
                }
                applyUpdateBtn.textContent = `Обновить до v${info.latest_version}`;
                applyUpdateBtn.style.display = 'block';
            } else {
                updateMessage.innerHTML = `<span style="color:#5cb85c;">✓ У вас актуальная версия (${info.current_version})</span>`;
            }
        } catch (e: any) {
            checkUpdateBtn.disabled = false;
            checkUpdateBtn.textContent = 'Проверить';
            updateMessage.textContent = `Ошибка проверки: ${e?.message || e}`;
            updateMessage.style.color = '#d9534f';
        }
    });
}

if (applyUpdateBtn) {
    applyUpdateBtn.addEventListener('click', async () => {
        if (!currentLatestUpdate?.download_url) return;
        applyUpdateBtn.disabled = true;
        applyUpdateBtn.textContent = 'Загрузка и установка...';
        updateMessage.textContent = 'Загрузка новой версии и подготовка перезапуска...';
        updateMessage.style.color = 'var(--m-text)';

        try {
            await App.InstallUpdate(currentLatestUpdate.download_url);
            updateMessage.innerHTML = '<span style="color:#5cb85c; font-weight:bold;">✓ Обновление установлено! Перезапуск...</span>';
        } catch (e: any) {
            applyUpdateBtn.disabled = false;
            applyUpdateBtn.textContent = 'Повторить обновление';
            updateMessage.textContent = `Ошибка установки: ${e?.message || e}`;
            updateMessage.style.color = '#d9534f';
        }
    });
}

document.getElementById('settings-btn')!.addEventListener('click', openSettings);
document.getElementById('close-settings')!.addEventListener('click', () => {
    (document.getElementById('settings-dialog') as HTMLDialogElement).close();
});

document.getElementById('save-settings')!.addEventListener('click', async () => {
    const key = (document.getElementById('key-input') as HTMLInputElement).value.trim();
    if (key.length > 0 && key.length !== 36) {
        alert('UUID должен быть 36 символов (или оставьте пустым)');
        return;
    }

    const ruToggle = document.getElementById('rudirect-toggle') as HTMLInputElement;
    const ruDirect = ruToggle ? ruToggle.checked : true;

    const selectedRadio = document.querySelector('input[name="split-mode"]:checked') as HTMLInputElement;
    const splitMode = selectedRadio ? selectedRadio.value : 'bypass';

    const domainsText = (document.getElementById('domains-input') as HTMLTextAreaElement).value;
    const domains = domainsText.split('\n').map(s => s.trim()).filter(s => s.length > 0);

    const cidrsText = (document.getElementById('cidrs-input') as HTMLTextAreaElement).value;
    const cidrs = cidrsText.split('\n').map(s => s.trim()).filter(s => s.length > 0);

    if (key) {
        await App.SaveKey(key);
    }
    await App.SaveRoutingConfig(ruDirect, splitMode, cidrs, domains);

    (document.getElementById('settings-dialog') as HTMLDialogElement).close();
    statusLabel.textContent = 'Настройки сохранены';
});
