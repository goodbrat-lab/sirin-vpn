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
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import kotlin.math.cos
import kotlin.math.sin

/**
 * Цветовая палитра интерфейса.
 */
object Brand {
    // --- как в логотипе. Значок приложения берёт эти же числа из XML ---

    /** Заливка сферы в логотипе. */
    val sphereLogo = Color(0xFF11141A)

    /** Контур и точки в логотипе. */
    val edgeLogo = Color(0xFF5B8AFF)

    /** Линии в логотипе. */
    val sirinLogo = Color(0xFF454E69)

    // --- для чёрного экрана ---

    /** Фон приложения. Именно чёрный: на OLED он не горит. */
    val space = Color(0xFF000000)

    /** Заливка сферы на чёрном. */
    val sphere = Color(0xFF171C26)

    /** Контур и точки на чёрном. */
    val edge = Color(0xFF6F98FF)

    /** Линии на чёрном. */
    val sirin = Color(0xFF525D80)

    /** Контур и точки в белом режиме (туннель на релее, белые списки). */
    val edgeWhite = Color(0xFFFFFFFF)

    /** Контур и полюса, пока связи нет. */
    val edgeOff = Color(0xFF3A4358)

    /** Линии, пока связи нет. */
    val sirinOff = Color(0xFF272D3A)

    /** Основной текст. Не чисто белый: на чёрном он режет глаз. */
    val text = Color(0xFFD7DCE8)

    /** Второстепенный текст и значки. */
    val dim = Color(0xFF7C859E)
}

/**
 * Кнопка настроек: шестерёнка.
 */
@Composable
fun GearButton(
    modifier: Modifier = Modifier,
    size: Dp = 40.dp,
    tint: Color = Brand.sirin,
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
        val ring = r * 0.42f
        val tooth = ring * 0.8f

        drawCircle(tint, r, c, style = Stroke(width = ring))

        // Восемь зубцов — короткие лучи наружу.
        for (i in 0 until 8) {
            val a = i * 0.7854f // 45°
            val from = Offset(c.x + cos(a) * r, c.y + sin(a) * r)
            val to = Offset(c.x + cos(a) * (r * 1.45f), c.y + sin(a) * (r * 1.45f))
            drawLine(tint, from, to, strokeWidth = tooth, cap = StrokeCap.Round)
        }
    }
}

// ----------------------------------------------------------------------
// Значки кнопок
// ----------------------------------------------------------------------

/**
 * Общая обвязка: квадратная область нажатия и своя фигура внутри.
 */
@Composable
fun IconBtn(
    modifier: Modifier = Modifier,
    size: Dp = 40.dp,
    enabled: Boolean = true,
    tint: Color = Brand.dim,
    draw: androidx.compose.ui.graphics.drawscope.DrawScope.(Color, Float) -> Unit,
) {
    Canvas(modifier = modifier.size(size)) {
        draw(
            if (enabled) tint else tint.copy(alpha = 0.35f),
            this.size.minDimension * RADIUS_FRACTION,
        )
    }
}

/** Радиус фигуры от меньшей стороны. Запас на толщину линий — в IconBtn. */
private const val RADIUS_FRACTION = 0.26f

/** Толще этого линии в значках не бывают. Доля радиуса. */
private const val STROKE_MAX = 0.34f

/**
 * Назад: стрелка влево.
 */
@Composable
fun BackButton(modifier: Modifier = Modifier, onClick: () -> Unit) {
    Row {
        IconBtn(modifier = modifier.glassPress { onClick() }.glassCircle(), tint = Brand.text) { tint, r ->
            val c = Offset(size.width / 2f, size.height / 2f)
            val w = r * STROKE_MAX
            val cap = StrokeCap.Round
            drawLine(tint, Offset(c.x + r, c.y), Offset(c.x - r, c.y), strokeWidth = w, cap = cap)
            drawLine(tint, Offset(c.x - r, c.y), Offset(c.x - r * 0.1f, c.y - r * 0.7f), strokeWidth = w, cap = cap)
            drawLine(tint, Offset(c.x - r, c.y), Offset(c.x - r * 0.1f, c.y + r * 0.7f), strokeWidth = w, cap = cap)
        }
        Spacer(modifier = Modifier.size(14.dp))
    }
}

