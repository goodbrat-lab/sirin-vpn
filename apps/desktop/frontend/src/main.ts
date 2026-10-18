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
function openSettings() {
    const dialog = document.getElementById('settings-dialog') as HTMLDialogElement;
    App.GetKey().then(key => {
        (document.getElementById('key-input') as HTMLInputElement).value = key || '';
    });
    dialog.showModal();
}

document.getElementById('settings-btn')!.addEventListener('click', openSettings);
document.getElementById('close-settings')!.addEventListener('click', () => {
    (document.getElementById('settings-dialog') as HTMLDialogElement).close();
});
document.getElementById('save-settings')!.addEventListener('click', async () => {
    const key = (document.getElementById('key-input') as HTMLInputElement).value.trim();
    if (key.length !== 36) {
        alert('UUID должен быть 36 символов');
        return;
    }
    await App.SaveKey(key);
    (document.getElementById('settings-dialog') as HTMLDialogElement).close();
    statusLabel.textContent = 'Ключ сохранён';
});
