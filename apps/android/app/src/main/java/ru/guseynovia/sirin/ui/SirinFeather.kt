package ru.guseynovia.sirin.ui

import androidx.compose.animation.core.*
import androidx.compose.foundation.Canvas
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
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Rect
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.*
import androidx.compose.ui.graphics.drawscope.DrawScope
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.drawscope.withTransform
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.delay
import ru.guseynovia.sirin.Brand
import kotlin.math.cos
import kotlin.math.sin

enum class FeatherState {
    DISCONNECTED,
    CONNECTING,
    CONNECTED,
    ERROR
}

/**
 * Главный визуальный блок экрана SIRIN VPN (State Machine):
 * - Центральный элемент: Логотип Сирин с замком
 * - Анимация потока пакетов/частиц
 * - Интерактивный статус
 * - Круглая кнопка питания (Power Button)
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
        // 1. Центральный элемент — Логотип Сирин с замком и цепями
        SirinLogoWithPadlock(
            state = state,
            modifier = Modifier.size(210.dp)
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
 * 1. Центральный элемент: Логотип Сирин с замком (Cyber-Neon).
 * - Мифическая дева-птица Сирин с короной (кокошником), симметричными кибер-крыльями,
 *   цифровым замком и разорванными цифровыми цепями.
 * - 4 состояния:
 *   - DISCONNECTED: Затемненный (opacity 65%), замок не светится.
 *   - CONNECTING: Цепи и замок поочередно вспыхивают (рукопожатие / handshake).
 *   - CONNECTED: 100% яркость, замок зажжен (вспышка при переходе в стейт).
 *   - ERROR: Glitch-эффект на 0.5 сек при сбое, замок горит красным.
 */
@Composable
fun SirinLogoWithPadlock(
    state: FeatherState,
    modifier: Modifier = Modifier
) {
    val infiniteTransition = rememberInfiniteTransition(label = "sirinLogoTransition")

    // Парение крыльев для CONNECTED
    val hoverY by infiniteTransition.animateFloat(
        initialValue = -3f,
        targetValue = 3f,
        animationSpec = infiniteRepeatable(
            animation = tween(2400, easing = FastOutSlowInEasing),
            repeatMode = RepeatMode.Reverse
        ),
        label = "hoverY"
    )

    // Handshake: поочередное мерцание цепей и замка для CONNECTING (1.2 сек)
    val handshakePhase by infiniteTransition.animateFloat(
        initialValue = 0f,
        targetValue = 1f,
        animationSpec = infiniteRepeatable(
            animation = tween(1200, easing = LinearEasing),
            repeatMode = RepeatMode.Restart
        ),
        label = "handshakePhase"
    )

    // Вспышка замка при входе в CONNECTED
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
        initialValue = -5f,
        targetValue = 5f,
        animationSpec = infiniteRepeatable(
            animation = tween(50, easing = LinearEasing),
            repeatMode = RepeatMode.Reverse
        ),
        label = "glitchJitter"
    )

    Canvas(modifier = modifier) {
        val cx = size.width / 2f
        val cy = size.height / 2f
        val scale = size.minDimension / 210f

        val glitchOffsetX = if (isGlitching) glitchJitter * scale else 0f
        val currentOffsetY = if (state == FeatherState.CONNECTED) hoverY * scale else 0f

        val alpha = when (state) {
            FeatherState.DISCONNECTED -> 0.65f
            FeatherState.CONNECTING -> 0.90f
            FeatherState.CONNECTED -> 1.0f
            FeatherState.ERROR -> 0.95f
        }

        withTransform({
            translate(left = glitchOffsetX, top = currentOffsetY)
        }) {
            // Внешнее свечение ореола
            if (state == FeatherState.CONNECTED || state == FeatherState.CONNECTING) {
                val haloColor = if (state == FeatherState.CONNECTED) Brand.cyberBlue else Brand.neonOrange
                drawCircle(
                    brush = Brush.radialGradient(
                        0.0f to haloColor.copy(alpha = 0.25f * flashAnim.value),
                        0.5f to haloColor.copy(alpha = 0.08f),
                        1.0f to Color.Transparent,
                        center = Offset(cx, cy - 10f * scale),
                        radius = 100f * scale
                    ),
                    radius = 100f * scale,
                    center = Offset(cx, cy - 10f * scale)
                )
            }

            // Отрисовка силуэта птицы-девы Сирин (Корона, Лик, Крылья)
            drawSirinFigure(cx, cy, scale, state, alpha)

            // Разорванные цифровые цепи и летающие кибер-кубы
            drawBrokenChainsAndCubes(cx, cy, scale, state, handshakePhase, alpha)

            // Центральный цифровой замок (Padlock)
            drawDigitalPadlock(cx, cy, scale, state, handshakePhase, flashAnim.value, alpha)
        }
    }
}

