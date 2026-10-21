package ru.guseynovia.sirin

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.size
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.StrokeJoin
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import kotlin.math.cos
import kotlin.math.sin

/**
 * Цветовая палитра интерфейса SIRIN VPN:
 * Стиль «Крипто-защита» (Dark Material & Cyber-Neon).
 */
object Brand {
    // --- Основные цвета палитры ---
    /** Основной фон приложения: Глубокий темный #0B0C10 */
    val bg = Color(0xFF0B0C10)
    val space = Color(0xFF0B0C10) // OLED deep dark

    /** Цвет карточек и плашек: #151623 */
    val cardBg = Color(0xFF151623)
    val sphere = Color(0xFF151623)

    /** Градиент акцента (ВКЛ): От Cyber Blue (#00FFFF) к Neon Purple (#8A2BE2) */
    val cyberBlue = Color(0xFF00FFFF)
    val neonPurple = Color(0xFF8A2BE2)
    val edge = Color(0xFF00FFFF)
    val sirin = Color(0xFF8A2BE2)

    /** Цвет ошибки: Neon Red/Crimson (#FF3366) */
    val neonRed = Color(0xFFFF3366)
    val error = Color(0xFFFF3366)

    /** Цвет ожидания / подключения: Neon Orange/Yellow (#FFB300) */
    val neonOrange = Color(0xFFFFB300)
    val connecting = Color(0xFFFFB300)

    /** Приглушенный серо-синий контур (Отключено) */
    val mutedOutline = Color(0xFF2A364F)
    val edgeOff = Color(0xFF2A364F)
    val sirinOff = Color(0xFF1F2438)

    /** Текст */
    val text = Color(0xFFE2E8F0)
    val dim = Color(0xFF7C859E)
    val edgeWhite = Color(0xFFFFFFFF)

    // Градиенты бренда
    val cyberGradient = Brush.linearGradient(
        colors = listOf(cyberBlue, neonPurple)
    )
    val cyberGradientHorizontal = Brush.horizontalGradient(
        colors = listOf(cyberBlue, neonPurple)
    )
    val mutedGradient = Brush.linearGradient(
        colors = listOf(Color(0xFF3B4863), Color(0xFF222B3D))
    )
}

/**
 * Кнопка настроек: шестерёнка в контурном стиле (Line Art).
 */
@Composable
fun GearButton(
    modifier: Modifier = Modifier,
    size: Dp = 40.dp,
    tint: Color? = null,
    onClick: () -> Unit,
) {
    Canvas(
        modifier = modifier
            .size(size)
            .semantics { contentDescription = "Настройки" }
            .glassPress(onClickLabel = "открыть") { onClick() }
            .glassCircle()
    ) {
        val c = Offset(this.size.width / 2f, this.size.height / 2f)
        val unit = this.size.minDimension
        val r = unit * 0.24f
        val ring = 1.8f.dp.toPx()
        val tooth = 1.8f.dp.toPx()

        val brush = if (tint != null) {
            androidx.compose.ui.graphics.SolidColor(tint)
        } else {
            Brush.linearGradient(
                colors = listOf(Brand.cyberBlue, Brand.neonPurple),
                start = Offset(c.x - r, c.y - r),
                end = Offset(c.x + r, c.y + r)
            )
        }

        drawCircle(brush = brush, radius = r, center = c, style = Stroke(width = ring))

        // 8 зубцов
        for (i in 0 until 8) {
            val a = i * 0.7854f
            val from = Offset(c.x + cos(a) * r, c.y + sin(a) * r)
            val to = Offset(c.x + cos(a) * (r * 1.45f), c.y + sin(a) * (r * 1.45f))
            drawLine(brush = brush, start = from, end = to, strokeWidth = tooth, cap = StrokeCap.Round)
        }
    }
}

/**
 * Общая обвязка для контурных кнопок-иконок.
 */
@Composable
fun IconBtn(
    modifier: Modifier = Modifier,
    size: Dp = 40.dp,
    enabled: Boolean = true,
    tint: Color? = null,
    draw: androidx.compose.ui.graphics.drawscope.DrawScope.(Brush, Float) -> Unit,
) {
    Canvas(modifier = modifier.size(size)) {
        val c = Offset(this.size.width / 2f, this.size.height / 2f)
        val r = this.size.minDimension * RADIUS_FRACTION
        val brush = if (tint != null) {
            androidx.compose.ui.graphics.SolidColor(if (enabled) tint else tint.copy(alpha = 0.35f))
        } else {
            if (enabled) {
                Brush.linearGradient(
                    colors = listOf(Brand.cyberBlue, Brand.neonPurple),
                    start = Offset(c.x - r, c.y - r),
                    end = Offset(c.x + r, c.y + r)
                )
            } else {
                androidx.compose.ui.graphics.SolidColor(Brand.dim.copy(alpha = 0.35f))
            }
        }
        draw(brush, r)
    }
}

private const val RADIUS_FRACTION = 0.28f

/**
 * Назад: тонкая контурная стрелка влево.
 */
