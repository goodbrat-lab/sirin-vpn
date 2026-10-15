package ru.guseynovia.sirin.ui

import androidx.compose.animation.core.*
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.*
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.*
import androidx.compose.ui.graphics.drawscope.*
import androidx.compose.ui.unit.dp
import kotlin.math.cos
import kotlin.math.sin

enum class FeatherState {
    DISCONNECTED,
    CONNECTING,
    CONNECTED,
    ERROR
}

/**
 * Визуальный элемент "Перо Сирин" (Sirin Feather).
 * Символ птицы Сирин с четырьмя состояниями туннеля:
 * - DISCONNECTED: приглушённое серое перо (#3B4452 -> #4A5568 -> #5A6678), обводка 1px #6B7280 (0.4), наклон 0°.
 * - CONNECTING: золотое перо, наклон 0°→15°, пульсация 1.5 с.
 * - CONNECTED: парящее золотое перо, асимметрия +12%, очин, тень 60x8px на cy+60, покачивание ±3px, искры.
 * - ERROR: красное перо, наклон 30° (упало), мелкое дрожание.
 */
@Composable
fun SirinFeather(
    state: FeatherState,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    onClick: () -> Unit = {},
) {
    val infiniteTransition = rememberInfiniteTransition(label = "sirinFeather")

    // Парение по Y для CONNECTED (±3 px)
    val floatY by infiniteTransition.animateFloat(
        initialValue = -3f,
        targetValue = 3f,
        animationSpec = infiniteRepeatable(
            animation = tween(2200, easing = FastOutSlowInEasing),
            repeatMode = RepeatMode.Reverse
        ),
        label = "floatY"
    )

    // Плавное покачивание по углу для CONNECTED (±2°)
    val swayAngle by infiniteTransition.animateFloat(
        initialValue = -2f,
        targetValue = 2f,
        animationSpec = infiniteRepeatable(
            animation = tween(3200, easing = FastOutSlowInEasing),
            repeatMode = RepeatMode.Reverse
        ),
        label = "swayAngle"
    )

    // Пульсация свечения для CONNECTING (1.5 сек)
    val pulseAlpha by infiniteTransition.animateFloat(
        initialValue = 0.35f,
        targetValue = 1.0f,
        animationSpec = infiniteRepeatable(
            animation = tween(1500, easing = FastOutSlowInEasing),
            repeatMode = RepeatMode.Reverse
        ),
        label = "pulseAlpha"
    )

    // Подъём угла при CONNECTING (0° → 15°)
    val connectingAngle by infiniteTransition.animateFloat(
        initialValue = 0f,
        targetValue = 15f,
        animationSpec = infiniteRepeatable(
            animation = tween(1500, easing = FastOutSlowInEasing),
            repeatMode = RepeatMode.Reverse
        ),
        label = "connectingAngle"
    )

    // Быстрое дрожание при ERROR (±2°)
    val jitterAngle by infiniteTransition.animateFloat(
        initialValue = -2f,
        targetValue = 2f,
        animationSpec = infiniteRepeatable(
            animation = tween(60, easing = LinearEasing),
            repeatMode = RepeatMode.Reverse
        ),
        label = "jitterAngle"
    )

    // Вращение орбитальных искр для CONNECTED
    val sparkAngle by infiniteTransition.animateFloat(
        initialValue = 0f,
        targetValue = 360f,
        animationSpec = infiniteRepeatable(
            animation = tween(9000, easing = LinearEasing),
            repeatMode = RepeatMode.Restart
        ),
        label = "sparkAngle"
    )

    val presses = remember { MutableInteractionSource() }

    Canvas(
        modifier = modifier
            .size(200.dp)
            .clickable(
                interactionSource = presses,
                indication = null,
                enabled = enabled,
                onClick = onClick,
            )
    ) {
        val cx = size.width / 2f
        val cy = size.height / 2f
        val scale = size.minDimension / 200f

        val offsetY = if (state == FeatherState.CONNECTED) floatY * scale else 0f

        // Угол поворота относительно вертикали
        val angle = when (state) {
            FeatherState.DISCONNECTED -> 0f
            FeatherState.CONNECTING -> connectingAngle
            FeatherState.CONNECTED -> 15f + swayAngle
            FeatherState.ERROR -> 30f + jitterAngle
        }

        // 1. Тень под пером (только CONNECTED, cy + 60, эллипс 60x8px, blur 4px)
        if (state == FeatherState.CONNECTED) {
            val shadowCenter = Offset(cx, cy + 60f * scale)
            val shadowWidth = 60f * scale
            val shadowHeight = 8f * scale
            drawOval(
                brush = Brush.radialGradient(
                    0.0f to Color.Black.copy(alpha = 0.30f),
                    0.6f to Color.Black.copy(alpha = 0.12f),
                    1.0f to Color.Transparent,
                    center = shadowCenter,
                    radius = shadowWidth / 2f
                ),
                topLeft = Offset(shadowCenter.x - shadowWidth / 2f, shadowCenter.y - shadowHeight / 2f),
                size = Size(shadowWidth, shadowHeight)
            )
        }

        // 2. Радиальное свечение (Connecting, Connected, Error)
        if (state != FeatherState.DISCONNECTED) {
            val glowColor = when (state) {
                FeatherState.ERROR -> Color(0xFFEF4444)
                FeatherState.CONNECTING -> Color(0xFFE8C36F).copy(alpha = pulseAlpha)
                FeatherState.CONNECTED -> Color(0xFFE8C36F)
                FeatherState.DISCONNECTED -> Color.Transparent
            }
            val glowRadius = 110f * scale
            val featherCenterY = cy - 8f * scale + offsetY
            drawCircle(
                brush = Brush.radialGradient(
                    0.0f to glowColor.copy(alpha = 0.35f * (if (state == FeatherState.CONNECTING) pulseAlpha else 1f)),
                    0.6f to glowColor.copy(alpha = 0.12f * (if (state == FeatherState.CONNECTING) pulseAlpha else 1f)),
                    1.0f to Color.Transparent,
                    center = Offset(cx, featherCenterY),
                    radius = glowRadius
                ),
                radius = glowRadius,
                center = Offset(cx, featherCenterY)
            )
        }

        // 3. Отрисовка пера из logo_sirin_feather_v2.svg
        withTransform({
            translate(left = cx, top = cy - 8f * scale + offsetY)
            rotate(degrees = angle, pivot = Offset.Zero)
            scale(scale = scale * 0.31f, pivot = Offset.Zero)
            translate(left = -256f, top = -256f)
        }) {
            drawFeatherVector(state)
        }

        // 4. Искры вокруг пера (только CONNECTED)
        if (state == FeatherState.CONNECTED) {
            drawSparks(cx, cy - 8f * scale + offsetY, scale, sparkAngle)
        }
    }
}