/**
 * Отрисовка фигуры Сирин (Кокошник, лик девы, крылья-щиты).
 */
private fun DrawScope.drawSirinFigure(
    cx: Float,
    cy: Float,
    scale: Float,
    state: FeatherState,
    baseAlpha: Float
) {
    val wingStroke = 1.8f.dp.toPx()
    val crownStroke = 2.0f.dp.toPx()

    val wingColor = when (state) {
        FeatherState.DISCONNECTED -> Brand.mutedOutline
        FeatherState.CONNECTING -> Brand.neonOrange.copy(alpha = 0.8f)
        FeatherState.CONNECTED -> Brand.cyberBlue
        FeatherState.ERROR -> Brand.neonRed.copy(alpha = 0.8f)
    }

    val primaryBrush = when (state) {
        FeatherState.DISCONNECTED -> Brush.linearGradient(
            listOf(Color(0xFF384358), Color(0xFF222938))
        )
        FeatherState.ERROR -> Brush.linearGradient(
            listOf(Brand.neonRed, Color(0xFF881028))
        )
        FeatherState.CONNECTING -> Brush.linearGradient(
            listOf(Brand.neonOrange, Color(0xFFC77800))
        )
        FeatherState.CONNECTED -> Brush.linearGradient(
            listOf(Brand.cyberBlue, Brand.neonPurple),
            start = Offset(cx - 90f * scale, cy - 80f * scale),
            end = Offset(cx + 90f * scale, cy + 80f * scale)
        )
    }

    // 1. Корона Сирин (Кокошник) с кибер-орнаментом
    val crownPath = Path().apply {
        moveTo(cx, cy - 88f * scale)
        cubicTo(cx + 18f * scale, cy - 82f * scale, cx + 32f * scale, cy - 66f * scale, cx + 36f * scale, cy - 46f * scale)
        lineTo(cx - 36f * scale, cy - 46f * scale)
        cubicTo(cx - 32f * scale, cy - 66f * scale, cx - 18f * scale, cy - 82f * scale, cx, cy - 88f * scale)
        close()
    }
    drawPath(crownPath, primaryBrush, alpha = baseAlpha * 0.85f)
    drawPath(
        crownPath,
        wingColor.copy(alpha = baseAlpha),
        style = Stroke(width = crownStroke)
    )

    // Кристалл / верхушка кокошника
    drawCircle(
        color = if (state == FeatherState.CONNECTED) Color.White else wingColor,
        radius = 3.5f * scale,
        center = Offset(cx, cy - 88f * scale)
    )

    // 2. Лик девы (минималистичный кибер-контур)
    val facePath = Path().apply {
        moveTo(cx - 16f * scale, cy - 46f * scale)
        cubicTo(cx - 16f * scale, cy - 22f * scale, cx - 8f * scale, cy - 14f * scale, cx, cy - 8f * scale)
        cubicTo(cx + 8f * scale, cy - 14f * scale, cx + 16f * scale, cy - 22f * scale, cx + 16f * scale, cy - 46f * scale)
    }
    drawPath(
        facePath,
        color = Color(0xFFE2E8F0).copy(alpha = baseAlpha * 0.9f),
        style = Stroke(width = 1.6f.dp.toPx(), cap = StrokeCap.Round)
    )

    // Глаза девы (тонкие кибер-штрихи)
    drawLine(
        color = wingColor,
        start = Offset(cx - 10f * scale, cy - 28f * scale),
        end = Offset(cx - 4f * scale, cy - 26f * scale),
        strokeWidth = 1.6f.dp.toPx(),
        cap = StrokeCap.Round
    )
    drawLine(
        color = wingColor,
        start = Offset(cx + 10f * scale, cy - 28f * scale),
        end = Offset(cx + 4f * scale, cy - 26f * scale),
        strokeWidth = 1.6f.dp.toPx(),
        cap = StrokeCap.Round
    )

    // 3. Величественные симметричные крылья Сирин (Левое и Правое)
    // Левое крыло
    val leftWing = Path().apply {
        moveTo(cx - 24f * scale, cy - 36f * scale)
        cubicTo(cx - 60f * scale, cy - 65f * scale, cx - 85f * scale, cy - 40f * scale, cx - 94f * scale, cy - 8f * scale)
        cubicTo(cx - 86f * scale, cy + 22f * scale, cx - 65f * scale, cy + 46f * scale, cx - 35f * scale, cy + 54f * scale)
        cubicTo(cx - 48f * scale, cy + 32f * scale, cx - 55f * scale, cy + 8f * scale, cx - 44f * scale, cy - 12f * scale)
        cubicTo(cx - 36f * scale, cy - 24f * scale, cx - 28f * scale, cy - 30f * scale, cx - 24f * scale, cy - 36f * scale)
        close()
    }
    drawPath(leftWing, primaryBrush, alpha = baseAlpha * 0.45f)
    drawPath(leftWing, wingColor.copy(alpha = baseAlpha), style = Stroke(width = wingStroke))

    // Внутренние перья левого крыла
    for (i in 1..3) {
        val fProgress = i / 4f
        val startPt = Offset(cx - 35f * scale - fProgress * 20f * scale, cy - 20f * scale + fProgress * 30f * scale)
        val endPt = Offset(cx - 80f * scale + fProgress * 25f * scale, cy - 25f * scale + fProgress * 48f * scale)
        drawLine(wingColor.copy(alpha = baseAlpha * 0.7f), startPt, endPt, strokeWidth = 1.3f.dp.toPx(), cap = StrokeCap.Round)
    }

    // Правое крыло (зеркально)
    val rightWing = Path().apply {
        moveTo(cx + 24f * scale, cy - 36f * scale)
        cubicTo(cx + 60f * scale, cy - 65f * scale, cx + 85f * scale, cy - 40f * scale, cx + 94f * scale, cy - 8f * scale)
        cubicTo(cx + 86f * scale, cy + 22f * scale, cx + 65f * scale, cy + 46f * scale, cx + 35f * scale, cy + 54f * scale)
        cubicTo(cx + 48f * scale, cy + 32f * scale, cx + 55f * scale, cy + 8f * scale, cx + 44f * scale, cy - 12f * scale)
        cubicTo(cx + 36f * scale, cy - 24f * scale, cx + 28f * scale, cy - 30f * scale, cx + 24f * scale, cy - 36f * scale)
        close()
    }
    drawPath(rightWing, primaryBrush, alpha = baseAlpha * 0.45f)
    drawPath(rightWing, wingColor.copy(alpha = baseAlpha), style = Stroke(width = wingStroke))

    // Внутренние перья правого крыла
    for (i in 1..3) {
        val fProgress = i / 4f
        val startPt = Offset(cx + 35f * scale + fProgress * 20f * scale, cy - 20f * scale + fProgress * 30f * scale)
        val endPt = Offset(cx + 80f * scale - fProgress * 25f * scale, cy - 25f * scale + fProgress * 48f * scale)
        drawLine(wingColor.copy(alpha = baseAlpha * 0.7f), startPt, endPt, strokeWidth = 1.3f.dp.toPx(), cap = StrokeCap.Round)
    }
}