@Composable
fun BackButton(modifier: Modifier = Modifier, onClick: () -> Unit) {
    Row {
        IconBtn(modifier = modifier.glassPress { onClick() }.glassCircle(), tint = Brand.text) { brush, r ->
            val c = Offset(size.width / 2f, size.height / 2f)
            val w = 2.dp.toPx()
            val cap = StrokeCap.Round
            drawLine(brush, Offset(c.x + r, c.y), Offset(c.x - r, c.y), strokeWidth = w, cap = cap)
            drawLine(brush, Offset(c.x - r, c.y), Offset(c.x - r * 0.1f, c.y - r * 0.7f), strokeWidth = w, cap = cap)
            drawLine(brush, Offset(c.x - r, c.y), Offset(c.x - r * 0.1f, c.y + r * 0.7f), strokeWidth = w, cap = cap)
        }
        Spacer(modifier = Modifier.size(14.dp))
    }
}

/** Копировать: два перекрывающихся листа в контурном стиле. */
@Composable
fun CopyButton(modifier: Modifier = Modifier, onClick: () -> Unit) {
    IconBtn(modifier = modifier.glassPress { onClick() }.glassCircle(), tint = Brand.text) { brush, r ->
        val c = Offset(size.width / 2f, size.height / 2f)
        val w = 1.8f.dp.toPx()
        val s = r * 1.15f
        val corner = CornerRadius(r * 0.25f)
        drawRoundRect(brush, Offset(c.x - r * 0.15f, c.y - r * 0.15f), Size(s, s), corner, style = Stroke(width = w))
        drawRoundRect(brush, Offset(c.x - r * 0.95f, c.y - r * 0.95f), Size(s, s), corner, style = Stroke(width = w))
    }
}

/** Поделиться: стрелка вверх из основания в контурном стиле. */
@Composable
fun ShareButton(modifier: Modifier = Modifier, onClick: () -> Unit) {
    IconBtn(modifier = modifier.glassPress { onClick() }.glassCircle(), tint = Brand.text) { brush, r ->
        val c = Offset(size.width / 2f, size.height / 2f)
        val w = 1.8f.dp.toPx()
        val cap = StrokeCap.Round
        drawLine(brush, Offset(c.x, c.y + r * 0.55f), Offset(c.x, c.y - r), strokeWidth = w, cap = cap)
        drawLine(brush, Offset(c.x, c.y - r), Offset(c.x - r * 0.55f, c.y - r * 0.4f), strokeWidth = w, cap = cap)
        drawLine(brush, Offset(c.x, c.y - r), Offset(c.x + r * 0.55f, c.y - r * 0.4f), strokeWidth = w, cap = cap)
        drawLine(brush, Offset(c.x - r, c.y + r * 0.3f), Offset(c.x - r, c.y + r), strokeWidth = w, cap = cap)
        drawLine(brush, Offset(c.x + r, c.y + r * 0.3f), Offset(c.x + r, c.y + r), strokeWidth = w, cap = cap)
        drawLine(brush, Offset(c.x - r, c.y + r), Offset(c.x + r, c.y + r), strokeWidth = w, cap = cap)
    }
}

/** Удалить: крестик. */
@Composable
fun DeleteButton(modifier: Modifier = Modifier, onClick: () -> Unit) {
    IconBtn(
        modifier = modifier.glassPress { onClick() }.glassCircle(),
        size = 32.dp,
        tint = Brand.text,
    ) { brush, r0 ->
        val r = r0 * 0.707f * 0.6f
        val c = Offset(size.width / 2f, size.height / 2f)
        val w = 1.8f.dp.toPx()
        val cap = StrokeCap.Round
        drawLine(brush, Offset(c.x - r, c.y - r), Offset(c.x + r, c.y + r), strokeWidth = w, cap = cap)
        drawLine(brush, Offset(c.x + r, c.y - r), Offset(c.x - r, c.y + r), strokeWidth = w, cap = cap)
    }
}

/**
 * Ссылки: тонкие контурные звенья цепи с кибер-градиентом.
 */
@Composable
fun LinkIcon(modifier: Modifier = Modifier, tint: Color? = null) {
    IconBtn(modifier = modifier, tint = tint) { brush, r ->
        val c = Offset(size.width / 2f, size.height / 2f)
        val w = 1.8f.dp.toPx()
        val stroke = Stroke(width = w, cap = StrokeCap.Round, join = StrokeJoin.Round)

        val linkW = r * 1.1f
        val linkH = r * 0.65f
        val radius = CornerRadius(linkH / 2f)

        // Первое звено с наклоном -30°
        drawRoundRect(
            brush = brush,
            topLeft = Offset(c.x - linkW * 0.75f, c.y - linkH / 2f),
            size = Size(linkW, linkH),
            cornerRadius = radius,
            style = stroke
        )
        // Второе звено
        drawRoundRect(
            brush = brush,
            topLeft = Offset(c.x - linkW * 0.25f, c.y - linkH / 2f),
            size = Size(linkW, linkH),
            cornerRadius = radius,
            style = stroke
        )
    }
}

