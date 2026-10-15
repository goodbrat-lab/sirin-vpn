/**
 * SirinFeather — Визуальный компонент "Перо Сирин" (HTML5 Canvas 2D).
 * 
 * Точное воспроизведение векторной геометрии logo_sirin_feather_v2.svg:
 * - 'disconnected': приглушённый вертикальный градиент #3B4452 -> #4A5568 -> #5A6678,
 *                   обводка 1px #6B7280 (0.4), наклон 0°.
 * - 'connecting': золотое перо, наклон 0°→15°, пульсация свечения 1.5 с.
 * - 'connected': парящее золотое перо, асимметрия (+12% справа), очин, тень 60x8px на cy+60,
 *                лазурный акцент пушка Сирин, покачивание ±3px, искры по орбитам.
 * - 'error': красное перо, наклон 30° (упало), мелкое дрожание.
 */
class SirinFeather {
    constructor(canvas, initialState = 'disconnected') {
        this.canvas = canvas;
        this.ctx = canvas.getContext('2d');
        this.state = initialState;
        this.t = 0;
        this.raf = null;
        this.start();
    }

    setState(state) {
        this.state = state;
    }

    start() {
        if (this.raf) return;
        let lastTime = performance.now();
        const loop = (now) => {
            const dt = (now - lastTime) / 1000;
            lastTime = now;
            this.t += dt;
            this.draw();
            this.raf = requestAnimationFrame(loop);
        };
        this.raf = requestAnimationFrame(loop);
    }

    stop() {
        if (this.raf) {
            cancelAnimationFrame(this.raf);
            this.raf = null;
        }
    }

    draw() {
        const { width, height } = this.canvas;
        const ctx = this.ctx;
        ctx.clearRect(0, 0, width, height);

        const cx = width / 2;
        const cy = height / 2;
        const scale = Math.min(width, height) / 200;

        // Расчёт параметров анимации
        let offsetY = 0;
        let angle = 0;
        let pulseAlpha = 1.0;

        if (this.state === 'connected') {
            offsetY = Math.sin(this.t * (Math.PI * 2 / 2.2)) * 3 * scale;
            const sway = Math.sin(this.t * (Math.PI * 2 / 3.2)) * 2;
            angle = 15 + sway;
        } else if (this.state === 'connecting') {
            // Пульсация 1.5 сек (0.35 .. 1.0)
            const p = (Math.sin(this.t * (Math.PI * 2 / 1.5)) + 1) / 2;
            pulseAlpha = 0.35 + p * 0.65;
            // Подъём угла 0° .. 15°
            angle = 7.5 + Math.sin(this.t * (Math.PI * 2 / 3.0)) * 7.5;
        } else if (this.state === 'error') {
            // Быстрое дрожание ±2°
            const jitter = Math.sin(this.t * 60) * 2;
            angle = 30 + jitter;
        } else {
            // Disconnected
            angle = 0;
        }

        // 1. Тень под пером (только CONNECTED, cy + 60, эллипс 60x8px, blur 4px)
        if (this.state === 'connected') {
            ctx.save();
            const shadowY = cy + 60 * scale;
            const rx = 30 * scale; // 60px ширина
            const ry = 4 * scale;  // 8px высота
            ctx.beginPath();
            ctx.ellipse(cx, shadowY, rx, ry, 0, 0, Math.PI * 2);
            ctx.fillStyle = 'rgba(0, 0, 0, 0.45)';
            ctx.filter = 'blur(4px)';
            ctx.fill();
            ctx.restore();
        }

        // 2. Радиальное свечение
        if (this.state !== 'disconnected') {
            ctx.save();
            const glowRadius = 110 * scale;
            const glowGrad = ctx.createRadialGradient(cx, cy - 8 * scale + offsetY, 0, cx, cy - 8 * scale + offsetY, glowRadius);
            if (this.state === 'error') {
                glowGrad.addColorStop(0, 'rgba(239, 68, 68, 0.35)');
                glowGrad.addColorStop(0.6, 'rgba(239, 68, 68, 0.12)');
                glowGrad.addColorStop(1, 'rgba(239, 68, 68, 0)');
            } else if (this.state === 'connecting') {
                glowGrad.addColorStop(0, `rgba(232, 195, 111, ${0.35 * pulseAlpha})`);
                glowGrad.addColorStop(0.6, `rgba(232, 195, 111, ${0.12 * pulseAlpha})`);
                glowGrad.addColorStop(1, 'rgba(232, 195, 111, 0)');
            } else {
                glowGrad.addColorStop(0, 'rgba(232, 195, 111, 0.35)');
                glowGrad.addColorStop(0.6, 'rgba(232, 195, 111, 0.12)');
                glowGrad.addColorStop(1, 'rgba(232, 195, 111, 0)');
            }
            ctx.fillStyle = glowGrad;
            ctx.beginPath();
            ctx.arc(cx, cy - 8 * scale + offsetY, glowRadius, 0, Math.PI * 2);
            ctx.fill();
            ctx.restore();
        }

        // 3. Отрисовка пера (векторы logo_sirin_feather_v2.svg)
        ctx.save();
        ctx.translate(cx, cy - 8 * scale + offsetY);
        ctx.rotate(angle * Math.PI / 180);
        ctx.scale(scale * 0.31, scale * 0.31);
        ctx.translate(-256, -256);

        this.drawFeatherVectors(ctx);
        ctx.restore();

        // 4. Искры по орбитам (только connected)
        if (this.state === 'connected') {
            this.drawSparks(ctx, cx, cy - 8 * scale + offsetY, scale);
        }
    }