/**
 * Разорванные цифровые цепи и плавающие кибер-кубы по бокам от замка.
 */
private fun DrawScope.drawBrokenChainsAndCubes(
    cx: Float,
    cy: Float,
    scale: Float,
    state: FeatherState,
    handshakePhase: Float,
    baseAlpha: Float
) {
    val chainAlpha = when (state) {
        FeatherState.DISCONNECTED -> 0.35f
        FeatherState.CONNECTING -> (0.35f + 0.65f * sin(handshakePhase * 2 * Math.PI.toFloat()).coerceIn(0f, 1f))
        FeatherState.CONNECTED -> 0.90f
        FeatherState.ERROR -> 0.60f
    } * baseAlpha

    val chainColor = when (state) {
        FeatherState.DISCONNECTED -> Brand.mutedOutline
        FeatherState.CONNECTING -> Brand.neonOrange
        FeatherState.CONNECTED -> Brand.cyberBlue
        FeatherState.ERROR -> Brand.neonRed
    }

    val strokeWidth = 1.7f.dp.toPx()
    val lockY = cy + 28f * scale

    // Левая разорванная цепь (2 звена с разрывом)
    val leftLink1Center = Offset(cx - 36f * scale, lockY)
    drawRoundRect(
        color = chainColor.copy(alpha = chainAlpha),
        topLeft = Offset(leftLink1Center.x - 9f * scale, leftLink1Center.y - 6f * scale),
        size = Size(18f * scale, 12f * scale),
        cornerRadius = CornerRadius(4f * scale),
        style = Stroke(width = strokeWidth)
    )
    val leftLink2Center = Offset(cx - 56f * scale, lockY - 6f * scale)
    drawRoundRect(
        color = chainColor.copy(alpha = chainAlpha * 0.7f),
        topLeft = Offset(leftLink2Center.x - 8f * scale, leftLink2Center.y - 5f * scale),
        size = Size(16f * scale, 10f * scale),
        cornerRadius = CornerRadius(3f * scale),
        style = Stroke(width = strokeWidth)
    )

    // Правая разорванная цепь
    val rightLink1Center = Offset(cx + 36f * scale, lockY)
    drawRoundRect(
        color = chainColor.copy(alpha = chainAlpha),
        topLeft = Offset(rightLink1Center.x - 9f * scale, rightLink1Center.y - 6f * scale),
        size = Size(18f * scale, 12f * scale),
        cornerRadius = CornerRadius(4f * scale),
        style = Stroke(width = strokeWidth)
    )
    val rightLink2Center = Offset(cx + 56f * scale, lockY - 6f * scale)
    drawRoundRect(
        color = chainColor.copy(alpha = chainAlpha * 0.7f),
        topLeft = Offset(rightLink2Center.x - 8f * scale, rightLink2Center.y - 5f * scale),
        size = Size(16f * scale, 10f * scale),
        cornerRadius = CornerRadius(3f * scale),
        style = Stroke(width = strokeWidth)
    )

    // Плавающие кибер-кубы (Cyber-Cubes / ромбы)
    val cubeOffsets = listOf(
        Offset(cx - 68f * scale, lockY + 20f * scale),
        Offset(cx + 68f * scale, lockY + 20f * scale),
        Offset(cx - 48f * scale, lockY + 36f * scale),
        Offset(cx + 48f * scale, lockY + 36f * scale)
    )
    for ((idx, pt) in cubeOffsets.withIndex()) {
        val cubeSize = (5f + (idx % 2) * 3f) * scale
        withTransform({
            translate(left = pt.x, top = pt.y)
            rotate(45f, pivot = Offset.Zero)
        }) {
            drawRect(
                color = chainColor.copy(alpha = chainAlpha * 0.6f),
                topLeft = Offset(-cubeSize / 2f, -cubeSize / 2f),
                size = Size(cubeSize, cubeSize),
                style = Stroke(width = 1.2f.dp.toPx())
            )
        }
    }
}

