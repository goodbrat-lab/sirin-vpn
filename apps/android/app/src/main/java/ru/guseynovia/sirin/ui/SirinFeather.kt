package ru.guseynovia.sirin.ui

import androidx.compose.animation.core.*
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.Image
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.collectIsPressedAsState
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.Text
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.*
import androidx.compose.ui.graphics.drawscope.DrawScope
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.delay
import ru.guseynovia.sirin.Brand
import ru.guseynovia.sirin.R
import kotlin.math.sin

enum class FeatherState {
    DISCONNECTED,
    CONNECTING,
    CONNECTED,
    ERROR
}

/**
 * Главный визуальный блок экрана SIRIN VPN (State Machine):
 * - Центральный элемент: Детализированный логотип Сирин с прозрачным фоном
 * - Анимация потока пакетов/частиц
 * - Интерактивный статус
 * - Круглая кнопка питания с классическим символом Power (IEC)
 */
@Composable
fun SirinFeather(
    state: FeatherState,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    onClick: () -> Unit = {},
) {
    Column(
        modifier = modifier.fillMaxWidth(),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.Center
    ) {
        // 1. Центральный элемент — Готовая детализированная иллюстрация Сирин
        SirinLogoWithPadlock(
            state = state,
            modifier = Modifier.size(220.dp)
        )

        // 2. Пространство между логотипом и кнопкой: анимация пакетов и статус
        Box(
            modifier = Modifier
                .fillMaxWidth()
                .height(84.dp),
            contentAlignment = Alignment.Center
        ) {
            // Анимация поднимающихся пакетов данных
            DataPacketParticles(
                state = state,
                modifier = Modifier.fillMaxSize()
            )

            // Текст статуса состояния
            SirinStatusText(
                state = state,
                modifier = Modifier.padding(horizontal = 24.dp)
            )
        }

        Spacer(modifier = Modifier.height(10.dp))

        // 3. Внизу — круглая кнопка питания (Power Button)
        SirinPowerButton(
            state = state,
            enabled = enabled,
            onClick = onClick
        )
    }
}

/**
 * 1. Центральный элемент: Детализированная иллюстрация Сирин (PNG на прозрачном фоне).
 * - 4 состояния:
 *   - DISCONNECTED: Затемненный (opacity 65%), мягкое затемнение.
 *   - CONNECTING: Пульсирующая теплая кибер-аура за логотипом.
 *   - CONNECTED: 100% яркость, яркое неоновое сине-пурпурное свечение с эффектом вспышки, парение.
 *   - ERROR: Glitch-эффект (цифровой джиттер) на 0.5 сек при сбое, красная кибер-подсветка.
 */