    drawFeatherVectors(ctx) {
        const isDisconnected = (this.state === 'disconnected');

        // Подготовка градиентов
        let goldLight = ctx.createLinearGradient(200, 400, 320, 50);
        let goldDark = ctx.createLinearGradient(190, 400, 260, 50);
        let calamusGrad = ctx.createLinearGradient(256, 400, 256, 465);
        let shaftGrad = ctx.createLinearGradient(256, 400, 256, 50);
        let blueGrad = ctx.createLinearGradient(220, 400, 290, 340);
        let tipColor = '#FFF8E0';

        if (isDisconnected) {
            // Вертикальный градиент #3B4452 -> #4A5568 -> #5A6678
            const discGrad = ctx.createLinearGradient(0, 400, 0, 50);
            discGrad.addColorStop(0, '#3B4452');
            discGrad.addColorStop(0.5, '#4A5568');
            discGrad.addColorStop(1, '#5A6678');

            goldLight = discGrad;
            goldDark = discGrad;

            calamusGrad = ctx.createLinearGradient(0, 400, 0, 465);
            calamusGrad.addColorStop(0, '#4A5568');
            calamusGrad.addColorStop(1, '#2D3540');

            shaftGrad = ctx.createLinearGradient(0, 400, 0, 50);
            shaftGrad.addColorStop(0, '#3B4452');
            shaftGrad.addColorStop(1, '#6B7A90');

            blueGrad = ctx.createLinearGradient(0, 400, 0, 340);
            blueGrad.addColorStop(0, '#242C35');
            blueGrad.addColorStop(1, '#333D4B');

            tipColor = '#5A6678';
        } else if (this.state === 'error') {
            goldLight.addColorStop(0, '#991B1B');
            goldLight.addColorStop(0.5, '#DC2626');
            goldLight.addColorStop(1, '#F87171');

            goldDark.addColorStop(0, '#7F1D1D');
            goldDark.addColorStop(0.5, '#B91C1C');
            goldDark.addColorStop(1, '#EF4444');

            calamusGrad.addColorStop(0, '#7F1D1D');
            calamusGrad.addColorStop(1, '#450A0A');

            shaftGrad.addColorStop(0, '#7F1D1D');
            shaftGrad.addColorStop(1, '#FECACA');

            blueGrad.addColorStop(0, '#450A0A');
            blueGrad.addColorStop(1, '#991B1B');

            tipColor = '#FECACA';
        } else {
            // Connected / Connecting
            goldLight.addColorStop(0, '#8C631A');
            goldLight.addColorStop(0.25, '#BA8F35');
            goldLight.addColorStop(0.55, '#E8C36F');
            goldLight.addColorStop(0.85, '#FCE4A6');
            goldLight.addColorStop(1, '#FFF8E0');

            goldDark.addColorStop(0, '#6E4A0E');
            goldDark.addColorStop(0.3, '#9C7323');
            goldDark.addColorStop(0.65, '#CCA140');
            goldDark.addColorStop(0.9, '#F2D183');
            goldDark.addColorStop(1, '#FFF2CA');

            // Очин темнее основного на 15-20%
            calamusGrad.addColorStop(0, '#8C631A');
            calamusGrad.addColorStop(0.5, '#6B470D');
            calamusGrad.addColorStop(1, '#4E3309');

            shaftGrad.addColorStop(0, '#6E4A0E');
            shaftGrad.addColorStop(0.35, '#E8C36F');
            shaftGrad.addColorStop(0.75, '#FFF2CA');
            shaftGrad.addColorStop(1, '#FFF8E0');

            blueGrad.addColorStop(0, '#1E3E8A');
            blueGrad.addColorStop(0.5, '#3B82F6');
            blueGrad.addColorStop(1, '#93C5FD');

            tipColor = '#FFF8E0';
        }

        // Вспомогательная функция для обводки 1px #6B7280 (0.4) в DISCONNECTED
        const strokeDisconnected = () => {
            if (isDisconnected) {
                ctx.save();
                ctx.strokeStyle = 'rgba(107, 114, 128, 0.40)';
                ctx.lineWidth = 1;
                ctx.stroke();
                ctx.restore();
            }
        };

        // 1. Левое полу-опахало
        ctx.fillStyle = goldDark;
        ctx.beginPath();
        ctx.moveTo(254, 398);
        ctx.bezierCurveTo(241, 390, 226, 370, 218, 348);
        ctx.bezierCurveTo(225, 344, 235, 346, 244, 352);
        ctx.bezierCurveTo(214, 330, 198, 298, 196, 266);
        ctx.bezierCurveTo(206, 262, 218, 266, 229, 274);
        ctx.bezierCurveTo(200, 244, 196, 204, 207, 166);
        ctx.bezierCurveTo(216, 162, 228, 166, 238, 174);
        ctx.bezierCurveTo(216, 140, 224, 102, 246, 70);
        ctx.bezierCurveTo(252, 60, 256, 54, 260, 50);
        ctx.bezierCurveTo(256, 150, 250, 270, 254, 398);
        ctx.closePath();
        ctx.fill();
        strokeDisconnected();

        // 2. Правое полу-опахало (на 12% шире)
        ctx.fillStyle = goldLight;
        ctx.beginPath();
        ctx.moveTo(257, 398);
        ctx.bezierCurveTo(272, 390, 292, 370, 302, 348);
        ctx.bezierCurveTo(293, 344, 280, 346, 270, 352);
        ctx.bezierCurveTo(305, 330, 326, 298, 328, 266);
        ctx.bezierCurveTo(316, 262, 300, 266, 287, 274);
        ctx.bezierCurveTo(324, 244, 329, 204, 316, 166);
        ctx.bezierCurveTo(304, 162, 288, 166, 277, 174);
        ctx.bezierCurveTo(308, 140, 302, 102, 278, 70);
        ctx.bezierCurveTo(271, 60, 266, 54, 260, 50);
        ctx.bezierCurveTo(258, 150, 253, 270, 257, 398);
        ctx.closePath();
        ctx.fill();
        strokeDisconnected();

        // 3. Лазурный акцент Сирин у основания
        ctx.save();
        ctx.globalAlpha = isDisconnected ? 0.5 : 0.85;
        ctx.fillStyle = blueGrad;

        // Левый пушок
        ctx.beginPath();
        ctx.moveTo(254, 398);
        ctx.bezierCurveTo(243, 392, 229, 374, 222, 354);
        ctx.bezierCurveTo(229, 350, 238, 354, 246, 362);
        ctx.bezierCurveTo(251, 372, 253, 386, 254, 398);
        ctx.closePath();
        ctx.fill();

        // Правый пушок
        ctx.beginPath();
        ctx.moveTo(257, 398);
        ctx.bezierCurveTo(270, 392, 288, 374, 297, 354);
        ctx.bezierCurveTo(288, 350, 276, 354, 267, 362);
        ctx.bezierCurveTo(261, 372, 258, 386, 257, 398);
        ctx.closePath();
        ctx.fill();
        ctx.restore();

        // 4. Текстурные бородки (штрихи лучей пера)
        if (!isDisconnected) {
            ctx.save();
            ctx.lineCap = 'round';

            const drawLine = (x1, y1, x2, y2, w) => {
                ctx.lineWidth = w;
                ctx.beginPath();
                ctx.moveTo(x1, y1);
                ctx.lineTo(x2, y2);
                ctx.stroke();
            };

            // Правые лучи
            ctx.strokeStyle = '#FFF8E0';
            ctx.globalAlpha = 0.45;
            drawLine(256, 352, 288, 328, 1.5);
            drawLine(252, 274, 310, 244, 1.6);
            drawLine(254, 193, 306, 150, 1.6);
            drawLine(258, 118, 288, 82, 1.4);

            // Левые лучи
            ctx.strokeStyle = '#FFF2CA';
            ctx.globalAlpha = 0.32;
            drawLine(254, 352, 228, 330, 1.4);
            drawLine(250, 274, 206, 246, 1.5);
            drawLine(252, 193, 208, 152, 1.5);
            drawLine(256, 118, 234, 84, 1.3);

            ctx.restore();
        }

        // 5. ОЧИН (основание): утолщение стержня (~x1.8), полая форма, цвет темнее на 15-20%
        ctx.fillStyle = calamusGrad;
        ctx.beginPath();
        ctx.moveTo(253, 465);
        ctx.bezierCurveTo(252, 450, 251.5, 430, 252.5, 400);
        ctx.lineTo(258.5, 400);
        ctx.bezierCurveTo(259.5, 430, 259, 450, 258, 465);
        ctx.bezierCurveTo(257.5, 468, 253.5, 468, 253, 465);
        ctx.closePath();
        ctx.fill();
        strokeDisconnected();

        // Полый срез очина
        ctx.fillStyle = isDisconnected ? '#242C35' : 'rgba(61, 40, 6, 0.7)';
        ctx.beginPath();
        ctx.ellipse(255.5, 465.5, 2.5, 1.2, 0, 0, Math.PI * 2);
        ctx.fill();

        // 6. Центральный стержень (рахис) с живым S-изгибом 2-3%
        ctx.fillStyle = shaftGrad;
        ctx.beginPath();
        ctx.moveTo(254, 400);
        ctx.bezierCurveTo(249, 280, 253, 160, 259.2, 50);
        ctx.lineTo(260.8, 50);
        ctx.bezierCurveTo(255, 160, 251, 280, 257, 400);
        ctx.closePath();
        ctx.fill();

        // 7. Световой блик на гребне стержня
        if (!isDisconnected) {
            ctx.save();
            ctx.strokeStyle = '#FFFFFF';
            ctx.lineWidth = 1.3;
            ctx.lineCap = 'round';
            ctx.globalAlpha = 0.9;
            ctx.beginPath();
            ctx.moveTo(256, 390);
            ctx.bezierCurveTo(250, 280, 254, 160, 260, 54);
            ctx.stroke();
            ctx.restore();
        }

        // 8. Закруглённый кончик пера
        ctx.fillStyle = tipColor;
        ctx.beginPath();
        ctx.arc(260, 51, 3, 0, Math.PI * 2);
        ctx.fill();
    }