/**
 * Отрисовка геометрии пера по точным векторным кривым logo_sirin_feather_v2.svg
 */
private fun DrawScope.drawFeatherVector(state: FeatherState) {
    // 1. Палитра и градиенты
    val isDisconnected = (state == FeatherState.DISCONNECTED)

    // Вертикальный градиент для DISCONNECTED: #3B4452 -> #4A5568 -> #5A6678
    val disconnectedBrush = Brush.verticalGradient(
        colors = listOf(Color(0xFF5A6678), Color(0xFF4A5568), Color(0xFF3B4452)),
        startY = 50f,
        endY = 400f
    )
    val disconnectedStroke = Color(0xFF6B7280).copy(alpha = 0.40f)

    val goldDarkBrush = when (state) {
        FeatherState.DISCONNECTED -> disconnectedBrush
        FeatherState.ERROR -> Brush.linearGradient(
            colors = listOf(Color(0xFF7F1D1D), Color(0xFFB91C1C), Color(0xFFEF4444)),
            start = Offset(190f, 400f),
            end = Offset(260f, 50f)
        )
        else -> Brush.linearGradient(
            colors = listOf(Color(0xFF6E4A0E), Color(0xFF9C7323), Color(0xFFCCA140), Color(0xFFF2D183), Color(0xFFFFF2CA)),
            start = Offset(190f, 400f),
            end = Offset(260f, 50f)
        )
    }

    val goldLightBrush = when (state) {
        FeatherState.DISCONNECTED -> disconnectedBrush
        FeatherState.ERROR -> Brush.linearGradient(
            colors = listOf(Color(0xFF991B1B), Color(0xFFDC2626), Color(0xFFF87171)),
            start = Offset(200f, 400f),
            end = Offset(320f, 50f)
        )
        else -> Brush.linearGradient(
            colors = listOf(Color(0xFF8C631A), Color(0xFFBA8F35), Color(0xFFE8C36F), Color(0xFFFCE4A6), Color(0xFFFFF8E0)),
            start = Offset(200f, 400f),
            end = Offset(320f, 50f)
        )
    }

    val calamusBrush = when (state) {
        FeatherState.DISCONNECTED -> Brush.verticalGradient(
            colors = listOf(Color(0xFF4A5568), Color(0xFF2D3540)),
            startY = 400f,
            endY = 465f
        )
        FeatherState.ERROR -> Brush.verticalGradient(
            colors = listOf(Color(0xFF7F1D1D), Color(0xFF450A0A)),
            startY = 400f,
            endY = 465f
        )
        else -> Brush.verticalGradient(
            colors = listOf(Color(0xFF8C631A), Color(0xFF6B470D), Color(0xFF4E3309)),
            startY = 400f,
            endY = 465f
        )
    }

    val shaftBrush = when (state) {
        FeatherState.DISCONNECTED -> Brush.verticalGradient(
            colors = listOf(Color(0xFF6B7A90), Color(0xFF3B4452)),
            startY = 50f,
            endY = 400f
        )
        FeatherState.ERROR -> Brush.linearGradient(
            colors = listOf(Color(0xFF7F1D1D), Color(0xFFFECACA)),
            start = Offset(256f, 400f),
            end = Offset(256f, 50f)
        )
        else -> Brush.linearGradient(
            colors = listOf(Color(0xFF6E4A0E), Color(0xFFE8C36F), Color(0xFFFFF2CA), Color(0xFFFFF8E0)),
            start = Offset(256f, 400f),
            end = Offset(256f, 50f)
        )
    }

    val blueBrush = when (state) {
        FeatherState.DISCONNECTED -> Brush.linearGradient(
            colors = listOf(Color(0xFF242C35), Color(0xFF333D4B)),
            start = Offset(220f, 400f),
            end = Offset(290f, 340f)
        )
        FeatherState.ERROR -> Brush.linearGradient(
            colors = listOf(Color(0xFF450A0A), Color(0xFF991B1B)),
            start = Offset(220f, 400f),
            end = Offset(290f, 340f)
        )
        else -> Brush.linearGradient(
            colors = listOf(Color(0xFF1E3E8A), Color(0xFF3B82F6), Color(0xFF93C5FD)),
            start = Offset(220f, 400f),
            end = Offset(290f, 340f)
        )
    }

    val tipColor = when (state) {
        FeatherState.DISCONNECTED -> Color(0xFF5A6678)
        FeatherState.ERROR -> Color(0xFFFECACA)
        else -> Color(0xFFFFF8E0)
    }

    // 2. Левое полу-опахало
    val leftVane = Path().apply {
        moveTo(254f, 398f)
        cubicTo(241f, 390f, 226f, 370f, 218f, 348f)
        cubicTo(225f, 344f, 235f, 346f, 244f, 352f)
        cubicTo(214f, 330f, 198f, 298f, 196f, 266f)
        cubicTo(206f, 262f, 218f, 266f, 229f, 274f)
        cubicTo(200f, 244f, 196f, 204f, 207f, 166f)
        cubicTo(216f, 162f, 228f, 166f, 238f, 174f)
        cubicTo(216f, 140f, 224f, 102f, 246f, 70f)
        cubicTo(252f, 60f, 256f, 54f, 260f, 50f)
        cubicTo(256f, 150f, 250f, 270f, 254f, 398f)
        close()
    }
    drawPath(leftVane, goldDarkBrush)
    if (isDisconnected) {
        drawPath(leftVane, disconnectedStroke, style = Stroke(width = 1f))
    }

    // 3. Правое полу-опахало (на 12% шире левого)
    val rightVane = Path().apply {
        moveTo(257f, 398f)
        cubicTo(272f, 390f, 292f, 370f, 302f, 348f)
        cubicTo(293f, 344f, 280f, 346f, 270f, 352f)
        cubicTo(305f, 330f, 326f, 298f, 328f, 266f)
        cubicTo(316f, 262f, 300f, 266f, 287f, 274f)
        cubicTo(324f, 244f, 329f, 204f, 316f, 166f)
        cubicTo(304f, 162f, 288f, 166f, 277f, 174f)
        cubicTo(308f, 140f, 302f, 102f, 278f, 70f)
        cubicTo(271f, 60f, 266f, 54f, 260f, 50f)
        cubicTo(258f, 150f, 253f, 270f, 257f, 398f)
        close()
    }
    drawPath(rightVane, goldLightBrush)
    if (isDisconnected) {
        drawPath(rightVane, disconnectedStroke, style = Stroke(width = 1f))
    }

    // 4. Лазурный пушок Сирин у основания
    val leftFluff = Path().apply {
        moveTo(254f, 398f)
        cubicTo(243f, 392f, 229f, 374f, 222f, 354f)
        cubicTo(229f, 350f, 238f, 354f, 246f, 362f)
        cubicTo(251f, 372f, 253f, 386f, 254f, 398f)
        close()
    }
    drawPath(leftFluff, blueBrush, alpha = if (isDisconnected) 0.5f else 0.85f)

    val rightFluff = Path().apply {
        moveTo(257f, 398f)
        cubicTo(270f, 392f, 288f, 374f, 297f, 354f)
        cubicTo(288f, 350f, 276f, 354f, 267f, 362f)
        cubicTo(261f, 372f, 258f, 386f, 257f, 398f)
        close()
    }
    drawPath(rightFluff, blueBrush, alpha = if (isDisconnected) 0.5f else 0.85f)

    // 5. Текстурные бородки (лучи пера)
    if (!isDisconnected) {
        val lightRayColor = Color(0xFFFFF8E0)
        val darkRayColor = Color(0xFFFFF2CA)

        // Правые лучи
        drawLine(lightRayColor.copy(alpha = 0.45f), Offset(256f, 352f), Offset(288f, 328f), strokeWidth = 1.5f, cap = StrokeCap.Round)
        drawLine(lightRayColor.copy(alpha = 0.45f), Offset(252f, 274f), Offset(310f, 244f), strokeWidth = 1.6f, cap = StrokeCap.Round)
        drawLine(lightRayColor.copy(alpha = 0.45f), Offset(254f, 193f), Offset(306f, 150f), strokeWidth = 1.6f, cap = StrokeCap.Round)
        drawLine(lightRayColor.copy(alpha = 0.45f), Offset(258f, 118f), Offset(288f, 82f), strokeWidth = 1.4f, cap = StrokeCap.Round)

        // Левые лучи
        drawLine(darkRayColor.copy(alpha = 0.32f), Offset(254f, 352f), Offset(228f, 330f), strokeWidth = 1.4f, cap = StrokeCap.Round)
        drawLine(darkRayColor.copy(alpha = 0.32f), Offset(250f, 274f), Offset(206f, 246f), strokeWidth = 1.5f, cap = StrokeCap.Round)
        drawLine(darkRayColor.copy(alpha = 0.32f), Offset(252f, 193f), Offset(208f, 152f), strokeWidth = 1.5f, cap = StrokeCap.Round)
        drawLine(darkRayColor.copy(alpha = 0.32f), Offset(256f, 118f), Offset(234f, 84f), strokeWidth = 1.3f, cap = StrokeCap.Round)
    }

    // 6. ОЧИН (основание): утолщение стержня (~x1.8), полая форма, цвет темнее на 15-20%
    val calamus = Path().apply {
        moveTo(253f, 465f)
        cubicTo(252f, 450f, 251.5f, 430f, 252.5f, 400f)
        lineTo(258.5f, 400f)
        cubicTo(259.5f, 430f, 259f, 450f, 258f, 465f)
        cubicTo(257.5f, 468f, 253.5f, 468f, 253f, 465f)
        close()
    }
    drawPath(calamus, calamusBrush)
    if (isDisconnected) {
        drawPath(calamus, disconnectedStroke, style = Stroke(width = 1f))
    }
    // Полый срез очина
    drawOval(
        color = if (isDisconnected) Color(0xFF242C35) else Color(0xFF3D2806).copy(alpha = 0.7f),
        topLeft = Offset(253f, 464.5f),
        size = Size(5f, 2.4f)
    )

    // 7. Центральный стержень (рахис) с живым S-изгибом 2-3%
    val rachis = Path().apply {
        moveTo(254f, 400f)
        cubicTo(249f, 280f, 253f, 160f, 259.2f, 50f)
        lineTo(260.8f, 50f)
        cubicTo(255f, 160f, 251f, 280f, 257f, 400f)
        close()
    }
    drawPath(rachis, shaftBrush)

    // 8. Световой блик на гребне стержня
    if (!isDisconnected) {
        val highlightPath = Path().apply {
            moveTo(256f, 390f)
            cubicTo(250f, 280f, 254f, 160f, 260f, 54f)
        }
        drawPath(highlightPath, Color.White.copy(alpha = 0.9f), style = Stroke(width = 1.3f, cap = StrokeCap.Round))
    }

    // 9. Закруглённый кончик (купол с переходом в #FFF8E0)
    drawCircle(tipColor, radius = 3f, center = Offset(260f, 51f))
}