@Composable
fun SirinLogoWithPadlock(
    state: FeatherState,
    modifier: Modifier = Modifier
) {
    val infiniteTransition = rememberInfiniteTransition(label = "sirinLogoTransition")

    // Парение крыльев для CONNECTED
    val hoverY by infiniteTransition.animateFloat(
        initialValue = -3.5f,
        targetValue = 3.5f,
        animationSpec = infiniteRepeatable(
            animation = tween(2600, easing = FastOutSlowInEasing),
            repeatMode = RepeatMode.Reverse
        ),
        label = "hoverY"
    )

    // Пульсация свечения для CONNECTING
    val pulseGlow by infiniteTransition.animateFloat(
        initialValue = 0.45f,
        targetValue = 0.95f,
        animationSpec = infiniteRepeatable(
            animation = tween(1200, easing = FastOutSlowInEasing),
            repeatMode = RepeatMode.Reverse
        ),
        label = "pulseGlow"
    )

    // Вспышка свечения при переходе в CONNECTED
    val flashAnim = remember { Animatable(1f) }
    LaunchedEffect(state) {
        if (state == FeatherState.CONNECTED) {
            flashAnim.snapTo(1.6f)
            flashAnim.animateTo(1.0f, animationSpec = tween(600, easing = FastOutSlowInEasing))
        }
    }

    // Glitch эффект для ERROR на 0.5 секунды
    var isGlitching by remember { mutableStateOf(false) }
    LaunchedEffect(state) {
        if (state == FeatherState.ERROR) {
            isGlitching = true
            delay(500)
            isGlitching = false
        } else {
            isGlitching = false
        }
    }

    val glitchJitter by infiniteTransition.animateFloat(
        initialValue = -6f,
        targetValue = 6f,
        animationSpec = infiniteRepeatable(
            animation = tween(45, easing = LinearEasing),
            repeatMode = RepeatMode.Reverse
        ),
        label = "glitchJitter"
    )

    val currentOffsetY = if (state == FeatherState.CONNECTED) hoverY else 0f
    val glitchOffsetX = if (isGlitching) glitchJitter else 0f

    val alpha = when (state) {
        FeatherState.DISCONNECTED -> 0.65f
        FeatherState.CONNECTING -> 0.95f
        FeatherState.CONNECTED -> 1.0f
        FeatherState.ERROR -> 0.95f
    }

    Box(
        modifier = modifier
            .graphicsLayer {
                translationX = glitchOffsetX
                translationY = currentOffsetY
                this.alpha = alpha
            },
        contentAlignment = Alignment.Center
    ) {
        // Фоновое свечение (Aura) за иллюстрацией
        if (state != FeatherState.DISCONNECTED) {
            Canvas(modifier = Modifier.fillMaxSize()) {
                val cx = size.width / 2f
                val cy = size.height / 2f
                val radius = size.minDimension * 0.48f

                val glowColor = when (state) {
                    FeatherState.CONNECTED -> Brand.cyberBlue
                    FeatherState.CONNECTING -> Brand.neonOrange
                    FeatherState.ERROR -> Brand.neonRed
                    FeatherState.DISCONNECTED -> Color.Transparent
                }

                val glowMultiplier = if (state == FeatherState.CONNECTED) {
                    flashAnim.value
                } else if (state == FeatherState.CONNECTING) {
                    pulseGlow
                } else {
                    1.0f
                }

                drawCircle(
                    brush = Brush.radialGradient(
                        0.0f to glowColor.copy(alpha = 0.42f * glowMultiplier),
                        0.55f to glowColor.copy(alpha = 0.12f * glowMultiplier),
                        1.0f to Color.Transparent,
                        center = Offset(cx, cy),
                        radius = radius
                    ),
                    radius = radius,
                    center = Offset(cx, cy)
                )

                if (state == FeatherState.CONNECTED) {
                    drawCircle(
                        brush = Brush.radialGradient(
                            0.0f to Brand.neonPurple.copy(alpha = 0.35f * flashAnim.value),
                            0.7f to Color.Transparent,
                            center = Offset(cx, cy),
                            radius = radius * 0.8f
                        ),
                        radius = radius * 0.8f,
                        center = Offset(cx, cy)
                    )
                }
            }
        }

        // Готовая детализированная иллюстрация Сирин на прозрачном фоне
        Image(
            painter = painterResource(id = R.drawable.sirin_logo_cyber),
            contentDescription = "Сирин",
            modifier = Modifier
                .fillMaxSize()
                .padding(4.dp),
            contentScale = ContentScale.Fit,
            colorFilter = if (state == FeatherState.ERROR) {
                ColorFilter.tint(Brand.neonRed.copy(alpha = 0.18f), BlendMode.SrcAtop)
            } else null
        )
    }
}

/**
 * 2. Анимация светящихся частиц/пакетов данных (Data Packets).
 * - Летят снизу вверх (из-под кнопки питания к логотипу).
 * - DISCONNECTED: анимация статична / скрыта.
 * - CONNECTING: подъем частиц с оранжево-желтым свечением.
 * - CONNECTED: бесшовный loop летящих вверх сине-пурпурных частиц.
 * - ERROR: анимация обрывается.
 */