    drawSparks(ctx, cx, cy, scale) {
        const sparkRadii = [82, 96, 74, 88, 92, 78, 85, 98];
        const sparkSizes = [2.6, 1.8, 2.2, 3.0, 1.6, 2.4, 2.0, 2.8];
        const sparkAlphas = [0.9, 0.6, 0.85, 0.95, 0.5, 0.8, 0.7, 0.9];
        const baseAngle = (this.t * 40) % 360;

        for (let i = 0; i < 8; i++) {
            const angleDeg = baseAngle + i * 45 + (i * 7);
            const angleRad = angleDeg * Math.PI / 180;
            const r = sparkRadii[i] * scale;
            const sx = cx + r * Math.cos(angleRad);
            const sy = cy + r * Math.sin(angleRad);

            const pSize = sparkSizes[i] * scale;
            const alpha = sparkAlphas[i];

            ctx.save();
            // Золотистое свечение
            ctx.fillStyle = `rgba(252, 228, 166, ${alpha * 0.4})`;
            ctx.beginPath();
            ctx.arc(sx, sy, pSize * 2.2, 0, Math.PI * 2);
            ctx.fill();

            // Ядро искры
            ctx.fillStyle = `rgba(255, 240, 190, ${alpha})`;
            ctx.beginPath();
            ctx.arc(sx, sy, pSize, 0, Math.PI * 2);
            ctx.fill();
            ctx.restore();
        }
    }
}

if (typeof window !== 'undefined') {
    window.SirinFeather = SirinFeather;
}