/** Копировать: два перекрывающихся листа. */
@Composable
fun CopyButton(modifier: Modifier = Modifier, onClick: () -> Unit) {
    IconBtn(modifier = modifier.glassPress { onClick() }.glassCircle(), tint = Brand.text) { tint, r ->
        val c = Offset(size.width / 2f, size.height / 2f)
        val w = r * 0.26f
        val s = r * 1.15f
        val corner = CornerRadius(r * 0.25f)
        drawRoundRect(tint, Offset(c.x - r * 0.15f, c.y - r * 0.15f), Size(s, s), corner, style = Stroke(width = w))
        drawRoundRect(tint, Offset(c.x - r * 0.95f, c.y - r * 0.95f), Size(s, s), corner, style = Stroke(width = w))
    }
}

/** Поделиться: стрелка вверх из основания. */
@Composable
fun ShareButton(modifier: Modifier = Modifier, onClick: () -> Unit) {
    IconBtn(modifier = modifier.glassPress { onClick() }.glassCircle(), tint = Brand.text) { tint, r ->
        val c = Offset(size.width / 2f, size.height / 2f)
        val w = r * 0.30f
        val cap = StrokeCap.Round
        drawLine(tint, Offset(c.x, c.y + r * 0.55f), Offset(c.x, c.y - r), strokeWidth = w, cap = cap)
        drawLine(tint, Offset(c.x, c.y - r), Offset(c.x - r * 0.55f, c.y - r * 0.4f), strokeWidth = w, cap = cap)
        drawLine(tint, Offset(c.x, c.y - r), Offset(c.x + r * 0.55f, c.y - r * 0.4f), strokeWidth = w, cap = cap)
        drawLine(tint, Offset(c.x - r, c.y + r * 0.3f), Offset(c.x - r, c.y + r), strokeWidth = w, cap = cap)
        drawLine(tint, Offset(c.x + r, c.y + r * 0.3f), Offset(c.x + r, c.y + r), strokeWidth = w, cap = cap)
        drawLine(tint, Offset(c.x - r, c.y + r), Offset(c.x + r, c.y + r), strokeWidth = w, cap = cap)
    }
}

/**
 * Удалить: крестик.
 */
@Composable
fun DeleteButton(modifier: Modifier = Modifier, onClick: () -> Unit) {
    IconBtn(
        modifier = modifier.glassPress { onClick() }.glassCircle(),
        size = 32.dp,
        tint = Brand.text,
    ) { tint, r0 ->
        val r = r0 * 0.707f * 0.6f
        val c = Offset(size.width / 2f, size.height / 2f)
        val w = r0 * 0.16f
        val cap = StrokeCap.Round
        drawLine(tint, Offset(c.x - r, c.y - r), Offset(c.x + r, c.y + r), strokeWidth = w, cap = cap)
        drawLine(tint, Offset(c.x + r, c.y - r), Offset(c.x - r, c.y + r), strokeWidth = w, cap = cap)
    }
}

/** Ссылки VK: звено цепи, двумя дугами. */
@Composable
fun LinkIcon(modifier: Modifier = Modifier, tint: Color = Brand.edge) {
    IconBtn(modifier = modifier, tint = tint) { c1, r ->
        val c = Offset(size.width / 2f, size.height / 2f)
        val w = r * 0.26f
        drawCircle(c1, r * 0.55f, Offset(c.x - r * 0.45f, c.y), style = Stroke(width = w))
        drawCircle(c1, r * 0.55f, Offset(c.x + r * 0.45f, c.y), style = Stroke(width = w))
    }
}

/** Приложения: четыре плитки сеткой. */
@Composable
fun AppsIcon(modifier: Modifier = Modifier, tint: Color = Brand.edge) {
    IconBtn(modifier = modifier, tint = tint) { c1, r ->
        val c = Offset(size.width / 2f, size.height / 2f)
        val side = r * 0.7f
        val gap = r * 0.18f
        for (dx in intArrayOf(-1, 1)) {
            for (dy in intArrayOf(-1, 1)) {
                drawRect(
                    color = c1,
                    topLeft = Offset(
                        c.x + dx * (gap + side) - side / 2f,
                        c.y + dy * (gap + side) - side / 2f,
                    ),
                    size = Size(side, side),
                    style = Stroke(width = r * 0.22f),
                )
            }
        }
    }
}

/** Логи: три строки текста. */
@Composable
fun LogIcon(modifier: Modifier = Modifier, tint: Color = Brand.edge) {
    IconBtn(modifier = modifier, tint = tint) { c1, r ->
        val c = Offset(size.width / 2f, size.height / 2f)
        val w = r * 0.26f
        for (i in -1..1) {
            val y = c.y + i * r * 0.6f
            val right = if (i == 1) c.x + r * 0.3f else c.x + r
            drawLine(c1, Offset(c.x - r, y), Offset(right, y), strokeWidth = w, cap = StrokeCap.Round)
        }
    }
}