@Composable
fun DataPacketParticles(
    state: FeatherState,
    modifier: Modifier = Modifier
) {
    if (state == FeatherState.DISCONNECTED || state == FeatherState.ERROR) {
        return
    }

    val infiniteTransition = rememberInfiniteTransition(label = "particlesTransition")
    val flowTime by infiniteTransition.animateFloat(
        initialValue = 0f,
        targetValue = 1f,
        animationSpec = infiniteRepeatable(
            animation = tween(2800, easing = LinearEasing),
            repeatMode = RepeatMode.Restart
        ),
        label = "flowTime"
    )

    Canvas(modifier = modifier) {
        val w = size.width
        val h = size.height

        val particleColor = if (state == FeatherState.CONNECTED) Brand.cyberBlue else Brand.neonOrange
        val altColor = if (state == FeatherState.CONNECTED) Brand.neonPurple else Color(0xFFFFD54F)

        val count = 16
        for (i in 0 until count) {
            val seed = (i * 0.173f)
            val progress = (flowTime + seed) % 1.0f

            // Подъем снизу вверх
            val y = h - (progress * h)

            // Распределение по X с легким схождением к центру
            val spreadX = (sin((i * 1.7f + progress * 2f).toDouble()).toFloat()) * (w * 0.28f)
            val x = (w / 2f) + spreadX

            // Плавное появление снизу и затухание у логотипа
            val alpha = (sin(progress * Math.PI).toFloat()).coerceIn(0f, 1f) * 0.85f
            val pSize = (1.8f + (i % 3) * 1.2f).dp.toPx()

            val pColor = if (i % 2 == 0) particleColor else altColor

            // Световой хвост (Trail)
            drawLine(
                brush = Brush.verticalGradient(
                    listOf(pColor.copy(alpha = alpha * 0.7f), Color.Transparent),
                    startY = y,
                    endY = y + 16f * progress
                ),
                start = Offset(x, y),
                end = Offset(x, y + 14f),
                strokeWidth = pSize * 0.8f,
                cap = StrokeCap.Round
            )

            // Ядро частицы
            drawCircle(
                color = pColor.copy(alpha = alpha),
                radius = pSize,
                center = Offset(x, y)
            )
        }
    }
}

/**
 * Текст статуса состояния:
 * - DISCONNECTED: «Нажмите для подключения» (серый)
 * - CONNECTING: «Установка туннеля... XX%» (оранжево-желтый)
 * - CONNECTED: «Туннель активен» (светло-синий)
 * - ERROR: «⚠️ Ошибка подключения. Выберите другой узел» (красный)
 */
@Composable
fun SirinStatusText(
    state: FeatherState,
    modifier: Modifier = Modifier
) {
    // Анимация прогресса процентов для CONNECTING
    var progressPercent by remember { mutableStateOf(15) }
    LaunchedEffect(state) {
        if (state == FeatherState.CONNECTING) {
            progressPercent = 18
            while (progressPercent < 95) {
                delay(120)
                progressPercent += (2..6).random()
            }
        }
    }

    when (state) {
        FeatherState.DISCONNECTED -> {
            Text(
                text = "Нажмите для подключения",
                color = Brand.dim,
                fontSize = 15.sp,
                fontWeight = FontWeight.Normal,
                textAlign = TextAlign.Center,
                modifier = modifier
            )
        }
        FeatherState.CONNECTING -> {
            Text(
                text = "Установка туннеля... $progressPercent%",
                color = Brand.neonOrange,
                fontSize = 15.sp,
                fontWeight = FontWeight.Medium,
                textAlign = TextAlign.Center,
                modifier = modifier
            )
        }
        FeatherState.CONNECTED -> {
            Text(
                text = "Туннель активен",
                color = Brand.cyberBlue,
                fontSize = 16.sp,
                fontWeight = FontWeight.SemiBold,
                textAlign = TextAlign.Center,
                modifier = modifier
            )
        }
        FeatherState.ERROR -> {
            Text(
                text = "⚠️ Ошибка подключения. Выберите другой узел",
                color = Brand.neonRed,
                fontSize = 14.sp,
                fontWeight = FontWeight.Medium,
                textAlign = TextAlign.Center,
                modifier = modifier
            )
        }
    }
}