/**
 * Приложения: контурная сетка тайлов (Split Tunneling Icon).
 */
@Composable
fun AppsIcon(modifier: Modifier = Modifier, tint: Color? = null) {
    IconBtn(modifier = modifier, tint = tint) { brush, r ->
        val c = Offset(size.width / 2f, size.height / 2f)
        val side = r * 0.7f
        val gap = r * 0.22f
        val stroke = Stroke(width = 1.8f.dp.toPx(), cap = StrokeCap.Round, join = StrokeJoin.Round)
        val corner = CornerRadius(4.dp.toPx())

        for (dx in intArrayOf(-1, 1)) {
            for (dy in intArrayOf(-1, 1)) {
                drawRoundRect(
                    brush = brush,
                    topLeft = Offset(
                        c.x + dx * (gap + side) - side / 2f,
                        c.y + dy * (gap + side) - side / 2f,
                    ),
                    size = Size(side, side),
                    cornerRadius = corner,
                    style = stroke
                )
            }
        }
    }
}

/**
 * Логи: контурный документ с градиентными строками (Server Log Icon).
 */
@Composable
fun LogIcon(modifier: Modifier = Modifier, tint: Color? = null) {
    IconBtn(modifier = modifier, tint = tint) { brush, r ->
        val c = Offset(size.width / 2f, size.height / 2f)
        val w = 1.8f.dp.toPx()
        val stroke = Stroke(width = w, cap = StrokeCap.Round, join = StrokeJoin.Round)

        // Контур терминала / документа
        val docW = r * 1.6f
        val docH = r * 1.8f
        drawRoundRect(
            brush = brush,
            topLeft = Offset(c.x - docW / 2f, c.y - docH / 2f),
            size = Size(docW, docH),
            cornerRadius = CornerRadius(4.dp.toPx()),
            style = stroke
        )

        // Внутренние строчки лога
        val startX = c.x - docW * 0.35f
        val endX1 = c.x + docW * 0.35f
        val endX2 = c.x + docW * 0.15f
        val endX3 = c.x + docW * 0.25f

        drawLine(brush, Offset(startX, c.y - docH * 0.22f), Offset(endX1, c.y - docH * 0.22f), strokeWidth = w, cap = StrokeCap.Round)
        drawLine(brush, Offset(startX, c.y), Offset(endX2, c.y), strokeWidth = w, cap = StrokeCap.Round)
        drawLine(brush, Offset(startX, c.y + docH * 0.22f), Offset(endX3, c.y + docH * 0.22f), strokeWidth = w, cap = StrokeCap.Round)
    }
}

/**
 * Ключ: тонкий контурный ключ с зубцами.
 */
@Composable
fun KeyIcon(modifier: Modifier = Modifier, tint: Color? = null) {
    IconBtn(modifier = modifier, tint = tint) { brush, r ->
        val c = Offset(size.width / 2f, size.height / 2f)
        val w = 1.8f.dp.toPx()
        val headC = Offset(c.x - r * 0.38f, c.y - r * 0.38f)
        drawCircle(brush = brush, radius = r * 0.42f, center = headC, style = Stroke(width = w))

        val shaftStart = Offset(headC.x + r * 0.28f, headC.y + r * 0.28f)
        val shaftEnd = Offset(c.x + r * 0.78f, c.y + r * 0.78f)
        drawLine(brush, shaftStart, shaftEnd, strokeWidth = w, cap = StrokeCap.Round)

        val tooth1Start = Offset(shaftEnd.x - r * 0.24f, shaftEnd.y - r * 0.24f)
        val tooth1End = Offset(tooth1Start.x + r * 0.22f, tooth1Start.y - r * 0.22f)
        drawLine(brush, tooth1Start, tooth1End, strokeWidth = w, cap = StrokeCap.Round)

        val tooth2Start = shaftEnd
        val tooth2End = Offset(tooth2Start.x + r * 0.24f, tooth2Start.y - r * 0.24f)
        drawLine(brush, tooth2Start, tooth2End, strokeWidth = w, cap = StrokeCap.Round)
    }
}

/**
 * Щит: контурный щит для Kill Switch.
 */
@Composable
fun ShieldIcon(modifier: Modifier = Modifier, tint: Color? = null) {
    IconBtn(modifier = modifier, tint = tint) { brush, r ->
        val c = Offset(size.width / 2f, size.height / 2f)
        val w = 1.8f.dp.toPx()
        val path = androidx.compose.ui.graphics.Path().apply {
            moveTo(c.x, c.y - r * 0.82f)
            lineTo(c.x + r * 0.72f, c.y - r * 0.48f)
            lineTo(c.x + r * 0.58f, c.y + r * 0.35f)
            lineTo(c.x, c.y + r * 0.85f)
            lineTo(c.x - r * 0.58f, c.y + r * 0.35f)
            lineTo(c.x - r * 0.72f, c.y - r * 0.48f)
            close()
        }
        drawPath(path, brush = brush, style = Stroke(width = w, cap = StrokeCap.Round, join = StrokeJoin.Round))
    }
}