/**
 * Центральный цифровой замок (Padlock) Сирин.
 */
private fun DrawScope.drawDigitalPadlock(
    cx: Float,
    cy: Float,
    scale: Float,
    state: FeatherState,
    handshakePhase: Float,
    flashMultiplier: Float,
    baseAlpha: Float
) {
    val lockY = cy + 28f * scale
    val lockW = 34f * scale
    val lockH = 26f * scale

    val isLockActive = (state == FeatherState.CONNECTED)
    val isConnecting = (state == FeatherState.CONNECTING)
    val isError = (state == FeatherState.ERROR)

    // Handshake: замок вспыхивает в противофазе цепям
    val lockGlowAlpha = when {
        isLockActive -> 1.0f * flashMultiplier
        isConnecting -> (0.3f + 0.7f * cos(handshakePhase * 2 * Math.PI.toFloat()).coerceIn(0f, 1f))
        isError -> 1.0f
        else -> 0.15f
    }

    val lockColor = when {
        isError -> Brand.neonRed
        isConnecting -> Brand.neonOrange
        isLockActive -> Brand.cyberBlue
        else -> Color(0xFF384358)
    }

    // 1. Свечение вокруг замка
    if (isLockActive || isConnecting || isError) {
        val glowRadius = 32f * scale * flashMultiplier
        drawCircle(
            brush = Brush.radialGradient(
                0.0f to lockColor.copy(alpha = 0.45f * lockGlowAlpha * baseAlpha),
                0.6f to lockColor.copy(alpha = 0.12f * lockGlowAlpha * baseAlpha),
                1.0f to Color.Transparent,
                center = Offset(cx, lockY),
                radius = glowRadius
            ),
            radius = glowRadius,
            center = Offset(cx, lockY)
        )
    }

    // 2. Дужка замка (Shackle)
    val shackleRadius = 10f * scale
    val shackleStroke = 2.4f.dp.toPx()
    val shacklePath = Path().apply {
        moveTo(cx - shackleRadius, lockY - lockH / 2f)
        lineTo(cx - shackleRadius, lockY - lockH / 2f - 9f * scale)
        cubicTo(
            cx - shackleRadius, lockY - lockH / 2f - 19f * scale,
            cx + shackleRadius, lockY - lockH / 2f - 19f * scale,
            cx + shackleRadius, lockY - lockH / 2f - 9f * scale
        )
        lineTo(cx + shackleRadius, lockY - lockH / 2f)
    }
    drawPath(
        shacklePath,
        color = lockColor.copy(alpha = baseAlpha),
        style = Stroke(width = shackleStroke, cap = StrokeCap.Round)
    )

    // 3. Корпус замка (Body)
    val bodyRect = Rect(cx - lockW / 2f, lockY - lockH / 2f, cx + lockW / 2f, lockY + lockH / 2f)
    val cornerRadius = CornerRadius(6f * scale)

    // Заливка корпуса
    val bodyBrush = when {
        isLockActive -> Brush.verticalGradient(
            listOf(Color(0xFF0F1E38), Color(0xFF15162B))
        )
        isError -> Brush.verticalGradient(
            listOf(Color(0xFF2B0E14), Color(0xFF15101A))
        )
        isConnecting -> Brush.verticalGradient(
            listOf(Color(0xFF2B1E0E), Color(0xFF15141A))
        )
        else -> Brush.verticalGradient(
            listOf(Color(0xFF151824), Color(0xFF0E1018))
        )
    }
    drawRoundRect(
        brush = bodyBrush,
        topLeft = Offset(bodyRect.left, bodyRect.top),
        size = Size(bodyRect.width, bodyRect.height),
        cornerRadius = cornerRadius,
        alpha = baseAlpha
    )

    // Обводка корпуса
    drawRoundRect(
        color = lockColor.copy(alpha = baseAlpha),
        topLeft = Offset(bodyRect.left, bodyRect.top),
        size = Size(bodyRect.width, bodyRect.height),
        cornerRadius = cornerRadius,
        style = Stroke(width = 2.dp.toPx())
    )

    // 4. Скважина / цифровой чип по центру замка
    val keyholeY = lockY - 2f * scale
    val keyholeColor = if (isLockActive) Color.White else lockColor
    drawCircle(
        color = keyholeColor.copy(alpha = baseAlpha),
        radius = 3f * scale,
        center = Offset(cx, keyholeY)
    )
    val keyPath = Path().apply {
        moveTo(cx - 2f * scale, keyholeY)
        lineTo(cx + 2f * scale, keyholeY)
        lineTo(cx + 1.2f * scale, keyholeY + 8f * scale)
        lineTo(cx - 1.2f * scale, keyholeY + 8f * scale)
        close()
    }
    drawPath(keyPath, color = keyholeColor.copy(alpha = baseAlpha))
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
    var progressPercent by remember { mutableStateOf(12) }
    LaunchedEffect(state) {
        if (state == FeatherState.CONNECTING) {
            progressPercent = 15
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
 * - CONNECTED: Яркое неоновое сине-пурпурное свечение, значок "Power" внутри активен.
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

    // Breathing пульсация для CONNECTING (1.2 сек)
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

            // 1. Анимация свечения / волн
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

            // 2. Значок "Power" внутри (круговая дуга + вертикальный луч)
            drawPowerIcon(cx, cy, radius * 0.45f, state)
        }
    }
}

/**
 * Отрисовка символа питания (Power Symbol ⏻).
 */
private fun DrawScope.drawPowerIcon(
    cx: Float,
    cy: Float,
    iconRadius: Float,
    state: FeatherState
) {
    val strokeWidth = 3.2f.dp.toPx()

    val iconColor = when (state) {
        FeatherState.DISCONNECTED -> Color(0xFF5A6678)
        FeatherState.CONNECTING -> Brand.neonOrange
        FeatherState.CONNECTED -> Color.White
        FeatherState.ERROR -> Brand.neonRed
    }

    // Разомкнутая круговая дуга (от 50° до 310°)
    drawArc(
        color = iconColor,
        startAngle = 135f,
        sweepAngle = 270f,
        useCenter = false,
        topLeft = Offset(cx - iconRadius, cy - iconRadius),
        size = Size(iconRadius * 2f, iconRadius * 2f),
        style = Stroke(width = strokeWidth, cap = StrokeCap.Round)
    )

    // Вертикальный штрих питания сверху
    drawLine(
        color = iconColor,
        start = Offset(cx, cy - iconRadius * 1.2f),
        end = Offset(cx, cy - iconRadius * 0.1f),
        strokeWidth = strokeWidth,
        cap = StrokeCap.Round
    )
}