/**
 * 3. Круглая кнопка питания (Power Button).
 * - DISCONNECTED: Приглушенный серо-синий контур, без свечения.
 * - CONNECTING: Пульсирует (breathing animation) желто-оранжевым цветом.
 * - CONNECTED: Яркое неоновое сине-пурпурное свечение, классический значок "Power" внутри активен.
 * - ERROR: Горит красным цветом.
 */
@Composable
fun SirinPowerButton(
    state: FeatherState,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    onClick: () -> Unit
) {
    val infiniteTransition = rememberInfiniteTransition(label = "powerButtonTransition")

    // Breathing пульсация для CONNECTING (1.1 сек)
    val breatheScale by infiniteTransition.animateFloat(
        initialValue = 1.0f,
        targetValue = 1.08f,
        animationSpec = infiniteRepeatable(
            animation = tween(1100, easing = FastOutSlowInEasing),
            repeatMode = RepeatMode.Reverse
        ),
        label = "breatheScale"
    )

    // Расширяющееся внешнее кольцо пульсации для CONNECTING
    val pulseRingRadius by infiniteTransition.animateFloat(
        initialValue = 0f,
        targetValue = 1f,
        animationSpec = infiniteRepeatable(
            animation = tween(1600, easing = LinearEasing),
            repeatMode = RepeatMode.Restart
        ),
        label = "pulseRingRadius"
    )

    val interactionSource = remember { MutableInteractionSource() }
    val isPressed by interactionSource.collectIsPressedAsState()

    val pressScale by animateFloatAsState(
        targetValue = if (isPressed && enabled) 0.93f else 1.0f,
        animationSpec = spring(dampingRatio = 0.6f, stiffness = 700f),
        label = "pressScale"
    )

    val currentScale = pressScale * if (state == FeatherState.CONNECTING) breatheScale else 1.0f

    Box(
        modifier = modifier
            .size(96.dp)
            .graphicsLayer {
                scaleX = currentScale
                scaleY = currentScale
            }
            .clip(CircleShape)
            .clickable(
                interactionSource = interactionSource,
                indication = null,
                enabled = enabled,
                onClick = onClick
            ),
        contentAlignment = Alignment.Center
    ) {
        Canvas(modifier = Modifier.fillMaxSize()) {
            val cx = size.width / 2f
            val cy = size.height / 2f
            val radius = size.minDimension / 2f - 4.dp.toPx()

            // 1. Анимация свечения / волн вокруг кнопки
            when (state) {
                FeatherState.DISCONNECTED -> {
                    // Без свечения, темный фон карточки
                    drawCircle(
                        color = Color(0xFF151623),
                        radius = radius,
                        center = Offset(cx, cy)
                    )
                    // Приглушенный серо-синий контур
                    drawCircle(
                        color = Brand.mutedOutline,
                        radius = radius,
                        center = Offset(cx, cy),
                        style = Stroke(width = 2.dp.toPx())
                    )
                }

                FeatherState.CONNECTING -> {
                    // Внешнее пульсирующее кольцо
                    val outerR = radius + pulseRingRadius * 16.dp.toPx()
                    drawCircle(
                        color = Brand.neonOrange.copy(alpha = (1f - pulseRingRadius) * 0.45f),
                        radius = outerR,
                        center = Offset(cx, cy),
                        style = Stroke(width = 2.dp.toPx())
                    )
                    // Внутреннее свечение
                    drawCircle(
                        brush = Brush.radialGradient(
                            0.0f to Brand.neonOrange.copy(alpha = 0.40f),
                            0.7f to Brand.neonOrange.copy(alpha = 0.12f),
                            1.0f to Color.Transparent,
                            center = Offset(cx, cy),
                            radius = radius * 1.2f
                        ),
                        radius = radius * 1.2f,
                        center = Offset(cx, cy)
                    )
                    drawCircle(
                        color = Color(0xFF1A1715),
                        radius = radius,
                        center = Offset(cx, cy)
                    )
                    drawCircle(
                        color = Brand.neonOrange,
                        radius = radius,
                        center = Offset(cx, cy),
                        style = Stroke(width = 2.5f.dp.toPx())
                    )
                }

                FeatherState.CONNECTED -> {
                    // Яркое неоновое сине-пурпурное свечение (Aura)
                    drawCircle(
                        brush = Brush.radialGradient(
                            0.0f to Brand.cyberBlue.copy(alpha = 0.55f),
                            0.5f to Brand.neonPurple.copy(alpha = 0.25f),
                            1.0f to Color.Transparent,
                            center = Offset(cx, cy),
                            radius = radius * 1.45f
                        ),
                        radius = radius * 1.45f,
                        center = Offset(cx, cy)
                    )
                    // Градиентная подложка кнопки
                    drawCircle(
                        brush = Brush.radialGradient(
                            listOf(Color(0xFF16253D), Color(0xFF121422)),
                            center = Offset(cx, cy),
                            radius = radius
                        ),
                        radius = radius,
                        center = Offset(cx, cy)
                    )
                    // Градиентная неоновая рамка кнопки
                    drawCircle(
                        brush = Brush.linearGradient(
                            listOf(Brand.cyberBlue, Brand.neonPurple),
                            start = Offset(cx - radius, cy - radius),
                            end = Offset(cx + radius, cy + radius)
                        ),
                        radius = radius,
                        center = Offset(cx, cy),
                        style = Stroke(width = 3.dp.toPx())
                    )
                }

                FeatherState.ERROR -> {
                    // Красное свечение ошибки
                    drawCircle(
                        brush = Brush.radialGradient(
                            0.0f to Brand.neonRed.copy(alpha = 0.55f),
                            0.6f to Brand.neonRed.copy(alpha = 0.15f),
                            1.0f to Color.Transparent,
                            center = Offset(cx, cy),
                            radius = radius * 1.3f
                        ),
                        radius = radius * 1.3f,
                        center = Offset(cx, cy)
                    )
                    drawCircle(
                        color = Color(0xFF221118),
                        radius = radius,
                        center = Offset(cx, cy)
                    )
                    drawCircle(
                        color = Brand.neonRed,
                        radius = radius,
                        center = Offset(cx, cy),
                        style = Stroke(width = 2.5f.dp.toPx())
                    )
                }
            }

            // 2. Классический значок "Power" внутри (IEC 60417-5009)
            drawPowerIcon(cx, cy, radius * 0.44f, state)
        }
    }
}