/**
 * Искры вокруг пера (8 частиц на орбитах)
 */
private fun DrawScope.drawSparks(cx: Float, cy: Float, scale: Float, baseAngle: Float) {
    val sparkRadii = floatArrayOf(82f, 96f, 74f, 88f, 92f, 78f, 85f, 98f)
    val sparkSizes = floatArrayOf(2.6f, 1.8f, 2.2f, 3.0f, 1.6f, 2.4f, 2.0f, 2.8f)
    val sparkAlphas = floatArrayOf(0.9f, 0.6f, 0.85f, 0.95f, 0.5f, 0.8f, 0.7f, 0.9f)

    for (i in 0 until 8) {
        val angleDeg = baseAngle + i * 45f + (i * 7f)
        val angleRad = Math.toRadians(angleDeg.toDouble())
        val r = sparkRadii[i] * scale
        val sx = cx + (r * cos(angleRad)).toFloat()
        val sy = cy + (r * sin(angleRad)).toFloat()

        val pSize = sparkSizes[i] * scale
        val alpha = sparkAlphas[i]

        // Золотистое свечение искры
        drawCircle(
            color = Color(0xFFFCE4A6).copy(alpha = alpha * 0.4f),
            radius = pSize * 2.2f,
            center = Offset(sx, sy)
        )
        // Ядро искры
        drawCircle(
            color = Color(0xFFFFF0BE).copy(alpha = alpha),
            radius = pSize,
            center = Offset(sx, sy)
        )
    }
}