/**
 * Отрисовка классического символа питания (Power Symbol IEC 60417-5009):
 * Разомкнутая окружность с верхним симметричным вырезом и вертикальной чертой сверху в центр.
 */
private fun DrawScope.drawPowerIcon(
    cx: Float,
    cy: Float,
    iconRadius: Float,
    state: FeatherState
) {
    val strokeWidth = 3.6f.dp.toPx()

    val iconColor = when (state) {
        FeatherState.DISCONNECTED -> Color(0xFF5A6678)
        FeatherState.CONNECTING -> Brand.neonOrange
        FeatherState.CONNECTED -> Color.White
        FeatherState.ERROR -> Brand.neonRed
    }

    // Разомкнутая окружность с вырезом строго сверху (-90° / 12 часов)
    // Симметричный вырез 65°: дуга начинается от -57.5° и идет по часовой стрелке 295°
    drawArc(
        color = iconColor,
        startAngle = -57.5f,
        sweepAngle = 295f,
        useCenter = false,
        topLeft = Offset(cx - iconRadius, cy - iconRadius),
        size = Size(iconRadius * 2f, iconRadius * 2f),
        style = Stroke(width = strokeWidth, cap = StrokeCap.Round)
    )

    // Строго вертикальный штрих питания сверху через вырез
    drawLine(
        color = iconColor,
        start = Offset(cx, cy - iconRadius * 1.15f),
        end = Offset(cx, cy + 1.dp.toPx()),
        strokeWidth = strokeWidth,
        cap = StrokeCap.Round
    )
}
